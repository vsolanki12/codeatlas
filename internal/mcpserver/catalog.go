package mcpserver

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

type graphTarget struct {
	// Index is populated by load, which resolve calls before handing a target
	// to a tool. It is never nil in a tool handler.
	Index            *query.Index
	loadOnce         sync.Once
	loadErr          error
	GraphPath        string
	RepositoryID     string
	RepositoryRemote string
	GoModule         string
	Aliases          []string
	Key              string
}

// load decodes this repository's graph the first time it is selected. A
// server configured with several repositories otherwise paid the full decode
// of every graph at startup: on repositories the size of HyperShift that is
// hundreds of megabytes resident per repository before any tool is called.
func (target *graphTarget) load() error {
	target.loadOnce.Do(func() {
		target.Index, target.loadErr = query.LoadGraph(target.GraphPath)
	})
	return target.loadErr
}

type graphCatalog struct {
	targets []*graphTarget
	byAlias map[string][]*graphTarget
}

type repositorySummary struct {
	ID               string `json:"id"`
	RepositoryID     string `json:"repositoryId,omitempty"`
	RepositoryRemote string `json:"repositoryRemote,omitempty"`
	GoModule         string `json:"goModule,omitempty"`
	Commit           string `json:"commit,omitempty"`
}

func loadGraphCatalog(graphPaths []string) (*graphCatalog, error) {
	catalog := &graphCatalog{byAlias: make(map[string][]*graphTarget)}
	byRepositoryID := make(map[string]string)
	for _, graphPath := range graphPaths {
		header, err := storage.ReadGraphHeader(graphPath)
		if err != nil {
			return nil, fmt.Errorf("load graph %s: %w", graphPath, err)
		}
		if header.RepositoryID != "" {
			if previousPath, exists := byRepositoryID[header.RepositoryID]; exists {
				return nil, fmt.Errorf("graphs %s and %s have the same repository ID %s; load one graph per repository", previousPath, graphPath, header.RepositoryID)
			}
			byRepositoryID[header.RepositoryID] = graphPath
		}
		catalog.addHeader(header, graphPath)
	}
	if len(catalog.targets) == 0 {
		return nil, fmt.Errorf("at least one graph is required")
	}
	sort.Slice(catalog.targets, func(i, j int) bool { return catalog.targets[i].Key < catalog.targets[j].Key })
	return catalog, nil
}

func singleGraphCatalog(idx *query.Index, graphPath string) *graphCatalog {
	catalog := &graphCatalog{byAlias: make(map[string][]*graphTarget)}
	catalog.add(idx, graphPath)
	return catalog
}

// add registers an already-decoded graph, used when the server was given a
// single graph that the caller has loaded itself.
func (catalog *graphCatalog) add(idx *query.Index, graphPath string) {
	metadata := idx.GraphMetadata()
	target := catalog.addHeader(storage.Header{
		Repository: metadata.Repository, RepositoryID: metadata.RepositoryID,
		RepositoryRemote: metadata.RepositoryRemote, GoModule: metadata.GoModule,
		Commit: metadata.Commit,
	}, graphPath)
	target.Index = idx
	target.loadOnce.Do(func() {})
}

func (catalog *graphCatalog) addHeader(header storage.Header, graphPath string) *graphTarget {
	metadata := header
	target := &graphTarget{
		GraphPath: graphPath, RepositoryID: metadata.RepositoryID,
		RepositoryRemote: metadata.RepositoryRemote, GoModule: metadata.GoModule,
	}
	aliases := []string{metadata.RepositoryID, metadata.RepositoryRemote, metadata.GoModule, filepath.Base(metadata.Repository), graphPath}
	graphDirectory := filepath.Base(filepath.Dir(graphPath))
	if graphDirectory != "." && graphDirectory != string(filepath.Separator) {
		aliases = append(aliases, graphDirectory)
	}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || alias == "." || alias == string(filepath.Separator) {
			continue
		}
		addAlias(target, alias)
		for _, prefix := range []string{"git:", "go:", "configured:"} {
			if strings.HasPrefix(alias, prefix) && len(alias) > len(prefix) {
				addAlias(target, strings.TrimPrefix(alias, prefix))
			}
		}
	}
	if len(target.Aliases) == 0 {
		addAlias(target, "graph")
	}
	switch {
	case metadata.RepositoryID != "":
		target.Key = metadata.RepositoryID
	case metadata.GoModule != "":
		target.Key = metadata.GoModule
	case filepath.Base(metadata.Repository) != "." && filepath.Base(metadata.Repository) != string(filepath.Separator):
		target.Key = filepath.Base(metadata.Repository)
	default:
		target.Key = filepath.Base(graphPath)
	}
	baseKey := target.Key
	for collision := 0; catalog.hasKey(target.Key); collision++ {
		disambiguator := metadata.Commit
		if len(disambiguator) > 8 {
			disambiguator = disambiguator[:8]
		}
		if disambiguator == "" {
			disambiguator = filepath.Base(graphPath)
		}
		if collision > 0 {
			disambiguator += "@" + filepath.Clean(graphPath)
		}
		if collision > 1 {
			disambiguator += fmt.Sprintf("-%d", collision+1)
		}
		target.Key = baseKey + "@" + disambiguator
	}
	addAlias(target, target.Key)
	sort.Strings(target.Aliases)
	catalog.targets = append(catalog.targets, target)
	for _, alias := range target.Aliases {
		key := strings.ToLower(alias)
		catalog.byAlias[key] = append(catalog.byAlias[key], target)
	}
	return target
}

func (catalog *graphCatalog) hasKey(key string) bool {
	for _, target := range catalog.targets {
		if target.Key == key {
			return true
		}
	}
	return false
}

func addAlias(target *graphTarget, alias string) {
	for _, existing := range target.Aliases {
		if strings.EqualFold(existing, alias) {
			return
		}
	}
	target.Aliases = append(target.Aliases, alias)
}

func (catalog *graphCatalog) resolve(selector string) (*graphTarget, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(catalog.targets) == 1 {
			return catalog.loaded(catalog.targets[0])
		}
		return nil, fmt.Errorf("repository is required; available repositories: %s", catalog.availableIDs())
	}
	candidates := catalog.byAlias[strings.ToLower(selector)]
	switch len(candidates) {
	case 1:
		return catalog.loaded(candidates[0])
	case 0:
		return nil, fmt.Errorf("repository %q not found; available repositories: %s", selector, catalog.availableIDs())
	default:
		return nil, fmt.Errorf("repository selector %q is ambiguous; use a unique repository ID or module path", selector)
	}
}

// loaded decodes the selected repository's graph before any tool sees it, so
// a handler never has to distinguish a loaded target from an unloaded one.
func (catalog *graphCatalog) loaded(target *graphTarget) (*graphTarget, error) {
	if err := target.load(); err != nil {
		return nil, fmt.Errorf("load graph %s: %w", target.GraphPath, err)
	}
	return target, nil
}

func (catalog *graphCatalog) availableIDs() string {
	ids := make([]string, 0, len(catalog.targets))
	for _, target := range catalog.targets {
		ids = append(ids, target.Key)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}
