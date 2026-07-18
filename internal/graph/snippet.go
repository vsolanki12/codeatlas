package graph

import (
	"os"
	"strings"
)

func ReadSnippet(filePath string, line int) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")

	// line is 1-based (from AST positions), slice is 0-based
	index := line - 1
	if index < 0 || index >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[index])
}
