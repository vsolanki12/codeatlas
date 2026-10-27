// Package domain defines the core data types for the Atlas graph.
package domain

import "strings"

// Source records where in the codebase an entity or relationship was discovered, including the parser used, the file path, and the line number.
type Source struct {
	Parser  string `json:"parser"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
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
	KindTemplate
	KindUnknown
	KindField
	KindType
	KindInterface
)

// Entity is a single discovered element in the codebase, such as controller, CRD,
// function, package, test, document, or other component that Atlas tracks.
type Entity struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Kind                EntityKind          `json:"kind"`
	Description         string              `json:"description,omitempty"`
	Generated           bool                `json:"generated,omitempty"`
	ResourceOperations  []ResourceOperation `json:"resourceOperations,omitempty"`
	ReferenceSites      []ReferenceSite     `json:"referenceSites,omitempty"`
	Package             string              `json:"package,omitempty"`
	Files               []string            `json:"files,omitempty"`
	ReconcileMethod     string              `json:"reconcileMethod,omitempty"`
	Watches             []string            `json:"watches,omitempty"`
	WatchMethods        []string            `json:"watchMethods,omitempty"`
	WatchSites          []Site              `json:"watchSites,omitempty"`
	Creates             []string            `json:"creates,omitempty"`
	CreateSites         []Site              `json:"createSites,omitempty"`
	Calls               []string            `json:"calls,omitempty"`
	CallSites           []Site              `json:"callSites,omitempty"`
	Implements          []string            `json:"implements,omitempty"`
	ImplementationSites []Site              `json:"implementationSites,omitempty"`
	EnvVars             []string            `json:"env_vars,omitempty"`
	Imports             []string            `json:"imports,omitempty"`
	ImportSites         []Site              `json:"importSites,omitempty"`
	Literals            []string            `json:"literals,omitempty"`
	Properties          []string            `json:"properties,omitempty"`
	Embeds              []string            `json:"embeds,omitempty"`
	EmbedSites          []Site              `json:"embedSites,omitempty"`
	LastAuthor          string              `json:"lastAuthor,omitempty"`
	LastModified        string              `json:"lastModified,omitempty"`
	ChangeCount         int                 `json:"changeCount,omitempty"`
	Content             string              `json:"content,omitempty"`
	Source              Source              `json:"source"`
}

// Site records the exact source location associated with a named fact on an
// entity, such as a call, watch, or //go:embed pattern.
type Site struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
}

// ReferenceSite preserves each exact typed field usage, including repeated
// references that share one entity-to-field relationship. It proves source
// identity only, without interpreting reads, writes, or assertions.
type ReferenceSite struct {
	Target string `json:"target"`
	Source Source `json:"source"`
}

// QualifiedName returns the scoped display name for an entity, such as
// "canary/reconciler" or "v3rpc/quotaLeaseServer.LeaseServer".
//
// Repositories legitimately declare the same name in many packages — every
// OpenShift operator controller package declares its own unexported
// reconciler — which leaves the bare name ambiguous and unusable for lookup.
// A field or method is declared inside a type rather than directly in the
// package, so it is qualified by its declaring type as well; without that, a
// field named after its own type collides with the type itself.
//
// The result depends only on this entity, so adding or removing other
// entities never renames it. It is empty when it would add nothing.
func QualifiedName(id, packagePath, name string) string {
	if packagePath == "" || name == "" {
		return ""
	}
	segment := packagePath
	if index := strings.LastIndex(segment, "/"); index >= 0 {
		segment = segment[index+1:]
	}
	if segment == "" {
		return ""
	}
	if owner := declaringType(id, packagePath, name); owner != "" {
		return segment + "/" + owner + "." + name
	}
	if strings.EqualFold(segment, name) {
		return ""
	}
	return segment + "/" + name
}

// declaringType reads the owning type out of a member entity's deterministic
// ID. It returns empty for anything declared directly in a package.
func declaringType(id, packagePath, name string) string {
	colon := strings.Index(id, ":")
	if colon < 0 {
		return ""
	}
	remainder, found := strings.CutPrefix(id[colon+1:], packagePath+".")
	if !found {
		return ""
	}
	owner, member, found := strings.Cut(remainder, ".")
	if !found || owner == "" || member != name {
		return ""
	}
	return owner
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
		"template",
		"unknown",
		"field",
		"type",
		"interface",
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
	case "template":
		*k = KindTemplate
	case "field":
		*k = KindField
	case "type":
		*k = KindType
	case "interface":
		*k = KindInterface
	default:
		*k = KindUnknown
	}
	return nil
}
