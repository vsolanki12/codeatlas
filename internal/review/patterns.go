package review

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

const (
	patternMatchesObserved      = "MATCHES_OBSERVED"
	patternDiffersObserved      = "DIFFERS_OBSERVED"
	patternInsufficientEvidence = "INSUFFICIENT_EVIDENCE"
	minimumPatternPeers         = 3
	maxPatternEvidence          = 4
)

var loggingMethods = map[string]bool{
	"Debug":      true,
	"Debugf":     true,
	"Info":       true,
	"Infof":      true,
	"Warn":       true,
	"Warnf":      true,
	"Error":      true,
	"Errorf":     true,
	"V":          true,
	"WithName":   true,
	"WithValues": true,
}

func analyzePatterns(er EntityReview, idx *query.Index) []PatternFinding {
	if er.Entity == nil || idx == nil {
		return nil
	}

	findings := make([]PatternFinding, 0, 4)
	switch er.Entity.Kind {
	case domain.KindFunction:
		if finding := analyzeFunctionNaming(er, idx); finding.Area != "" {
			findings = append(findings, finding)
		}
		if finding := analyzeFunctionSignals("error_handling", er, idx, errorSignals, "error-handling"); finding.Area != "" {
			findings = append(findings, finding)
		}
		if finding := analyzeFunctionSignals("logging", er, idx, loggingSignals, "logging"); finding.Area != "" {
			findings = append(findings, finding)
		}
	case domain.KindController:
		if finding := analyzeControllerStructure(er, idx); finding.Area != "" {
			findings = append(findings, finding)
		}
	}

	if len(findings) > maxPatternEvidence {
		findings = findings[:maxPatternEvidence]
	}
	return findings
}

func analyzeFunctionNaming(er EntityReview, idx *query.Index) PatternFinding {
	if er.Entity.Package == "" {
		return PatternFinding{}
	}

	peers := samePackageFunctions(er.Entity, idx)
	if len(peers) < 2 {
		return PatternFinding{
			Area:    "function_naming",
			Status:  patternInsufficientEvidence,
			Summary: fmt.Sprintf("There are only %d comparable same-package functions; naming convention cannot be established.", len(peers)),
			Evidence: []PatternEvidence{
				patternEntityEvidence(er.Entity, "changed function under review"),
			},
		}
	}

	exported := 0
	for _, peer := range peers {
		if isExportedName(peer.Name) {
			exported++
		}
	}
	majorityExported := exported*2 > len(peers)
	changedExported := isExportedName(er.Entity.Name)
	status := patternMatchesObserved
	convention := "unexported"
	if majorityExported {
		convention = "exported"
	}
	if changedExported != majorityExported {
		status = patternDiffersObserved
	}

	evidence := []PatternEvidence{
		patternEntityEvidence(er.Entity, fmt.Sprintf("changed function is %s", exportedLabel(changedExported))),
	}
	evidenceTruncated := false
	for _, peer := range peers {
		if len(evidence) >= maxPatternEvidence {
			evidenceTruncated = true
			break
		}
		evidence = append(evidence, patternEntityEvidence(peer, fmt.Sprintf("comparable function is %s", exportedLabel(isExportedName(peer.Name)))))
	}

	return PatternFinding{
		Area:   "function_naming",
		Status: status,
		Summary: fmt.Sprintf(
			"Function is %s; %s naming is observed in %d/%d comparable same-package functions.",
			exportedLabel(changedExported), convention, exportedCount(majorityExported, exported, len(peers)), len(peers)),
		Evidence:          evidence,
		EvidenceTruncated: evidenceTruncated,
	}
}

