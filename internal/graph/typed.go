package graph

import (
	"fmt"
	goast "go/ast"
	"go/build"
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
// edge at all. Only targets that already exist as CodeAtlas entities are
// emitted, including tests and field references from the shared pass.
//
// The loader is source-backed for packages in the scanned repository and uses
// the standard compiler importer for dependencies outside the graph. This
// keeps the pass repository-wide without making the graph depend on generated
// export data or a particular build cache.
func BuildTypedCallRelationships(root string, files []domain.File, entities []domain.Entity) []domain.Relationship {
	context := domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH, BuildTags: build.Default.BuildTags, CgoEnabled: build.Default.CgoEnabled}
	relationships, _ := BuildTypedRelationships(root, files, entities, context)
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
	path    string
	files   map[string]bool
	variant string
}

type typedPackage struct {
	pkg       *types.Package
	info      *types.Info
	fset      *token.FileSet
	files     []string
	astByPath map[string]*goast.File
}

type sourcePackageLoader struct {
	root         string
	specs        map[string]*sourcePackageSpec
	checked      map[string]*typedPackage
	checking     map[string]bool
	fallback     types.Importer
	buildContext *build.Context
	coverage     *domain.TypeAnalysisCoverage
	attempted    map[string]bool
}

func (l *sourcePackageLoader) Import(path string) (*types.Package, error) {
	if checked := l.checked[path]; checked != nil && checked.pkg != nil {
		return checked.pkg, nil
	}
	spec := l.specs[path]
	if spec == nil {
		if l.coverage != nil {
			l.coverage.ExternalImports++
		}
		pkg, err := l.fallback.Import(path)
		if err != nil && l.coverage != nil {
			l.coverage.ExternalImportFailures++
		}
		return pkg, err
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
	if l.attempted != nil && l.attempted[path] {
		return nil
	}
	if l.attempted != nil {
		l.attempted[path] = true
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
	var errors []string
	buildContext := &build.Default
	if l.buildContext != nil {
		buildContext = l.buildContext
	}
	for _, relative := range files {
		absolute := filepath.Join(l.root, filepath.FromSlash(relative))
		if matched, err := buildContext.MatchFile(filepath.Dir(absolute), filepath.Base(absolute)); err != nil || !matched {
			if err != nil {
				errors = append(errors, err.Error())
			}
			continue
		}
		astFile, err := parser.ParseFile(fset, absolute, nil, parser.ParseComments)
		if err != nil || astFile.Name == nil {
			if err != nil {
				errors = append(errors, err.Error())
			}
			continue
		}
		astFiles = append(astFiles, astFile)
		astByPath[relative] = astFile
	}
	if len(astFiles) == 0 {
		l.recordDiagnostics(spec, errors)
		return nil
	}

	info := &types.Info{
		Uses:       make(map[*goast.Ident]types.Object),
		Selections: make(map[*goast.SelectorExpr]*types.Selection),
		Types:      make(map[goast.Expr]types.TypeAndValue),
		Defs:       make(map[*goast.Ident]types.Object),
	}
	config := &types.Config{
		Importer:    l,
		FakeImportC: true,
		Error:       func(err error) { errors = append(errors, err.Error()) },
		Sizes:       types.SizesFor("gc", buildContext.GOARCH),
	}
	pkg, _ := config.Check(spec.path, fset, astFiles, info)
	l.recordDiagnostics(spec, errors)
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

func (l *sourcePackageLoader) recordDiagnostics(spec *sourcePackageSpec, errors []string) {
	if l.coverage == nil {
		return
	}
	if len(errors) == 0 {
		l.coverage.CheckedPackages++
		return
	}
	l.coverage.FailedPackages++
	sort.Strings(errors)
	seen := make(map[string]bool)
	count := 0
	for _, message := range errors {
		message = strings.ReplaceAll(message, l.root+string(filepath.Separator), "")
		if seen[message] {
			continue
		}
		seen[message] = true
		l.coverage.DiagnosticCount++
		if count >= 8 {
			l.coverage.DiagnosticsTruncated = true
			continue
		}
		l.coverage.Diagnostics = append(l.coverage.Diagnostics, domain.TypeAnalysisDiagnostic{Package: spec.path, Variant: spec.variant, Message: message})
		count++
	}
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
