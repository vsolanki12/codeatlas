package domain

import (
	"encoding/json"
	"fmt"
)

// CurrentEntityIdentity identifies the repository-unique entity ID scheme
// emitted by the current scanner. A graph without this marker is legacy and
// must be fully rescanned before incremental facts are reused.
const CurrentEntityIdentity = "repository-path-v1"

// Graph is the top-level container for an Atlas scan result.
type Graph struct {
	Schema           string            `json:"schema"`
	SchemaVersion    string            `json:"schemaVersion"`
	EntityIdentity   string            `json:"entityIdentity,omitempty"`
	GeneratedAt      string            `json:"generatedAt"`
	Repository       string            `json:"repository"`
	Commit           string            `json:"commit"`
	Branch           string            `json:"branch"`
	ScanDuration     string            `json:"scanDuration"`
	ScanComplete     bool              `json:"scanComplete"`
	ScanWarnings     []string          `json:"scanWarnings,omitempty"`
	Entities         []Entity          `json:"entities"`
	Relationship     []Relationship    `json:"relationships"`
	FileTimestamps   map[string]string `json:"fileTimestamps,omitempty"`
	FileFingerprints map[string]string `json:"fileFingerprints,omitempty"`
	Views            map[string]View   `json:"views,omitempty"`
	Questions        map[string]string `json:"questions,omitempty"`
}

// MarshalJSON writes the canonical graph schema. The Go field remains named
// Relationship for source compatibility with the original implementation,
// while the wire format uses the documented plural key.
func (g Graph) MarshalJSON() ([]byte, error) {
	type wireGraph struct {
		Schema           string            `json:"schema"`
		SchemaVersion    string            `json:"schemaVersion"`
		EntityIdentity   string            `json:"entityIdentity,omitempty"`
		GeneratedAt      string            `json:"generatedAt"`
		Repository       string            `json:"repository"`
		Commit           string            `json:"commit"`
		Branch           string            `json:"branch"`
		ScanDuration     string            `json:"scanDuration"`
		ScanComplete     bool              `json:"scanComplete"`
		ScanWarnings     []string          `json:"scanWarnings,omitempty"`
		Entities         []Entity          `json:"entities"`
		Relationships    []Relationship    `json:"relationships"`
		FileTimestamps   map[string]string `json:"fileTimestamps,omitempty"`
		FileFingerprints map[string]string `json:"fileFingerprints,omitempty"`
		Views            map[string]View   `json:"views,omitempty"`
		Questions        map[string]string `json:"questions,omitempty"`
	}
	return json.Marshal(wireGraph{
		Schema: g.Schema, SchemaVersion: g.SchemaVersion, EntityIdentity: g.EntityIdentity, GeneratedAt: g.GeneratedAt,
		Repository: g.Repository, Commit: g.Commit, Branch: g.Branch,
		ScanDuration: g.ScanDuration, ScanComplete: g.ScanComplete, ScanWarnings: g.ScanWarnings,
		Entities: g.Entities, Relationships: g.Relationship,
		FileTimestamps: g.FileTimestamps, FileFingerprints: g.FileFingerprints,
		Views: g.Views, Questions: g.Questions,
	})
}

// UnmarshalJSON accepts both the canonical plural relationship key and the
// legacy singular key so existing graphs can be migrated by rescanning.
func (g *Graph) UnmarshalJSON(data []byte) error {
	type wireGraph struct {
		Schema           string            `json:"schema"`
		SchemaVersion    string            `json:"schemaVersion"`
		EntityIdentity   string            `json:"entityIdentity"`
		GeneratedAt      string            `json:"generatedAt"`
		Repository       string            `json:"repository"`
		Commit           string            `json:"commit"`
		Branch           string            `json:"branch"`
		ScanDuration     string            `json:"scanDuration"`
		ScanComplete     bool              `json:"scanComplete"`
		ScanWarnings     []string          `json:"scanWarnings"`
		Entities         []Entity          `json:"entities"`
		Relationships    []Relationship    `json:"relationships"`
		LegacyRelations  []Relationship    `json:"relationship"`
		FileTimestamps   map[string]string `json:"fileTimestamps"`
		FileFingerprints map[string]string `json:"fileFingerprints"`
		Views            map[string]View   `json:"views"`
		Questions        map[string]string `json:"questions"`
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
		Repository: w.Repository, Commit: w.Commit, Branch: w.Branch,
		ScanDuration: w.ScanDuration, ScanComplete: w.ScanComplete, ScanWarnings: w.ScanWarnings,
		Entities: w.Entities, Relationship: relations,
		FileTimestamps: w.FileTimestamps, FileFingerprints: w.FileFingerprints,
		Views: w.Views, Questions: w.Questions,
	}
	return nil
}