func analyzeFunctionSignals(area string, er EntityReview, idx *query.Index, signaler func(*domain.Entity, []AddedLine) []string, label string) PatternFinding {
	changedSignals := signaler(er.Entity, er.addedLines)
	if len(changedSignals) == 0 {
		return PatternFinding{}
	}

	peers := samePackageFunctions(er.Entity, idx)
	if len(peers) < minimumPatternPeers || er.addedLinesTruncated {
		summary := fmt.Sprintf("Observed %s signals [%s], but comparable repository evidence is insufficient.", label, strings.Join(changedSignals, ", "))
		if er.addedLinesTruncated {
			summary += " Added diff context was bounded."
		}
		return PatternFinding{
			Area:    area,
			Status:  patternInsufficientEvidence,
			Summary: summary,
			Evidence: []PatternEvidence{
				patternEntityEvidence(er.Entity, "changed function signals: "+strings.Join(changedSignals, ", ")),
			},
		}
	}

	counts := make(map[string]int, len(changedSignals))
	peerSignals := make(map[string][]string, len(peers))
	for _, peer := range peers {
		signals := signaler(peer, nil)
		peerSignals[peer.ID] = signals
		for _, signal := range signals {
			counts[signal]++
		}
	}

	allMatch := true
	for _, signal := range changedSignals {
		if counts[signal]*2 <= len(peers) {
			allMatch = false
			break
		}
	}
	status := patternMatchesObserved
	if !allMatch {
		status = patternDiffersObserved
	}

	countParts := make([]string, 0, len(changedSignals))
	for _, signal := range changedSignals {
		countParts = append(countParts, fmt.Sprintf("%s=%d/%d peers", signal, counts[signal], len(peers)))
	}
	summary := fmt.Sprintf("Observed %s signals [%s]; comparable peer frequencies: %s.", label, strings.Join(changedSignals, ", "), strings.Join(countParts, "; "))

	evidence := []PatternEvidence{
		patternEntityEvidence(er.Entity, "changed function signals: "+strings.Join(changedSignals, ", ")),
	}
	evidenceTruncated := false
	for _, line := range er.addedLines {
		if addedLineMatchesSignals(line.Text, changedSignals) {
			if len(evidence) >= maxPatternEvidence {
				evidenceTruncated = true
				continue
			}
			evidence = append(evidence, PatternEvidence{
				File:   line.File,
				Line:   line.Line,
				Detail: "added line: " + strings.TrimSpace(line.Text),
			})
		}
	}
	for _, peer := range peers {
		if len(evidence) >= maxPatternEvidence {
			evidenceTruncated = true
			break
		}
		if containsAny(peerSignals[peer.ID], changedSignals) {
			evidence = append(evidence, patternEntityEvidence(peer, "comparable function has an observed "+label+" signal"))
		}
	}

	return PatternFinding{Area: area, Status: status, Summary: summary, Evidence: evidence, EvidenceTruncated: evidenceTruncated}
}

func analyzeControllerStructure(er EntityReview, idx *query.Index) PatternFinding {
	if len(er.Entity.Watches) == 0 {
		return PatternFinding{}
	}

	changedMethods := controllerMethods(er.Entity)
	if len(changedMethods) == 0 {
		return PatternFinding{
			Area:    "controller_structure",
			Status:  patternInsufficientEvidence,
			Summary: "Controller watches resources, but registration methods are not recorded in the graph.",
			Evidence: []PatternEvidence{
				patternEntityEvidence(er.Entity, "watched resources: "+strings.Join(er.Entity.Watches, ", ")),
			},
		}
	}

	controllers := idx.Lookup(domain.KindController.String(), "", 0)
	peers := make([]*domain.Entity, 0, len(controllers))
	for _, controller := range controllers {
		if controller == nil || controller.ID == er.Entity.ID || controller.Package != er.Entity.Package {
			continue
		}
		if len(controllerMethods(controller)) == 0 {
			continue
		}
		peers = append(peers, controller)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })
	if len(peers) < minimumPatternPeers {
		return PatternFinding{
			Area:    "controller_structure",
			Status:  patternInsufficientEvidence,
			Summary: fmt.Sprintf("Controller registration methods [%s] have only %d comparable same-package controllers.", strings.Join(changedMethods, ", "), len(peers)),
			Evidence: []PatternEvidence{
				patternEntityEvidence(er.Entity, "controller registration methods: "+strings.Join(changedMethods, ", ")),
			},
		}
	}

	counts := make(map[string]int)
	for _, peer := range peers {
		counts[strings.Join(controllerMethods(peer), ",")]++
	}
	changedSignature := strings.Join(changedMethods, ",")
	status := patternDiffersObserved
	if counts[changedSignature]*2 > len(peers) {
		status = patternMatchesObserved
	}

	frequency := make([]string, 0, len(counts))
	for signature, count := range counts {
		frequency = append(frequency, fmt.Sprintf("[%s]=%d/%d peers", signature, count, len(peers)))
	}
	sort.Strings(frequency)
	evidence := []PatternEvidence{
		patternEntityEvidence(er.Entity, "controller registration methods: "+strings.Join(changedMethods, ", ")),
	}
	evidenceTruncated := false
	for _, peer := range peers {
		if len(evidence) >= maxPatternEvidence {
			evidenceTruncated = true
			break
		}
		if strings.Join(controllerMethods(peer), ",") == changedSignature {
			evidence = append(evidence, patternEntityEvidence(peer, "comparable controller uses the same registration methods"))
		}
	}

	return PatternFinding{
		Area:              "controller_structure",
		Status:            status,
		Summary:           fmt.Sprintf("Controller registration methods [%s]; comparable peer frequencies: %s.", strings.Join(changedMethods, ", "), strings.Join(frequency, "; ")),
		Evidence:          evidence,
		EvidenceTruncated: evidenceTruncated,
	}
}

