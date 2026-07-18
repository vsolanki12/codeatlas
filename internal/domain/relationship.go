package domain

// The kind of connection between two entities in the Atlas graph.
type RelationshipType string

const (
	RelReconciles   RelationshipType = "reconciles"
	RelCreates      RelationshipType = "creates"
	RelOwns         RelationshipType = "owns"
	RelWatches      RelationshipType = "watches"
	RelCalls        RelationshipType = "calls"
	RelTestedBy     RelationshipType = "tested_by"
	RelDocumentedIn RelationshipType = "documented_in"
	RelDependsOn    RelationshipType = "depends_on"
	RelImports      RelationshipType = "imports"
	RelImplements   RelationshipType = "implements"
	RelEmits        RelationshipType = "emits"
	RelContains     RelationshipType = "contains"
	RelPartOf       RelationshipType = "part_of"
)

// How certain Atlas is that a relationship exists.
type Confidence string

const (
	ConfidenceProven   Confidence = "proven"
	ConfidenceInferred Confidence = "inferred"
)

// Proof of why a relationship exists - file, line, and snippet from the source.
type Evidence struct {
	Parser  string `json:"parser"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// A directed edge between two entities, carrying type, confidence, and evidence.
type Relationship struct {
	ID         string           `json:"id"`
	From       string           `json:"from"`
	To         string           `json:"to"`
	Type       RelationshipType `json:"type"`
	Confidence Confidence       `json:"confidence"`
	Evidence   Evidence         `json:"evidence"`
}

// NewRelationshipID builds a deterministic relationship ID in the format from--type--to.
func NewRelationshipID(from string, relType RelationshipType, to string) string {
	return from + "--" + string(relType) + "--" + to
}
