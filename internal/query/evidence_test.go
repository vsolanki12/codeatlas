package query

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func evidenceTestGraph() domain.Graph {
	field := domain.Entity{ID: "field:example.com/hypershift/api.NodePoolManagement.AutoRepair", Name: "AutoRepair", Kind: domain.KindField,
		Package: "example.com/hypershift/api", Description: "Enable replacement of unhealthy NodePool machines.", Source: domain.Source{Parser: "go-ast", File: "api/nodepool_types.go", Line: 541, EndLine: 548}}
	impl := domain.Entity{ID: "function:example.com/hypershift/controllers/nodepool.CAPI.Reconcile", Name: "CAPI.Reconcile", Kind: domain.KindFunction,
		Package: "example.com/hypershift/controllers/nodepool", Properties: []string{"nodePool.Spec.Management.AutoRepair"}, Source: domain.Source{Parser: "go-ast", File: "controllers/nodepool/capi.go", Line: 100, EndLine: 400}}
	helper := domain.Entity{ID: "function:example.com/hypershift/controllers/nodepool.reconcileMachineHealthCheck", Name: "reconcileMachineHealthCheck", Kind: domain.KindFunction,
		Source: domain.Source{Parser: "go-ast", File: "controllers/nodepool/capi.go", Line: 726, EndLine: 760}}
	test := domain.Entity{ID: "test:example.com/hypershift/controllers/nodepool.TestCAPIReconcile", Name: "TestCAPIReconcile", Kind: domain.KindTest,
		Source: domain.Source{Parser: "test", File: "controllers/nodepool/capi_test.go", Line: 1316, EndLine: 1480}}
	setter := domain.Entity{ID: "function:example.com/hypershift/applyconfiguration.NodePoolManagement.WithAutoRepair", Name: "WithAutoRepair", Kind: domain.KindFunction, Generated: true,
		Source: domain.Source{Parser: "go-ast", File: "applyconfiguration/nodepoolmanagement.go", Line: 40, EndLine: 46}}
	other := domain.Entity{ID: "field:example.com/other.ClusterManagement.AutoRepair", Name: "AutoRepair", Kind: domain.KindField,
		Source: domain.Source{Parser: "go-ast", File: "api/cluster_types.go", Line: 5, EndLine: 8}}
	graph := domain.Graph{SchemaVersion: "1.6.0", Repository: "example.com/hypershift", Commit: "revision-one", ScanComplete: true,
		Entities: []domain.Entity{setter, other, impl, test, field, helper}}
	for _, edge := range []struct {
		from, to string
		kind     domain.RelationshipType
		file     string
		line     int
	}{
		{impl.ID, field.ID, domain.RelReferences, "controllers/nodepool/capi.go", 170},
		{impl.ID, helper.ID, domain.RelCalls, "controllers/nodepool/capi.go", 180},
		{impl.ID, test.ID, domain.RelTestedBy, "controllers/nodepool/capi_test.go", 1350},
	} {
		graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(edge.from, edge.kind, edge.to), From: edge.from, To: edge.to, Type: edge.kind,
			Confidence: domain.ConfidenceProven, Evidence: domain.Evidence{Parser: "go-types", File: edge.file, Line: edge.line, Snippet: "source-backed evidence", Reason: "resolved call or selector"}})
	}
	return graph
}

