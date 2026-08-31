package review

import (
	"strconv"
	"strings"
)

type FileStatus string

const (
	FileAdded    FileStatus = "A"
	FileModified FileStatus = "M"
	FileDeleted  FileStatus = "D"
	FileRenamed  FileStatus = "R"
)

type Hunk struct {
	OldStart int    `json:"oldStart"`
	OldCount int    `json:"oldCount"`
	NewStart int    `json:"newStart"`
	NewCount int    `json:"newCount"`
	Header   string `json:"header,omitempty"`
}

// AddedLine retains bounded head-side text for deterministic pattern checks.
// It is an internal review aid and is intentionally omitted from serialized
// FileDiff output; the bounded diff excerpt is the public changed-text field.
type AddedLine struct {
	File string
	Line int
	Text string
}

// DeletedLine retains bounded base-side text for conservative new-entity
// detection. It is an internal review aid and is omitted from JSON output.
type DeletedLine struct {
	File string
	Line int
	Text string
}

type FileDiff struct {
	Path                    string        `json:"path"`
	OldPath                 string        `json:"oldPath,omitempty"`
	Status                  FileStatus    `json:"status"`
	Hunks                   []Hunk        `json:"hunks"`
	AddedLines              int           `json:"addedLines"`
	DeletedLines            int           `json:"deletedLines"`
	AddedContent            []AddedLine   `json:"-"`
	AddedContentTruncated   bool          `json:"-"`
	DeletedContent          []DeletedLine `json:"-"`
	DeletedContentTruncated bool          `json:"-"`
}

const maxPatternAddedLinesPerFile = 400

func ParseDiff(output string) []FileDiff {
	var files []FileDiff
	var current *FileDiff
	currentOldLine := 0
	currentNewLine := 0
	inHunk := false
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				files = append(files, *current)
			}
			current = &FileDiff{Status: FileModified}
			currentOldLine = 0
			currentNewLine = 0
			inHunk = false
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				current.Path = parts[1]
				current.OldPath = current.Path
			}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "new file mode"):
			current.Status = FileAdded
		case strings.HasPrefix(line, "deleted file mode"):
			current.Status = FileDeleted
		case strings.HasPrefix(line, "rename from "):
			current.OldPath = strings.TrimPrefix(line, "rename from ")
			current.Status = FileRenamed
		case strings.HasPrefix(line, "rename to "):
			current.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "--- a/"):
			current.OldPath = strings.TrimPrefix(line, "--- a/")
		case line == "--- /dev/null":
			// new file
		case strings.HasPrefix(line, "+++ b/"):
			current.Path = strings.TrimPrefix(line, "+++ b/")
		case line == "+++ /dev/null":
			// deleted file
		case strings.HasPrefix(line, "@@ "):
			hunk := parseHunkHeader(line)
			current.Hunks = append(current.Hunks, hunk)
			currentOldLine = hunk.OldStart
			currentNewLine = hunk.NewStart
			inHunk = true
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			current.AddedLines++
			if inHunk {
				if len(current.AddedContent) < maxPatternAddedLinesPerFile {
					current.AddedContent = append(current.AddedContent, AddedLine{File: current.Path, Line: currentNewLine, Text: strings.TrimPrefix(line, "+")})
				} else {
					current.AddedContentTruncated = true
				}
				currentNewLine++
			}
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			current.DeletedLines++
			if inHunk {
				if len(current.DeletedContent) < maxPatternAddedLinesPerFile {
					current.DeletedContent = append(current.DeletedContent, DeletedLine{File: current.OldPath, Line: currentOldLine, Text: strings.TrimPrefix(line, "-")})
				} else {
					current.DeletedContentTruncated = true
				}
				currentOldLine++
			}
		case inHunk && strings.HasPrefix(line, " "):
			currentOldLine++
			currentNewLine++
		}
	}

	if current != nil {
		files = append(files, *current)
	}

	return files
}

func parseHunkHeader(line string) Hunk {
	h := Hunk{}
	if !strings.HasPrefix(line, "@@ ") {
		return h
	}
	rest := line[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return h
	}

	if end+3 < len(rest) {
		h.Header = strings.TrimSpace(rest[end+3:])
	}

	rangeStr := rest[:end]
	parts := strings.Fields(rangeStr)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			h.OldStart, h.OldCount = parseRange(p[1:])
		} else if strings.HasPrefix(p, "+") {
			h.NewStart, h.NewCount = parseRange(p[1:])
		}
	}

	return h
}

func parseRange(s string) (int, int) {
	parts := strings.SplitN(s, ",", 2)
	start, _ := strconv.Atoi(parts[0])
	count := 1
	if len(parts) == 2 {
		count, _ = strconv.Atoi(parts[1])
	}
	return start, count
}
