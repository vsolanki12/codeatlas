package query

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func FormatEntity(e *domain.Entity) string {
	desc := e.Description
	if len(desc) > 80 {
		desc = desc[:77] + "..."
	}
	if desc == "" {
		desc = "-"
	}
	return fmt.Sprintf("%s | %s:%d | %s", e.ID, e.Source.File, e.Source.Line, desc)
}

func FormatEntityFull(e *domain.Entity) string {
	var b strings.Builder
	truncated := false
	fmt.Fprintf(&b, "ID: %s\n", e.ID)
	fmt.Fprintf(&b, "Name: %s\n", e.Name)
	fmt.Fprintf(&b, "Kind: %s\n", e.Kind)
	fmt.Fprintf(&b, "Package: %s\n", e.Package)
	fmt.Fprintf(&b, "File: %s:%d\n", e.Source.File, e.Source.Line)
	if e.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", e.Description)
	}
	if e.Kind == domain.KindDocument && e.Content != "" {
		content := e.Content
		if len(content) > 600 {
			content = content[:597] + "..."
			truncated = true
		}
		fmt.Fprintf(&b, "Content: %s\n", content)
	}
	truncated = formatBoundedStrings(&b, "Watches", e.Watches, 12) || truncated
	truncated = formatBoundedStrings(&b, "Creates", e.Creates, 12) || truncated
	truncated = formatBoundedStrings(&b, "Calls", e.Calls, 12) || truncated
	truncated = formatBoundedStrings(&b, "Implements", e.Implements, 8) || truncated
	truncated = formatBoundedStrings(&b, "EnvVars", e.EnvVars, 12) || truncated
	truncated = formatBoundedStrings(&b, "Imports", e.Imports, 12) || truncated
	truncated = formatBoundedStrings(&b, "Literals", e.Literals, 12) || truncated
	truncated = formatBoundedStrings(&b, "Properties", e.Properties, 16) || truncated
	truncated = formatBoundedStrings(&b, "Embeds", e.Embeds, 8) || truncated
	truncated = formatBoundedStrings(&b, "Files", e.Files, 20) || truncated
	if e.LastAuthor != "" {
		fmt.Fprintf(&b, "LastAuthor: %s\n", e.LastAuthor)
	}
	if e.LastModified != "" {
		fmt.Fprintf(&b, "LastModified: %s\n", e.LastModified)
	}
	if e.ChangeCount > 0 {
		fmt.Fprintf(&b, "ChangeCount: %d\n", e.ChangeCount)
	}
	if truncated {
		b.WriteString("[TRUNCATED: one or more entity lists were capped; omitted entries are not evidence of absence.]\n")
	}
	return b.String()
}

func formatBoundedStrings(b *strings.Builder, label string, values []string, max int) bool {
	if len(values) == 0 {
		return false
	}
	shown := values
	truncated := false
	if max > 0 && len(values) > max {
		shown = values[:max]
		truncated = true
	}
	if truncated {
		shown = append(append([]string(nil), shown...), fmt.Sprintf("...+%d more", len(values)-max))
	}
	fmt.Fprintf(b, "%s: %s\n", label, strings.Join(shown, ", "))
	return truncated
}

func FormatEntityDetailList(entities []*domain.Entity) string {
	if len(entities) == 0 {
		return "No matching entities.\n"
	}
	var b strings.Builder
	for _, e := range entities {
		b.WriteString(FormatEntityFull(e))
		b.WriteByte('\n')
	}
	return b.String()
}

func FormatRelationship(r *domain.Relationship) string {
	file := filepath.Base(r.Evidence.File)
	result := fmt.Sprintf("%s --%s--> %s | %s | %s:%d",
		r.From, r.Type, r.To, r.Confidence, file, r.Evidence.Line)
	if r.Evidence.Reason != "" {
		result += " | evidence: " + r.Evidence.Reason
	}
	return result
}