func TestEvidencePreservesScopedBehaviorAndTestSpans(t *testing.T) {
	idx := newIndex(evidenceTestGraph())
	for _, question := range []string{"How does AutoRepair work in NodePool?", "how does autoRepair work in Nodepool?", "how does autorepair work in nodepool?"} {
		packet, err := idx.Evidence(EvidenceRequest{Question: question, Scope: "NodePool", Intent: "understand"})
		if err != nil {
			t.Fatal(err)
		}
		if packet.Status != "ok" {
			t.Fatalf("%q: status %s", question, packet.Status)
		}
		if len(packet.Entities) < 3 || packet.Entities[0].Kind != "field" || packet.Entities[1].Name != "CAPI.Reconcile" || packet.Entities[2].Kind != "test" {
			t.Fatalf("declaration, implementation, test were not prioritized: %+v", packet.Entities)
		}
		seen := make(map[string]bool)
		for _, e := range packet.Entities {
			if seen[e.ID] {
				t.Fatalf("duplicate entity %s", e.ID)
			}
			seen[e.ID] = true
			if strings.Contains(e.ID, "example.com/other") {
				t.Fatal("scope leaked an unrelated declaration")
			}
		}
		seen = make(map[string]bool)
		for _, r := range packet.Relationships {
			if seen[r.ID] {
				t.Fatal("duplicate relationship")
			}
			seen[r.ID] = true
			if r.Evidence.File == "" {
				t.Fatal("evidence dropped")
			}
		}
		usage, testSpan := false, false
		for _, s := range packet.Sources {
			if s.Source.EndLine < s.Source.Line {
				t.Fatal("invalid span")
			}
			if s.Role == "usage" && s.Source.File == "controllers/nodepool/capi.go" && s.Source.Line <= 170 && s.Source.EndLine >= 170 {
				usage = true
			}
			if s.Role == "test" && s.Source.Line == 1316 && s.Source.EndLine == 1480 {
				testSpan = true
			}
		}
		if !usage || !testSpan {
			t.Fatalf("lost behavioral or late test span: %+v", packet.Sources)
		}
	}
}

func TestEvidencePrioritizesPrimaryTestsOverUtilityTests(t *testing.T) {
	graph := evidenceTestGraph()
	implementation := graph.Entities[2].ID
	utility := domain.Entity{ID: "function:example.com/hypershift/controllers/nodepool.Condition", Name: "Condition", Kind: domain.KindFunction,
		Source: domain.Source{Parser: "go-ast", File: "controllers/nodepool/conditions.go", Line: 10, EndLine: 20}}
	graph.Entities = append(graph.Entities, utility)
	graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(implementation, domain.RelCalls, utility.ID), From: implementation, To: utility.ID, Type: domain.RelCalls,
		Confidence: domain.ConfidenceProven, Evidence: domain.Evidence{Parser: "go-types", File: "controllers/nodepool/capi.go", Line: 171, Reason: "typed call"}})
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("test:example.com/hypershift/controllers/nodepool.TestCondition%02d", i)
		graph.Entities = append(graph.Entities, domain.Entity{ID: id, Name: fmt.Sprintf("TestCondition%02d", i), Kind: domain.KindTest,
			Source: domain.Source{Parser: "test", File: "controllers/nodepool/conditions_test.go", Line: 30 + i*10, EndLine: 39 + i*10}})
		graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(utility.ID, domain.RelTestedBy, id), From: utility.ID, To: id, Type: domain.RelTestedBy,
			Confidence: domain.ConfidenceProven, Evidence: domain.Evidence{Parser: "go-types", File: "controllers/nodepool/conditions_test.go", Line: 35 + i*10, Reason: "typed test call"}})
	}
	packet, err := newIndex(graph).Evidence(EvidenceRequest{Question: "How does AutoRepair work in NodePool?", Scope: "nodepool", BudgetBytes: 16384})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Entities) < 3 || packet.Entities[1].ID != implementation || packet.Entities[2].Name != "TestCAPIReconcile" {
		t.Fatalf("indirect utility tests displaced primary evidence: %+v", packet.Entities)
	}
	helper := false
	for _, entity := range packet.Entities {
		if entity.Name == "reconcileMachineHealthCheck" {
			helper = true
		}
	}
	if !helper {
		t.Fatal("utility test fanout displaced the concept's implementation helper")
	}
}

func TestEvidenceRetainsRepeatedFieldReferenceFocus(t *testing.T) {
	graph := evidenceTestGraph()
	fieldID := graph.Entities[4].ID
	test := &graph.Entities[3]
	test.Source.EndLine = 2010
	for _, line := range []int{1354, 1543, 1940} {
		test.ReferenceSites = append(test.ReferenceSites, domain.ReferenceSite{Target: fieldID, Source: domain.Source{Parser: "go-types", File: test.Source.File, Line: line, EndLine: line, Column: 7}})
	}
	packet, err := newIndex(graph).Evidence(EvidenceRequest{Question: "How does AutoRepair work in NodePool?", Scope: "nodepool"})
	if err != nil {
		t.Fatal(err)
	}
	focus := make(map[int]bool)
	for _, source := range packet.Sources {
		if source.EntityID == test.ID && source.Role == "usage" {
			if source.FocusLine < source.Source.Line || source.FocusLine > source.Source.EndLine {
				t.Fatalf("focus outside selected source: %+v", source)
			}
			focus[source.FocusLine] = true
		}
	}
	for _, line := range []int{1354, 1543, 1940} {
		if !focus[line] {
			t.Fatalf("repeated field reference at %d was lost", line)
		}
	}
}

