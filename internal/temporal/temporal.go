package temporal

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

type fileHistory struct {
	LastAuthor   string
	LastModified string
	ChangeCount  int
}

func gitFileHistory(repoDir, filePath string) (*fileHistory, error) {
	logCmd := exec.Command("git", "log", "-1", "--format=%ae %at", "--", filePath)
	logCmd.Dir = repoDir
	var logOut bytes.Buffer
	logCmd.Stdout = &logOut
	if err := logCmd.Run(); err != nil {
		return nil, nil
	}
	line := strings.TrimSpace(logOut.String())
	if line == "" {
		return nil, nil
	}

	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected git log output: %q", line)
	}
	email := parts[0]
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	countCmd := exec.Command("git", "rev-list", "--count", "HEAD", "--", filePath)
	countCmd.Dir = repoDir
	var countOut bytes.Buffer
	countCmd.Stdout = &countOut
	if err := countCmd.Run(); err != nil {
		return nil, fmt.Errorf("git rev-list: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(countOut.String()))
	if err != nil {
		return nil, fmt.Errorf("parse count: %w", err)
	}

	return &fileHistory{
		LastAuthor:   email,
		LastModified: time.Unix(ts, 0).UTC().Format(time.RFC3339),
		ChangeCount:  count,
	}, nil
}

func Enrich(repoDir string, entities []domain.Entity) error {
	cache := map[string]*fileHistory{}

	for i := range entities {
		file := entities[i].Source.File
		if file == "" {
			continue
		}
		if _, ok := cache[file]; !ok {
			h, err := gitFileHistory(repoDir, file)
			if err != nil {
				cache[file] = nil
				continue
			}
			cache[file] = h
		}
		h := cache[file]
		if h == nil {
			continue
		}
		entities[i].LastAuthor = h.LastAuthor
		entities[i].LastModified = h.LastModified
		entities[i].ChangeCount = h.ChangeCount
	}
	return nil
}
