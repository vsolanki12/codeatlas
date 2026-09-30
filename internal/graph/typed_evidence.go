package graph

import (
	"fmt"
	goast "go/ast"
	"go/build"
	"go/importer"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// BuildTypedRelationships enriches source observations using isolated
// production, internal-test, and external-test package variants. Errors are
// reported without discarding partial exact identities or guessing targets.
// Resource observations on entities are upgraded in place before edge building.
func BuildTypedRelationships(root string, files []domain.File, entities []domain.Entity, context domain.BuildContext) ([]domain.Relationship, domain.TypeAnalysisCoverage) {
	var coverage domain.TypeAnalysisCoverage
	coverage.DependencyMode = "repository source; external dependencies use host compiler export data"
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		coverage.Diagnostics = []domain.TypeAnalysisDiagnostic{{Message: err.Error()}}
		return nil, coverage
	}
	buildContext := build.Default
	if context.GOOS != "" {
		buildContext.GOOS = context.GOOS
	}
	if context.GOARCH != "" {
		buildContext.GOARCH = context.GOARCH
	}
	buildContext.BuildTags = append([]string(nil), context.BuildTags...)
	buildContext.CgoEnabled = context.CgoEnabled
	available := make(map[string]bool)
	for _, file := range files {
		if filepath.Ext(file.RelativePath) == ".go" {
			coverage.Files++
			matched, err := buildContext.MatchFile(filepath.Join(rootAbs, filepath.Dir(file.RelativePath)), filepath.Base(file.RelativePath))
			if err == nil && matched {
				available[file.RelativePath] = true
			} else {
				coverage.ExcludedFiles++
			}
		}
	}
	byPackage := make(map[string]map[string]bool)
	entityByFunction := make(map[string]int)
	fieldIDs := make(map[string]bool)
	for i, entity := range entities {
		resetResourceEnrichment(&entities[i])
		entities[i].ReferenceSites = nil
		if entity.Package != "" && available[entity.Source.File] {
			if byPackage[entity.Package] == nil {
				byPackage[entity.Package] = make(map[string]bool)
			}
			byPackage[entity.Package][entity.Source.File] = true
		}
		if entity.Kind == domain.KindPackage {
			for _, file := range entity.Files {
				if available[file] {
					if byPackage[entity.Package] == nil {
						byPackage[entity.Package] = make(map[string]bool)
					}
					byPackage[entity.Package][file] = true
				}
			}
		}
		if entity.Kind == domain.KindFunction {
			entityByFunction[entity.ID] = i
		}
		if entity.Kind == domain.KindTest {
			entityByFunction["function:"+strings.TrimPrefix(entity.ID, "test:")] = i
		}
		if entity.Kind == domain.KindField {
			fieldIDs[entity.ID] = true
		}
	}
	specs := make(map[string]*sourcePackageSpec)
	for packagePath, packageFiles := range byPackage {
		production, internal := make(map[string]bool), make(map[string]bool)
		for file := range packageFiles {
			if !strings.HasSuffix(file, "_test.go") {
				production[file] = true
			}
			internal[file] = true
		}
		external := strings.HasSuffix(packagePath, "_test") && len(production) == 0
		if external {
			specs[packagePath] = &sourcePackageSpec{path: packagePath, files: internal, variant: "external-test"}
		} else {
			if len(production) > 0 {
				specs[packagePath] = &sourcePackageSpec{path: packagePath, files: production, variant: "production"}
			}
			if len(internal) > len(production) {
				specs[packagePath+"#test"] = &sourcePackageSpec{path: packagePath, files: internal, variant: "internal-test"}
			}
		}
	}
	coverage.Packages = len(specs)
	loader := &sourcePackageLoader{root: rootAbs, specs: specs, checked: make(map[string]*typedPackage), checking: make(map[string]bool), attempted: make(map[string]bool), fallback: importer.Default(), buildContext: &buildContext, coverage: &coverage}
	keys := make([]string, 0, len(specs))
	for key := range specs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seenSites := make(map[string]bool)
	byID := make(map[string]domain.Relationship)
	add := func(from, to string, relation domain.RelationshipType, evidence domain.Evidence) {
		if from == to {
			return
		}
		id := domain.NewRelationshipID(from, relation, to)
		if _, ok := byID[id]; !ok {
			byID[id] = domain.Relationship{ID: id, From: from, To: to, Type: relation, Confidence: domain.ConfidenceProven, Evidence: evidence}
		}
	}
	for _, key := range keys {
		checked := loader.check(key)
		if checked == nil || checked.pkg == nil || checked.info == nil {
			continue
		}
		for _, file := range checked.files {
			astFile := checked.astByPath[file]
			if astFile == nil {
				continue
			}
			// Production bodies are processed using their production package once.
			if specs[key].variant == "internal-test" && !strings.HasSuffix(file, "_test.go") {
				continue
			}
			for _, declaration := range astFile.Decls {
				fn, ok := declaration.(*goast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				callerKey := typedFunctionID(checked.pkg.Path(), fn)
				index, ok := entityByFunction[callerKey]
				if !ok {
					continue
				}
				caller := &entities[index]
				goast.Inspect(fn.Body, func(node goast.Node) bool {
					if node == nil {
						return false
					}
					position := checked.fset.Position(node.Pos())
					evidence := domain.Evidence{Parser: "go-types", File: file, Line: position.Line, Snippet: ReadSnippet(filepath.Join(rootAbs, file), position.Line)}
					switch expression := node.(type) {
					case *goast.CallExpr:
						siteKey := fmt.Sprintf("%s:%d", file, position.Offset)
						if seenSites[siteKey] {
							return true
						}
						seenSites[siteKey] = true
						target := typedCallTarget(checked.info, expression)
						_, builtin := checked.info.Uses[callIdentifier(expression)].(*types.Builtin)
						conversion := checked.info.Types[expression.Fun].IsType()
						if target == nil && !builtin && !conversion {
							coverage.UnresolvedCalls++
						} else if target != nil {
							coverage.ResolvedCalls++
						}
						if target != nil {
							if targetIndex, found := entityByFunction[typedFunctionObjectID(target)]; found {
								targetEntity := entities[targetIndex]
								evidence.Reason = "Go type checker resolved the static call target"
								add(caller.ID, targetEntity.ID, domain.RelCalls, evidence)
								if caller.Kind == domain.KindTest {
									evidence.Reason = "test directly invokes the typed function; invocation does not prove assertions or behavior coverage"
									add(targetEntity.ID, caller.ID, domain.RelTestedBy, evidence)
								}
								if fn.Name.Name == "Reconcile" && fn.Recv != nil {
									controllerID := "controller:" + checked.pkg.Path() + "." + typedReceiverName(fn)
									for _, entity := range entities {
										if entity.ID == controllerID {
											add(controllerID, targetEntity.ID, domain.RelCalls, evidence)
											break
										}
									}
								}
							}
						}
						upgradeResourceObservation(caller, checked.info, expression, target, file, position.Line, position.Column)
					case *goast.SelectorExpr:
						selection := checked.info.Selections[expression]
						fieldID := typedFieldID(selection)
						if fieldIDs[fieldID] {
							coverage.ResolvedReferences++
							fieldPosition := checked.fset.Position(expression.Sel.Pos())
							caller.ReferenceSites = append(caller.ReferenceSites, domain.ReferenceSite{Target: fieldID, Source: domain.Source{Parser: "go-types", File: file, Line: fieldPosition.Line, Column: fieldPosition.Column, EndLine: checked.fset.Position(expression.Sel.End()).Line}})
							evidence.Line = fieldPosition.Line
							evidence.Snippet = ReadSnippet(filepath.Join(rootAbs, file), fieldPosition.Line)
							evidence.Reason = "Go type checker resolved the selected field declaration"
							add(caller.ID, fieldID, domain.RelReferences, evidence)
						}
					case *goast.CompositeLit:
						for _, element := range expression.Elts {
							keyValue, ok := element.(*goast.KeyValueExpr)
							if !ok {
								continue
							}
							name, ok := keyValue.Key.(*goast.Ident)
							if !ok {
								continue
							}
							fieldID := typedLiteralFieldID(checked.info.TypeOf(expression), name.Name)
							if !fieldIDs[fieldID] {
								continue
							}
							coverage.ResolvedReferences++
							fieldPosition := checked.fset.Position(name.Pos())
							caller.ReferenceSites = append(caller.ReferenceSites, domain.ReferenceSite{Target: fieldID, Source: domain.Source{Parser: "go-types", File: file, Line: fieldPosition.Line, Column: fieldPosition.Column, EndLine: checked.fset.Position(name.End()).Line}})
							evidence.Line = fieldPosition.Line
							evidence.Snippet = ReadSnippet(filepath.Join(rootAbs, file), fieldPosition.Line)
							evidence.Reason = "Go type checker resolved the keyed struct field declaration"
							add(caller.ID, fieldID, domain.RelReferences, evidence)
						}
					}
					return true
				})
			}
		}
	}
	linkTypedTestHelpers(entities, byID)
	result := make([]domain.Relationship, 0, len(byID))
	for _, relationship := range byID {
		result = append(result, relationship)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	sort.Slice(coverage.Diagnostics, func(i, j int) bool {
		a, b := coverage.Diagnostics[i], coverage.Diagnostics[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Variant != b.Variant {
			return a.Variant < b.Variant
		}
		return a.Message < b.Message
	})
	return result, coverage
}

// A test helper chain is a static invocation association, not execution or
// assertion coverage. Traverse only functions declared in test files and keep
// the final call site's evidence plus the exact helper path in the reason.
func linkTypedTestHelpers(entities []domain.Entity, relationships map[string]domain.Relationship) {
	byEntity := make(map[string]domain.Entity, len(entities))
	calls := make(map[string][]domain.Relationship)
	for _, entity := range entities {
		byEntity[entity.ID] = entity
	}
	for _, relation := range relationships {
		if relation.Type == domain.RelCalls && relation.Confidence == domain.ConfidenceProven {
			calls[relation.From] = append(calls[relation.From], relation)
		}
	}
	for from := range calls {
		sort.Slice(calls[from], func(i, j int) bool { return calls[from][i].ID < calls[from][j].ID })
	}
	type helperPath struct {
		entity string
		path   []string
	}
	for _, test := range entities {
		if test.Kind != domain.KindTest {
			continue
		}
		queue := []helperPath{{entity: test.ID, path: []string{test.ID}}}
		visited := make(map[string]bool)
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if visited[current.entity] || len(current.path) > 6 {
				continue
			}
			visited[current.entity] = true
			for _, call := range calls[current.entity] {
				target, exists := byEntity[call.To]
				if !exists || target.Kind != domain.KindFunction {
					continue
				}
				path := append(append([]string(nil), current.path...), target.ID)
				if current.entity != test.ID {
					id := domain.NewRelationshipID(target.ID, domain.RelTestedBy, test.ID)
					if _, exists := relationships[id]; !exists {
						evidence := call.Evidence
						evidence.Reason = "statically resolved test helper call chain: " + strings.Join(path, " -> ") + "; does not prove execution, assertions, or behavior coverage"
						relationships[id] = domain.Relationship{ID: id, From: target.ID, To: test.ID, Type: domain.RelTestedBy, Confidence: domain.ConfidenceProven, Evidence: evidence}
					}
				}
				if strings.HasSuffix(target.Source.File, "_test.go") {
					queue = append(queue, helperPath{entity: target.ID, path: path})
				}
			}
		}
	}
}

func typedFieldID(selection *types.Selection) string {
	if selection == nil || selection.Kind() != types.FieldVal {
		return ""
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok || !field.IsField() || field.Pkg() == nil {
		return ""
	}
	// Follow embedded-field indices to the declaring named struct.
	current := selection.Recv()
	indices := selection.Index()
	for depth, index := range indices {
		current = types.Unalias(current)
		for {
			pointer, ok := current.(*types.Pointer)
			if !ok {
				break
			}
			current = pointer.Elem()
			current = types.Unalias(current)
		}
		named, ok := current.(*types.Named)
		if !ok {
			return ""
		}
		structure, ok := named.Underlying().(*types.Struct)
		if !ok || index >= structure.NumFields() {
			return ""
		}
		if depth == len(indices)-1 {
			return "field:" + named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + field.Name()
		}
		current = structure.Field(index).Type()
	}
	return ""
}

func typedLiteralFieldID(value types.Type, fieldName string) string {
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
		value = types.Unalias(value)
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	structure, ok := named.Underlying().(*types.Struct)
	if !ok {
		return ""
	}
	for i := 0; i < structure.NumFields(); i++ {
		if structure.Field(i).Name() == fieldName {
			return "field:" + named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + fieldName
		}
	}
	return ""
}

func callIdentifier(call *goast.CallExpr) *goast.Ident {
	identifier, _ := call.Fun.(*goast.Ident)
	return identifier
}

func upgradeResourceObservation(entity *domain.Entity, info *types.Info, call *goast.CallExpr, target *types.Func, file string, line, column int) {
	selector, ok := call.Fun.(*goast.SelectorExpr)
	if !ok {
		return
	}
	method := selector.Sel.Name
	if method != "Create" && method != "CreateOrUpdate" && method != "CreateOrPatch" {
		return
	}
	argument := 1
	if method != "Create" {
		argument = 2
	}
	objectType := ""
	if argument < len(call.Args) {
		if object := info.TypeOf(call.Args[argument]); object != nil {
			objectType = types.TypeString(object, func(pkg *types.Package) string { return pkg.Path() })
			if strings.Contains(objectType, "invalid type") {
				objectType = ""
			}
		}
	}
	for i := range entity.ResourceOperations {
		operation := &entity.ResourceOperations[i]
		if operation.Source.File != file || operation.Source.Line != line || operation.Source.Column != column || operation.Operation != method {
			continue
		}
		if objectType != "" {
			operation.ObjectType = objectType
			operation.Source.Parser = "go-types"
		}
		if target != nil {
			operation.Method = strings.TrimPrefix(typedFunctionObjectID(target), "function:")
			operation.Source.Parser = "go-types"
			// Static identity proves the operation only for supported APIs.
			if target.Pkg() != nil && strings.HasPrefix(target.Pkg().Path(), "sigs.k8s.io/controller-runtime/") {
				operation.Confidence = domain.ConfidenceProven
				operation.Source.Parser = "go-types"
			}
		}
		if objectType != "" {
			kind := namedTypeName(info.TypeOf(call.Args[argument]))
			if kind != "" {
				found := false
				for _, existing := range entity.Creates {
					if existing == kind {
						found = true
						break
					}
				}
				if !found {
					entity.Creates = append(entity.Creates, kind)
					source := operation.Source
					source.Parser = "go-types"
					entity.CreateSites = append(entity.CreateSites, domain.Site{Name: kind, Source: source})
				}
			}
		}
	}
}

// Incremental entities may contain old enrichment. Restore their immutable AST
// observations so an importer failure cannot silently preserve stale types.
func resetResourceEnrichment(entity *domain.Entity) {
	typedSites := false
	sites := entity.CreateSites[:0]
	for _, site := range entity.CreateSites {
		if site.Source.Parser == "go-types" {
			typedSites = true
			continue
		}
		sites = append(sites, site)
	}
	entity.CreateSites = sites
	if typedSites {
		entity.Creates = nil
		for _, site := range sites {
			entity.Creates = append(entity.Creates, site.Name)
		}
	}
	for i := range entity.ResourceOperations {
		operation := &entity.ResourceOperations[i]
		operation.Method = operation.ObservedMethod
		operation.ObjectType = operation.ObservedObjectType
		operation.Source.Parser = "go-ast"
		operation.Confidence = domain.ConfidenceInferred
	}
}
