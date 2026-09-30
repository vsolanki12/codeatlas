package graph

import (
	"go/build"
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/parser"
)

func TestTypedTestVariantsFieldsAndHelperMethods(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"p/p.go": `package p
type Management struct { AutoRepair bool }
type NodePool struct { Management }
type CAPI struct{}
func (*CAPI) Reconcile(pool *NodePool) { if pool.AutoRepair { repair() }; _ = Management{AutoRepair: true} }
func repair() {}
`,
		"p/p_test.go": `package p
import "testing"
type Scenario struct{}
func (*Scenario) Run() { c := &CAPI{}; c.Reconcile(&NodePool{}) }
func TestReconcile(t *testing.T) { c := &CAPI{}; c.Reconcile(&NodePool{}) }
func TestViaHelper(t *testing.T) { s := &Scenario{}; s.Run() }
`,
		"p/external_test.go": `package p_test
import (
 "testing"
 p "example.com/repo/p"
)
func TestReconcile(t *testing.T) { c := &p.CAPI{}; c.Reconcile(&p.NodePool{}) }
`,
	})
	files := []domain.File{{RelativePath: "p/p.go"}, {RelativePath: "p/p_test.go"}, {RelativePath: "p/external_test.go"}}
	var entities []domain.Entity
	for _, file := range files {
		var found []domain.Entity
		var err error
		if strings.HasSuffix(file.RelativePath, "_test.go") {
			found, err = parser.NewTestParserForRepo(repo).Parse(file)
		} else {
			found, err = parser.NewGoParserForRepo(repo).Parse(file)
		}
		if err != nil {
			t.Fatal(err)
		}
		entities = append(entities, found...)
	}
	context := domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH, CgoEnabled: build.Default.CgoEnabled}
	relationships, coverage := BuildTypedRelationships(repo, files, entities, context)
	if coverage.FailedPackages != 0 || coverage.CheckedPackages != 3 {
		t.Fatalf("test variants were not independently checked: %+v", coverage)
	}
	byID := make(map[string]domain.Relationship)
	for _, relation := range relationships {
		byID[relation.ID] = relation
	}
	capi := "function:example.com/repo/p.CAPI.Reconcile"
	for _, testID := range []string{"test:example.com/repo/p.TestReconcile", "test:example.com/repo/p_test.TestReconcile"} {
		id := domain.NewRelationshipID(capi, domain.RelTestedBy, testID)
		relation, ok := byID[id]
		if !ok || relation.Confidence != domain.ConfidenceProven || relation.Evidence.Parser != "go-types" {
			t.Fatalf("missing typed receiver-test evidence %s: %+v", id, relationships)
		}
	}
	helperTest := domain.NewRelationshipID(capi, domain.RelTestedBy, "test:example.com/repo/p.TestViaHelper")
	if relation, exists := byID[helperTest]; !exists || !strings.Contains(relation.Evidence.Reason, "Scenario.Run") || !strings.Contains(relation.Evidence.Reason, "does not prove execution") {
		t.Fatalf("missing explicitly qualified helper-chain test evidence: %+v", relation)
	}
	for _, expected := range []string{
		domain.NewRelationshipID("function:example.com/repo/p.Scenario.Run", domain.RelCalls, capi),
		domain.NewRelationshipID("test:example.com/repo/p.TestViaHelper", domain.RelCalls, "function:example.com/repo/p.Scenario.Run"),
		domain.NewRelationshipID(capi, domain.RelReferences, "field:example.com/repo/p.Management.AutoRepair"),
	} {
		if _, ok := byID[expected]; !ok {
			t.Fatalf("missing helper/promoted-field evidence %s: %+v", expected, relationships)
		}
	}
	if coverage.ResolvedReferences < 2 {
		t.Fatalf("selector and keyed literal references were not both retained: %+v", coverage)
	}
}

func TestTypedCoverageReportsPartialFailureAndExplicitBuildContext(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"p/p.go":       "package p\nfunc Caller() { Callee(); unknown() }\nfunc Callee() {}\n",
		"p/variant.go": "//go:build customtag\n\npackage p\nfunc Tagged() {}\n",
	})
	files := []domain.File{{RelativePath: "p/p.go"}, {RelativePath: "p/variant.go"}}
	entities := parseTypedFixture(t, repo, files)
	context := domain.BuildContext{GOOS: "linux", GOARCH: "amd64"}
	relationships, coverage := BuildTypedRelationships(repo, files, entities, context)
	if coverage.FailedPackages != 1 || len(coverage.Diagnostics) == 0 || coverage.ExcludedFiles != 1 || coverage.UnresolvedCalls != 1 {
		t.Fatalf("partial failure/variant not reported: %+v", coverage)
	}
	if len(relationships) != 1 || relationships[0].To != "function:example.com/repo/p.Callee" {
		t.Fatalf("partial exact evidence lost: %+v", relationships)
	}
	context.BuildTags = []string{"customtag"}
	_, tagged := BuildTypedRelationships(repo, files, entities, context)
	if tagged.ExcludedFiles != 0 {
		t.Fatalf("explicit build tag ignored: %+v", tagged)
	}
}

