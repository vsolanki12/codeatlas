# Atlas V1 — Learn Go by Building

**Start:** July 15, 2026 (Days 1-4 done)
**Target:** August 29, 2026
**Daily commitment:** 90 minutes (60 min coding, 20 min review, 10 min commit)
**Goal:** Ship `atlas scan` + HTML viewer. Learn Go from zero to intermediate along the way.

---

## How This Works

- Every day has ONE small task that produces working, tested code
- Go topics come from the code you write that day, never separate theory
- Each day has an interview question to reinforce the concept
- Hint levels reduce as you grow:

| Weeks | Hint Level | What You Get |
|-------|-----------|--------------|
| 1-2 (Days 1-9) | **Detailed** | 5+ hints, syntax examples, bash analogies, almost hand-holding |
| 3-4 (Days 10-19) | **Guided** | 3-4 hints, conceptual direction, you handle syntax |
| 5-6 (Days 20-29) | **Light** | 1-2 hints, just pointers |
| 7 (Days 30-34) | **Solo** | Just the task. You figure it out. |

---

## Milestone Map

| Milestone | Days | Dates | Done When | Capability |
|-----------|------|-------|-----------|------------|
| 1. Discovery | 1-5 | Jul 15-16 | `go test ./internal/discovery/` passes + repo validation rejects invalid paths | **Cap 1 (discovery)** |
| 2. Domain Types + Go Parsing | 6-19 | Jul 17 - Aug 5 | Parser extracts functions, packages, controllers. Classification routes files to parsers. | **Cap 1 (routing) complete at Day 14** |
| 3. Relationships | 20-24 | Aug 6-12 | Every edge has file + line + snippet evidence | Cap 3 |
| 4. Atlas Graph | 25-30 | Aug 13-20 | `atlas scan` → deterministic JSON | Cap 4 |
| 5. Viewer V1 | 31-34 | Jul 18 - TBD | Open HTML, click HostedCluster, see everything | Cap 5 |
| Buffer | — | — | Stuck days, polish, demo prep | — |

**Capability 1 fully complete = Day 14 (Jul 29)** — discovery returns files, validation catches bad paths, Parser interface + classification routes files to correct parser.

---

## Progress Tracker

| Day | Date | Task | Status |
|-----|------|------|--------|
| 1 | Jul 15 (Tue) | Domain types + discovery walker | Done |
| 2 | Jul 15 (Tue) | Understand your own code | Done |
| 3 | Jul 15 (Tue) | iota, custom types (learning exercise — code removed per design) | Done |
| 4 | Jul 15 (Tue) | Table-driven tests for discovery | Done |
| 5 | Jul 16 (Wed) | Repository validation + error handling | Done |
| 6 | Jul 16 (Wed) | Entity struct + Kind enum | Done |
| 7 | Jul 16 (Wed) | Relationship + Evidence structs | Done |
| 8 | Jul 21 (Mon) | Graph struct + JSON tags | Done |
| 9 | Jul 22 (Tue) | JSON marshal: write Graph to file | Done |
| 10 | Jul 23 (Wed) | go/ast basics: parse one file, print function names | Done |
| 11 | Jul 24 (Thu) | Extract function signatures + docs | Done |
| 12 | Jul 25 (Fri) | Methods vs functions: receiver detection | Done |
| 13 | Jul 28 (Mon) | Package extraction | Done |
| 14 | Jul 29 (Tue) | Parser interface + classification routing ← **Cap 1 complete** | Done |
| 15 | Jul 30 (Wed) | Controller detection: find Reconcile() | Done |
| 16 | Jul 31 (Thu) | Controller watches: SetupWithManager | Done |
| 17 | Aug 1 (Fri) | Tests for Go parser | Done |
| 18 | Aug 4 (Mon) | YAML parser: extract CRDs | Done |
| 19 | Aug 5 (Tue) | Markdown + Test parsers | Done |
| 20 | Aug 6 (Wed) | Relationship builder: reconciles, creates | Done |
| 21 | Aug 7 (Thu) | Relationship builder: calls, tested_by | Done |
| 22 | Aug 8 (Fri) | Evidence: extract snippets from source | Done |
| 23 | Aug 11 (Mon) | Deterministic IDs | Done |
| 24 | Aug 12 (Tue) | Relationship tests | Done |
| 25 | Aug 13 (Wed) | Graph validation | Done |
| 26 | Aug 14 (Thu) | JSON storage: Write + Read | Done |
| 27 | Aug 15 (Fri) | Scanner orchestrator | Done |
| 28 | Aug 18 (Mon) | CLI: `atlas scan` command | Done |
| 29 | Jul 18 (Fri) | End-to-end: scan real HyperShift (11,290 entities, 432 rels, 4.3MB JSON) | Done |
| 30 | Jul 18 (Fri) | Viewer: topic-centric HTML, relationship flow diagrams, group filters | Done |
| 31 | Jul 18 (Fri) | Viewer: sub-component discovery, component topics, navigation history | Done |
| 32 | Jul 18 (Fri) | Scanner: CRD descriptions from YAML `openAPIV3Schema.description` | Done |
| 33 | Jul 18 (Fri) | Scanner: Go type doc comments from `ast.GenDecl`/`ast.TypeSpec` | Done |
| 34 | Jul 18 (Fri) | Scanner: Parse Reconcile() body calls + output path fix | Done |

