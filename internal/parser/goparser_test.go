package parser

import (
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

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
		{"controller with watches", "testdata/controller.go", 11, 9, 1},
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
		for _, expected := range []string{"Get", "CreateOrUpdate", "validateConfig"} {
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
