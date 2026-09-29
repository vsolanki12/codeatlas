package parser

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

var _ Parser = (*GoParser)(nil)

type GoParser struct {
	fset            *token.FileSet
	rootDir         string
	packageResolver *packageResolver
	useImportPaths  bool
}

func NewGoParser() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

// NewGoParserForRepo creates a parser whose entity and import identities are
// unique within the repository. NewGoParser remains available for callers
// that parse isolated fixture files and need the legacy short identities.
func NewGoParserForRepo(repoPath string) *GoParser {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	return &GoParser{
		fset:            token.NewFileSet(),
		rootDir:         filepath.Clean(abs),
		packageResolver: newPackageResolver(abs),
		useImportPaths:  true,
	}
}

func (p *GoParser) Parse(file domain.File) ([]domain.Entity, error) {
	astFile, err := p.parseFile(file.RelativePath)
	if err != nil {
		return nil, err
	}

	packageName := astFile.Name.Name
	packagePath := packageName
	if p.packageResolver != nil {
		packagePath = p.packageResolver.resolve(file.RelativePath, packageName)
	}
	var entities []domain.Entity
	type controllerFacts struct {
		isController bool
		source       domain.Source
		setupSource  domain.Source
		watches      []string
		watchMethods []string
		watchSites   []domain.Site
		creates      []string
		createSites  []domain.Site
		calls        []string
		callSites    []domain.Site
	}
	controllers := make(map[string]*controllerFacts)
	getController := func(name string) *controllerFacts {
		facts := controllers[name]
		if facts == nil {
			facts = &controllerFacts{}
			controllers[name] = facts
		}
		return facts
	}

	typeComments := make(map[string]string)
	implPairs := make(map[string][]string) // structName -> []interfaceName
	implSites := make(map[string][]domain.Site)
	importAliases := buildImportAliasMap(astFile, p.useImportPaths)

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
				detectImplements(s, implPairs, implSites, file.RelativePath, p.fset)
			}
		}
	}

	embedSites := extractEmbedSites(astFile, p.fset, file.RelativePath)
	importSites := extractImportSites(astFile, file.RelativePath, p.fset)
	entities = append(entities, domain.Entity{
		ID:          fmt.Sprintf("package:%s", packagePath),
		Name:        packageName,
		Kind:        domain.KindPackage,
		Package:     packagePath,
		Files:       []string{file.RelativePath},
		Imports:     extractImports(astFile),
		ImportSites: importSites,
		Embeds:      extractEmbeds(astFile),
		EmbedSites:  embedSites,
		Source: domain.Source{
			Parser: "go",
			File:   file.RelativePath,
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
			id = fmt.Sprintf("function:%s.%s.%s", packagePath, recv, fn.Name.Name)
		} else {
			id = fmt.Sprintf("function:%s.%s", packagePath, fn.Name.Name)
		}

		var calls []string
		var callSites []domain.Site
		var creates []string
		var createSites []domain.Site
		var envVars []string
		var literals []string
		if fn.Body != nil {
			calls, callSites, envVars = extractCallsAndEnvVars(fn.Body, importAliases, file.RelativePath, p.fset)
			creates, createSites = extractCreateSites(fn.Body, file.RelativePath, p.fset)
			literals = extractLiterals(fn.Body)
		}

		var implements []string
		if recv != "" {
			implements = implPairs[recv]
		}
		position := p.fset.Position(fn.Pos())
		endPosition := p.fset.Position(fn.End())

		entities = append(entities, domain.Entity{
			ID:                  id,
			Name:                fn.Name.Name,
			Kind:                domain.KindFunction,
			Description:         description,
			Package:             packagePath,
			Calls:               calls,
			CallSites:           callSites,
			Creates:             creates,
			CreateSites:         createSites,
			Implements:          implements,
			ImplementationSites: implSites[recv],
			EnvVars:             envVars,
			Literals:            literals,
			Source: domain.Source{
				Parser:  "go",
				File:    file.RelativePath,
				Line:    position.Line,
				EndLine: endPosition.Line,
			},
		})

		if fn.Name.Name == "Reconcile" && recv != "" {
			facts := getController(recv)
			facts.isController = true
			facts.source = domain.Source{
				Parser:  "go",
				File:    file.RelativePath,
				Line:    position.Line,
				EndLine: endPosition.Line,
			}
			facts.calls = calls
			facts.callSites = callSites
		}
		if recv != "" && len(createSites) > 0 {
			facts := getController(recv)
			facts.creates = appendUniqueStrings(facts.creates, creates)
			facts.createSites = appendUniqueDomainSites(facts.createSites, createSites)
		}

		if fn.Name.Name == "SetupWithManager" && fn.Body != nil && recv != "" {
			facts := getController(recv)
			facts.isController = true
			if facts.setupSource.File == "" {
				facts.setupSource = domain.Source{
					Parser:  "go",
					File:    file.RelativePath,
					Line:    position.Line,
					EndLine: endPosition.Line,
				}
			}
			var watches []string
			var watchMethods []string
			var watchSites []domain.Site
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
								watches = append(watches, typeName)
								watchMethods = append(watchMethods, methodName)
								watchSites = append(watchSites, domain.Site{
									Name: typeName,
									Source: domain.Source{
										Parser: "go-ast",
										File:   file.RelativePath,
										Line:   p.fset.Position(callExpr.Pos()).Line,
									},
								})
							}
						}
					}
				}
				return true
			})
			// The AST visits the outer Complete call before the nested builder
			// calls, so reverse only this setup method's facts. Reversing one
			// file-wide accumulator caused a second controller in the same file
			// to steal or reorder the first controller's watches.
			for i, j := 0, len(watches)-1; i < j; i, j = i+1, j-1 {
				watches[i], watches[j] = watches[j], watches[i]
				watchMethods[i], watchMethods[j] = watchMethods[j], watchMethods[i]
				watchSites[i], watchSites[j] = watchSites[j], watchSites[i]
			}
			facts.watches = append(facts.watches, watches...)
			facts.watchMethods = append(facts.watchMethods, watchMethods...)
			facts.watchSites = append(facts.watchSites, watchSites...)
		}

		return true
	})

	var controllerNames []string
	for name, facts := range controllers {
		if facts.isController {
			controllerNames = append(controllerNames, name)
		}
	}
	sort.Strings(controllerNames)
	for _, controllerTypeName := range controllerNames {
		facts := controllers[controllerTypeName]
		source := facts.source
		if source.File == "" {
			source = facts.setupSource
		}
		entities = append(entities, domain.Entity{
			ID:                  fmt.Sprintf("controller:%s.%s", packagePath, controllerTypeName),
			Name:                controllerTypeName,
			Kind:                domain.KindController,
			Description:         typeComments[controllerTypeName],
			Package:             packagePath,
			Files:               []string{file.RelativePath},
			Source:              source,
			Watches:             facts.watches,
			WatchMethods:        facts.watchMethods,
			WatchSites:          facts.watchSites,
			Creates:             facts.creates,
			CreateSites:         facts.createSites,
			Calls:               facts.calls,
			CallSites:           facts.callSites,
			Implements:          implPairs[controllerTypeName],
			ImplementationSites: implSites[controllerTypeName],
		})
	}

	return entities, nil
}

