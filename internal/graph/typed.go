package graph

import (
	"fmt"
	goast "go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// BuildTypedCallRelationships resolves calls with the standard Go type
// checker. It is intentionally an enrichment pass: a type-checking failure
// never creates a guessed edge, and callers retain the AST-derived edge or no
// edge at all. Only targets that already exist as CodeAtlas function entities
// are emitted.
//
// The loader is source-backed for packages in the scanned repository and uses
// the standard compiler importer for dependencies outside the graph. This
// keeps the pass repository-wide without making the graph depend on generated
// export data or a particular build cache.
func BuildTypedCallRelationships(root string, files []domain.File, entities []domain.Entity) []domain.Relationship {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}

	fileSet := make(map[string]bool, len(files))
	for _, file := range files {
		fileSet[filepath.ToSlash(filepath.Clean(file.RelativePath))] = true
	}

	specs := make(map[string]*sourcePackageSpec)
	for _, entity := range entities {
		if entity.Kind != domain.KindPackage {
			continue
		}
		packagePath := entity.Package
		if packagePath == "" {
			packagePath = strings.TrimPrefix(entity.ID, "package:")
		}
		if packagePath == "" {
			continue
		}
		spec := specs[packagePath]
		if spec == nil {
			spec = &sourcePackageSpec{path: packagePath, files: make(map[string]bool)}
			specs[packagePath] = spec
		}
		paths := append([]string(nil), entity.Files...)
		if entity.Source.File != "" {
			paths = append(paths, entity.Source.File)
		}
		for _, relative := range paths {
			relative = filepath.ToSlash(filepath.Clean(relative))
			if !fileSet[relative] || filepath.Ext(relative) != ".go" || strings.HasSuffix(relative, "_test.go") {
				continue
			}
			spec.files[relative] = true
		}
	}

	loader := &sourcePackageLoader{
		root:     rootAbs,
		specs:    specs,
		checked:  make(map[string]*typedPackage),
		checking: make(map[string]bool),
		fallback: importer.Default(),
	}

	functionIDs := make(map[string]bool)
	controllerIDs := make(map[string]bool)
	for _, entity := range entities {
		switch entity.Kind {
		case domain.KindFunction:
			functionIDs[entity.ID] = true
		case domain.KindController:
			controllerIDs[entity.ID] = true
		}
	}

	var packagePaths []string
	for packagePath := range specs {
		packagePaths = append(packagePaths, packagePath)
	}
	sort.Strings(packagePaths)

	var relationships []domain.Relationship
	seen := make(map[string]bool)
	for _, packagePath := range packagePaths {
		checked := loader.check(packagePath)
		if checked == nil || checked.pkg == nil || checked.info == nil {
			continue
		}
		for _, file := range checked.files {
			astFile := checked.astByPath[file]
			if astFile == nil {
				continue
			}
			for _, declaration := range astFile.Decls {
				fn, ok := declaration.(*goast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				callerID := typedFunctionID(packagePath, fn)
				if !functionIDs[callerID] {
					continue
				}
				callerIDs := []string{callerID}
				if receiver := typedReceiverName(fn); receiver != "" && fn.Name.Name == "Reconcile" {
					controllerID := "controller:" + packagePath + "." + receiver
					if controllerIDs[controllerID] {
						callerIDs = append(callerIDs, controllerID)
					}
				}

				goast.Inspect(fn.Body, func(node goast.Node) bool {
					call, ok := node.(*goast.CallExpr)
					if !ok {
						return true
					}
					target := typedCallTarget(checked.info, call)
					if target == nil {
						return true
					}
					targetID := typedFunctionObjectID(target)
					if targetID == "" || !functionIDs[targetID] {
						return true
					}
					position := checked.fset.Position(call.Pos())
					relative := filepath.ToSlash(filepath.Clean(filePathRelative(rootAbs, position.Filename)))
					if relative == "." || relative == "" {
						relative = position.Filename
					}
					for _, caller := range callerIDs {
						relationship := domain.Relationship{
							ID:         domain.NewRelationshipID(caller, domain.RelCalls, targetID),
							From:       caller,
							To:         targetID,
							Type:       domain.RelCalls,
							Confidence: domain.ConfidenceProven,
							Evidence: domain.Evidence{
								Parser:  "go-types",
								File:    relative,
								Line:    position.Line,
								Snippet: ReadSnippet(filepath.Join(rootAbs, relative), position.Line),
								Reason:  "Go type checker resolved the call target",
							},
						}
						if relationship.From == relationship.To || seen[relationship.ID] {
							continue
						}
						seen[relationship.ID] = true
						relationships = append(relationships, relationship)
					}
					return true
				})
			}
		}
	}

	sort.Slice(relationships, func(i, j int) bool { return relationships[i].ID < relationships[j].ID })
	return relationships
}

// MergeRelationships replaces a weaker relationship with a stronger one
// having the same deterministic ID. It is used to upgrade AST name matches
// when the type checker supplies exact target identity.
func MergeRelationships(base, additions []domain.Relationship) []domain.Relationship {
	byID := make(map[string]domain.Relationship, len(base)+len(additions))
	for _, relationship := range base {
		byID[relationship.ID] = relationship
	}
	for _, relationship := range additions {
		existing, ok := byID[relationship.ID]
		if !ok || relationshipRank(relationship) > relationshipRank(existing) {
			byID[relationship.ID] = relationship
		}
	}
	result := make([]domain.Relationship, 0, len(byID))
	for _, relationship := range byID {
		result = append(result, relationship)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func relationshipRank(relationship domain.Relationship) int {
	if relationship.Confidence == domain.ConfidenceProven {
		return 2
	}
	return 1
}

type sourcePackageSpec struct {
	path  string
	files map[string]bool
}

type typedPackage struct {
	pkg       *types.Package
	info      *types.Info
	fset      *token.FileSet
	files     []string
	astByPath map[string]*goast.File
}

type sourcePackageLoader struct {
	root     string
	specs    map[string]*sourcePackageSpec
	checked  map[string]*typedPackage
	checking map[string]bool
	fallback types.Importer
}

func (l *sourcePackageLoader) Import(path string) (*types.Package, error) {
	if checked := l.checked[path]; checked != nil && checked.pkg != nil {
		return checked.pkg, nil
	}
	spec := l.specs[path]
	if spec == nil {
		return l.fallback.Import(path)
	}
	if l.checking[path] {
		return nil, fmt.Errorf("type-check import cycle involving %s", path)
	}
	checked := l.check(path)
	if checked == nil || checked.pkg == nil {
		return nil, fmt.Errorf("could not type-check package %s", path)
	}
	return checked.pkg, nil
}

func (l *sourcePackageLoader) check(path string) *typedPackage {
	if checked := l.checked[path]; checked != nil {
		return checked
	}
	if l.checking[path] {
		return nil
	}
	spec := l.specs[path]
	if spec == nil {
		return nil
	}
	l.checking[path] = true
	defer delete(l.checking, path)

	files := make([]string, 0, len(spec.files))
	for relative := range spec.files {
		files = append(files, relative)
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var astFiles []*goast.File
	astByPath := make(map[string]*goast.File)
	for _, relative := range files {
		absolute := filepath.Join(l.root, filepath.FromSlash(relative))
		if matched, err := build.Default.MatchFile(filepath.Dir(absolute), filepath.Base(absolute)); err != nil || !matched {
			continue
		}
		astFile, err := parser.ParseFile(fset, absolute, nil, parser.ParseComments)
		if err != nil || astFile.Name == nil {
			continue
		}
		astFiles = append(astFiles, astFile)
		astByPath[relative] = astFile
	}
	if len(astFiles) == 0 {
		return nil
	}

	info := &types.Info{
		Uses:       make(map[*goast.Ident]types.Object),
		Selections: make(map[*goast.SelectorExpr]*types.Selection),
	}
	config := &types.Config{
		Importer:    l,
		FakeImportC: true,
		Error:       func(error) {},
	}
	pkg, _ := config.Check(path, fset, astFiles, info)
	if pkg == nil {
		return nil
	}
	checked := &typedPackage{
		pkg:       pkg,
		info:      info,
		fset:      fset,
		files:     files,
		astByPath: astByPath,
	}
	l.checked[path] = checked
	return checked
}

func typedCallTarget(info *types.Info, call *goast.CallExpr) *types.Func {
	if info == nil || call == nil {
		return nil
	}
	var object types.Object
	switch function := call.Fun.(type) {
	case *goast.Ident:
		object = info.Uses[function]
	case *goast.SelectorExpr:
		if selection := info.Selections[function]; selection != nil {
			object = selection.Obj()
		}
		if object == nil {
			object = info.Uses[function.Sel]
		}
	}
	target, _ := object.(*types.Func)
	return target
}

func typedFunctionObjectID(function *types.Func) string {
	if function == nil || function.Pkg() == nil {
		return ""
	}
	packagePath := function.Pkg().Path()
	if packagePath == "" {
		return ""
	}
	if signature, ok := function.Type().(*types.Signature); ok && signature.Recv() != nil {
		if receiver := namedTypeName(signature.Recv().Type()); receiver != "" {
			return "function:" + packagePath + "." + receiver + "." + function.Name()
		}
	}
	return "function:" + packagePath + "." + function.Name()
}

func typedFunctionID(packagePath string, function *goast.FuncDecl) string {
	if function == nil || function.Name == nil {
		return ""
	}
	receiver := typedReceiverName(function)
	if receiver != "" {
		return "function:" + packagePath + "." + receiver + "." + function.Name.Name
	}
	return "function:" + packagePath + "." + function.Name.Name
}

func typedReceiverName(function *goast.FuncDecl) string {
	if function == nil || function.Recv == nil || len(function.Recv.List) == 0 {
		return ""
	}
	return namedTypeName(function.Recv.List[0].Type)
}

func namedTypeName(expression any) string {
	switch value := expression.(type) {
	case goast.Expr:
		switch value := value.(type) {
		case *goast.Ident:
			return value.Name
		case *goast.StarExpr:
			return namedTypeName(value.X)
		case *goast.IndexExpr:
			return namedTypeName(value.X)
		case *goast.IndexListExpr:
			return namedTypeName(value.X)
		case *goast.SelectorExpr:
			return value.Sel.Name
		}
	case types.Type:
		switch value := value.(type) {
		case *types.Pointer:
			return namedTypeName(value.Elem())
		case interface{ Obj() *types.TypeName }:
			if object := value.Obj(); object != nil {
				return object.Name()
			}
		}
	}
	return ""
}

func filePathRelative(root string, filename string) string {
	if filename == "" {
		return ""
	}
	if relative, err := filepath.Rel(root, filename); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(filename)
}
