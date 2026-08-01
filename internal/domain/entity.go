// Package domain defines the core data types for the Atlas graph.
package domain

import "strings"

// Source records where in the codebase an entity or relationship was discovered, including the parser used, the file path, and the line number.
type Source struct {
	Parser string `json:"parser"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// EntityKind categorizes what an entity represents in the Atlas graph.
type EntityKind int

// Standard entity kinds discovered by Atlas.
const (
	KindOperator EntityKind = iota
	KindController
	KindCRD
	KindFunction
	KindPackage
	KindTest
	KindDocument
	KindResource
	KindUnknown
)

// Entity is a single discovered element in the codebase, such as controller, CRD,
// function, package, test, document, or other component that Atlas tracks.
type Entity struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        EntityKind `json:"kind"`
	Description string     `json:"description,omitempty"`
	Package     string     `json:"package,omitempty"`
	Files       []string   `json:"files,omitempty"`
	Watches    []string   `json:"watches,omitempty"`
	Calls      []string   `json:"calls,omitempty"`
	Implements []string   `json:"implements,omitempty"`
	EnvVars      []string `json:"env_vars,omitempty"`
	Imports      []string `json:"imports,omitempty"`
	Literals     []string `json:"literals,omitempty"`
	Properties   []string `json:"properties,omitempty"`
	Embeds       []string `json:"embeds,omitempty"`
	LastAuthor   string   `json:"lastAuthor,omitempty"`
	LastModified string   `json:"lastModified,omitempty"`
	ChangeCount  int      `json:"changeCount,omitempty"`
	Content      string   `json:"content,omitempty"`
	Source      Source     `json:"source"`
}

// String returns the lowercase name of the entity kind.
func (k EntityKind) String() string {
	name := [...]string{
		"operator",
		"controller",
		"crd",
		"function",
		"package",
		"test",
		"document",
		"resource",
		"unknown",
	}
	if int(k) < len(name) {
		return name[k]
	}
	return "unknown"
}

// MarshalJSON writes the entity kind as a lowercase JSON string.
func (k EntityKind) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.String() + `"`), nil
}

// UnmarshalJSON parses a lowercase JSON string back into an EntityKind.
func (k *EntityKind) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	switch s {
	case "operator":
		*k = KindOperator
	case "controller":
		*k = KindController
	case "crd":
		*k = KindCRD
	case "function":
		*k = KindFunction
	case "package":
		*k = KindPackage
	case "test":
		*k = KindTest
	case "document":
		*k = KindDocument
	case "resource":
		*k = KindResource
	default:
		*k = KindUnknown
	}
	return nil
}
