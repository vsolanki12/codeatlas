package domain

import "testing"

func TestGraphValidateScanCoverage(t *testing.T) {
	valid := Graph{
		Schema:        "codeatlas",
		SchemaVersion: CurrentSchemaVersion,
		ScanFiles:     []ScanFile{{Path: "main.go", Status: ScanFileParsed, Parser: "go"}},
		ScanCoverage:  &ScanCoverage{Discovered: 1, Parsed: 1},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid scan coverage rejected: %v", err)
	}

	invalid := valid
	invalid.ScanCoverage = &ScanCoverage{Discovered: 2, Parsed: 1}
	if err := invalid.Validate(); err == nil {
		t.Fatal("inconsistent scan coverage should be rejected")
	}
}

func TestGraphValidateScanCoverageMatchesStatuses(t *testing.T) {
	graph := Graph{
		Schema:        "codeatlas",
		SchemaVersion: CurrentSchemaVersion,
		ScanFiles: []ScanFile{
			{Path: "main.go", Status: ScanFileParsed, Parser: "go"},
			{Path: "notes.txt", Status: ScanFileIgnored, Reason: "no parser"},
		},
		ScanCoverage: &ScanCoverage{Discovered: 2, Parsed: 0, Ignored: 2},
	}
	if err := graph.Validate(); err == nil {
		t.Fatal("coverage/status mismatch should be rejected")
	}
}

func TestGraphValidateCompleteRejectsFailedScanFile(t *testing.T) {
	graph := Graph{
		Schema:        "codeatlas",
		SchemaVersion: CurrentSchemaVersion,
		ScanComplete:  true,
		ScanFiles:     []ScanFile{{Path: "broken.go", Status: ScanFileFailed, Parser: "go", Reason: "syntax error"}},
		ScanCoverage:  &ScanCoverage{Discovered: 1, Failed: 1},
	}
	if err := graph.Validate(); err == nil {
		t.Fatal("complete graph with failed scan file should be rejected")
	}
}
