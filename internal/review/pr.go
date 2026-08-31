package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const (
	maxPRBodyBytes  = 12 * 1024
	maxPRFiles      = 100
	githubDiffMedia = "application/vnd.github.v3.diff"
)

// PRMetadata is the deterministic metadata returned by GitHub for a pull
// request. The PR description is user-provided input and is kept separate
// from graph-derived facts in ReviewResult.
type PRMetadata struct {
	Repository       string   `json:"repository"`
	Number           int      `json:"number"`
	URL              string   `json:"url,omitempty"`
	Title            string   `json:"title,omitempty"`
	Body             string   `json:"body,omitempty"`
	BodyTruncated    bool     `json:"bodyTruncated,omitempty"`
	Author           string   `json:"author,omitempty"`
	BaseRef          string   `json:"baseRef,omitempty"`
	BaseSHA          string   `json:"baseSHA,omitempty"`
	HeadRef          string   `json:"headRef,omitempty"`
	HeadSHA          string   `json:"headSHA,omitempty"`
	ChangedFileCount int      `json:"changedFileCount,omitempty"`
	Additions        int      `json:"additions,omitempty"`
	Deletions        int      `json:"deletions,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	Files            []PRFile `json:"files,omitempty"`
	FilesTruncated   bool     `json:"filesTruncated,omitempty"`
}

// PRFile is the small deterministic file summary returned by the GitHub pull
// request files endpoint. The complete changed text remains in diffExcerpt.
type PRFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
	Changes   int    `json:"changes,omitempty"`
}

type prRef struct {
	Owner  string
	Repo   string
	Number int
}

type pullResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	ChangedFiles int `json:"changed_files"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	Labels       []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

type pullFileResponse struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
}

type githubRunner func(args ...string) ([]byte, error)

// ParsePRRef accepts the repository/number form documented by the CLI:
// owner/repository/number. Keeping the input grammar narrow prevents a PR
// argument from becoming an unvalidated command or API path fragment.
func ParsePRRef(value string) (string, int, error) {
	ref, err := parsePRRef(value)
	if err != nil {
		return "", 0, err
	}
	return ref.Owner + "/" + ref.Repo, ref.Number, nil
}

func parsePRRef(value string) (prRef, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(value), "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return prRef{}, fmt.Errorf("invalid PR reference %q: use owner/repository/number", value)
	}
	for _, part := range parts[:2] {
		if strings.ContainsAny(part, "\\:?#%") {
			return prRef{}, fmt.Errorf("invalid PR reference %q: owner and repository contain unsupported characters", value)
		}
	}
	number, err := strconv.Atoi(parts[2])
	if err != nil || number <= 0 {
		return prRef{}, fmt.Errorf("invalid PR reference %q: number must be positive", value)
	}
	return prRef{Owner: parts[0], Repo: parts[1], Number: number}, nil
}

func (r prRef) repository() string {
	return r.Owner + "/" + r.Repo
}

func (r prRef) endpoint() string {
	return fmt.Sprintf("repos/%s/%s/pulls/%d", r.Owner, r.Repo, r.Number)
}

func fetchPR(ref prRef, run githubRunner) (PRMetadata, string, error) {
	metadataOutput, err := run("api", ref.endpoint())
	if err != nil {
		return PRMetadata{}, "", fmt.Errorf("fetch PR metadata: %w", err)
	}

	var response pullResponse
	if err := json.Unmarshal(metadataOutput, &response); err != nil {
		return PRMetadata{}, "", fmt.Errorf("decode PR metadata: %w", err)
	}
	if response.Number == 0 {
		response.Number = ref.Number
	}
	body, bodyTruncated := boundPRBody(response.Body)
	metadata := PRMetadata{
		Repository:       ref.repository(),
		Number:           response.Number,
		URL:              response.HTMLURL,
		Title:            response.Title,
		Body:             body,
		BodyTruncated:    bodyTruncated,
		Author:           response.User.Login,
		BaseRef:          response.Base.Ref,
		BaseSHA:          response.Base.SHA,
		HeadRef:          response.Head.Ref,
		HeadSHA:          response.Head.SHA,
		ChangedFileCount: response.ChangedFiles,
		Additions:        response.Additions,
		Deletions:        response.Deletions,
	}
	for _, label := range response.Labels {
		if label.Name != "" {
			metadata.Labels = append(metadata.Labels, label.Name)
		}
	}
	sort.Strings(metadata.Labels)

	diff, err := run("api", "--header", "Accept: "+githubDiffMedia, ref.endpoint())
	if err != nil {
		return PRMetadata{}, "", fmt.Errorf("fetch PR diff: %w", err)
	}

	filesOutput, err := run("api", ref.endpoint()+"/files?per_page=100")
	if err != nil {
		return PRMetadata{}, "", fmt.Errorf("fetch PR files: %w", err)
	}
	var files []pullFileResponse
	if err := json.Unmarshal(filesOutput, &files); err != nil {
		return PRMetadata{}, "", fmt.Errorf("decode PR files: %w", err)
	}
	if len(files) > maxPRFiles {
		metadata.FilesTruncated = true
		files = files[:maxPRFiles]
	}
	for _, file := range files {
		metadata.Files = append(metadata.Files, PRFile{
			Path: file.Filename, OldPath: file.PreviousFilename, Status: file.Status,
			Additions: file.Additions, Deletions: file.Deletions, Changes: file.Changes,
		})
	}
	sort.Slice(metadata.Files, func(i, j int) bool {
		if metadata.Files[i].Path == metadata.Files[j].Path {
			return metadata.Files[i].OldPath < metadata.Files[j].OldPath
		}
		return metadata.Files[i].Path < metadata.Files[j].Path
	})
	if metadata.ChangedFileCount > len(metadata.Files) {
		metadata.FilesTruncated = true
	}

	return metadata, string(diff), nil
}

func runGitHub(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return out, nil
}

func boundPRBody(body string) (string, bool) {
	if len(body) <= maxPRBodyBytes {
		return body, false
	}
	const marker = "\n\n[PR DESCRIPTION TRUNCATED]\n"
	limit := maxPRBodyBytes - len(marker)
	if limit < 0 {
		return marker, true
	}
	return body[:limit] + marker, true
}
