package domain

import (
	"encoding/json"
	"fmt"
)

// CurrentEntityIdentity identifies the repository-unique entity ID scheme
// emitted by the current scanner. A graph without this marker is legacy and
// must be fully rescanned before incremental facts are reused.
const CurrentEntityIdentity = "repository-path-v1"

// CurrentSchemaVersion is the graph schema emitted by the current scanner.
// New fields are optional so older graphs remain readable by newer consumers.
const CurrentSchemaVersion = "1.6.0"

// ScanFileStatus describes how the scanner accounted for a discovered file.
// A status is deliberately more precise than a warning string: consumers can
// distinguish parsed facts, reused facts, intentional omissions, and failures.
type ScanFileStatus string

const (
	ScanFileParsed  ScanFileStatus = "parsed"
	ScanFileReused  ScanFileStatus = "reused"
	ScanFileIgnored ScanFileStatus = "ignored"
	ScanFileFailed  ScanFileStatus = "failed"
)

// ScanFile records the deterministic disposition of one discovered file.
// Files with no supported parser are retained as ignored instead of silently
// disappearing from the graph's coverage accounting.
type ScanFile struct {
	Path        string         `json:"path"`
	Status      ScanFileStatus `json:"status"`
	Parser      string         `json:"parser,omitempty"`
	EntityCount int            `json:"entityCount,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

// ScanCoverage summarizes the file inventory represented by ScanFiles.
// Parsed and reused both mean that the graph has facts for the file; reused
// identifies facts carried forward by an incremental scan.
type ScanCoverage struct {
	Discovered int `json:"discovered"`
	Parsed     int `json:"parsed"`
	Reused     int `json:"reused"`
	Ignored    int `json:"ignored"`
	Failed     int `json:"failed"`
}

// Graph is the top-level container for an Atlas scan result.
type Graph struct {
	Schema              string                `json:"schema"`
	SchemaVersion       string                `json:"schemaVersion"`
	EntityIdentity      string                `json:"entityIdentity,omitempty"`
	ExtractorVersion    string                `json:"extractorVersion,omitempty"`
	ExtractorBuild      string                `json:"extractorBuild,omitempty"`
	ExtractionSignature string                `json:"extractionSignature,omitempty"`
	BuildContext        *BuildContext         `json:"buildContext,omitempty"`
	TypeAnalysis        *TypeAnalysisCoverage `json:"typeAnalysis,omitempty"`
	GeneratedAt         string                `json:"generatedAt"`
	Repository          string                `json:"repository"`
	Commit              string                `json:"commit"`
	Branch              string                `json:"branch"`
	ScanDuration        string                `json:"scanDuration"`
	ScanComplete        bool                  `json:"scanComplete"`
	ScanWarnings        []string              `json:"scanWarnings,omitempty"`
	ScanFiles           []ScanFile            `json:"scanFiles,omitempty"`
	ScanCoverage        *ScanCoverage         `json:"scanCoverage,omitempty"`
	Entities            []Entity              `json:"entities"`
	Relationship        []Relationship        `json:"relationships"`
	FileTimestamps      map[string]string     `json:"fileTimestamps,omitempty"`
	FileFingerprints    map[string]string     `json:"fileFingerprints,omitempty"`
	Views               map[string]View       `json:"views,omitempty"`
	Questions           map[string]string     `json:"questions,omitempty"`
}

// MarshalJSON writes the canonical graph schema. The Go field remains named
// Relationship for source compatibility with the original implementation,
// while the wire format uses the documented plural key.
func (g Graph) MarshalJSON() ([]byte, error) {
	type wireGraph struct {
		Schema              string                `json:"schema"`
		SchemaVersion       string                `json:"schemaVersion"`
		EntityIdentity      string                `json:"entityIdentity,omitempty"`
		ExtractorVersion    string                `json:"extractorVersion,omitempty"`
		ExtractorBuild      string                `json:"extractorBuild,omitempty"`
		ExtractionSignature string                `json:"extractionSignature,omitempty"`
		BuildContext        *BuildContext         `json:"buildContext,omitempty"`
		TypeAnalysis        *TypeAnalysisCoverage `json:"typeAnalysis,omitempty"`
		GeneratedAt         string                `json:"generatedAt"`
		Repository          string                `json:"repository"`
		Commit              string                `json:"commit"`
		Branch              string                `json:"branch"`
		ScanDuration        string                `json:"scanDuration"`
		ScanComplete        bool                  `json:"scanComplete"`
		ScanWarnings        []string              `json:"scanWarnings,omitempty"`
		ScanFiles           []ScanFile            `json:"scanFiles,omitempty"`
		ScanCoverage        *ScanCoverage         `json:"scanCoverage,omitempty"`
		Entities            []Entity              `json:"entities"`
		Relationships       []Relationship        `json:"relationships"`
		FileTimestamps      map[string]string     `json:"fileTimestamps,omitempty"`
		FileFingerprints    map[string]string     `json:"fileFingerprints,omitempty"`
		Views               map[string]View       `json:"views,omitempty"`
		Questions           map[string]string     `json:"questions,omitempty"`
	}
	return json.Marshal(wireGraph{
		Schema: g.Schema, SchemaVersion: g.SchemaVersion, EntityIdentity: g.EntityIdentity, GeneratedAt: g.GeneratedAt,
		ExtractorVersion: g.ExtractorVersion, ExtractorBuild: g.ExtractorBuild, ExtractionSignature: g.ExtractionSignature, BuildContext: g.BuildContext, TypeAnalysis: g.TypeAnalysis,
		Repository: g.Repository, Commit: g.Commit, Branch: g.Branch,
		ScanDuration: g.ScanDuration, ScanComplete: g.ScanComplete, ScanWarnings: g.ScanWarnings,
		ScanFiles: g.ScanFiles, ScanCoverage: g.ScanCoverage,
		Entities: g.Entities, Relationships: g.Relationship,
		FileTimestamps: g.FileTimestamps, FileFingerprints: g.FileFingerprints,
		Views: g.Views, Questions: g.Questions,
	})
}

// UnmarshalJSON accepts both the canonical plural relationship key and the
// legacy singular key so existing graphs can be migrated by rescanning.
func (g *Graph) UnmarshalJSON(data []byte) error {
	type wireGraph struct {
		Schema              string                `json:"schema"`
		SchemaVersion       string                `json:"schemaVersion"`
		EntityIdentity      string                `json:"entityIdentity"`
		ExtractorVersion    string                `json:"extractorVersion"`
		ExtractorBuild      string                `json:"extractorBuild"`
		ExtractionSignature string                `json:"extractionSignature"`
		BuildContext        *BuildContext         `json:"buildContext"`
		TypeAnalysis        *TypeAnalysisCoverage `json:"typeAnalysis"`
		GeneratedAt         string                `json:"generatedAt"`
		Repository          string                `json:"repository"`
		Commit              string                `json:"commit"`
		Branch              string                `json:"branch"`
		ScanDuration        string                `json:"scanDuration"`
		ScanComplete        bool                  `json:"scanComplete"`
		ScanWarnings        []string              `json:"scanWarnings"`
		ScanFiles           []ScanFile            `json:"scanFiles"`
		ScanCoverage        *ScanCoverage         `json:"scanCoverage"`
		Entities            []Entity              `json:"entities"`
		Relationships       []Relationship        `json:"relationships"`
		LegacyRelations     []Relationship        `json:"relationship"`
		FileTimestamps      map[string]string     `json:"fileTimestamps"`
		FileFingerprints    map[string]string     `json:"fileFingerprints"`
		Views               map[string]View       `json:"views"`
		Questions           map[string]string     `json:"questions"`
	}
	var w wireGraph
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	if w.Relationships != nil && w.LegacyRelations != nil {
		return fmt.Errorf("graph contains both relationships and legacy relationship keys")
	}
	relations := w.Relationships
	if relations == nil {
		relations = w.LegacyRelations
	}
	*g = Graph{
		Schema: w.Schema, SchemaVersion: w.SchemaVersion, EntityIdentity: w.EntityIdentity, GeneratedAt: w.GeneratedAt,
		ExtractorVersion: w.ExtractorVersion, ExtractorBuild: w.ExtractorBuild, ExtractionSignature: w.ExtractionSignature, BuildContext: w.BuildContext, TypeAnalysis: w.TypeAnalysis,
		Repository: w.Repository, Commit: w.Commit, Branch: w.Branch,
		ScanDuration: w.ScanDuration, ScanComplete: w.ScanComplete, ScanWarnings: w.ScanWarnings,
		ScanFiles: w.ScanFiles, ScanCoverage: w.ScanCoverage,
		Entities: w.Entities, Relationship: relations,
		FileTimestamps: w.FileTimestamps, FileFingerprints: w.FileFingerprints,
		Views: w.Views, Questions: w.Questions,
	}
	return nil
}
