package domain

import "testing"

func TestExtractionSignatureTracksContractAndBuildSemantics(t *testing.T) {
	context := BuildContext{GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.26", BuildTags: []string{"b", "a", "a"}}
	want := ExtractionSignature("v1", context)
	context.BuildTags = []string{"a", "b"}
	if got := ExtractionSignature("v1", context); got != want {
		t.Fatalf("equivalent tags changed signature: %s != %s", got, want)
	}
	for name, modify := range map[string]func(*BuildContext){
		"platform":     func(c *BuildContext) { c.GOOS = "darwin" },
		"architecture": func(c *BuildContext) { c.GOARCH = "arm64" },
		"toolchain":    func(c *BuildContext) { c.GoVersion = "go1.27" },
		"tags":         func(c *BuildContext) { c.BuildTags = []string{"c"} },
	} {
		t.Run(name, func(t *testing.T) {
			changed := context
			modify(&changed)
			if ExtractionSignature("v1", changed) == want {
				t.Fatal("changed build semantics retained signature")
			}
		})
	}
	if ExtractionSignature("v2", context) == want {
		t.Fatal("extractor update retained signature")
	}
	if ExtractionSignature("v1", context, "sha256:first") == ExtractionSignature("v1", context, "sha256:second") {
		t.Fatal("different executable builds retained signature")
	}
}