func FormatEntityList(entities []*domain.Entity) string {
	if len(entities) == 0 {
		return "No matching entities.\n"
	}
	var b strings.Builder
	for _, e := range entities {
		b.WriteString(FormatEntity(e))
		b.WriteByte('\n')
	}
	return b.String()
}

func FormatRelationshipList(rels []*domain.Relationship) string {
	if len(rels) == 0 {
		return "No relationships.\n"
	}
	var b strings.Builder
	for _, r := range rels {
		b.WriteString("  ")
		b.WriteString(FormatRelationship(r))
		b.WriteByte('\n')
	}
	return b.String()
}

func FormatSubgraph(sg *Subgraph) string {
	if len(sg.Entities) == 0 {
		return "Empty subgraph.\n"
	}

	outgoing := make(map[string][]*domain.Relationship)
	incoming := make(map[string][]*domain.Relationship)
	for _, r := range sg.Relationships {
		outgoing[r.From] = append(outgoing[r.From], r)
		incoming[r.To] = append(incoming[r.To], r)
	}

	var b strings.Builder
	for _, e := range sg.Entities {
		fmt.Fprintf(&b, "%s | %s:%d\n", e.ID, e.Source.File, e.Source.Line)
		if e.Description != "" {
			desc := e.Description
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			fmt.Fprintf(&b, "  %s\n", desc)
		}
		for _, r := range outgoing[e.ID] {
			fmt.Fprintf(&b, "  --%s--> %s\n", r.Type, r.To)
		}
		for _, r := range incoming[e.ID] {
			fmt.Fprintf(&b, "  <--%s-- %s\n", r.Type, r.From)
		}
	}
	if sg.Truncated {
		b.WriteString("[TRUNCATED: result limit reached; request narrower context or depth.]")
		b.WriteByte('\n')
	}
	return b.String()
}

func FormatStats(s *GraphStats) string {
	var b strings.Builder
	if s.SchemaVersion != "" {
		fmt.Fprintf(&b, "schema: codeatlas/%s\n", s.SchemaVersion)
	}
	if s.EntityIdentity != "" {
		fmt.Fprintf(&b, "entity identity: %s\n", s.EntityIdentity)
	}
	if s.Repository != "" {
		fmt.Fprintf(&b, "repository: %s\n", s.Repository)
	}
	if s.Commit != "" {
		fmt.Fprintf(&b, "commit: %s\n", s.Commit)
	}
	if s.Branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", s.Branch)
	}
	if s.GeneratedAt != "" {
		fmt.Fprintf(&b, "generated: %s\n", s.GeneratedAt)
	}
	if s.ScanComplete {
		b.WriteString("scan: complete\n")
	} else {
		b.WriteString("scan: incomplete or legacy graph\n")
	}
	for _, warning := range s.ScanWarnings {
		fmt.Fprintf(&b, "scan warning: %s\n", warning)
	}
	fmt.Fprintf(&b, "entities: %d\n", s.TotalEntities)

	eKeys := make([]string, 0, len(s.EntityCounts))
	for k := range s.EntityCounts {
		eKeys = append(eKeys, k)
	}
	sort.Strings(eKeys)
	for _, k := range eKeys {
		fmt.Fprintf(&b, "  %s: %d\n", k, s.EntityCounts[k])
	}

	fmt.Fprintf(&b, "relationships: %d\n", s.TotalRels)

	rKeys := make([]string, 0, len(s.RelCounts))
	for k := range s.RelCounts {
		rKeys = append(rKeys, k)
	}
	sort.Strings(rKeys)
	for _, k := range rKeys {
		fmt.Fprintf(&b, "  %s: %d\n", k, s.RelCounts[k])
	}

	return b.String()
}

