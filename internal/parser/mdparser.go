package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

var _ Parser = (*MarkdownParser)(nil)

type MarkdownParser struct{}

func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{}
}

func (p *MarkdownParser) Parse(file domain.File) ([]domain.Entity, error) {
	filePath := file.RelativePath

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open markdown file: %w", err)
	}
	defer f.Close()

	var headings []string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#") {
			cleanedHeading := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if cleanedHeading != "" {
				headings = append(headings, cleanedHeading)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error heading markdown file context: %w", err)
	}

	fileName := filepath.Base(filePath)

	summaryDescription := strings.Join(headings, "; ")

	var entities []domain.Entity
	entities = append(entities, domain.Entity{
		ID:          fmt.Sprintf("document:%s", fileName),
		Name:        fileName,
		Kind:        domain.KindDocument,
		Description: summaryDescription,
		Package:     "documentation",
		Source: domain.Source{
			Parser: "markdown",
			File:   filePath,
			Line:   1,
		},
	})
	return entities, nil
}