Buffer: Aug 27-29 (Wed-Fri) — 3 days for stuck days or overflow.

---

# MILESTONE 1 — DISCOVERY (Days 1-5)

---

## Day 1 (Jul 15) — DONE

Built: `domain.File`, `domain.Repository`, `discovery.Scan()`.

---

## Day 2 (Jul 15) — DONE

**No new code. Read what you wrote and own every line.**

### Go Topics (from your existing code)
- Functions vs Methods
- Slices (dynamic arrays)
- Multiple return values
- Error handling pattern
- Anonymous functions (callbacks)

### Your Task

Open `internal/discovery/discovery.go`. For each block below, write a one-line comment above it in your own words explaining what it does and why. Then delete the comments before you commit — they're training wheels.

### Hints (Level: Detailed)

**1. The method signature (line 14)**
```go
func (d *Discovery) Scan(repo domain.Repository) ([]domain.File, error) {
```
- `func Scan(repo)` — a standalone function. Anyone can call it: `Scan(repo)`
- `func (d *Discovery) Scan(repo)` — a method on Discovery. You call it: `d.Scan(repo)`
- Think of `d` as Python's `self` or JavaScript's `this`. It's the object calling the method.
- `*Discovery` — the `*` means pointer receiver. The method gets the real object, not a copy. Rule of thumb for now: always use `*`.
- `([]domain.File, error)` — Go functions can return multiple values. The `(result, error)` pattern is THE most common Go pattern. You'll write it hundreds of times.

**2. The slice (line 19)**
```go
var files []domain.File
```
- `[]domain.File` = a list of File structs. The `[]` makes it a slice.
- Bash analogy: like an array, but it grows automatically.
- `var files []domain.File` starts it empty (nil). You add to it with `append()`.
- This is different from `files := []domain.File{}` — both are empty, but nil vs empty-but-initialized. For now, doesn't matter.

**3. Error handling (throughout)**
```go
if err != nil {
    return nil, err
}
```
- Go has NO try/catch. Functions that can fail return error as the last value.
- The caller MUST check: if err is not nil, something went wrong, deal with it here.
- `nil` in Go = nothing/null. `err != nil` means "there IS an error."
- This pattern repeats everywhere. It's verbose on purpose — no error is silently swallowed.

**4. The callback / anonymous function (line 20)**
```go
filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
    // ... your logic for each file
})
```
- `filepath.WalkDir` needs YOU to tell it what to do with each file.
- You pass a function directly — no name, defined right there. Called an anonymous function.
- Bash analogy: `find . -exec command {} \;` — you give `find` a command to run per file. Here you give `WalkDir` a function to run per file.
- `return filepath.SkipDir` = "skip this entire directory." That's how `.git` and `vendor` are excluded.
- `return nil` = "everything is fine, keep going."

**5. append (line 45)**
```go
files = append(files, domain.File{
    RelativePath: relPath,
    Size:         info.Size(),
    ModifiedTime: info.ModTime(),
})
```
- `append` adds an item to a slice and returns the new slice.
- You MUST reassign: `files = append(files, item)`. Without `files = `, the append is silently lost. This is a common beginner bug.
- `domain.File{...}` — creates a new File struct with fields filled in. This is a struct literal.
- `info.Size()` — calling a method on the `info` object. The parentheses `()` mean it's a function call, not a field access.

### Interview Question

> **"In Go, what's the difference between a function and a method? When would you use each?"**

**Key points for your answer:**
- A function is standalone: `func DoThing(x int) error`
- A method is attached to a type: `func (d *Discovery) DoThing(x int) error`
- Use a method when the behavior belongs to a specific type (Discovery does scanning)
- Use a function when the behavior is general (e.g., a utility like `filepath.Ext()`)
- The receiver `(d *Discovery)` is really just a first parameter that Go puts before the function name — syntactic sugar for `DoThing(d *Discovery, x int)`

