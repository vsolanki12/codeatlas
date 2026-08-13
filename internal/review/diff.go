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
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Header   string
}

type FileDiff struct {
	Path         string
	OldPath      string
	Status       FileStatus
	Hunks        []Hunk
	AddedLines   int
	DeletedLines int
}

func ParseDiff(output string) []FileDiff {
	var files []FileDiff
	var current *FileDiff
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				files = append(files, *current)
			}
			current = &FileDiff{Status: FileModified}
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
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			current.AddedLines++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			current.DeletedLines++
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
