package review

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

var noisePackages = map[string]bool{
	"fmt": true, "errors": true, "strings": true, "log": true,
	"context": true, "os": true, "io": true, "bytes": true,
	"strconv": true, "sort": true, "sync": true, "time": true,
	"reflect": true, "math": true, "regexp": true,
}

var noiseMethods = map[string]bool{
	"DeepCopy": true, "DeepCopyObject": true, "DeepCopyInto": true,
	"Patch": true, "Status": true,
	"Get": true, "List": true, "Create": true, "Update": true, "Delete": true,
	"MergeFrom": true, "StrategicMergeFrom": true,
	"String": true, "Error": true,
}

func FormatReview(r *ReviewResult) string {
	var b strings.Builder

	b.WriteString("CODEATLAS PR REVIEW\n")
	b.WriteString("===================\n\n")

	fmt.Fprintf(&b, "Base: %s | Head: %s\n\n", r.Base, r.Head)
	if r.Graph.Commit != "" {
		fmt.Fprintf(&b, "Graph: %s (%s, %s)\n", r.Graph.Commit, r.GraphFreshness, scanStatus(r.Graph.ScanComplete))
	} else {
		fmt.Fprintf(&b, "Graph: commit unavailable (%s, %s)\n", r.GraphFreshness, scanStatus(r.Graph.ScanComplete))
	}
	b.WriteByte('\n')

	if r.PR != nil {
		b.WriteString("Pull request metadata (GitHub)\n")
		b.WriteString("------------------------------\n")
		fmt.Fprintf(&b, "Repository: %s | Number: #%d\n", r.PR.Repository, r.PR.Number)
		if r.PR.Title != "" {
			fmt.Fprintf(&b, "Title: %s\n", r.PR.Title)
		}
		if r.PR.Author != "" {
			fmt.Fprintf(&b, "Author: %s\n", r.PR.Author)
		}
		if r.PR.URL != "" {
			fmt.Fprintf(&b, "URL: %s\n", r.PR.URL)
		}
		if r.PR.BaseRef != "" || r.PR.HeadRef != "" {
			fmt.Fprintf(&b, "Refs: %s (%s) -> %s (%s)\n", r.PR.BaseRef, shortSHA(r.PR.BaseSHA), r.PR.HeadRef, shortSHA(r.PR.HeadSHA))
		}
		if len(r.PR.Labels) > 0 {
			fmt.Fprintf(&b, "Labels: %s\n", strings.Join(r.PR.Labels, ", "))
		}
		if r.PR.ChangedFileCount > 0 || r.PR.Additions > 0 || r.PR.Deletions > 0 {
			fmt.Fprintf(&b, "GitHub totals: %d files, +%d/-%d\n", r.PR.ChangedFileCount, r.PR.Additions, r.PR.Deletions)
		}
		if len(r.PR.Files) > 0 {
			b.WriteString("GitHub files:\n")
			for _, file := range r.PR.Files {
				status := file.Status
				if status == "" {
					status = "changed"
				}
				fmt.Fprintf(&b, "  - %s [%s] (+%d/-%d)\n", file.Path, status, file.Additions, file.Deletions)
			}
			if r.PR.FilesTruncated {
				b.WriteString("  - [TRUNCATED: GitHub file list is incomplete]\n")
			}
		}
		if r.PR.Body != "" {
			b.WriteString("What This PR Does (user-provided, untrusted):\n")
			for _, line := range strings.Split(r.PR.Body, "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
			if r.PR.BodyTruncated {
				b.WriteString("  [TRUNCATED: PR description is incomplete]\n")
			}
		}
		b.WriteByte('\n')
	}

	// Summary
	b.WriteString("Summary\n")
	b.WriteString("-------\n")

	fileNames := make([]string, len(r.ChangedFiles))
	for i, f := range r.ChangedFiles {
		name := filepath.Base(f.Path)
		if f.AddedLines > 0 || f.DeletedLines > 0 {
			name += fmt.Sprintf(" (+%d/-%d)", f.AddedLines, f.DeletedLines)
		}
		fileNames[i] = name
	}
	fmt.Fprintf(&b, "%d files changed: %s\n", len(r.ChangedFiles), strings.Join(fileNames, ", "))

	var summaryParts []string
	if len(r.Functions) > 0 {
		kind := entityKindLabel(r.Functions)
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s modified", len(r.Functions), kind))
	}
	addedTests := 0
	modifiedTests := 0
	for _, t := range r.Tests {
		if t.IsAdded {
			addedTests++
		} else {
			modifiedTests++
		}
	}
	if addedTests > 0 {
		noun := "test"
		if addedTests > 1 {
			noun = "tests"
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s added", addedTests, noun))
	}
	if modifiedTests > 0 {
		noun := "test"
		if modifiedTests > 1 {
			noun = "tests"
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s modified", modifiedTests, noun))
	}
	if len(summaryParts) > 0 {
		b.WriteString(strings.Join(summaryParts, ", "))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	if r.DiffExcerpt != "" {
		b.WriteString("Changed diff evidence (bounded)\n")
		b.WriteString("------------------------------\n")
		b.WriteString("```diff\n")
		b.WriteString(r.DiffExcerpt)
		if !strings.HasSuffix(r.DiffExcerpt, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
		if r.DiffExcerptTruncated {
			b.WriteString("[TRUNCATED: omitted changed lines are not available for exact-line review.]\n")
		}
		b.WriteByte('\n')
	}

	if len(r.ReviewLeads) > 0 {
		b.WriteString("Deterministic Review Leads\n")
		b.WriteString("--------------------------\n")
		b.WriteString("These are evidence-backed prompts for deeper inspection, not confirmed defects.\n")
		for _, lead := range r.ReviewLeads {
			fmt.Fprintf(&b, "  - [%s] %s (%s)\n", lead.Status, lead.Kind, reviewLeadLocation(lead))
			fmt.Fprintf(&b, "    %s\n", lead.Summary)
			for _, evidence := range lead.Evidence {
				fmt.Fprintf(&b, "    evidence: %s\n", reviewLeadEvidenceLabel(evidence))
			}
			if len(lead.SuggestedChecks) > 0 {
				fmt.Fprintf(&b, "    checks: %s\n", strings.Join(lead.SuggestedChecks, "; "))
			}
		}
		b.WriteByte('\n')
	}

	// Changes
	if len(r.Functions) > 0 {
		b.WriteString("Changes\n")
		b.WriteString("-------\n")

		for i, er := range r.Functions {
			if i > 0 {
				b.WriteByte('\n')
			}

			fmt.Fprintf(&b, "%s\n", reviewEntityLabel(er.Entity))
			fmt.Fprintf(&b, "  File: %s:%d\n", er.Entity.Source.File, er.Entity.Source.Line)
			if len(er.ChangedFiles) > 0 {
				fmt.Fprintf(&b, "  Changed file(s): %s\n", strings.Join(er.ChangedFiles, ", "))
			}
			if len(er.Hunks) > 0 {
				b.WriteString("  Changed hunk(s):\n")
				for _, hunk := range er.Hunks {
					fmt.Fprintf(&b, "    - @@ -%d,%d +%d,%d @@ %s\n", hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount, hunk.Header)
				}
			}

			kindDesc := er.Entity.Kind.String()
			recv := receiverType(er.Entity)
			if recv != "" {
				kindDesc += fmt.Sprintf(" (%s method)", recv)
			}
			fmt.Fprintf(&b, "  Kind: %s\n", kindDesc)

			if er.Approximate {
				b.WriteString("  Mapping: approximate\n")
			}
			b.WriteByte('\n')

			if len(er.Callers) > 0 {
				b.WriteString("  Called by:\n")
				for _, c := range er.Callers {
					fmt.Fprintf(&b, "    - %s() (%s:%d)\n", c.Name, filepath.Base(c.Source.File), c.Source.Line)
				}
				b.WriteByte('\n')
			}

			significant := significantCallees(er.Callees)
			if len(significant) > 0 {
				b.WriteString("  Calls:\n")
				for _, c := range significant {
					fmt.Fprintf(&b, "    - %s\n", c)
				}
				b.WriteByte('\n')
			}

			if len(er.Relationships) > 0 {
				b.WriteString("  Graph relationships (with evidence):\n")
				for _, rel := range er.Relationships {
					fmt.Fprintf(&b, "    - %s -> %s [%s] (%s:%d) %s\n", rel.Type, rel.To, rel.Confidence, filepath.Base(rel.Evidence.File), rel.Evidence.Line, rel.Evidence.Reason)
				}
				if er.RelationshipsTruncated {
					b.WriteString("    - [TRUNCATED: relationship context capped]\n")
				}
				b.WriteByte('\n')
			}

			if len(er.Controllers) > 0 || len(er.Resources) > 0 {
				b.WriteString("  Blast radius:\n")
				if len(er.Controllers) > 0 {
					names := make([]string, len(er.Controllers))
					for i, c := range er.Controllers {
						names[i] = c.Name
					}
					fmt.Fprintf(&b, "    Controllers: %s\n", strings.Join(names, ", "))
				}
				if len(er.Resources) > 0 {
					names := make([]string, len(er.Resources))
					for i, r := range er.Resources {
						names[i] = r.Name
					}
					fmt.Fprintf(&b, "    Resources: %s\n", strings.Join(names, ", "))
				}
				b.WriteByte('\n')
			}

			if len(er.Tests) > 0 {
				b.WriteString("  Existing tests (graph):\n")
				for _, t := range er.Tests {
					fmt.Fprintf(&b, "    - %s (%s:%d)\n", t.Name, filepath.Base(t.Source.File), t.Source.Line)
				}
				b.WriteByte('\n')
			}

			if len(er.Patterns) > 0 {
				b.WriteString("  Pattern observations (evidence-backed review leads):\n")
				for _, finding := range er.Patterns {
					fmt.Fprintf(&b, "    - [%s] %s: %s\n", finding.Status, finding.Area, finding.Summary)
					for _, evidence := range finding.Evidence {
						location := evidence.EntityID
						if evidence.File != "" {
							location = fmt.Sprintf("%s:%d", filepath.Base(evidence.File), evidence.Line)
							if evidence.EntityID != "" {
								location += " (" + evidence.EntityID + ")"
							}
						}
						fmt.Fprintf(&b, "      evidence: %s — %s\n", location, evidence.Detail)
					}
					if finding.EvidenceTruncated {
						b.WriteString("      evidence: [TRUNCATED: pattern evidence is bounded]\n")
					}
				}
				b.WriteByte('\n')
			}
		}
	} else {
		b.WriteString("No Atlas entities mapped to changed lines.\n\n")
	}

	// Tests
	if len(r.Tests) > 0 {
		b.WriteString("Tests\n")
		b.WriteString("-----\n")
		for _, tl := range r.Tests {
			status := "MODIFIED"
			if tl.IsAdded {
				status = "ADDED"
			}
			fmt.Fprintf(&b, "  %s: %s (%s:%d)\n", status, tl.Test.Name, filepath.Base(tl.Test.Source.File), tl.Test.Source.Line)

			if len(tl.Targets) > 0 {
				names := make([]string, len(tl.Targets))
				for i, t := range tl.Targets {
					names[i] = t.Name + "()"
				}
				label := "HEURISTIC"
				if tl.Confidence == domain.ConfidenceProven {
					label = "PROVEN"
				} else if tl.Confidence == domain.ConfidenceInferred {
					label = "INFERRED"
				}
				fmt.Fprintf(&b, "  Targets: %s (%s — %s)\n", strings.Join(names, ", "), label, tl.Reason)
			} else {
				b.WriteString("  Targets: unknown\n")
			}
			b.WriteString("  Changed behavior covered: INSUFFICIENT_EVIDENCE\n")
		}
		b.WriteByte('\n')
	}

	if len(r.TestAssessments) > 0 {
		b.WriteString("Test analysis\n")
		b.WriteString("-------------\n")
		for _, assessment := range r.TestAssessments {
			fmt.Fprintf(&b, "  %s | behavior coverage: %s\n", formatTestAssessmentSummary(assessment), assessment.Coverage)
			fmt.Fprintf(&b, "    Reason: %s\n", assessment.Reason)
			if len(assessment.LinkedTests) > 0 {
				b.WriteString("    Stored test links:\n")
				for _, reference := range assessment.LinkedTests {
					fmt.Fprintf(&b, "      - %s\n", formatTestReference(reference))
				}
				if assessment.LinkedTestsTruncated {
					b.WriteString("      - [TRUNCATED: stored test links are bounded]\n")
				}
			}
			if len(assessment.InferredTests) > 0 {
				b.WriteString("    Inferred test links (not proof):\n")
				for _, reference := range assessment.InferredTests {
					fmt.Fprintf(&b, "      - %s\n", formatTestReference(reference))
				}
				if assessment.InferredTestsTruncated {
					b.WriteString("      - [TRUNCATED: inferred test links are bounded]\n")
				}
			}
		}
		b.WriteByte('\n')
	}

	// Unmapped files
	if len(r.UnmappedFiles) > 0 {
		b.WriteString("Unmapped Changed Files\n")
		b.WriteString("----------------------\n")
		for _, f := range r.UnmappedFiles {
			fmt.Fprintf(&b, "  %s (no Atlas entities — new or unscanned)\n", f)
		}
		b.WriteByte('\n')
	}

	// Heuristics and limitations are kept separate from deterministic result
	// sections so reviewers can see what Atlas did not prove.
	b.WriteString("Heuristics\n")
	b.WriteString("----------\n")
	if len(r.Heuristics) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, h := range r.Heuristics {
			fmt.Fprintf(&b, "- %s\n", h)
		}
	}
	b.WriteByte('\n')

	b.WriteString("LLM Interpretation\n")
	b.WriteString("------------------\n")
	if len(r.LLMInterpretation) == 0 {
		b.WriteString("- none; this deterministic review does not invoke an LLM\n")
	} else {
		for _, interpretation := range r.LLMInterpretation {
			fmt.Fprintf(&b, "- %s\n", interpretation)
		}
	}
	b.WriteByte('\n')

	// Limitations
	b.WriteString("Evidence Limitations\n")
	b.WriteString("--------------------\n")
	for _, l := range r.Limitations {
		fmt.Fprintf(&b, "- %s\n", l)
	}

	return b.String()
}

func scanStatus(complete bool) string {
	if complete {
		return "complete"
	}
	return "incomplete"
}

func shortSHA(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func formatTestReference(reference TestReference) string {
	if reference.Test == nil {
		return "<unknown test>"
	}
	location := fmt.Sprintf("%s:%d", filepath.Base(reference.Test.Source.File), reference.Test.Source.Line)
	confidence := strings.ToUpper(string(reference.Confidence))
	if confidence == "" {
		confidence = "UNKNOWN"
	}
	return fmt.Sprintf("%s (%s) [%s] — %s", reference.Test.Name, location, confidence, reference.Reason)
}

func significantCallees(calls []string) []string {
	var result []string
	for _, c := range calls {
		if isSignificantCallee(c) {
			result = append(result, c)
		}
	}
	return result
}

func isSignificantCallee(name string) bool {
	method := name
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		if noisePackages[parts[0]] {
			return false
		}
		method = parts[1]
	}
	return !noiseMethods[method]
}

func receiverType(e *domain.Entity) string {
	parts := strings.SplitN(e.ID, ":", 2)
	if len(parts) < 2 {
		return ""
	}
	dotParts := strings.Split(parts[1], ".")
	if len(dotParts) >= 3 {
		return dotParts[len(dotParts)-2]
	}
	return ""
}

func entityKindLabel(entities []EntityReview) string {
	if len(entities) == 0 {
		return "entities"
	}
	first := entities[0].Entity.Kind
	allSame := true
	for _, e := range entities[1:] {
		if e.Entity.Kind != first {
			allSame = false
			break
		}
	}
	if allSame {
		k := first.String()
		if len(entities) > 1 {
			return k + "s"
		}
		return k
	}
	return "entities"
}

func reviewEntityLabel(entity *domain.Entity) string {
	if entity == nil {
		return "<unknown entity>"
	}
	if entity.Kind == domain.KindFunction {
		return entity.Name + "()"
	}
	return entity.Name + " [" + entity.Kind.String() + "]"
}