func TestEvidenceFailsClosedAndPreservesExactIDs(t *testing.T) {
	idx := newIndex(evidenceTestGraph())
	for _, req := range []EvidenceRequest{
		{Question: "How does NotInGraphFeature work?"},
		{Question: "AutoRepair", Entity: "function:example.com/hypershift.Missing"},
		{Question: "How does function:example.com/hypershift.Missing work?"},
		{Question: "How does AutoRepair work with MissingScope?"},
	} {
		packet, err := idx.Evidence(req)
		if err != nil {
			t.Fatal(err)
		}
		if packet.Status != "no_match" || len(packet.Entities) != 0 || len(packet.Relationships) != 0 {
			t.Fatalf("missing evidence did not fail closed: %+v", packet)
		}
	}
	packet, err := idx.Evidence(EvidenceRequest{Entity: "AutoRepair"})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Status != "ambiguous" || len(packet.Candidates) != 2 || len(packet.Entities) != 0 {
		t.Fatalf("ambiguous name chose an entity: %+v", packet)
	}
	id := "function:example.com/hypershift/controllers/nodepool.CAPI.Reconcile"
	packet, err = idx.Evidence(EvidenceRequest{Question: "How does " + id + " work?"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range packet.Entities {
		if e.ID == id {
			found = true
		}
	}
	if packet.Status != "ok" || !found {
		t.Fatalf("full ID was not preserved: %+v", packet)
	}
}

func TestEvidenceStrictBudgetAndSnapshotContinuation(t *testing.T) {
	graph := evidenceTestGraph()
	for i := 0; i < 40; i++ {
		graph.Entities = append(graph.Entities, domain.Entity{ID: fmt.Sprintf("function:example.com/hypershift/controllers/nodepool.helper%02d", i), Name: fmt.Sprintf("helper%02d", i), Kind: domain.KindFunction,
			Properties: []string{"nodePool.AutoRepair"}, Source: domain.Source{Parser: "go-ast", File: fmt.Sprintf("controllers/nodepool/helper%02d.go", i), Line: 10, EndLine: 40}})
	}
	idx := newIndex(graph)
	req := EvidenceRequest{Question: "AutoRepair NodePool", BudgetBytes: 4096}
	seen := make(map[string]bool)
	for pages := 0; pages < 100; pages++ {
		packet, err := idx.Evidence(req)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(packet)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) > req.BudgetBytes {
			t.Fatalf("oversized packet %d > %d", len(encoded), req.BudgetBytes)
		}
		if packet.Status == "budget_exhausted" {
			t.Fatal("ordinary entity selection cannot advance at 4 KiB")
		}
		for _, e := range packet.Entities {
			seen[e.ID] = true
		}
		if packet.NextOffset == 0 {
			break
		}
		if packet.NextOffset <= req.Offset {
			t.Fatalf("continuation did not advance: %+v", packet)
		}
		req.Offset, req.GraphFingerprint = packet.NextOffset, packet.GraphFingerprint
	}
	if len(seen) < 43 {
		t.Fatalf("continuation lost entities: %d", len(seen))
	}
	if _, err := idx.Evidence(EvidenceRequest{Question: req.Question, Offset: 1}); err == nil {
		t.Fatal("unbound continuation accepted")
	}
	graph.Commit = "revision-two"
	if _, err := newIndex(graph).Evidence(req); err == nil {
		t.Fatal("continuation accepted a different snapshot")
	}
	if _, err := idx.Evidence(EvidenceRequest{Question: req.Question, BudgetBytes: 2047}); err == nil {
		t.Fatal("unsafe budget accepted")
	}
}

func TestEvidenceDoesNotTrimOversizedRelationshipProof(t *testing.T) {
	graph := evidenceTestGraph()
	graph.Relationship[0].Evidence.Snippet = strings.Repeat("proof ", 2000)
	packet, err := newIndex(graph).Evidence(EvidenceRequest{Question: "AutoRepair NodePool", BudgetBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(packet)
	if len(encoded) > 4096 {
		t.Fatal("proof bypassed byte budget")
	}
	omitted := false
	for _, o := range packet.Omissions {
		if o.Reason == "relationship_budget" {
			omitted = true
		}
	}
	if !omitted {
		t.Fatal("oversized relationship evidence was silently omitted")
	}
	for _, r := range packet.Relationships {
		if r.ID == graph.Relationship[0].ID {
			t.Fatal("oversized proof should remain retrievable through its exact endpoints, not be shortened")
		}
	}
}

func TestEvidenceGraphWarningsRemainBoundedAndReported(t *testing.T) {
	graph := evidenceTestGraph()
	for i := 0; i < 100; i++ {
		graph.ScanWarnings = append(graph.ScanWarnings, strings.Repeat("warning ", 100))
	}
	packet, err := newIndex(graph).Evidence(EvidenceRequest{Entity: graph.Entities[2].ID, BudgetBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Graph.ScanWarnings) != 8 || !packet.Truncated {
		t.Fatalf("warnings were not bounded/reported: %+v", packet)
	}
	encoded, _ := json.Marshal(packet)
	if len(encoded) > 8192 {
		t.Fatal("warning metadata bypassed budget")
	}
}

func TestSearchKindRetainsFullTextSemantics(t *testing.T) {
	idx := newIndex(evidenceTestGraph())
	page, err := idx.SearchKindPage("nodepool AutoRepair", "function", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entities) != 2 {
		t.Fatalf("kind filter changed AND/full-text search: %+v", page)
	}
	if _, err := idx.SearchKindPage("autorepair", "not_a_kind", 0, 20); err == nil {
		t.Fatal("invalid kind accepted")
	}
}

func TestAskDistinguishesMissingAndAmbiguous(t *testing.T) {
	idx := newIndex(evidenceTestGraph())
	missing := idx.Ask("missing", "understand")
	if missing.Status != "no_match" || missing.Match != "no_match" || missing.Ambiguous {
		t.Fatalf("missing entity misclassified: %+v", missing)
	}
	ambiguous := idx.Ask("AutoRepair", "understand")
	if ambiguous.Status != "ambiguous" || !ambiguous.Ambiguous {
		t.Fatalf("ambiguous entity misclassified: %+v", ambiguous)
	}
	if CompactAsk(missing).Status != "no_match" {
		t.Fatal("compact result lost status")
	}
}

func TestExplainMetadataAndDegreeCapsPreserveOtherBranches(t *testing.T) {
	first := &domain.Entity{ID: "function:pkg.a", Name: "a", Kind: domain.KindFunction, Properties: make([]string, 11)}
	second := &domain.Entity{ID: "function:pkg.b", Name: "b", Kind: domain.KindFunction}
	root := &ExplainNode{Entity: &domain.Entity{ID: "function:pkg.root"}, Children: []*ExplainNode{{Entity: first}, {Entity: second}}}
	compact := CompactExplain(&ExplainResult{Root: root, TotalNodes: 3})
	if len(compact.Root.Children) != 2 || compact.ReturnedNodes != 3 || !compact.Capped {
		t.Fatalf("metadata truncation removed siblings: %+v", compact)
	}
	graph := domain.Graph{Entities: []domain.Entity{{ID: "function:pkg.root", Name: "root", Kind: domain.KindFunction}}}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("function:pkg.call%02d", i)
		leaf := id + "Leaf"
		graph.Entities = append(graph.Entities, domain.Entity{ID: id, Name: id, Kind: domain.KindFunction}, domain.Entity{ID: leaf, Name: leaf, Kind: domain.KindFunction})
		graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(graph.Entities[0].ID, domain.RelCalls, id), From: graph.Entities[0].ID, To: id, Type: domain.RelCalls}, domain.Relationship{ID: domain.NewRelationshipID(id, domain.RelCalls, leaf), From: id, To: leaf, Type: domain.RelCalls})
	}
	full := newIndex(graph).Explain("function:pkg.root", 2)
	if len(full.Root.Children) != 10 || len(full.Root.Children[9].Children) != 1 {
		t.Fatal("one degree cap suppressed other branch expansion")
	}
	if strings.Contains(FormatExplanationCompact(full), "capped at 100") {
		t.Fatal("edge limit mislabeled as node limit")
	}
}

func TestCompactNestedTruncationPropagates(t *testing.T) {
	e := &domain.Entity{ID: "function:pkg.a", Properties: make([]string, 11)}
	view := &domain.View{Calls: make([]string, 13)}
	if !CompactAsk(&AskResult{View: view}).Truncated {
		t.Fatal("view truncation lost")
	}
	if !CompactInvestigate(&InvestigateResult{OutRels: map[domain.RelationshipType][]ResolvedRel{domain.RelCalls: {{Target: e}}}}).Truncated {
		t.Fatal("target truncation lost")
	}
	if !newIndex(domain.Graph{}).CompactEntityRelationshipPageResult(e, RelationshipPage{}).Truncated {
		t.Fatal("entity metadata truncation lost")
	}
}

func TestEvidenceKeepsScopeReferencesFromDisplacingBehavior(t *testing.T) {
	graph := evidenceTestGraph()
	impl := graph.Entities[2]
	for i := 0; i < 30; i++ {
		field := domain.Entity{ID: fmt.Sprintf("field:example.com/hypershift/api.NodePool.SpecField%02d", i), Name: fmt.Sprintf("SpecField%02d", i), Kind: domain.KindField,
			Source: domain.Source{Parser: "go-ast", File: "api/nodepool_types.go", Line: 600 + i, EndLine: 600 + i}}
		graph.Entities = append(graph.Entities, field)
		graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(impl.ID, domain.RelReferences, field.ID), From: impl.ID, To: field.ID, Type: domain.RelReferences,
			Confidence: domain.ConfidenceProven, Evidence: domain.Evidence{Parser: "go-types", File: impl.Source.File, Line: 200 + i, Snippet: "nodePool.Spec.OtherField"}})
	}
	packet, err := newIndex(graph).Evidence(EvidenceRequest{Question: "AutoRepair NodePool", Scope: "NodePool", BudgetBytes: 16384})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range packet.Entities {
		if strings.Contains(e.ID, "SpecField") {
			t.Fatalf("scope token expanded every unrelated API field: %s", e.ID)
		}
	}
	found := false
	for _, e := range packet.Entities {
		if strings.Contains(e.ID, "reconcileMachineHealthCheck") {
			found = true
		}
	}
	if !found {
		t.Fatal("unrelated field references displaced the implementation helper")
	}
}