func samePackageFunctions(entity *domain.Entity, idx *query.Index) []*domain.Entity {
	if entity == nil || idx == nil {
		return nil
	}
	all := idx.Lookup(domain.KindFunction.String(), "", 0)
	peers := make([]*domain.Entity, 0, len(all))
	for _, candidate := range all {
		if candidate == nil || candidate.ID == entity.ID || candidate.Package != entity.Package {
			continue
		}
		peers = append(peers, candidate)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })
	return peers
}

func errorSignals(entity *domain.Entity, added []AddedLine) []string {
	signals := make(map[string]bool)
	if entity != nil {
		for _, call := range entity.Calls {
			if signal := errorCallSignal(call); signal != "" {
				signals[signal] = true
			}
		}
		for _, literal := range entity.Literals {
			if strings.Contains(literal, "%w") {
				signals["wrapped error"] = true
			}
		}
	}
	for _, line := range added {
		text := strings.TrimSpace(line.Text)
		if strings.Contains(text, "%w") {
			signals["wrapped error"] = true
		}
		if strings.Contains(text, "return err") {
			signals["return err"] = true
		}
	}
	return sortedSignals(signals)
}

func errorCallSignal(call string) string {
	call = strings.ToLower(strings.TrimSpace(call))
	switch {
	case strings.HasSuffix(call, "fmt.errorf"):
		return "fmt.Errorf"
	case strings.HasSuffix(call, "errors.new"):
		return "errors.New"
	case strings.HasSuffix(call, "errors.wrap"):
		return "errors.Wrap"
	case strings.HasSuffix(call, "errors.wrapf"):
		return "errors.Wrapf"
	case strings.HasSuffix(call, "errors.join"):
		return "errors.Join"
	case strings.HasSuffix(call, "errors.is"):
		return "errors.Is"
	case strings.HasSuffix(call, "errors.as"):
		return "errors.As"
	}
	return ""
}

func loggingSignals(entity *domain.Entity, added []AddedLine) []string {
	signals := make(map[string]bool)
	if entity != nil {
		for _, call := range entity.Calls {
			if signal := loggingCallSignal(call); signal != "" {
				signals[signal] = true
			}
		}
	}
	for _, line := range added {
		for method := range loggingMethods {
			if strings.Contains(line.Text, "."+method+"(") {
				signals[method] = true
			}
		}
	}
	return sortedSignals(signals)
}

func loggingCallSignal(call string) string {
	call = strings.TrimSpace(call)
	dot := strings.LastIndex(call, ".")
	if dot < 1 || dot == len(call)-1 {
		return ""
	}
	packageName := call[:dot]
	if slash := strings.LastIndex(packageName, "/"); slash >= 0 {
		packageName = packageName[slash+1:]
	}
	if packageName == "fmt" || packageName == "errors" {
		return ""
	}
	method := call[dot+1:]
	if loggingMethods[method] {
		return method
	}
	return ""
}

func controllerMethods(entity *domain.Entity) []string {
	if entity == nil {
		return nil
	}
	methods := make([]string, 0, len(entity.WatchMethods))
	seen := make(map[string]bool)
	for _, method := range entity.WatchMethods {
		method = strings.TrimSpace(method)
		if method == "" || seen[method] {
			continue
		}
		seen[method] = true
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func patternEntityEvidence(entity *domain.Entity, detail string) PatternEvidence {
	return PatternEvidence{
		EntityID: entity.ID,
		File:     entity.Source.File,
		Line:     entity.Source.Line,
		Detail:   detail,
	}
}

func sortedSignals(signals map[string]bool) []string {
	result := make([]string, 0, len(signals))
	for signal := range signals {
		result = append(result, signal)
	}
	sort.Strings(result)
	return result
}

func addedLineMatchesSignals(text string, signals []string) bool {
	for _, signal := range signals {
		switch signal {
		case "wrapped error":
			if strings.Contains(text, "%w") {
				return true
			}
		case "return err":
			if strings.Contains(text, "return err") {
				return true
			}
		default:
			if strings.Contains(text, signal) {
				return true
			}
		}
	}
	return false
}

func containsAny(values, wanted []string) bool {
	set := make(map[string]bool, len(wanted))
	for _, value := range wanted {
		set[value] = true
	}
	for _, value := range values {
		if set[value] {
			return true
		}
	}
	return false
}

func isExportedName(name string) bool {
	runeValue, _ := utf8.DecodeRuneInString(name)
	return runeValue != utf8.RuneError && unicode.IsUpper(runeValue)
}

func exportedLabel(exported bool) string {
	if exported {
		return "exported"
	}
	return "unexported"
}

func exportedCount(majorityExported bool, exported, total int) int {
	if majorityExported {
		return exported
	}
	return total - exported
}