func FormatAsk(r *AskResult) string {
	var b strings.Builder
	if r.Graph.SchemaVersion != "" {
		fmt.Fprintf(&b, "Graph: codeatlas/%s | commit=%s | scan=%s\n", r.Graph.SchemaVersion, r.Graph.Commit, graphScanStatus(r.Graph.ScanComplete))
		if r.Match != "" && r.Match != "id" {
			fmt.Fprintf(&b, "Entity match: %s\n", r.Match)
		}
		if len(r.Graph.ScanWarnings) > 0 {
			fmt.Fprintf(&b, "Graph warnings: %d\n", len(r.Graph.ScanWarnings))
		}
		b.WriteByte('\n')
	}
	if r.Entity == nil {
		if r.Ambiguous && len(r.Candidates) > 0 {
			b.WriteString("Ambiguous entity; use an exact ID:\n")
			for _, candidate := range r.Candidates {
				fmt.Fprintf(&b, "- %s | %s:%d\n", candidate.ID, candidate.Source.File, candidate.Source.Line)
			}
		} else {
			b.WriteString("No matching entity.\n")
		}
		return b.String()
	}

	hasView := r.View != nil
	if hasView {
		if r.Detail {
			b.WriteString(FormatView(r.View))
		} else {
			b.WriteString(FormatViewCompact(r.View))
		}
	} else {
		if r.Detail {
			b.WriteString(FormatEntityFull(r.Entity))
		} else {
			b.WriteString(FormatEntity(r.Entity))
		}
	}

	if r.QAHit != "" {
		fmt.Fprintf(&b, "\n--- Quick Answer ---\n%s\n", r.QAHit)
		if len(r.QAEvidence) > 0 {
			b.WriteString("Quick-answer evidence:\n")
			b.WriteString(FormatRelationshipList(r.QAEvidence))
		}
	}

	if r.Explanation != nil {
		b.WriteString("\n--- Execution Flow ---\n")
		if hasView && !r.Detail {
			b.WriteString(FormatExplanationCompact(r.Explanation))
		} else {
			b.WriteString(FormatExplanation(r.Explanation))
		}
	}
	if r.Impact != nil {
		b.WriteString("\n--- Blast Radius ---\n")
		if r.Detail {
			b.WriteString(FormatImpact(r.Impact))
		} else {
			b.WriteString(FormatImpactCompact(r.Impact))
		}
	}
	if r.Investigation != nil {
		b.WriteString("\n--- Investigation ---\n")
		if r.Detail {
			b.WriteString(FormatInvestigation(r.Investigation))
		} else {
			b.WriteString(FormatInvestigationCompact(r.Investigation))
		}
	}

	return b.String()
}

func graphScanStatus(complete bool) string {
	if complete {
		return "complete"
	}
	return "incomplete or legacy"
}