func buildImportAliasMap(astFile *ast.File, useImportPaths bool) map[string]string {
	aliases := make(map[string]string)
	for _, imp := range astFile.Imports {
		if imp.Name == nil || imp.Name.Name == "." || imp.Name.Name == "_" {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		pkgName := filepath.Base(path)
		if useImportPaths {
			aliases[pkgName] = path
			if imp.Name.Name != pkgName {
				aliases[imp.Name.Name] = path
			}
		} else if imp.Name.Name != pkgName {
			aliases[imp.Name.Name] = pkgName
		}
	}
	return aliases
}

func extractCallsAndEnvVars(body *ast.BlockStmt, importAliases map[string]string, filePath string, fset *token.FileSet) ([]string, []domain.Site, []string) {
	seen := make(map[string]bool)
	var calls []string
	var sites []domain.Site
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
				prefix := ident.Name
				if resolved, ok := importAliases[prefix]; ok {
					prefix = resolved
				}
				name = prefix + "." + fun.Sel.Name
			} else {
				// Preserve the complete receiver chain. Reducing an expression such
				// as r.RegistryProvider.Reconcile to just Reconcile lets the graph
				// builder attach it to an unrelated same-named method.
				var expression bytes.Buffer
				if err := format.Node(&expression, fset, fun); err == nil {
					name = expression.String()
				}
			}
		case *ast.Ident:
			name = fun.Name
		}

		if name != "" && !seen[name] {
			seen[name] = true
			calls = append(calls, name)
			sites = append(sites, domain.Site{
				Name: name,
				Source: domain.Source{
					Parser: "go-ast",
					File:   filePath,
					Line:   fset.Position(callExpr.Pos()).Line,
				},
			})
		}
		return true
	})
	return calls, sites, envVars
}

var createMethodNames = map[string]bool{
	"Create":         true,
	"CreateOrUpdate": true,
	"CreateOrPatch":  true,
}

