package parser

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// Compile-Time Guard ensuring TestParser satisfies our universal interface contract
var _ Parser = (*TestParser)(nil)

type TestParser struct {
	fset            *token.FileSet
	rootDir         string
	packageResolver *packageResolver
	useImportPaths  bool
}

func NewTestParser() *TestParser {
	return &TestParser{
		fset: token.NewFileSet(),
	}
}

// NewTestParserForRepo mirrors NewGoParserForRepo so test entities use the
// same repository-unique package identity as the production code they test.
func NewTestParserForRepo(repoPath string) *TestParser {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	return &TestParser{
		fset:            token.NewFileSet(),
		rootDir:         filepath.Clean(abs),
		packageResolver: newPackageResolver(abs),
		useImportPaths:  true,
	}
}

func (p *TestParser) Parse(file domain.File) ([]domain.Entity, error) {
	goParser := &GoParser{fset: p.fset, rootDir: p.rootDir, packageResolver: p.packageResolver, useImportPaths: p.useImportPaths}
	entities, err := goParser.Parse(file)
	if err != nil {
		return nil, err
	}
	astFile, err := goParser.parseFile(file.RelativePath)
	if err != nil {
		return nil, err
	}
	tests := make(map[string]bool)
	for _, declaration := range astFile.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
			tests[fn.Name.Name] = true
		}
	}
	// Keep helper functions and receiver methods as functions. Their call chains
	// are evidence linking executable tests to the implementations they invoke.
	for i := range entities {
		entity := &entities[i]
		if entity.Kind == domain.KindFunction && tests[entity.Name] && entity.ID == "function:"+entity.Package+"."+entity.Name {
			entity.ID = "test:" + entity.Package + "." + entity.Name
			entity.Kind = domain.KindTest
			entity.Source.Parser = "test"
		}
	}
	return entities, nil
}
