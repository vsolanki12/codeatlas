package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

var _ Parser = (*GoParser)(nil)

type GoParser struct {
	fset *token.FileSet
}

func NewGoParser() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

func (p *GoParser) Parse(file domain.File) ([]domain.Entity, error) {
	astFile, err := p.parseFile(file.RelativePath)
	if err != nil {
		return nil, err
	}

	packageName := astFile.Name.Name
	dirPath := filepath.Dir(file.RelativePath)
	var entities []domain.Entity
	var controllerTypeName string
	var controllerSource domain.Source
	var controllerWatches []string
	var controllerCalls []string
	var setupHelperMethods []string
	var setupReceiverVar string

	typeComments := make(map[string]string)
	implPairs := make(map[string][]string) // structName -> []interfaceName

	for _, decl := range astFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				doc := ""
				if s.Doc != nil {
					doc = strings.TrimSpace(s.Doc.Text())
				} else if genDecl.Doc != nil && len(genDecl.Specs) == 1 {
					doc = strings.TrimSpace(genDecl.Doc.Text())
				}
				if doc != "" {
					typeComments[s.Name.Name] = doc
				}
			case *ast.ValueSpec:
				if genDecl.Tok != token.VAR {
					continue
				}
				detectImplements(s, implPairs)
			}
		}
	}

	entities = append(entities, domain.Entity{
		ID:      fmt.Sprintf("package:%s", packageName),
		Name:    packageName,
		Kind:    domain.KindPackage,
		Package: packageName,
		Files:   []string{file.RelativePath},
		Imports: extractImports(astFile),
		Embeds:  extractEmbeds(astFile),
		Source: domain.Source{
			Parser: "go",
			File:   dirPath,
			Line:   1,
		},
	})

	ast.Inspect(astFile, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		description := ""
		if fn.Doc != nil {
			description = strings.TrimSpace(fn.Doc.Text())
		}

		recv := receiverTypeName(fn)
		var id string
		if recv != "" {
			id = fmt.Sprintf("function:%s.%s.%s", packageName, recv, fn.Name.Name)
		} else {
			id = fmt.Sprintf("function:%s.%s", packageName, fn.Name.Name)
		}

		var calls []string
		var envVars []string
		var literals []string
		if fn.Body != nil {
			calls, envVars = extractCallsAndEnvVars(fn.Body)
			literals = extractLiterals(fn.Body)
		}

		var implements []string
		if recv != "" {
			implements = implPairs[recv]
		}

		entities = append(entities, domain.Entity{
			ID:          id,
			Name:        fn.Name.Name,
			Kind:        domain.KindFunction,
			Description: description,
			Package:     packageName,
			Calls:       calls,
			Implements:  implements,
			EnvVars:     envVars,
			Literals:    literals,
			Source: domain.Source{
				Parser: "go",
				File:   file.RelativePath,
				Line:   p.fset.Position(fn.Pos()).Line,
			},
		})

		if fn.Name.Name == "Reconcile" && recv != "" {
			controllerTypeName = recv
			controllerSource = domain.Source{
				Parser: "go",
				File:   file.RelativePath,
				Line:   p.fset.Position(fn.Pos()).Line,
			}
			controllerCalls = calls
		}

		if fn.Name.Name == "SetupWithManager" && fn.Body != nil {
			if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
				setupReceiverVar = fn.Recv.List[0].Names[0].Name
			}
			ast.Inspect(fn.Body, func(innerNode ast.Node) bool {
				callExpr, ok := innerNode.(*ast.CallExpr)
				if !ok {
					return true
				}
				if selector, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
					methodName := selector.Sel.Name
					if methodName == "For" || methodName == "Owns" || methodName == "Watches" {
						if len(callExpr.Args) > 0 {
							if typeName := extractTypeName(callExpr.Args[0]); typeName != "" {
								controllerWatches = append(controllerWatches, typeName)
							}
						}
					}
					if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == setupReceiverVar {
						setupHelperMethods = append(setupHelperMethods, methodName)
					}
				}
				return true
			})
			for i, j := 0, len(controllerWatches)-1; i < j; i, j = i+1, j-1 {
				controllerWatches[i], controllerWatches[j] = controllerWatches[j], controllerWatches[i]
			}
		}

		return true
	})

	if controllerTypeName != "" {
		var props []string
		if len(setupHelperMethods) > 0 {
			props = extractK8sTypesFromHelpers(astFile, controllerTypeName, setupReceiverVar, setupHelperMethods)
		}
		entities = append(entities, domain.Entity{
			ID:          fmt.Sprintf("controller:%s.%s", packageName, controllerTypeName),
			Name:        controllerTypeName,
			Kind:        domain.KindController,
			Description: typeComments[controllerTypeName],
			Package:     packageName,
			Source:      controllerSource,
			Watches:     controllerWatches,
			Calls:       controllerCalls,
			Implements:  implPairs[controllerTypeName],
			Properties:  props,
		})
	}

	return entities, nil
}