func FormatView(v *domain.View) string {
	var b strings.Builder
	truncated := false
	fmt.Fprintf(&b, "=== %s (%s) ===\n", v.EntityName, v.Kind)
	fmt.Fprintf(&b, "ID: %s\n", v.EntityID)
	fmt.Fprintf(&b, "File: %s\n", v.File)
	if v.Package != "" {
		fmt.Fprintf(&b, "Package: %s\n", v.Package)
	}
	if v.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", v.Description)
	}

	if v.Reconciles != "" || len(v.Creates) > 0 || len(v.Watches) > 0 {
		b.WriteString("\n--- Manages ---\n")
		if v.Reconciles != "" {
			fmt.Fprintf(&b, "Reconciles: %s\n", v.Reconciles)
		}
		truncated = formatBoundedStrings(&b, "Creates", v.Creates, 20) || truncated
		truncated = formatBoundedStrings(&b, "Watches", v.Watches, 20) || truncated
	}

	if v.ReconciledBy != "" || len(v.CreatedBy) > 0 || len(v.CalledBy) > 0 {
		b.WriteString("\n--- Managed By ---\n")
		if v.ReconciledBy != "" {
			fmt.Fprintf(&b, "Reconciled by: %s\n", v.ReconciledBy)
		}
		truncated = formatBoundedStrings(&b, "Created by", v.CreatedBy, 20) || truncated
		truncated = formatBoundedStrings(&b, "Called by", v.CalledBy, 20) || truncated
	}

	if len(v.Calls) > 0 {
		b.WriteString("\n--- Calls (" + fmt.Sprint(len(v.Calls)) + ") ---\n")
		truncated = formatBoundedStrings(&b, "Calls", v.Calls, 20) || truncated
	}

	if len(v.Relationships) > 0 {
		b.WriteString("\n--- Relationship Evidence ---\n")
		relationships := v.Relationships
		if len(relationships) > 40 {
			relationships = relationships[:40]
			truncated = true
		}
		for _, rel := range relationships {
			fmt.Fprintf(&b, "%s %s %s [%s] %s:%d | %s\n",
				rel.Direction, rel.Type, rel.EntityID, rel.Confidence,
				rel.Evidence.File, rel.Evidence.Line, rel.Evidence.Reason)
		}
	}

	fmt.Fprintf(&b, "\n--- Tests (%d) ---\n", v.TestCount)
	if v.TestCount > 0 {
		truncated = formatBoundedStrings(&b, "Tests", v.Tests, 20) || truncated
	} else {
		b.WriteString("(none)\n")
	}

	if len(v.Files) > 0 {
		fmt.Fprintf(&b, "\n--- Files (%d) ---\n", len(v.Files))
		files := v.Files
		if len(files) > 20 {
			files = files[:20]
			truncated = true
		}
		for _, f := range files {
			b.WriteString(f)
			b.WriteByte('\n')
		}
	}

	if len(v.Owners) > 0 || v.ChangeCount > 0 {
		b.WriteString("\n--- Ownership ---\n")
		if len(v.Owners) > 0 {
			truncated = formatBoundedStrings(&b, "Owners", v.Owners, 20) || truncated
		}
		if v.ChangeCount > 0 {
			fmt.Fprintf(&b, "Changes: %d\n", v.ChangeCount)
		}
		if v.LastModified != "" {
			fmt.Fprintf(&b, "Last modified: %s by %s\n", v.LastModified, v.LastAuthor)
		}
	}
	if truncated {
		b.WriteString("\n[TRUNCATED: one or more view lists were capped; omitted entries are not evidence of absence.]\n")
	}

	return b.String()
}

// FormatViewCompact renders the pre-computed view without repeating every
// field. The structured query result remains the authority for complete
// fields and relationship evidence; this representation is the default text
// form for token-sensitive consumers.
func FormatViewCompact(v *domain.View) string {
	if v == nil {
		return "No view found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) | %s\n", v.EntityName, v.Kind, v.File)
	fmt.Fprintf(&b, "ID: %s\n", v.EntityID)
	if v.Package != "" {
		fmt.Fprintf(&b, "Package: %s\n", v.Package)
	}
	if v.Description != "" {
		desc := v.Description
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		fmt.Fprintf(&b, "Description: %s\n", desc)
	}
	if v.Reconciles != "" {
		fmt.Fprintf(&b, "Reconciles: %s\n", v.Reconciles)
	}
	formatBoundedStrings(&b, "Creates", v.Creates, 8)
	formatBoundedStrings(&b, "Watches", v.Watches, 8)
	formatBoundedStrings(&b, "Calls", v.Calls, 8)
	if v.ReconciledBy != "" {
		formatBoundedStrings(&b, "Reconciled by", []string{v.ReconciledBy}, 1)
	}
	formatBoundedStrings(&b, "Created by", v.CreatedBy, 8)
	formatBoundedStrings(&b, "Called by", v.CalledBy, 8)
	formatBoundedStrings(&b, "Tests", v.Tests, 12)
	if v.TestCount > len(v.Tests) {
		fmt.Fprintf(&b, "Test count: %d\n", v.TestCount)
	}
	formatBoundedStrings(&b, "Files", v.Files, 8)
	formatBoundedStrings(&b, "Owners", v.Owners, 8)
	if v.ChangeCount > 0 {
		fmt.Fprintf(&b, "Changes: %d\n", v.ChangeCount)
	}
	if v.LastModified != "" {
		fmt.Fprintf(&b, "Last modified: %s by %s\n", v.LastModified, v.LastAuthor)
	}
	if len(v.Relationships) > 0 {
		fmt.Fprintf(&b, "Relationship evidence: %d entries (see structured output)\n", len(v.Relationships))
	}
	return b.String()
}