func TestTypedResourceHelperReturnPreservedWithoutManifest(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"p/p.go": `package p
type MachineHealthCheck struct{}
type Client struct{}
func (*Client) Create(ctx interface{}, object *MachineHealthCheck) {}
func object() *MachineHealthCheck { return &MachineHealthCheck{} }
func Reconcile(client *Client) { client.Create(nil, object()) }
`,
	})
	files := []domain.File{{RelativePath: "p/p.go"}}
	entities := parseTypedFixture(t, repo, files)
	BuildTypedRelationships(repo, files, entities, domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH})
	for _, entity := range entities {
		if entity.Name != "Reconcile" || entity.Kind != domain.KindFunction {
			continue
		}
		if len(entity.ResourceOperations) != 1 || entity.ResourceOperations[0].ObjectType != "*example.com/repo/p.MachineHealthCheck" {
			t.Fatalf("helper return object type unresolved: %+v", entity.ResourceOperations)
		}
		if len(entity.Creates) != 1 || entity.Creates[0] != "MachineHealthCheck" {
			t.Fatalf("typed create observation lost: %+v", entity.Creates)
		}
		for _, relation := range NewRelationshipBuilder(repo).Build(entities) {
			if relation.Type == domain.RelCreates {
				t.Fatalf("invented manifest target: %+v", relation)
			}
		}
		return
	}
	t.Fatal("missing Reconcile entity")
}

func TestTestVariantsForProductionPackageNamedWithTestSuffix(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"p/p.go":             "package foo_test\ntype CAPI struct{}\nfunc (*CAPI) Reconcile() {}\n",
		"p/p_test.go":        "package foo_test\nimport \"testing\"\nfunc TestInternal(t *testing.T) { (&CAPI{}).Reconcile() }\n",
		"p/external_test.go": "package foo_test_test\nimport (\"testing\"; p \"example.com/repo/p\")\nfunc TestExternal(t *testing.T) { (&p.CAPI{}).Reconcile() }\n",
	})
	files := []domain.File{{RelativePath: "p/p.go"}, {RelativePath: "p/p_test.go"}, {RelativePath: "p/external_test.go"}}
	var entities []domain.Entity
	for _, file := range files {
		var parsed []domain.Entity
		var err error
		if strings.HasSuffix(file.RelativePath, "_test.go") {
			parsed, err = parser.NewTestParserForRepo(repo).Parse(file)
		} else {
			parsed, err = parser.NewGoParserForRepo(repo).Parse(file)
		}
		if err != nil {
			t.Fatal(err)
		}
		entities = append(entities, parsed...)
	}
	relations, coverage := BuildTypedRelationships(repo, files, entities, domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH, CgoEnabled: build.Default.CgoEnabled})
	if coverage.FailedPackages != 0 || coverage.CheckedPackages != 3 {
		t.Fatalf("valid production package with _test name was mixed into external tests: %+v", coverage)
	}
	byID := make(map[string]bool)
	for _, relation := range relations {
		byID[relation.ID] = true
	}
	for _, testID := range []string{"test:example.com/repo/p.TestInternal", "test:example.com/repo/p_test.TestExternal"} {
		id := domain.NewRelationshipID("function:example.com/repo/p.CAPI.Reconcile", domain.RelTestedBy, testID)
		if !byID[id] {
			t.Fatalf("test identity/call evidence lost for production _test package: %s", id)
		}
	}
}

func TestTypedReferenceSitesPreserveRepeatedSelectorsAndLiterals(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"p/p.go": `package p
type Management struct { AutoRepair bool }
func Apply(m *Management) {
 m.AutoRepair = true
 _ = m.AutoRepair
 if m.AutoRepair && m.AutoRepair {}
 _ = Management{AutoRepair: true}
}
`,
	})
	files := []domain.File{{RelativePath: "p/p.go"}}
	entities := parseTypedFixture(t, repo, files)
	relations, _ := BuildTypedRelationships(repo, files, entities, domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH})
	fieldID := "field:example.com/repo/p.Management.AutoRepair"
	var apply *domain.Entity
	for i := range entities {
		if entities[i].Name == "Apply" {
			apply = &entities[i]
		}
	}
	if apply == nil || len(apply.ReferenceSites) != 5 {
		t.Fatalf("repeated usage sites collapsed: %+v", apply)
	}
	seen := make(map[domain.ReferenceSite]bool)
	for _, site := range apply.ReferenceSites {
		if seen[site] || site.Target != fieldID || site.Source.Column <= 0 || site.Source.EndLine != site.Source.Line {
			t.Fatalf("invalid or duplicate exact usage site: %+v", site)
		}
		seen[site] = true
	}
	if apply.ReferenceSites[2].Source.Line != apply.ReferenceSites[3].Source.Line || apply.ReferenceSites[2].Source.Column == apply.ReferenceSites[3].Source.Column {
		t.Fatalf("same-line selectors were not distinguished: %+v", apply.ReferenceSites)
	}
	referenceEdges := 0
	for _, relation := range relations {
		if relation.Type == domain.RelReferences && relation.From == apply.ID && relation.To == fieldID {
			referenceEdges++
		}
	}
	if referenceEdges != 1 {
		t.Fatalf("forward relationship identity duplicated: %d", referenceEdges)
	}
	BuildTypedRelationships(repo, files, entities, domain.BuildContext{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH})
	if len(apply.ReferenceSites) != 5 {
		t.Fatalf("reanalyzing retained duplicate/stale sites: %+v", apply.ReferenceSites)
	}
}