func TestNeighborsKeepsNearestEntitiesBeforeLexicallyEarlierDistantNodes(t *testing.T) {
	root := "function:pkg.zzRoot"
	near := "function:pkg.zzNear"
	graph := domain.Graph{Entities: []domain.Entity{{ID: root, Kind: domain.KindFunction}, {ID: near, Kind: domain.KindFunction}},
		Relationship: []domain.Relationship{{ID: domain.NewRelationshipID(root, domain.RelCalls, near), From: root, To: near, Type: domain.RelCalls}}}
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("function:pkg.aaDistant%02d", i)
		graph.Entities = append(graph.Entities, domain.Entity{ID: id, Kind: domain.KindFunction})
		graph.Relationship = append(graph.Relationship, domain.Relationship{ID: domain.NewRelationshipID(near, domain.RelCalls, id), From: near, To: id, Type: domain.RelCalls})
	}
	result := newIndex(graph).Neighbors(root, 2)
	found := false
	for _, e := range result.Entities {
		if e.ID == near {
			found = true
		}
	}
	if !found || !result.Truncated {
		t.Fatal("lexical cap evicted the immediate neighbor")
	}
}

func TestCompactResourceObservationsAreBoundedWithoutLosingTruncation(t *testing.T) {
	entity := &domain.Entity{ID: "function:pkg.create", Generated: true}
	for i := 0; i < 10; i++ {
		entity.ResourceOperations = append(entity.ResourceOperations, domain.ResourceOperation{Operation: "CreateOrUpdate", ObjectType: "MachineHealthCheck", Confidence: domain.ConfidenceProven, Source: domain.Source{Parser: "go-types", File: "capi.go", Line: i + 1}})
	}
	compact := CompactEntitySummary(entity)
	if len(compact.ResourceOperations) != 8 || !compact.Generated || !compact.Truncated {
		t.Fatalf("resource observations lost or unbounded: %+v", compact)
	}
}
