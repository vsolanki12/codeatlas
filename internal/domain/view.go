package domain

// View is a pre-computed engineering summary for a controller or CRD,
// generated deterministically during scanning.
type View struct {
	EntityID    string `json:"entityID"`
	EntityName  string `json:"entityName"`
	Kind        string `json:"kind"`
	Package     string `json:"package,omitempty"`
	File        string `json:"file"`
	Description string `json:"description,omitempty"`

	// What this entity does (controllers)
	Reconciles string   `json:"reconciles,omitempty"`
	Creates    []string `json:"creates,omitempty"`
	Watches    []string `json:"watches,omitempty"`
	Calls      []string `json:"calls,omitempty"`

	// What acts on this entity (CRDs, resources)
	ReconciledBy string   `json:"reconciledBy,omitempty"`
	CreatedBy    []string `json:"createdBy,omitempty"`
	CalledBy     []string `json:"calledBy,omitempty"`

	// Test coverage
	Tests     []string `json:"tests,omitempty"`
	TestCount int      `json:"testCount"`

	// Files and ownership
	Files  []string `json:"files,omitempty"`
	Owners []string `json:"owners,omitempty"`

	// Temporal
	ChangeCount  int    `json:"changeCount,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	LastAuthor   string `json:"lastAuthor,omitempty"`

	// Relationships keeps the evidence behind the denormalized fields above
	// available to consumers that use a view as their primary context.
	Relationships []ViewRelationship `json:"relationships,omitempty"`
}

// ViewRelationship is an evidence-preserving relationship summary embedded
// in a pre-computed view. It avoids forcing consumers to reconstruct why a
// view field was produced from the graph's full edge list.
type ViewRelationship struct {
	ID         string           `json:"id"`
	Direction  string           `json:"direction"`
	Type       RelationshipType `json:"type"`
	EntityID   string           `json:"entityID"`
	EntityName string           `json:"entityName"`
	Confidence Confidence       `json:"confidence"`
	Evidence   Evidence         `json:"evidence"`
}
