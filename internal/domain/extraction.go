package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CurrentExtractorVersion identifies the extraction contract, independently of
// the target repository commit and wire schema. Bump it when unchanged source
// would produce different parser observations or identities.
const CurrentExtractorVersion = "go-evidence-v2"

// BuildContext records the effective environment used for Go type enrichment.
// AST parsing still inventories all source files, including inactive variants.
type BuildContext struct {
	GOOS       string   `json:"goos"`
	GOARCH     string   `json:"goarch"`
	BuildTags  []string `json:"buildTags,omitempty"`
	GoVersion  string   `json:"goVersion"`
	CgoEnabled bool     `json:"cgoEnabled"`
}

// ExtractionSignature is the compatibility key for incremental parser reuse.
// Tag order and duplicates do not change build semantics or the signature.
// The optional executable fingerprint distinguishes binaries implementing the
// same declared extraction contract; old two-argument callers remain supported.
func ExtractionSignature(version string, context BuildContext, executableBuild ...string) string {
	context.BuildTags = append([]string(nil), context.BuildTags...)
	for i := range context.BuildTags {
		context.BuildTags[i] = strings.TrimSpace(context.BuildTags[i])
	}
	sort.Strings(context.BuildTags)
	tags := context.BuildTags[:0]
	for _, tag := range context.BuildTags {
		if tag = strings.TrimSpace(tag); tag != "" && (len(tags) == 0 || tags[len(tags)-1] != tag) {
			tags = append(tags, tag)
		}
	}
	context.BuildTags = tags
	build := ""
	if len(executableBuild) > 0 {
		build = executableBuild[0]
	}
	data, _ := json.Marshal(struct {
		Version string       `json:"version"`
		Context BuildContext `json:"context"`
		Build   string       `json:"build,omitempty"`
	}{version, context, build})
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}

// TypeAnalysisCoverage describes enrichment quality separately from parser
// completeness. Missing type information leaves observations unresolved.
type TypeAnalysisCoverage struct {
	DependencyMode         string                   `json:"dependencyMode"`
	ExternalImports        int                      `json:"externalImports"`
	ExternalImportFailures int                      `json:"externalImportFailures"`
	Packages               int                      `json:"packages"`
	CheckedPackages        int                      `json:"checkedPackages"`
	FailedPackages         int                      `json:"failedPackages"`
	Files                  int                      `json:"files"`
	ExcludedFiles          int                      `json:"excludedFiles"`
	ResolvedCalls          int                      `json:"resolvedCalls"`
	UnresolvedCalls        int                      `json:"unresolvedCalls"`
	ResolvedReferences     int                      `json:"resolvedReferences"`
	DiagnosticCount        int                      `json:"diagnosticCount"`
	DiagnosticsTruncated   bool                     `json:"diagnosticsTruncated,omitempty"`
	Diagnostics            []TypeAnalysisDiagnostic `json:"diagnostics,omitempty"`
}

type TypeAnalysisDiagnostic struct {
	Package string `json:"package"`
	Variant string `json:"variant"`
	Message string `json:"message"`
}

// ResourceOperation preserves the call-site observation even when no manifest
// entity exists. An empty ObjectType explicitly means type resolution failed.
type ResourceOperation struct {
	Operation          string     `json:"operation"`
	Method             string     `json:"method,omitempty"`
	ObjectType         string     `json:"objectType,omitempty"`
	ObservedMethod     string     `json:"observedMethod,omitempty"`
	ObservedObjectType string     `json:"observedObjectType,omitempty"`
	Confidence         Confidence `json:"confidence"`
	Source             Source     `json:"source"`
}
