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
	filePath := file.RelativePath

	// 1. Parse the target Go test file into an Abstract Syntax Tree (AST)
	parsePath := filePath
	if p.rootDir != "" {
		parsePath = filepath.Join(p.rootDir, filepath.FromSlash(filePath))
	}
	astFile, err := parser.ParseFile(p.fset, parsePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed parsing go test file %s: %w", filePath, err)
	}

	packageName := astFile.Name.Name
	packagePath := packageName
	if p.packageResolver != nil {
		packagePath = p.packageResolver.resolve(filePath, packageName)
	}
	var entities []domain.Entity

	// 2. Walk the AST to extract function declarations
	ast.Inspect(astFile, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// 3. Filter: Only extract standalone functions starting with "Test"
		// Check that it's a plain function (Recv == nil) and has the "Test" prefix
		if fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
			description := ""
			if fn.Doc != nil {
				description = strings.TrimSpace(fn.Doc.Text())
			}

			// 4. Create KindTest entity with format: test:pkg.TestFuncName
			calls, callSites, _ := extractCallsAndEnvVars(fn.Body, buildImportAliasMap(astFile, p.useImportPaths), filePath, p.fset)
			entities = append(entities, domain.Entity{
				ID:          fmt.Sprintf("test:%s.%s", packagePath, fn.Name.Name),
				Name:        fn.Name.Name,
				Kind:        domain.KindTest,
				Description: description,
				Package:     packagePath,
				Calls:       calls,
				CallSites:   callSites,
				Source: domain.Source{
					Parser:  "test",
					File:    filePath,
					Line:    p.fset.Position(fn.Pos()).Line,
					EndLine: p.fset.Position(fn.End()).Line,
				},
			})
		}
		return true
	})

	return entities, nil
}