var relDisplayOrder = []domain.RelationshipType{
	domain.RelReconciles, domain.RelCreates, domain.RelCalls,
	domain.RelWatches, domain.RelTestedBy, domain.RelOwns,
	domain.RelDocumentedIn, domain.RelDependsOn, domain.RelImports,
	domain.RelImplements, domain.RelEmits, domain.RelContains,
	domain.RelPartOf, domain.RelEmbeds,
}

func FormatInvestigation(r *InvestigateResult) string {
	var b strings.Builder

	b.WriteString("=== Entity ===\n")
	b.WriteString(FormatEntityFull(r.Entity))

	outCount := 0
	for _, rr := range r.OutRels {
		outCount += len(rr)
	}
	inCount := 0
	for _, rr := range r.InRels {
		inCount += len(rr)
	}
	fmt.Fprintf(&b, "\n=== Relationships (%d outgoing, %d incoming) ===\n", outCount, inCount)

	writeRels := func(rels map[domain.RelationshipType][]ResolvedRel, arrow string) {
		for _, rt := range orderedRelationshipTypes(rels) {
			rs := rels[rt]
			dir := "outgoing"
			if arrow == "<-" {
				dir = "incoming"
			}
			fmt.Fprintf(&b, "%s (%d %s):\n", rt, len(rs), dir)
			for _, rr := range rs {
				desc := rr.Target.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				if desc != "" {
					fmt.Fprintf(&b, "  %s %s | %s:%d | %s\n", arrow, rr.Target.ID, rr.Target.Source.File, rr.Target.Source.Line, desc)
				} else {
					fmt.Fprintf(&b, "  %s %s | %s:%d\n", arrow, rr.Target.ID, rr.Target.Source.File, rr.Target.Source.Line)
				}
				if rr.Rel != nil {
					fmt.Fprintf(&b, "    evidence: %s | %s:%d | %s\n", rr.Rel.Confidence, rr.Rel.Evidence.File, rr.Rel.Evidence.Line, rr.Rel.Evidence.Reason)
				}
			}
		}
	}

	writeRels(r.OutRels, "->")
	writeRels(r.InRels, "<-")

	fmt.Fprintf(&b, "\n=== Callers (%d) ===\n", len(r.Callers))
	if len(r.Callers) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, c := range r.Callers {
			b.WriteString(FormatEntity(c))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Tests (%d) ===\n", len(r.Tests))
	if len(r.Tests) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, t := range r.Tests {
			b.WriteString(FormatEntity(t))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Same File (%d others) ===\n", len(r.Siblings))
	if len(r.Siblings) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, s := range r.Siblings {
			b.WriteString(FormatEntity(s))
			b.WriteByte('\n')
		}
	}
	if r.Truncated {
		b.WriteString("\n[TRUNCATED: one or more relationship or sibling lists reached their limit.]\n")
	}

	return b.String()
}

// FormatInvestigationCompact is the bounded text counterpart to
// FormatInvestigation. It keeps identity, relationship direction, confidence,
// and evidence locations while leaving complete evidence payloads to the
// structured result.
func FormatInvestigationCompact(r *InvestigateResult) string {
	if r == nil || r.Entity == nil {
		return "Entity not found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Entity: %s\n", FormatEntity(r.Entity))

	writeRels := func(rels map[domain.RelationshipType][]ResolvedRel, arrow string) {
		for _, rt := range orderedRelationshipTypes(rels) {
			values := rels[rt]
			limit := len(values)
			if limit > 8 {
				limit = 8
			}
			fmt.Fprintf(&b, "%s %s (%d):\n", arrow, rt, len(values))
			for _, rr := range values[:limit] {
				if rr.Target == nil {
					continue
				}
				if rr.Rel == nil {
					fmt.Fprintf(&b, "  %s %s:%d\n", rr.Target.ID, rr.Target.Source.File, rr.Target.Source.Line)
					continue
				}
				fmt.Fprintf(&b, "  %s %s | %s:%d | %s | evidence: %s:%d\n",

					arrow, rr.Target.ID, rr.Target.Source.File, rr.Target.Source.Line,
					rr.Rel.Confidence, rr.Rel.Evidence.File, rr.Rel.Evidence.Line)
			}
			if limit < len(values) {
				fmt.Fprintf(&b, "  ...+%d more (not evidence of absence)\n", len(values)-limit)
			}
		}
	}

	writeRels(r.OutRels, "->")
	writeRels(r.InRels, "<-")
	formatCompactEntityList := func(label string, entities []*domain.Entity, max int) {
		fmt.Fprintf(&b, "%s (%d):\n", label, len(entities))
		limit := len(entities)
		if limit > max {
			limit = max
		}
		for _, entity := range entities[:limit] {
			b.WriteString("  ")
			b.WriteString(FormatEntity(entity))
			b.WriteByte('\n')
		}
		if limit < len(entities) {
			fmt.Fprintf(&b, "  ...+%d more (not evidence of absence)\n", len(entities)-limit)
		}
	}
	formatCompactEntityList("Callers", r.Callers, 12)
	formatCompactEntityList("Tests", r.Tests, 12)
	formatCompactEntityList("Same file", r.Siblings, 12)
	if r.Truncated {
		b.WriteString("[TRUNCATED: one or more relationship or entity lists reached their limit.]\n")
	}
	return b.String()
}

func orderedRelationshipTypes(rels map[domain.RelationshipType][]ResolvedRel) []domain.RelationshipType {
	seen := make(map[domain.RelationshipType]bool, len(rels))
	ordered := make([]domain.RelationshipType, 0, len(rels))
	for _, rt := range relDisplayOrder {
		if len(rels[rt]) == 0 {
			continue
		}
		seen[rt] = true
		ordered = append(ordered, rt)
	}
	var extras []domain.RelationshipType
	for rt, values := range rels {
		if len(values) > 0 && !seen[rt] {
			extras = append(extras, rt)
		}
	}
	sort.Slice(extras, func(i, j int) bool { return extras[i] < extras[j] })
	return append(ordered, extras...)
}

func FormatExplanation(r *ExplainResult) string {
	if r.Root == nil {
		return "Entity not found.\n"
	}
	var b strings.Builder
	formatExplainNode(&b, r.Root, 0)
	footer := fmt.Sprintf("%d nodes explored", r.TotalNodes)
	if r.Capped {
		footer += " (capped at 100 nodes)"
	}
	b.WriteString(footer)
	b.WriteByte('\n')
	return b.String()
}

func FormatExplanationCompact(r *ExplainResult) string {
	if r.Root == nil {
		return "Entity not found.\n"
	}
	var b strings.Builder
	grouped := make(map[domain.RelationshipType][]*ExplainNode)
	for _, child := range r.Root.Children {
		grouped[child.EdgeType] = append(grouped[child.EdgeType], child)
	}
	for _, edgeType := range explainEdgeOrder {
		children, ok := grouped[edgeType]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", edgeType)
		for _, child := range children {
			formatCompactNode(&b, child, 1)
		}
	}
	footer := fmt.Sprintf("%d nodes explored", r.TotalNodes)
	if r.Capped {
		footer += " (capped at 100 nodes)"
	}
	b.WriteString(footer)
	b.WriteByte('\n')
	return b.String()
}

func formatCompactNode(b *strings.Builder, node *ExplainNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	e := node.Entity
	name := shortName(e.ID)
	file := filepath.Base(e.Source.File)
	desc := e.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	if desc != "" {
		fmt.Fprintf(b, "%s%s | %s:%d | %s\n", prefix, name, file, e.Source.Line, desc)
	} else {
		fmt.Fprintf(b, "%s%s | %s:%d\n", prefix, name, file, e.Source.Line)
	}
	if node.Relationship != nil {
		fmt.Fprintf(b, "%s  evidence: %s | %s:%d\n", prefix, node.Relationship.Confidence, node.Relationship.Evidence.File, node.Relationship.Evidence.Line)
	}
	grouped := make(map[domain.RelationshipType][]*ExplainNode)
	for _, child := range node.Children {
		grouped[child.EdgeType] = append(grouped[child.EdgeType], child)
	}
	for _, edgeType := range explainEdgeOrder {
		children, ok := grouped[edgeType]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "%s  %s:\n", prefix, edgeType)
		for _, child := range children {
			formatCompactNode(b, child, indent+2)
		}
	}
}