func extractCallsAndEnvVars(body *ast.BlockStmt) ([]string, []string) {
	seen := make(map[string]bool)
	var calls []string
	seenEnv := make(map[string]bool)
	var envVars []string

	ast.Inspect(body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		var name string
		switch fun := callExpr.Fun.(type) {
		case *ast.SelectorExpr:
			if ident, ok := fun.X.(*ast.Ident); ok {
				// os.Getenv detection
				if ident.Name == "os" && fun.Sel.Name == "Getenv" && len(callExpr.Args) > 0 {
					if lit, ok := callExpr.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						envName := strings.Trim(lit.Value, `"`)
						if envName != "" && !seenEnv[envName] {
							seenEnv[envName] = true
							envVars = append(envVars, envName)
						}
					}
				}
				name = ident.Name + "." + fun.Sel.Name
			} else {
				name = fun.Sel.Name
			}
		case *ast.Ident:
			name = fun.Name
		}

		if name != "" && !seen[name] {
			seen[name] = true
			calls = append(calls, name)
		}
		return true
	})
	return calls, envVars
}

func detectImplements(spec *ast.ValueSpec, pairs map[string][]string) {
	if len(spec.Names) == 0 || spec.Names[0].Name != "_" {
		return
	}
	if spec.Type == nil || len(spec.Values) == 0 {
		return
	}

	var ifaceName string
	switch t := spec.Type.(type) {
	case *ast.Ident:
		ifaceName = t.Name
	case *ast.SelectorExpr:
		ifaceName = t.Sel.Name
	case *ast.StarExpr:
		switch inner := t.X.(type) {
		case *ast.Ident:
			ifaceName = inner.Name
		case *ast.SelectorExpr:
			ifaceName = inner.Sel.Name
		}
	}
	if ifaceName == "" {
		return
	}

	expr := spec.Values[0]
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	if paren, ok := expr.(*ast.ParenExpr); ok {
		expr = paren.X
	}

	var structName string
	switch v := expr.(type) {
	case *ast.CompositeLit:
		switch t := v.Type.(type) {
		case *ast.Ident:
			structName = t.Name
		case *ast.SelectorExpr:
			structName = t.Sel.Name
		}
	case *ast.Ident:
		if v.Name == "nil" {
			return
		}
		structName = v.Name
	case *ast.CallExpr:
		return
	}
	if structName == "" {
		return
	}

	pairs[structName] = append(pairs[structName], ifaceName)
}

func (p *GoParser) parseFile(path string) (*ast.File, error) {
	file, err := parser.ParseFile(p.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed parsing go file: %w", err)
	}
	return file, nil
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}

	switch t := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func Classify(files []domain.File) map[string][]domain.File {
	group := make(map[string][]domain.File)
	for _, f := range files {
		ext := filepath.Ext(f.RelativePath)
		group[ext] = append(group[ext], f)
	}
	return group
}

func extractImports(astFile *ast.File) []string {
	var imports []string
	for _, imp := range astFile.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}
	return imports
}

func extractLiterals(body *ast.BlockStmt) []string {
	seen := make(map[string]bool)
	var literals []string
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			s := strings.Trim(lit.Value, `"`+"`")
			if len(s) >= 4 && strings.ContainsAny(s, "./-_:") && !seen[s] {
				seen[s] = true
				literals = append(literals, s)
			}
			return true
		}
		if sel, ok := n.(*ast.SelectorExpr); ok {
			name := sel.Sel.Name
			if len(name) >= 8 && ast.IsExported(name) && !seen[name] {
				seen[name] = true
				literals = append(literals, name)
			}
			return true
		}
		return true
	})
	if len(literals) > 50 {
		literals = literals[:50]
	}
	return literals
}

func extractEmbeds(astFile *ast.File) []string {
	var embeds []string
	for _, cg := range astFile.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:embed ") {
				pattern := strings.TrimPrefix(c.Text, "//go:embed ")
				pattern = strings.TrimSpace(pattern)
				if pattern != "" {
					embeds = append(embeds, pattern)
				}
			}
		}
	}
	return embeds
}

func extractTypeName(expr ast.Expr) string {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}

	compLit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return ""
	}

	switch t := compLit.Type.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}

	return ""
}

var k8sResourceTypes = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"ReplicaSet": true, "Job": true, "CronJob": true,
	"Service": true, "ConfigMap": true, "Secret": true,
}

func extractK8sTypesFromHelpers(astFile *ast.File, controllerType, receiverVar string, helperNames []string) []string {
	helperSet := make(map[string]bool, len(helperNames))
	for _, name := range helperNames {
		helperSet[name] = true
	}

	seen := make(map[string]bool)
	var props []string

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil {
			continue
		}
		if receiverTypeName(fn) != controllerType {
			continue
		}
		if !helperSet[fn.Name.Name] {
			continue
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			unary, ok := n.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				return true
			}
			comp, ok := unary.X.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := comp.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			typeName := sel.Sel.Name
			if k8sResourceTypes[typeName] && !seen[typeName] {
				seen[typeName] = true
				props = append(props, "creates:"+typeName)
			}
			return true
		})
	}

	return props
}
