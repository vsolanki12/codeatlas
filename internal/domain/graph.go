package domain

// Graph is the top-level container for an Atlas scan result.
type Graph struct {
	Schema        string         `json:"schema"`
	SchemaVersion string         `json:"schemaVersion"`
	GeneratedAt   string         `json:"generatedAt"`
	Repository    string         `json:"repository"`
	Commit        string         `json:"commit"`
	Branch        string         `json:"branch"`
	ScanDuration  string         `json:"scanDuration"`
	Entities       []Entity          `json:"entities"`
	Relationship   []Relationship    `json:"relationship"`
	FileTimestamps map[string]string `json:"fileTimestamps,omitempty"`
}