func shortName(id string) string {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		return id
	}
	qualified := parts[1]
	if dot := strings.LastIndex(qualified, "."); dot >= 0 {
		return qualified[dot+1:]
	}
	return qualified
}

func formatExplainNode(b *strings.Builder, node *ExplainNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	e := node.Entity

	if indent == 0 {
		fmt.Fprintf(b, "%s | %s:%d\n", e.ID, e.Source.File, e.Source.Line)
		if e.Description != "" {
			fmt.Fprintf(b, "  %s\n", e.Description)
		}
	} else {
		desc := e.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		if desc != "" {
			fmt.Fprintf(b, "%s%s | %s:%d | %s\n", prefix, e.ID, e.Source.File, e.Source.Line, desc)
		} else {
			fmt.Fprintf(b, "%s%s | %s:%d\n", prefix, e.ID, e.Source.File, e.Source.Line)
		}
	}
	if node.Relationship != nil {
		fmt.Fprintf(b, "%s  evidence: %s | %s:%d | %s\n", prefix, node.Relationship.Confidence, node.Relationship.Evidence.File, node.Relationship.Evidence.Line, node.Relationship.Evidence.Reason)
	}

	grouped := make(map[domain.RelationshipType][]*ExplainNode)
	for _, child := range node.Children {
		grouped[child.EdgeType] = append(grouped[child.EdgeType], child)
	}
	for _, edgeType := range explainEdgeOrder {
		children, ok := grouped[edgeType]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "%s%s:\n", prefix, edgeType)
		for _, child := range children {
			formatExplainNode(b, child, indent+1)
		}
	}
}

