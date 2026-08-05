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
	fmt.Fprintf(&b, "ID: %s\n", e.ID)
	fmt.Fprintf(&b, "Name: %s\n", e.Name)
	fmt.Fprintf(&b, "Kind: %s\n", e.Kind)
	fmt.Fprintf(&b, "Package: %s\n", e.Package)
	fmt.Fprintf(&b, "File: %s:%d\n", e.Source.File, e.Source.Line)
	if e.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", e.Description)
	}
	if len(e.Watches) > 0 {
		fmt.Fprintf(&b, "Watches: %s\n", strings.Join(e.Watches, ", "))
	}
	if len(e.Calls) > 0 {
		calls := e.Calls
		if len(calls) > 10 {
			calls = append(calls[:10], fmt.Sprintf("...+%d more", len(e.Calls)-10))
		}
		fmt.Fprintf(&b, "Calls: %s\n", strings.Join(calls, ", "))
	}
	if len(e.Implements) > 0 {
		fmt.Fprintf(&b, "Implements: %s\n", strings.Join(e.Implements, ", "))
	}
	if len(e.EnvVars) > 0 {
		fmt.Fprintf(&b, "EnvVars: %s\n", strings.Join(e.EnvVars, ", "))
	}
	if len(e.Imports) > 0 {
		imports := e.Imports
		if len(imports) > 10 {
			imports = append(imports[:10], fmt.Sprintf("...+%d more", len(e.Imports)-10))
		}
		fmt.Fprintf(&b, "Imports: %s\n", strings.Join(imports, ", "))
	}
	if len(e.Literals) > 0 {
		lits := e.Literals
		if len(lits) > 10 {
			lits = append(lits[:10], fmt.Sprintf("...+%d more", len(e.Literals)-10))
		}
		fmt.Fprintf(&b, "Literals: %s\n", strings.Join(lits, ", "))
	}
	if len(e.Properties) > 0 {
		props := e.Properties
		if len(props) > 15 {
			props = append(props[:15], fmt.Sprintf("...+%d more", len(e.Properties)-15))
		}
		fmt.Fprintf(&b, "Properties: %s\n", strings.Join(props, ", "))
	}
	if len(e.Embeds) > 0 {
		fmt.Fprintf(&b, "Embeds: %s\n", strings.Join(e.Embeds, ", "))
	}
	if len(e.Files) > 0 {
		fmt.Fprintf(&b, "Files: %s\n", strings.Join(e.Files, ", "))
	}
	if e.LastAuthor != "" {
		fmt.Fprintf(&b, "LastAuthor: %s\n", e.LastAuthor)
	}
	if e.LastModified != "" {
		fmt.Fprintf(&b, "LastModified: %s\n", e.LastModified)
	}
	if e.ChangeCount > 0 {
		fmt.Fprintf(&b, "ChangeCount: %d\n", e.ChangeCount)
	}
	return b.String()
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
	return fmt.Sprintf("%s --%s--> %s | %s | %s:%d",
		r.From, r.Type, r.To, r.Confidence, file, r.Evidence.Line)
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
	return b.String()
}

func FormatStats(s *GraphStats) string {
	var b strings.Builder
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

	hasView := r.View != nil
	if hasView {
		b.WriteString(FormatView(r.View))
	} else {
		b.WriteString(FormatEntityFull(r.Entity))
	}

	if r.QAHit != "" {
		fmt.Fprintf(&b, "\n--- Quick Answer ---\n%s\n", r.QAHit)
	}

	if r.Explanation != nil {
		b.WriteString("\n--- Execution Flow ---\n")
		if hasView {
			b.WriteString(FormatExplanationCompact(r.Explanation))
		} else {
			b.WriteString(FormatExplanation(r.Explanation))
		}
	}
	if r.Impact != nil {
		b.WriteString("\n--- Blast Radius ---\n")
		b.WriteString(FormatImpactCompact(r.Impact))
	}
	if r.Investigation != nil {
		b.WriteString("\n--- Full Investigation ---\n")
		b.WriteString(FormatInvestigation(r.Investigation))
	}

	return b.String()
}

func FormatView(v *domain.View) string {
	var b strings.Builder
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
		if len(v.Creates) > 0 {
			fmt.Fprintf(&b, "Creates: %s\n", strings.Join(v.Creates, ", "))
		}
		if len(v.Watches) > 0 {
			fmt.Fprintf(&b, "Watches: %s\n", strings.Join(v.Watches, ", "))
		}
	}

	if v.ReconciledBy != "" || len(v.CreatedBy) > 0 || len(v.CalledBy) > 0 {
		b.WriteString("\n--- Managed By ---\n")
		if v.ReconciledBy != "" {
			fmt.Fprintf(&b, "Reconciled by: %s\n", v.ReconciledBy)
		}
		if len(v.CreatedBy) > 0 {
			fmt.Fprintf(&b, "Created by: %s\n", strings.Join(v.CreatedBy, ", "))
		}
		if len(v.CalledBy) > 0 {
			fmt.Fprintf(&b, "Called by: %s\n", strings.Join(v.CalledBy, ", "))
		}
	}

	if len(v.Calls) > 0 {
		fmt.Fprintf(&b, "\n--- Calls (%d) ---\n%s\n", len(v.Calls), strings.Join(v.Calls, ", "))
	}

	fmt.Fprintf(&b, "\n--- Tests (%d) ---\n", v.TestCount)
	if v.TestCount > 0 {
		fmt.Fprintf(&b, "%s\n", strings.Join(v.Tests, ", "))
	} else {
		b.WriteString("(none)\n")
	}

	if len(v.Files) > 0 {
		fmt.Fprintf(&b, "\n--- Files (%d) ---\n", len(v.Files))
		for _, f := range v.Files {
			b.WriteString(f)
			b.WriteByte('\n')
		}
	}

	if len(v.Owners) > 0 || v.ChangeCount > 0 {
		b.WriteString("\n--- Ownership ---\n")
		if len(v.Owners) > 0 {
			fmt.Fprintf(&b, "Owners: %s\n", strings.Join(v.Owners, ", "))
		}
		if v.ChangeCount > 0 {
			fmt.Fprintf(&b, "Changes: %d\n", v.ChangeCount)
		}
		if v.LastModified != "" {
			fmt.Fprintf(&b, "Last modified: %s by %s\n", v.LastModified, v.LastAuthor)
		}
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
		seen := make(map[domain.RelationshipType]bool)
		for _, rt := range relDisplayOrder {
			rs, ok := rels[rt]
			if !ok || len(rs) == 0 {
				continue
			}
			seen[rt] = true
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
			}
		}
		for rt, rs := range rels {
			if !seen[rt] && len(rs) > 0 {
				dir := "outgoing"
				if arrow == "<-" {
					dir = "incoming"
				}
				fmt.Fprintf(&b, "%s (%d %s):\n", rt, len(rs), dir)
				for _, rr := range rs {
					fmt.Fprintf(&b, "  %s %s | %s:%d\n", arrow, rr.Target.ID, rr.Target.Source.File, rr.Target.Source.Line)
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

	return b.String()
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
	fmt.Fprintf(b, "%s%s | %s:%d\n", prefix, name, file, e.Source.Line)
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

	return b.String()
}
