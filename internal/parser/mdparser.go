package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
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
	var contentLines []string
	pastTitle := false
	inCodeBlock := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			cleanedHeading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if cleanedHeading != "" {
				headings = append(headings, cleanedHeading)
				if !pastTitle {
					pastTitle = true
				}
			}
			continue
		}

		if pastTitle && len(contentLines) < 8 && trimmed != "" && !strings.HasPrefix(trimmed, "---") && !strings.HasPrefix(trimmed, "<!--") && !strings.HasPrefix(trimmed, "{{") {
			contentLines = append(contentLines, trimmed)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading markdown file: %w", err)
	}

	fileName := filepath.Base(filePath)
	summaryDescription := strings.Join(headings, "; ")

	content := strings.Join(contentLines, " ")
	if len(content) > 500 {
		content = content[:497] + "..."
	}

	var entities []domain.Entity
	entities = append(entities, domain.Entity{
		ID:          fmt.Sprintf("document:%s", fileName),
		Name:        fileName,
		Kind:        domain.KindDocument,
		Description: summaryDescription,
		Content:     content,
		Package:     "documentation",
		Source: domain.Source{
			Parser: "markdown",
			File:   filePath,
			Line:   1,
		},
	})
	return entities, nil
}
