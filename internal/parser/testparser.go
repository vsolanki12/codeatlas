package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

// Compile-Time Guard ensuring TestParser satisfies our universal interface contract
var _ Parser = (*TestParser)(nil)

type TestParser struct {
	fset *token.FileSet
}

func NewTestParser() *TestParser {
	return &TestParser{
		fset: token.NewFileSet(),
	}
}

func (p *TestParser) Parse(file domain.File) ([]domain.Entity, error) {
	filePath := file.RelativePath

	// 1. Parse the target Go test file into an Abstract Syntax Tree (AST)
	astFile, err := parser.ParseFile(p.fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed parsing go test file %s: %w", filePath, err)
	}

	packageName := astFile.Name.Name
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
			entities = append(entities, domain.Entity{
				ID:          fmt.Sprintf("test:%s.%s", packageName, fn.Name.Name),
				Name:        fn.Name.Name,
				Kind:        domain.KindTest,
				Description: description,
				Package:     packageName,
				Source: domain.Source{
					Parser: "test",
					File:   filePath,
					Line:   p.fset.Position(fn.Pos()).Line,
				},
			})
		}
		return true
	})

	return entities, nil
}