### Checkpoint

You can explain every line of `discovery.go` without looking at hints.

---

## Day 3 (Jul 15) — DONE

**Learning exercise.** Built `FileKind` enum with `iota`, `ClassifyFile` function, `switch` statement. Code was removed to align with the session-log design (classification is the parser's job, not discovery's). Go concepts learned: `iota`, custom types, `switch`, `strings.HasSuffix`. These concepts reappear on Day 6 when building `EntityKind`.

---

## Day 4 (Jul 15) — DONE

**First Go tests.** Table-driven tests for discovery.

### Go Topics (from today's code)
- `testing` package
- `t.Run()` subtests
- Table-driven test pattern
- `os.MkdirTemp` + `os.WriteFile`
- `defer` for cleanup

### Your Task

Write tests for discovery. Create a temporary directory with specific files, run Scan(), verify results match expectations. Use table-driven test style.

### Hints (Level: Detailed)

**1. Test file location and naming**
- Create `internal/discovery/discovery_test.go`
- Package: `package discovery` (same package = can access unexported fields)
- Test function: `func TestScan(t *testing.T)` — must start with `Test` and take `*testing.T`
- Run with: `go test ./internal/discovery/ -v`

**2. Creating a temp directory**
```go
func TestScan(t *testing.T) {
    dir, err := os.MkdirTemp("", "atlas-test-")
    if err != nil {
        t.Fatal(err)    // Fatal = stop this test immediately
    }
    defer os.RemoveAll(dir)  // cleanup when function exits
}
```
- `defer` runs when the function returns — guaranteed cleanup even if the test fails.
- Bash analogy: like setting a trap in bash (`trap 'rm -rf $tmpdir' EXIT`).

**3. Creating test files**
```go
os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main"), 0644)
os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(""), 0644)
os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0644)
os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(""), 0644)

// Create a subdirectory to skip
os.MkdirAll(filepath.Join(dir, ".git"), 0755)
os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0644)
```

