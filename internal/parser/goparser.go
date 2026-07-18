// Package parser extracts domain entities from Go source files using the AST.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

var _ Parser = (*GoParser)(nil)

// GoParser extracts entities from Go source files.
type GoParser struct {
	fset *token.FileSet
}

// NewGoParser returns a ready-to-use GoParser.
func NewGoParser() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

// Parse parses a Go source file and returns all discovered entities.
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
	var watches []string

	entities = append(entities, domain.Entity{
		ID:      fmt.Sprintf("package:%s", packageName),
		Name:    packageName,
		Kind:    domain.KindPackage,
		Package: packageName,
		Files:   []string{file.RelativePath},
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

		entities = append(entities, domain.Entity{
			ID:          id,
			Name:        fn.Name.Name,
			Kind:        domain.KindFunction,
			Description: description,
			Package:     packageName,
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
		}

		if fn.Name.Name == "SetupWithManager" && fn.Body != nil {
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
							}
						}
					}
				}
				return true
			})
			for i, j := 0, len(watches)-1; i < j; i, j = i+1, j-1 {
				watches[i], watches[j] = watches[j], watches[i]
			}
		}

		return true
	})

	if controllerTypeName != "" {
		entities = append(entities, domain.Entity{
			ID:      fmt.Sprintf("controller:%s.%s", packageName, controllerTypeName),
			Name:    controllerTypeName,
			Kind:    domain.KindController,
			Package: packageName,
			Source:  controllerSource,
			Watches: watches,
		})
	}

	return entities, nil
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

func extractTypeName(expr ast.Expr) string {
	// 1. Unwrap reference pointers (&Object{}) if present
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}

	// 2. Check if it's a structural composite initialization block
	compLit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return ""
	}

	// 3. Resolve the underlying text layout type names
	switch t := compLit.Type.(type) {
	case *ast.SelectorExpr:
		// Handles cross-package structs like: hyperv1.HostedCluster
		return t.Sel.Name
	case *ast.Ident:
		// Handles internal local package declarations like: LocalResource
		return t.Name
	}

	return ""
}
