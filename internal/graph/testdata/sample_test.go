package graph

import "testing"

func TestReadSnippet(t *testing.T) {
	got := ReadSnippet("testdata/sample.go", 3)
	expected := "func Hello() string {"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
