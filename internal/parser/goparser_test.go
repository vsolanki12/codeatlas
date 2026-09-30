package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestGoParserForRepo_EmitsControllerFromSetupFile(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/repo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	setupPath := filepath.Join(repo, "controllers", "setup.go")
	if err := os.MkdirAll(filepath.Dir(setupPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(setupPath, []byte(`package controllers

type Reconciler struct{}
type Widget struct{}
type Builder struct{}

func (r *Reconciler) SetupWithManager() error {
	return NewBuilder().For(&Widget{}).Complete(r)
}

func NewBuilder() *Builder { return &Builder{} }
func (b *Builder) For(obj interface{}) *Builder { return b }
func (b *Builder) Complete(obj interface{}) error { return nil }
`), 0644); err != nil {
		t.Fatal(err)
	}

	entities, err := NewGoParserForRepo(repo).Parse(domain.File{RelativePath: "controllers/setup.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, entity := range entities {
		if entity.Kind != domain.KindController {
			continue
		}
		if entity.ID != "controller:example.com/repo/controllers.Reconciler" {
			t.Fatalf("controller ID = %q, want repository-qualified ID", entity.ID)
		}
		if len(entity.Watches) != 1 || entity.Watches[0] != "Widget" {
			t.Fatalf("controller watches = %v, want [Widget]", entity.Watches)
		}
		if len(entity.WatchMethods) != 1 || entity.WatchMethods[0] != "For" {
			t.Fatalf("controller watch methods = %v, want [For]", entity.WatchMethods)
		}
		if len(entity.WatchSites) != 1 || entity.WatchSites[0].Source.File != "controllers/setup.go" {
			t.Fatalf("controller watch sites = %+v, want setup.go evidence", entity.WatchSites)
		}
		return
	}
	t.Fatal("expected a controller entity from SetupWithManager file")
}

func TestGoParserForRepo_EmitsEveryControllerInOneFile(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/repo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "controllers", "controllers.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`package controllers

type FirstReconciler struct{}
type SecondReconciler struct{}
type FirstResource struct{}
type SecondResource struct{}

func (r *FirstReconciler) Reconcile() {
	firstHelper()
}

func (r *FirstReconciler) SetupWithManager(mgr interface{}) error {
	return NewBuilder().For(&FirstResource{}).Complete(r)
}

func (r *SecondReconciler) Reconcile() {
	secondHelper()
}

func (r *SecondReconciler) SetupWithManager(mgr interface{}) error {
	return NewBuilder().For(&SecondResource{}).Complete(r)
}

func firstHelper()  {}
func secondHelper() {}
func NewBuilder() *Builder { return &Builder{} }
type Builder struct{}
func (b *Builder) For(obj interface{}) *Builder { return b }
func (b *Builder) Complete(obj interface{}) error { return nil }
`), 0644); err != nil {
		t.Fatal(err)
	}

	entities, err := NewGoParserForRepo(repo).Parse(domain.File{RelativePath: "controllers/controllers.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	controllers := make(map[string]domain.Entity)
	for _, entity := range entities {
		if entity.Kind == domain.KindController {
			controllers[entity.Name] = entity
		}
	}
	if len(controllers) != 2 {
		t.Fatalf("controller count = %d, want 2: %+v", len(controllers), controllers)
	}

	checks := map[string]string{
		"FirstReconciler":  "FirstResource",
		"SecondReconciler": "SecondResource",
	}
	for name, watch := range checks {
		controller, ok := controllers[name]
		if !ok {
			t.Fatalf("missing controller %q", name)
		}
		if len(controller.Watches) != 1 || controller.Watches[0] != watch {
			t.Errorf("%s watches = %v, want [%s]", name, controller.Watches, watch)
		}
		if len(controller.WatchMethods) != 1 || controller.WatchMethods[0] != "For" {
			t.Errorf("%s watch methods = %v, want [For]", name, controller.WatchMethods)
		}
		if len(controller.Calls) != 1 {
			t.Errorf("%s calls = %v, want one Reconcile call", name, controller.Calls)
		}
	}
}

func TestParseFixtures(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantTotal int
		wantFunc  int
		wantCntrl int
	}{
		{"empty file", "testdata/empty.go", 1, 0, 0},
		{"standalone functions", "testdata/functions.go", 4, 3, 0},
		{"methods", "testdata/methods.go", 3, 2, 0},
		{"controller with watches", "testdata/controller.go", 12, 9, 1},
	}

	p := NewGoParser()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entities, err := p.Parse(domain.File{RelativePath: tc.file})
			if err != nil {
				t.Fatalf("Parse() returned unexpected error: %v", err)
			}
			if len(entities) != tc.wantTotal {
				t.Errorf("Total entity count mismatch. Got %d, want %d", len(entities), tc.wantTotal)
			}

			gotFuncs := 0
			gotCntrl := 0
			for _, ent := range entities {
				switch ent.Kind {
				case domain.KindFunction:
					gotFuncs++
				case domain.KindController:
					gotCntrl++
				}
			}

			if gotFuncs != tc.wantFunc {
				t.Errorf("KindFunction count mismatch. Got %d, want %d", gotFuncs, tc.wantFunc)
			}

			if gotCntrl != tc.wantCntrl {
				t.Errorf("KindController count mismatch. Got %d, want %d", gotCntrl, tc.wantCntrl)
			}
		})
	}
}

func TestParseFunctionCalls(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/calls.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	helperA := byName["helperA"]
	if len(helperA.Calls) != 2 {
		t.Fatalf("helperA.Calls = %v, want 2 entries", helperA.Calls)
	}
	callSet := make(map[string]bool)
	for _, c := range helperA.Calls {
		callSet[c] = true
	}
	if !callSet["helperB"] || !callSet["processItem"] {
		t.Errorf("helperA.Calls = %v, want helperB and processItem", helperA.Calls)
	}

	helperB := byName["helperB"]
	if len(helperB.Calls) != 1 || helperB.Calls[0] != "processItem" {
		t.Errorf("helperB.Calls = %v, want [processItem]", helperB.Calls)
	}

	proc := byName["processItem"]
	if len(proc.Calls) != 0 {
		t.Errorf("processItem.Calls = %v, want empty", proc.Calls)
	}
}

func TestParseNestedReceiverCallPreservesFullSelector(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/nested_receiver_calls.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var caller *domain.Entity
	for i := range entities {
		if entities[i].ID == "function:nestedcalls.Reconciler.reconcile" {
			caller = &entities[i]
			break
		}
	}
	if caller == nil {
		t.Fatal("expected Reconciler.reconcile function entity")
	}
	if len(caller.Calls) != 1 || caller.Calls[0] != "r.RegistryProvider.Reconcile" {
		t.Fatalf("Calls = %v, want [r.RegistryProvider.Reconcile]", caller.Calls)
	}
	if len(caller.CallSites) != 1 || caller.CallSites[0].Name != "r.RegistryProvider.Reconcile" {
		t.Fatalf("CallSites = %+v, want the full nested selector", caller.CallSites)
	}
}

func TestParseImplements(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/implements.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		if ent.Kind == domain.KindFunction {
			byName[ent.ID] = ent
		}
	}

	isRS := byName["function:mycomp.myComponent.IsRequestServing"]
	if len(isRS.Implements) == 0 {
		t.Fatal("myComponent methods should have Implements populated")
	}
	found := false
	for _, impl := range isRS.Implements {
		if impl == "ComponentOptions" {
			found = true
		}
	}
	if !found {
		t.Errorf("myComponent.IsRequestServing.Implements = %v, want ComponentOptions", isRS.Implements)
	}
}

func TestParseEnvVars(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/envvars.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	conf := byName["configureFromEnv"]
	if len(conf.EnvVars) != 2 {
		t.Fatalf("configureFromEnv.EnvVars = %v, want 2 entries", conf.EnvVars)
	}
	envSet := make(map[string]bool)
	for _, e := range conf.EnvVars {
		envSet[e] = true
	}
	if !envSet["AWS_REGION"] || !envSet["PLATFORMS_INSTALLED"] {
		t.Errorf("EnvVars = %v, want AWS_REGION and PLATFORMS_INSTALLED", conf.EnvVars)
	}

	noEnv := byName["noEnvVars"]
	if len(noEnv.EnvVars) != 0 {
		t.Errorf("noEnvVars.EnvVars = %v, want empty", noEnv.EnvVars)
	}
}

func TestParseCreates(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/creates.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var controller *domain.Entity
	for i := range entities {
		if entities[i].Kind == domain.KindController {
			controller = &entities[i]
			break
		}
	}
	if controller == nil {
		t.Fatal("expected Reconciler controller entity")
	}
	if len(controller.Creates) != 2 || controller.Creates[0] != "Secret" || controller.Creates[1] != "ConfigMap" {
		t.Fatalf("Creates = %v, want [Secret ConfigMap]", controller.Creates)
	}
	if len(controller.CreateSites) != 2 {
		t.Fatalf("CreateSites = %v, want two sites", controller.CreateSites)
	}
	for _, site := range controller.CreateSites {
		if site.Source.Parser != "go-ast" || site.Source.File != "testdata/creates.go" || site.Source.Line == 0 {
			t.Errorf("invalid create site: %+v", site)
		}
	}
}

func TestParseImportAliasResolution(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/aliased_imports.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	fn := byName["registerComponents"]
	if fn.Kind != domain.KindFunction {
		t.Fatalf("registerComponents not found as function")
	}

	callSet := make(map[string]bool)
	for _, c := range fn.Calls {
		callSet[c] = true
	}

	// etcdv2.NewComponent should resolve to etcd.NewComponent
	if !callSet["etcd.NewComponent"] {
		t.Errorf("Expected etcd.NewComponent (resolved from etcdv2 alias), got %v", fn.Calls)
	}
	// kasv2.NewComponent should resolve to kas.NewComponent
	if !callSet["kas.NewComponent"] {
		t.Errorf("Expected kas.NewComponent (resolved from kasv2 alias), got %v", fn.Calls)
	}
	// olm.NewComponent should stay as olm.NewComponent (no alias)
	if !callSet["olm.NewComponent"] {
		t.Errorf("Expected olm.NewComponent (no alias needed), got %v", fn.Calls)
	}
	// Aliased names should NOT appear
	if callSet["etcdv2.NewComponent"] {
		t.Errorf("Should not have unresolved alias etcdv2.NewComponent in calls")
	}
	if callSet["kasv2.NewComponent"] {
		t.Errorf("Should not have unresolved alias kasv2.NewComponent in calls")
	}
}

func TestExtractLiterals(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/literals.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	build := byName["BuildCertSANs"]
	if len(build.Literals) == 0 {
		t.Fatal("BuildCertSANs should have literals")
	}
	litSet := make(map[string]bool)
	for _, l := range build.Literals {
		litSet[l] = true
	}
	if !litSet["*.etcd-discovery.%s.svc"] {
		t.Errorf("Expected literal '*.etcd-discovery.%%s.svc', got %v", build.Literals)
	}
	if !litSet["*.etcd-discovery.%s.svc.cluster.local"] {
		t.Errorf("Expected literal '*.etcd-discovery.%%s.svc.cluster.local', got %v", build.Literals)
	}
	// "127.0.0.1" has dots and length >= 4, should be included
	if !litSet["127.0.0.1"] {
		t.Errorf("Expected literal '127.0.0.1', got %v", build.Literals)
	}
	// "::1" is length 3, should be excluded
	if litSet["::1"] {
		t.Errorf("'::1' should be excluded (too short), got %v", build.Literals)
	}
	// "ok" is length 2, should be excluded
	if litSet["ok"] {
		t.Errorf("'ok' should be excluded (too short)")
	}

	simple := byName["SimpleFunc"]
	// "no-structural-chars" has a dash, length >= 4, should be included
	if len(simple.Literals) != 1 || simple.Literals[0] != "no-structural-chars" {
		t.Errorf("SimpleFunc.Literals = %v, want [no-structural-chars]", simple.Literals)
	}
}

func TestExtractEmbeds(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/embeds.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var pkg *domain.Entity
	for i := range entities {
		if entities[i].Kind == domain.KindPackage {
			pkg = &entities[i]
			break
		}
	}
	if pkg == nil {
		t.Fatal("Expected package entity")
	}

	if len(pkg.Embeds) != 2 {
		t.Fatalf("Embeds = %v, want 2 entries", pkg.Embeds)
	}
	embedSet := make(map[string]bool)
	for _, e := range pkg.Embeds {
		embedSet[e] = true
	}
	if !embedSet["*/*.yaml"] {
		t.Errorf("Expected embed pattern '*/*.yaml', got %v", pkg.Embeds)
	}
	if !embedSet["init-script.sh"] {
		t.Errorf("Expected embed pattern 'init-script.sh', got %v", pkg.Embeds)
	}
}

func TestExtractLiterals_Selectors(t *testing.T) {
	p := NewGoParser()
	entities, err := p.Parse(domain.File{RelativePath: "testdata/selectors.go"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	fn := byName["setCondition"]
	litSet := make(map[string]bool)
	for _, l := range fn.Literals {
		litSet[l] = true
	}
	if !litSet["PreviousCertificatesRevokedType"] {
		t.Errorf("Expected PreviousCertificatesRevokedType in literals, got %v", fn.Literals)
	}
	if !litSet["NewCertificatesTrustedType"] {
		t.Errorf("Expected NewCertificatesTrustedType in literals, got %v", fn.Literals)
	}
	if litSet["Do"] {
		t.Errorf("'Do' should be filtered (too short), got %v", fn.Literals)
	}
	if litSet["Sprintf"] {
		t.Errorf("'Sprintf' should be filtered (len < 8), got %v", fn.Literals)
	}
}

func TestParseEntityDetails(t *testing.T) {
	p := NewGoParser()

	t.Run("function IDs and descriptions", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/functions.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		byName := make(map[string]domain.Entity)
		for _, ent := range entities {
			byName[ent.Name] = ent
		}

		if byName["utils"].ID != "package:utils" {
			t.Errorf("Package ID = %q, want %q", byName["utils"].ID, "package:utils")
		}
		if byName["Add"].ID != "function:utils.Add" {
			t.Errorf("Add ID = %q, want %q", byName["Add"].ID, "function:utils.Add")
		}
		if byName["Add"].Description == "" {
			t.Error("Add should have a doc comment description")
		}
		if byName["multiply"].Description != "" {
			t.Errorf("multiply should have no description, got %q", byName["multiply"].Description)
		}
	})

	t.Run("method IDs with receiver", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/methods.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		byName := make(map[string]domain.Entity)
		for _, ent := range entities {
			byName[ent.Name] = ent
		}

		if byName["Get"].ID != "function:cache.Store.Get" {
			t.Errorf("Get ID = %q, want %q", byName["Get"].ID, "function:cache.Store.Get")
		}
		if byName["Size"].ID != "function:cache.Store.Size" {
			t.Errorf("Size ID = %q, want %q", byName["Size"].ID, "function:cache.Store.Size")
		}
	})

	t.Run("controller watches", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/controller.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		var controller *domain.Entity
		for i := range entities {
			if entities[i].Kind == domain.KindController {
				controller = &entities[i]
				break
			}
		}

		if controller == nil {
			t.Fatal("Expected a KindController entity, found none")
		}
		if controller.ID != "controller:fake.FakeReconciler" {
			t.Errorf("Controller ID = %q, want %q", controller.ID, "controller:fake.FakeReconciler")
		}
		if controller.Description != "FakeReconciler reconciles Fake resources." {
			t.Errorf("Controller Description = %q, want %q", controller.Description, "FakeReconciler reconciles Fake resources.")
		}

		if len(controller.Calls) == 0 {
			t.Fatal("Expected controller to have Calls from Reconcile() body")
		}
		callSet := make(map[string]bool)
		for _, c := range controller.Calls {
			callSet[c] = true
		}
		for _, expected := range []string{"r.client.Get", "CreateOrUpdate", "validateConfig"} {
			if !callSet[expected] {
				t.Errorf("Expected Calls to include %q, got %v", expected, controller.Calls)
			}
		}

		expectedWatches := []string{"HostedCluster", "Secret"}
		if len(controller.Watches) != len(expectedWatches) {
			t.Fatalf("Watches count mismatch. Got %v, want %v", controller.Watches, expectedWatches)
		}
		for i, w := range controller.Watches {
			if w != expectedWatches[i] {
				t.Errorf("Watch[%d] = %q, want %q", i, w, expectedWatches[i])
			}
		}
	})
}
