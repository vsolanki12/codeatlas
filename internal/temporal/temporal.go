package temporal

import (
	"bufio"
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

func batchEnrich(repoDir string, need map[string]bool) map[string]*fileHistory {
	result := make(map[string]*fileHistory, len(need))

	cmd := exec.Command("git", "log", "--format=%ae %at", "--name-only", "HEAD")
	cmd.Dir = repoDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result
	}
	if err := cmd.Start(); err != nil {
		return result
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentAuthor string
	var currentTime int64
	remaining := len(need)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if parts := strings.SplitN(line, " ", 2); len(parts) == 2 {
			if ts, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				currentAuthor = parts[0]
				currentTime = ts
				continue
			}
		}

		if !need[line] {
			continue
		}

		h, exists := result[line]
		if !exists {
			result[line] = &fileHistory{
				LastAuthor:   currentAuthor,
				LastModified: time.Unix(currentTime, 0).UTC().Format(time.RFC3339),
				ChangeCount:  1,
			}
			remaining--
		} else {
			h.ChangeCount++
		}

		if remaining == 0 {
			break
		}
	}

	cmd.Process.Kill()
	cmd.Wait()

	return result
}

func Enrich(repoDir string, entities []domain.Entity) error {
	need := make(map[string]bool)
	for i := range entities {
		f := entities[i].Source.File
		if f != "" {
			need[f] = true
		}
	}

	if len(need) == 0 {
		return nil
	}

	history := batchEnrich(repoDir, need)

	for i := range entities {
		f := entities[i].Source.File
		if h, ok := history[f]; ok {
			entities[i].LastAuthor = h.LastAuthor
			entities[i].LastModified = h.LastModified
			entities[i].ChangeCount = h.ChangeCount
		}
	}
	return nil
}
