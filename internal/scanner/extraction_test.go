package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func TestIncrementalReuseRequiresCompatibleExtractorAndBuildContext(t *testing.T) {
	for _, scenario := range []string{"legacy", "extractor", "binary", "platform", "tags"} {
		t.Run(scenario, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package p\ntype Config struct { AutoRepair bool }\nfunc Apply(c *Config) { if c.AutoRepair {} }\n"), 0644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "graph.json")
			first, err := Scan(repo, path, ScanOptions{})
			if err != nil {
				t.Fatal(err)
			}
			// A poisoned old observation makes it visible if parser facts survive
			// an incompatible extractor, even though repository files are unchanged.
			for i := range first.Graph.Entities {
				if first.Graph.Entities[i].Kind == domain.KindField {
					first.Graph.Entities[i].Description = "stale extraction"
				}
			}
			opts := ScanOptions{PreviousGraph: path}
			switch scenario {
			case "legacy":
				first.Graph.ExtractorVersion = ""
				first.Graph.ExtractorBuild = ""
				first.Graph.ExtractionSignature = ""
				first.Graph.BuildContext = nil
			case "extractor":
				first.Graph.ExtractorVersion = "older-extractor"
				first.Graph.ExtractionSignature = domain.ExtractionSignature(first.Graph.ExtractorVersion, *first.Graph.BuildContext, first.Graph.ExtractorBuild)
			case "binary":
				first.Graph.ExtractorBuild = "sha256:older-executable"
				first.Graph.ExtractionSignature = domain.ExtractionSignature(first.Graph.ExtractorVersion, *first.Graph.BuildContext, first.Graph.ExtractorBuild)
			case "platform":
				opts.GOOS = "linux"
				if first.Graph.BuildContext.GOOS == "linux" {
					opts.GOOS = "darwin"
				}
			case "tags":
				opts.BuildTags = []string{"custom"}
			}
			if err := storage.WriteGraph(path, first.Graph); err != nil {
				t.Fatal(err)
			}
			second, err := Scan(repo, path, opts)
			if err != nil {
				t.Fatal(err)
			}
			if second.Incremental {
				t.Fatal("incompatible extractor/build context reused parser observations")
			}
			for _, entity := range second.Graph.Entities {
				if entity.Description == "stale extraction" {
					t.Fatal("old observation survived full scan")
				}
			}
			if second.Graph.ExtractorVersion != domain.CurrentExtractorVersion || second.Graph.ExtractorBuild == "" || second.Graph.BuildContext == nil || second.Graph.ExtractionSignature == "" || second.Graph.TypeAnalysis == nil {
				t.Fatalf("missing extraction provenance: %+v", second.Graph)
			}
		})
	}
}

func TestParserCompletenessIsSeparateFromTypedCoverage(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package p\nfunc Apply() { unknown() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(repo, filepath.Join(t.TempDir(), "graph.json"), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Graph.ScanComplete || result.Graph.TypeAnalysis == nil || result.Graph.TypeAnalysis.FailedPackages != 1 || len(result.Graph.TypeAnalysis.Diagnostics) == 0 {
		t.Fatalf("parse/enrichment completeness conflated: complete=%v analysis=%+v", result.Graph.ScanComplete, result.Graph.TypeAnalysis)
	}
}

func TestIncrementalTypedResourceFactsRemainDeterministic(t *testing.T) {
	repo := t.TempDir()
	source := `package p
type Zebra struct{}
type Alpha struct{}
type Client struct{}
func (*Client) Create(ctx, object interface{}) {}
func zebra() *Zebra { return &Zebra{} }
func alpha() *Alpha { return &Alpha{} }
func Apply(c *Client) { c.Create(nil,zebra()); c.Create(nil,alpha()); c.Create(nil,&Alpha{}); c.Create(nil,&Alpha{}) }
`
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "graph.json")
	if _, err := Scan(repo, path, ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Scan(repo, path, ScanOptions{PreviousGraph: path})
	if err != nil {
		t.Fatal(err)
	}
	incremental, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Incremental || string(full) != string(incremental) {
		t.Fatal("typed resource facts changed across equivalent full/incremental scans")
	}
	for _, entity := range result.Graph.Entities {
		if entity.Name != "Apply" {
			continue
		}
		if len(entity.ResourceOperations) != 4 || entity.ResourceOperations[0].ObjectType == entity.ResourceOperations[1].ObjectType {
			t.Fatalf("distinct same-line operation object types were mixed: %+v", entity.ResourceOperations)
		}
	}
}

func TestIncrementalReferenceSitesRemainDeterministic(t *testing.T) {
	repo := t.TempDir()
	source := `package p
type Pool struct { AutoRepair bool }
func Apply(p *Pool) { _ = p.AutoRepair; _ = Pool{AutoRepair: true}; _ = p.AutoRepair }
`
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "graph.json")
	if _, err := Scan(repo, path, ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Scan(repo, path, ScanOptions{PreviousGraph: path})
	if err != nil {
		t.Fatal(err)
	}
	incremental, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Incremental || string(full) != string(incremental) {
		t.Fatal("reference sites changed across equivalent full/incremental scans")
	}
	for _, entity := range result.Graph.Entities {
		if entity.Name == "Apply" && len(entity.ReferenceSites) != 3 {
			t.Fatalf("typed sites were duplicated or dropped: %+v", entity.ReferenceSites)
		}
	}
}

func TestDuplicatePlatformDeclarationsPreferActiveSourceAndDocumentation(t *testing.T) {
	repo := t.TempDir()
	for name, source := range map[string]string{
		"config_darwin.go": "package p\ntype Config struct {\n// Darwin policy controls repairs.\nAutoRepair bool\n}\n",
		"config_linux.go":  "package p\ntype Config struct {\n// Linux policy controls repairs.\nAutoRepair bool\n}\n",
		"use.go":           "package p\nfunc Apply(c *Config) { _ = c.AutoRepair }\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Scan(repo, filepath.Join(t.TempDir(), "graph.json"), ScanOptions{GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range result.Graph.Entities {
		if entity.Kind != domain.KindField {
			continue
		}
		if entity.Source.File != "config_linux.go" || entity.Description != "Linux policy controls repairs." || entity.Source.Line != 3 || entity.Source.EndLine != 4 {
			t.Fatalf("typed linux reference exposes an inactive declaration/GoDoc: %+v", entity)
		}
		if len(entity.Files) != 2 {
			t.Fatalf("inactive declaration inventory was dropped: %+v", entity.Files)
		}
		if result.Graph.TypeAnalysis == nil || result.Graph.TypeAnalysis.ExcludedFiles != 1 {
			t.Fatalf("excluded variant not disclosed: %+v", result.Graph.TypeAnalysis)
		}
		return
	}
	t.Fatal("missing platform-specific field")
}