func FormatImpact(r *ImpactResult) string {
	var b strings.Builder

	desc := r.Entity.Description
	if desc == "" {
		desc = "-"
	}
	fmt.Fprintf(&b, "=== Impact: %s ===\n%s:%d | %s\n", r.Entity.ID, r.Entity.Source.File, r.Entity.Source.Line, desc)

	fmt.Fprintf(&b, "\n=== Call Chain (%d callers) ===\n", len(r.CallChain))
	if len(r.CallChain) == 0 {
		b.WriteString("(none — this is a top-level entry point)\n")
	} else {
		for _, c := range r.CallChain {
			b.WriteString(FormatEntity(c))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Controllers (%d) ===\n", len(r.Controllers))
	if len(r.Controllers) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, c := range r.Controllers {
			b.WriteString(FormatEntity(c))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Tests (%d) ===\n", len(r.Tests))
	if len(r.Tests) == 0 {
		b.WriteString("(none — no test coverage in call chain)\n")
	} else {
		for _, t := range r.Tests {
			b.WriteString(FormatEntity(t))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Resources (%d) ===\n", len(r.Resources))
	if len(r.Resources) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, r := range r.Resources {
			b.WriteString(FormatEntity(r))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Relationship Evidence (%d) ===\n", len(r.Relationships))
	if len(r.Relationships) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, relationship := range r.Relationships {
			b.WriteString(FormatRelationship(relationship))
			b.WriteByte('\n')
		}
	}

	fmt.Fprintf(&b, "\n=== Files Affected (%d) ===\n", len(r.Files))
	for _, f := range r.Files {
		b.WriteString(f)
		b.WriteByte('\n')
	}

	if len(r.RecentChanges) > 0 {
		fmt.Fprintf(&b, "\n=== Recent Changes (%d) ===\n", len(r.RecentChanges))
		for _, e := range r.RecentChanges {
			fmt.Fprintf(&b, "%s | changes=%d last=%s by=%s\n", e.ID, e.ChangeCount, e.LastModified, e.LastAuthor)
		}
	}

	if len(r.Owners) > 0 {
		fmt.Fprintf(&b, "\n=== Owners (%d) ===\n", len(r.Owners))
		for _, o := range r.Owners {
			b.WriteString(o)
			b.WriteByte('\n')
		}
	}
	if r.Truncated {
		b.WriteString("\n[TRUNCATED: impact traversal or result list reached its limit.]\n")
	}

	return b.String()
}

func FormatImpactCompact(r *ImpactResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s | %s:%d\n", shortName(r.Entity.ID), filepath.Base(r.Entity.Source.File), r.Entity.Source.Line)

	if len(r.CallChain) > 0 {
		fmt.Fprintf(&b, "\nCallers (%d): ", len(r.CallChain))
		names := make([]string, len(r.CallChain))
		for i, c := range r.CallChain {
			names[i] = shortName(c.ID)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	}

	if len(r.Controllers) > 0 {
		fmt.Fprintf(&b, "Controllers (%d): ", len(r.Controllers))
		names := make([]string, len(r.Controllers))
		for i, c := range r.Controllers {
			names[i] = shortName(c.ID)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	}

	if len(r.Tests) > 0 {
		fmt.Fprintf(&b, "Tests (%d): ", len(r.Tests))
		names := make([]string, len(r.Tests))
		for i, t := range r.Tests {
			names[i] = shortName(t.ID)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	} else {
		b.WriteString("Tests: none\n")
	}

	if len(r.Resources) > 0 {
		fmt.Fprintf(&b, "Resources (%d): ", len(r.Resources))
		names := make([]string, len(r.Resources))
		for i, r := range r.Resources {
			names[i] = shortName(r.ID)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	}

	if len(r.Relationships) > 0 {
		limit := len(r.Relationships)
		if limit > 20 {
			limit = 20
		}
		fmt.Fprintf(&b, "Relationships (%d):\n", len(r.Relationships))
		for _, relationship := range r.Relationships[:limit] {
			fmt.Fprintf(&b, "  %s\n", FormatRelationship(relationship))
		}
		if limit < len(r.Relationships) {
			b.WriteString("  ... relationship evidence capped ...\n")
		}
	}

	if len(r.Files) > 0 {
		fmt.Fprintf(&b, "Files (%d): ", len(r.Files))
		short := make([]string, len(r.Files))
		for i, f := range r.Files {
			short[i] = filepath.Base(f)
		}
		b.WriteString(strings.Join(short, ", "))
		b.WriteByte('\n')
	}

	if len(r.RecentChanges) > 0 {
		fmt.Fprintf(&b, "Recent changes: %d\n", len(r.RecentChanges))
	}

	if len(r.Owners) > 0 {
		fmt.Fprintf(&b, "Owners: %s\n", strings.Join(r.Owners, ", "))
	}
	if r.Truncated {
		b.WriteString("[TRUNCATED: impact traversal or result list reached its limit.]\n")
	}

	return b.String()
}