**4. Table-driven pattern**
```go
expected := []struct {
    path string
}{
    {"main.go"},
    {"main_test.go"},
    {"config.yaml"},
    {"README.md"},
    {"notes.txt"},
}

for _, tc := range expected {
    t.Run(tc.path, func(t *testing.T) {
        found := false
        for _, f := range files {
            if f.RelativePath == tc.path {
                found = true
            }
        }
        if !found {
            t.Errorf("File %s not found in scan results", tc.path)
        }
    })
}
```
- Each test case is a struct in a slice. Adding a new case = one line.
- `t.Run(name, func)` creates a subtest — failures report which case broke.
- `tc` stands for "test case" — common Go convention.
- `_` ignores the index (you don't need it).

**5. Checking results**
```go
if got != want {
    t.Errorf("got %d, want %d", got, want)
    // Errorf = report failure but keep running other checks
    // Fatalf = report failure and stop
}
```
- Go tests don't use assert libraries by default. Just `if got != want`.
- `t.Errorf` for "this is wrong but keep checking", `t.Fatalf` for "stop here."

### Interview Question

> **"What are table-driven tests in Go and why does the community prefer them?"**

**Key points:**
- A slice of test cases iterated with `t.Run()` — each case gets its own named subtest.
- Adding a new test case is one line, not a new function — reduces boilerplate.
- Failures show the exact case name: `--- FAIL: TestScan/main.go`.
- You can run a single case: `go test -run TestScan/main.go`.
- Used extensively in the Go standard library and in HyperShift tests.

### Checkpoint

`go test ./internal/discovery/ -v` shows named subtests, all passing. 5 test cases verifying discovered file paths.

---

## Day 5 (Jul 16) — Repository Validation + Error Handling

**Capability 1 discovery part completes today.** Discovery should reject bad inputs, not silently fall back.

### Go Topics (from today's code)
- Creating errors with `fmt.Errorf`
- `os.Stat` to check if a path exists
- `FileInfo.IsDir()` to check if it's a directory
- Constructor pattern: `New()` function
- Testing error cases (expecting errors in tests)

### Your Task

Add a `New(repo domain.Repository) (*Discovery, error)` constructor that validates `RootPath` exists and is a directory. Write tests for invalid paths, non-directory paths, and valid paths. Remove the `root == "" { root = "." }` fallback — empty path should be an error.

### Hints (Level: Detailed)

**1. The constructor pattern**
```go
func New(repo domain.Repository) (*Discovery, error) {
    // validate RootPath
    // return &Discovery{...}, nil on success
    // return nil, error on failure
}
```
- Go doesn't have constructors like Java/Python. Convention is a `New()` function that returns a pointer and an error.
- The caller does: `d, err := discovery.New(repo)` — same `(result, error)` pattern.

**2. Checking if a path exists**
```go
info, err := os.Stat(repo.RootPath)
if err != nil {
    return nil, fmt.Errorf("invalid repository path %q: %w", repo.RootPath, err)
}
```
- `os.Stat` returns file info and an error. If the path doesn't exist, `err` is non-nil.
- `%q` prints the string in quotes — helpful for debugging empty strings.
- `%w` wraps the original error — callers can check the underlying cause with `errors.Is()`.

**3. Checking if it's a directory**
```go
if !info.IsDir() {
    return nil, fmt.Errorf("repository path %q is not a directory", repo.RootPath)
}
```

**4. Creating custom errors with `fmt.Errorf`**
- `fmt.Errorf("message: %w", err)` — creates a new error that wraps the original.
- Bash analogy: like `echo "ERROR: $message" >&2; exit 1` — you format the message and signal failure.

**5. Testing error cases**
```go
func TestNewInvalidPath(t *testing.T) {
    _, err := New(domain.Repository{RootPath: "/nonexistent/path"})
    if err == nil {
        t.Fatal("expected error for nonexistent path, got nil")
    }
}

func TestNewFilePath(t *testing.T) {
    // Create a temp file (not directory)
    f, _ := os.CreateTemp("", "atlas-test-")
    defer os.Remove(f.Name())
    f.Close()

    _, err := New(domain.Repository{RootPath: f.Name()})
    if err == nil {
        t.Fatal("expected error for file path, got nil")
    }
}

func TestNewEmptyPath(t *testing.T) {
    _, err := New(domain.Repository{RootPath: ""})
    if err == nil {
        t.Fatal("expected error for empty path, got nil")
    }
}
```
- Testing errors means checking that `err != nil` when you expect failure.
- `t.Fatal` stops the test — if the error check fails, there's nothing else to verify.

### Interview Question

> **"How do you create and wrap errors in Go? What is `fmt.Errorf` with `%w`?"**

**Key points:**
- `fmt.Errorf("context: %w", err)` creates a new error wrapping the original.
- `errors.Is(err, os.ErrNotExist)` checks if any error in the chain matches.
- `errors.Unwrap(err)` gets the wrapped error.
- This replaced the old `pkg/errors` package. Since Go 1.13, wrapping is built in.
- Always add context when wrapping: "what were you trying to do when this failed?"

### Checkpoint

- `New()` returns error for: empty path, nonexistent path, file (not directory)
- `New()` returns `*Discovery` for valid directory
- `Scan()` uses the validated repo from the constructor (no more `root == ""` fallback)
- `go test ./internal/discovery/ -v` — all tests pass, including error cases

**After this day: Milestone 1 is complete. Capability 1 (discovery part) is done.**

---

# MILESTONE 2 — DOMAIN TYPES + GO PARSING (Days 6-19)

**Capability 1 is fully complete at Day 14** when the Parser interface + classification routing is built.

---

## Day 6 (Jul 17) — Entity Struct + Kind Enum

### Go Topics
- Struct composition (nested structs)
- Constants with `iota` (you learned this on Day 3 — now for real, in the right place)
- String methods on custom types (`func (k Kind) String() string`)
- Pointer vs value: when to use `*`

### Your Task
Create `internal/domain/entity.go`. Define the `Entity` struct matching your data model spec. Define `EntityKind` with iota (operator, controller, crd, function, package, test, document, resource). Add a `String()` method to `EntityKind`.

### Hints (Level: Detailed)
1. Look at your `data-model.md` — the Entity fields are already defined. Translate them to Go struct fields.
2. `source` is a nested struct — define `type Source struct { Parser string; File string; Line int }` and use it inside Entity.
3. Kind-specific fields (like `reconciles` for controllers) — use pointer types for optional fields: `Reconciles *string` means "might be nil."
4. A `String()` method makes your type printable: `fmt.Println(myKind)` calls `String()` automatically. This is Go's version of `toString()`.
5. File naming: one concept per file. `entity.go` for Entity, `files.go` for File. Don't cram everything into one file.

### Interview Question
> **"What is the `Stringer` interface in Go and why is implementing `String()` useful?"**

Key points: The `fmt` package checks if a type has `String() string`. If it does, `fmt.Println(x)` calls it automatically. It's an implicit interface — you don't declare `implements Stringer`. You just define the method and it works.

### Checkpoint
`go build ./internal/domain/` passes. You can create an Entity in a test and print it.

---

## Day 7 (Jul 18) — Relationship + Evidence Structs

### Go Topics
- More struct composition
- Enum with string values (vs int iota)
- Slice of structs as fields

### Your Task
Create `internal/domain/relationship.go`. Define `Relationship`, `Evidence`, `RelationshipType`, and `Confidence` types. Follow your data-model.md spec exactly.

### Hints (Level: Detailed)
1. `RelationshipType` can be string-based instead of int-based: `type RelationshipType string` with `const RelReconciles RelationshipType = "reconciles"`. String enums are easier to debug (print "reconciles" not "3").
2. `Evidence` struct: parser, file, line, snippet, reason — snippet and reason are optional, so just use `string` (empty string = not set, simpler than pointers for now).
3. `Confidence`: just two values — `ConfidenceProven` and `ConfidenceInferred`. Your data model says "two levels only, no percentages."
4. The relationship ID format from your spec: `from--type--to`. Add a function `func NewRelationshipID(from, relType, to string) string` that builds this.

### Interview Question
> **"When would you use `type X string` vs `type X int` for constants in Go?"**

Key points: String constants are self-documenting in JSON and logs ("reconciles" vs "3"). Int constants with iota are faster for comparisons and switches. Use string when the value is serialized or human-visible. Use int when it's internal-only.

### Checkpoint
Can create a Relationship with Evidence and print it. `NewRelationshipID` produces deterministic IDs.

---

## Day 8 (Jul 21) — Graph Struct + JSON Tags

### Go Topics
- JSON struct tags (`json:"fieldName"`)
- `omitempty`
- Exported vs unexported fields (capital letter)

### Your Task
Create `internal/domain/graph.go`. Define the `Graph` struct — the top-level container from your data model. It holds metadata (schema, version, commit, branch) and slices of entities and relationships. Add JSON tags to ALL domain structs (entity.go, relationship.go, graph.go).

### Hints (Level: Detailed)
1. JSON tags control how Go struct fields map to JSON keys:
   `Name string \`json:"name"\`` → `{"name": "..."}` in JSON.
2. `omitempty` skips the field if it's zero/empty:
   `Description string \`json:"description,omitempty"\`` — if empty string, won't appear in JSON.
3. Capitalized fields (`Name`) are exported (public). Lowercase (`name`) are unexported (private to package). JSON marshal only works with exported fields.
4. Add JSON tags to `entity.go` and `relationship.go` as well.
5. Test it: create a small Graph with one entity, marshal to JSON with `json.MarshalIndent(graph, "", "  ")`, and print it. Does it match the schema in your data-model.md?

### Interview Question
> **"What are struct tags in Go? What happens if a struct field is unexported — can it be marshaled to JSON?"**

Key points: Struct tags are metadata strings after a field declaration. The `json` package reads them to know the JSON key name. Unexported fields (lowercase) are invisible to `encoding/json` — they're silently skipped. This is a common gotcha for beginners.

### Checkpoint
`json.MarshalIndent` on a Graph produces JSON that matches your data-model.md schema structure.

---

## Day 9 (Jul 22) — Write Graph to File

### Go Topics
- `os.WriteFile` / `os.ReadFile`
- `json.Marshal` / `json.Unmarshal`
- Creating the storage package
- Import paths

### Your Task
Create `internal/storage/storage.go`. Two functions: `WriteGraph(path string, g domain.Graph) error` and `ReadGraph(path string) (domain.Graph, error)`. Write JSON to a file, read it back. Write a test that round-trips: write → read → compare.

### Hints (Level: Detailed)
1. `json.MarshalIndent(graph, "", "  ")` — the indent makes the JSON human-readable. The first `""` is the prefix (leave empty), `"  "` is two spaces per indent level.
2. `os.WriteFile(path, data, 0644)` — writes bytes to a file. `0644` is the permission (readable by everyone, writable by owner — same as bash).
3. `json.Unmarshal(data, &graph)` — the `&` passes a pointer to graph. Unmarshal fills it in. Without `&`, it can't modify your variable.
4. For the test: write a Graph, read it back, compare field by field. Don't compare JSON strings — compare the Go structs.
5. Import path for your own packages: `"github.com/vsolanki12/hypershift-atlas/internal/domain"` — this matches your `go.mod` module name.

### Interview Question
> **"What does the `&` operator do in Go? What's the difference between passing `graph` vs `&graph` to a function?"**

Key points: `&graph` gives the memory address (a pointer). The function can modify the original. Just `graph` passes a copy — changes inside the function don't affect the caller. `json.Unmarshal` needs `&graph` because it fills in the struct's fields — without the pointer, it would fill a copy and your original stays empty.

### Checkpoint
Test passes: WriteGraph creates a file, ReadGraph reads it back, entities and relationships match. The JSON file is human-readable.

---

## Day 10 (Jul 23) — go/ast Basics: Parse One File

**This is the hardest conceptual jump so far. Take your time.**

### Go Topics
- `go/parser` and `go/token` packages
- Abstract Syntax Tree (what it is)
- `ast.Inspect` — walking a tree
- Type assertions

### Your Task
Create `internal/parser/goparser.go`. Write a function that takes a single .go file path, parses it with `go/parser`, and prints every function name it finds. No entities yet — just print names.

### Hints (Level: Guided — starting to reduce)
1. `go/parser.ParseFile(fset, path, nil, parser.ParseComments)` — parses one Go file into an AST. `fset` is a `token.NewFileSet()` (tracks line numbers). The `nil` means "read from disk." `ParseComments` includes doc comments.
2. `ast.Inspect(file, func(n ast.Node) bool { ... })` — walks every node in the tree. Your function is called once per node. Return `true` to keep walking children.
3. To find functions: check if the node is a `*ast.FuncDecl` using a type assertion: `if fn, ok := n.(*ast.FuncDecl); ok { ... }`. Then `fn.Name.Name` is the function name.
4. Test it on a real file — your own `discovery.go`. It should find `Scan`.

### Interview Question
> **"What is a type assertion in Go? What happens if the assertion fails?"**

Key points: `x.(T)` checks if interface value `x` holds type `T`. Two forms: `v := x.(T)` panics if wrong. `v, ok := x.(T)` returns false in `ok` if wrong — always use this form. Used when you have an interface and need the concrete type underneath.

### Checkpoint
Run your parser on `discovery.go` — it prints `Scan`.

---

## Day 11 (Jul 24) — Extract Function Signatures + Docs

### Go Topics
- Working with AST node fields
- String formatting with `fmt.Sprintf`
- GoDoc extraction

### Your Task
Extend your parser to extract full function info: name, file, line number, signature string, and doc comment. Return `[]domain.Entity` with `kind: function` instead of printing.

### Hints (Level: Guided)
1. Line number: `fset.Position(fn.Pos()).Line` gives you the line number from the AST position.
2. Signature: build it from AST fields — `fn.Recv` (receiver), `fn.Name` (name), `fn.Type.Params` (parameters), `fn.Type.Results` (return types). Or use `go/format` to print the declaration.
3. Doc comment: `fn.Doc.Text()` gives the GoDoc. Check `fn.Doc != nil` first.
4. Each function becomes an Entity with `Kind: EntityKindFunction`. Use your domain types from Day 6.

### Interview Question
> **"How does the Go toolchain use the AST? Name two tools built on go/ast."**

Key points: `go vet` (static analysis), `gofmt` (formatting), `guru` (code navigation), `gopls` (language server), and code generators like `stringer`. The AST is the parsed representation of source code as a tree — tools operate on the tree instead of raw text.

### Checkpoint
Parser returns a slice of function entities with correct names, line numbers, and signatures. Tested against a known .go file.

---

## Day 12 (Jul 25) — Methods vs Functions: Receiver Detection

### Go Topics
- Method receivers in AST (`fn.Recv`)
- Pointer receivers vs value receivers
- Building entity IDs from receiver + name

### Your Task
Distinguish methods from standalone functions. If a function has a receiver (`fn.Recv != nil`), extract the receiver type name. Use it to build the entity ID: `function:pkg.ReceiverType.MethodName` for methods, `function:pkg.FunctionName` for standalone functions.

### Hints (Level: Guided)
1. `fn.Recv` is the receiver field list. For `func (r *HostedClusterReconciler) Reconcile(...)`, the receiver is `*HostedClusterReconciler`.
2. The receiver type is nested in the AST: `fn.Recv.List[0].Type`. It might be `*ast.Ident` (value receiver) or `*ast.StarExpr` (pointer receiver — the `*` adds a layer).
3. For pointer receiver: `starExpr.X.(*ast.Ident).Name` gets "HostedClusterReconciler" from `*HostedClusterReconciler`.
4. This is the foundation for controller detection on Day 15 — if the receiver type has a `Reconcile` method, it's a controller.

### Interview Question
> **"What's the difference between a value receiver and a pointer receiver in Go? When should you use each?"**

Key points: Value receiver (`func (d Discovery)`) gets a copy — can't modify the original. Pointer receiver (`func (d *Discovery)`) gets the real thing — can modify it. Use pointer when: the method modifies state, the struct is large (avoid copying), or consistency (if one method needs pointer, use pointer for all). In practice, almost always pointer.

### Checkpoint
Parser correctly identifies methods (with receiver) and functions (without). Entity IDs follow your data model's format.

---

## Days 13-19 (Jul 28 - Aug 5) — Remaining Parsers

From here, hint level drops to **Guided** (3-4 hints per day). Same format but you're handling more syntax on your own.

### Day 13 (Jul 28) — Package Extraction
- Go topic: `ast.File.Name` (package name), directory-level concepts
- Task: Extract package entities — name, path, list of Go files, list of test files
- Interview Q: "What's the difference between a package name and an import path in Go?"

### Day 14 (Jul 29) — Parser Interface + Classification Routing ← **CAPABILITY 1 COMPLETE**
- Go topic: Interfaces, the `Parser` interface pattern
- Task: Define `type Parser interface { Parse(file domain.File) ([]domain.Entity, error) }`. Make GoParser implement it. Add classification routing — a function that takes `[]domain.File` and groups them by extension, routing each group to the correct parser.
- This completes Capability 1: Discovery walks → Classification routes → Parser receives only its files.
- Interview Q: "How are interfaces implemented in Go? Do you need to declare 'implements'?"

### Day 15 (Jul 30) — Controller Detection
- Go topic: Combining AST patterns, struct analysis
- Task: Find structs that have a `Reconcile(context.Context, ctrl.Request)` method. Create controller entities.
- Interview Q: "How would you find all types that implement a specific interface using the AST?"

### Day 16 (Jul 31) — Controller Watches
- Go topic: Analyzing function bodies in AST, looking for specific call patterns
- Task: Find `SetupWithManager` and extract `For()`, `Watches()`, `Owns()` calls to determine watched resources.
- Interview Q: "What is the builder pattern and how does controller-runtime use it for SetupWithManager?"

### Day 17 (Aug 1) — Go Parser Tests
- Go topic: Test fixtures, `testdata/` directory convention
- Task: Create small .go files in `testdata/`, run parser, verify entities match expectations.
- Interview Q: "What is the `testdata` directory convention in Go?"

### Day 18 (Aug 4) — YAML Parser
- Go topic: Third-party packages, `go get`, `yaml.Unmarshal`
- Task: Parse YAML files, extract CRDs (look for `kind: CustomResourceDefinition`), Deployments, Services.
- Interview Q: "How does Go's module system work? What does `go get` vs `go mod tidy` do?"

### Day 19 (Aug 5) — Markdown + Test Parsers
- Go topic: `bufio.Scanner`, line-by-line file reading, naming convention matching
- Task: Markdown parser extracts headings. Test parser matches `_test.go` files to the entities they test.
- Interview Q: "How do you read a file line by line in Go?"

---

# MILESTONE 3 — RELATIONSHIPS (Days 20-24)

Hint level: **Light** (1-2 hints per day)

### Day 20 (Aug 6) — Relationship Builder: reconciles, creates
- Task: Build relationship builder. When a controller reconciles a CRD, emit a `reconciles` edge with evidence.
- Interview Q: "What is a graph edge? How would you represent a directed graph in Go?"

### Day 21 (Aug 7) — Relationship Builder: calls, tested_by
- Task: Add `calls` relationships from function call analysis. Add `tested_by` from test parser matching.
- Interview Q: "What's the difference between a map and a slice in Go? When is each appropriate?"

### Day 22 (Aug 8) — Evidence: Extract Snippets
- Task: When creating a relationship, read the source file and extract 1-2 lines around the evidence line.
- Interview Q: "How do you read specific lines from a file in Go?"

### Day 23 (Aug 11) — Deterministic IDs
- Task: Implement ID generation per your data model spec. Same commit → same IDs. Write tests proving determinism.
- Interview Q: "What does 'deterministic' mean in software? Why is it important for tools like Atlas?"

### Day 24 (Aug 12) — Relationship Tests
- Task: Test the relationship builder with known entities. Verify correct edges, correct evidence.
- Interview Q: "How do you test code that depends on other packages in Go? What's the difference between unit and integration tests?"

---

# MILESTONE 4 — ATLAS GRAPH (Days 25-30)

Hint level: **Light → Solo**

### Day 25 (Aug 13) — Graph Validation
- Task: Validate the graph: unique IDs, no orphan references, all relationship targets exist. Return aggregated errors.
- Interview Q: "How do you aggregate multiple errors in Go?"

### Day 26 (Aug 14) — JSON Storage
- Task: Wire up your Day 9 storage with the real Graph. Include schema version, commit hash, scan stats.
- Interview Q: "What is schema versioning and why does it matter for data formats?"

### Day 27 (Aug 15) — Scanner Orchestrator
- Task: Create `internal/scanner/scanner.go`. Orchestrate: discover → classify → parse → build relationships → validate → write JSON.
- Interview Q: "What is the single responsibility principle? How does the scanner package follow it?"

### Day 28 (Aug 18) — CLI: `atlas scan`
- Task: Create `cmd/atlas/main.go`. Use cobra (or just `flag` package) for `atlas scan /path`. Call scanner, print summary.
- Interview Q: "What is cobra in Go? How do CLI tools typically handle subcommands?"

### Day 29 (Jul 18) — DONE

**End-to-end: scan real HyperShift.** Pointed `atlas scan` at the full HyperShift repo. Produced 4.3MB JSON with 11,290 entities and 432 relationships.

---

### Day 30 (Jul 18) — DONE

**Viewer: topic-centric HTML.** Built `web/viewer.html` — loads atlas-graph.json, shows CRDs as browsable topics grouped by project (HyperShift, CAPI, Karpenter, etc.), relationship flow diagrams (controller → reconciles → CRD → creates → resources), clickable entity details, document ToC rendering, GitHub source links.

---

### Day 31 (Jul 18) — DONE

**Viewer: sub-components + navigation.** Added v2/ control plane component discovery (etcd, KAS, oauth, etc.) as top-level browsable topics. Implemented navigation history stack for proper Back button behavior. Added fallback controller matching by name and package.

---

# MILESTONE 5 — SCANNER IMPROVEMENTS (Days 32-34)

Hint level: **Solo** — just the task.

### Day 32 (Jul 18) — DONE

**Scanner: CRD descriptions from YAML.** Updated `yamlparser.go` to extract `spec.versions[].schema.openAPIV3Schema.description`. Result: 114/116 CRDs now have descriptions (up from 0).

---

### Day 33 (Jul 18) — DONE

**Scanner: Go type doc comments.** Updated `goparser.go` to extract doc comments from `ast.GenDecl`/`ast.TypeSpec` and attach them to controller entities. Result: 18/47 controllers now have descriptions (those with doc comments on their struct).

---

### Day 34 (Jul 18) — DONE

**Scanner: Reconcile() body calls.** Walk `Reconcile()` function bodies, extract `ast.CallExpr` targets (both `selector.Method()` and bare `Function()` calls), store them in a new `Calls` field on controller entities. Relationship builder emits `calls` edges to matching function entities. Also fixed scanner output path bug (relative paths resolved against repo dir after `os.Chdir`).

Result: 936 relationships (up from 432). New breakdown: 504 calls, 394 tested_by, 20 creates, 18 reconciles. All 47 controllers have calls data.

**The original 34-day learning plan is now complete.** All 68 tests pass across 7 packages.

---

# Quick Reference

## Go Concepts by Day

| Day | Concept | One-liner |
|-----|---------|-----------|
| 2 | Functions vs Methods | Methods have a receiver, functions don't |
| 2 | Slices | `[]T` is a dynamic list |
| 2 | Error handling | `if err != nil` — every time, no exceptions |
| 2 | Anonymous functions | Functions without names, passed as arguments |
| 3 | iota | Auto-incrementing constants for fake enums |
| 3 | Custom types | `type FileKind int` — new type from existing |
| 3 | switch | Pattern matching, no fall-through in Go |
| 4 | testing package | `func TestX(t *testing.T)` + `t.Run` subtests |
| 4 | defer | Cleanup that runs when function exits |
| 5 | fmt.Errorf + %w | Error creation and wrapping |
| 5 | os.Stat | Check if path exists, get file info |
| 5 | Constructor pattern | `New()` returns `(*T, error)` |
| 6 | Struct composition | Structs inside structs |
| 6 | Stringer interface | `String()` method for printing |
| 7 | String-based enums | `type X string` vs `type X int` |
| 8 | JSON struct tags | `json:"name,omitempty"` |
| 8 | Exported vs unexported | Capital = public, lowercase = private |
| 9 | Pointers (& and *) | `&x` = address of x, `*p` = value at address p |
| 10 | go/ast | Parsed Go source as a tree |
| 10 | Type assertions | `x.(T)` — check what's inside an interface |
| 11 | fmt.Sprintf | String formatting |
| 12 | Receiver types | Value vs pointer receiver |
| 14 | Interfaces | Implicit — just define the methods |
| 18 | go modules | go.mod, go get, go mod tidy |