func extractCreateSites(body *ast.BlockStmt, filePath string, fset *token.FileSet) ([]string, []domain.Site) {
	variableTypes := make(map[string]string)
	seen := make(map[string]bool)
	var creates []string
	var sites []domain.Site

	addVariable := func(name string, expr ast.Expr) {
		if typeName := createTypeName(expr, variableTypes); typeName != "" {
			variableTypes[name] = typeName
		}
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || i >= len(node.Rhs) {
					continue
				}
				addVariable(ident.Name, node.Rhs[i])
			}
		case *ast.DeclStmt:
			gen, ok := node.Decl.(*ast.GenDecl)
			if !ok {
				break
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if i < len(valueSpec.Values) {
						addVariable(name.Name, valueSpec.Values[i])
					} else if valueSpec.Type != nil {
						addVariable(name.Name, valueSpec.Type)
					}
				}
			}
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || !createMethodNames[selector.Sel.Name] {
				return true
			}
			if object := createObjectArg(node.Args); object != nil {
				typeName := createTypeName(object, variableTypes)
				if typeName != "" {
					position := fset.Position(node.Pos())
					key := typeName + "\x00" + filePath + "\x00" + fmt.Sprint(position.Line)
					if !seen[key] {
						seen[key] = true
						creates = append(creates, typeName)
						sites = append(sites, domain.Site{
							Name: typeName,
							Source: domain.Source{
								Parser: "go-ast",
								File:   filePath,
								Line:   position.Line,
							},
						})
					}
				}
			}
		}
		return true
	})

	return creates, sites
}

func createObjectArg(args []ast.Expr) ast.Expr {
	// controller-runtime Create/CreateOrUpdate/CreateOrPatch calls normally
	// receive context first and the object second. Small test clients often
	// expose a one-argument form, which is also supported. Restricting this to
	// the object position avoids treating a context or option literal as a
	// created Kubernetes resource.
	if len(args) >= 2 {
		return args[1]
	}
	if len(args) == 1 {
		return args[0]
	}
	return nil
}

func createTypeName(expr ast.Expr, variables map[string]string) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return variables[value.Name]
	case *ast.ParenExpr:
		return createTypeName(value.X, variables)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return createTypeName(value.X, variables)
		}
	case *ast.CompositeLit:
		return expressionTypeName(value.Type)
	}
	return ""
}

func expressionTypeName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.StarExpr:
		return expressionTypeName(value.X)
	case *ast.ArrayType:
		return expressionTypeName(value.Elt)
	}
	return ""
}

func appendUniqueStrings(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			seen[value] = true
			existing = append(existing, value)
		}
	}
	return existing
}

func appendUniqueDomainSites(existing, additions []domain.Site) []domain.Site {
	seen := make(map[string]bool, len(existing))
	for _, site := range existing {
		seen[site.Name+"\x00"+site.Source.File+"\x00"+fmt.Sprint(site.Source.Line)] = true
	}
	for _, site := range additions {
		key := site.Name + "\x00" + site.Source.File + "\x00" + fmt.Sprint(site.Source.Line)
		if !seen[key] {
			seen[key] = true
			existing = append(existing, site)
		}
	}
	return existing
}

func detectImplements(spec *ast.ValueSpec, pairs map[string][]string, sites map[string][]domain.Site, filePath string, fset *token.FileSet) {
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
	sites[structName] = append(sites[structName], domain.Site{
		Name: ifaceName,
		Source: domain.Source{
			Parser: "go-ast",
			File:   filePath,
			Line:   fset.Position(spec.Pos()).Line,
		},
	})
}

func (p *GoParser) parseFile(path string) (*ast.File, error) {
	parsePath := path
	if p.rootDir != "" {
		parsePath = filepath.Join(p.rootDir, filepath.FromSlash(path))
	}
	file, err := parser.ParseFile(p.fset, parsePath, nil, parser.ParseComments)
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

func extractImportSites(astFile *ast.File, filePath string, fset *token.FileSet) []domain.Site {
	var sites []domain.Site
	for _, imp := range astFile.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if importPath == "" {
			continue
		}
		sites = append(sites, domain.Site{
			Name: importPath,
			Source: domain.Source{
				Parser: "go-ast",
				File:   filePath,
				Line:   fset.Position(imp.Pos()).Line,
			},
		})
	}
	return sites
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
				for _, pattern := range strings.Fields(strings.TrimPrefix(c.Text, "//go:embed ")) {
					if pattern != "" {
						embeds = append(embeds, pattern)
					}
				}
			}
		}
	}
	return embeds
}

func extractEmbedSites(astFile *ast.File, fset *token.FileSet, filePath string) []domain.Site {
	var sites []domain.Site
	for _, cg := range astFile.Comments {
		for _, c := range cg.List {
			if !strings.HasPrefix(c.Text, "//go:embed ") {
				continue
			}
			for _, pattern := range strings.Fields(strings.TrimPrefix(c.Text, "//go:embed ")) {
				sites = append(sites, domain.Site{
					Name: pattern,
					Source: domain.Source{
						Parser: "go-ast",
						File:   filePath,
						Line:   fset.Position(c.Pos()).Line,
					},
				})
			}
		}
	}
	return sites
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
		if t.Sel.Name == "Kind" {
			for _, element := range compLit.Elts {
				keyValue, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := keyValue.Key.(*ast.Ident); ok && key.Name == "Type" {
					return extractTypeName(keyValue.Value)
				}
			}
		}
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}

	return ""
}
