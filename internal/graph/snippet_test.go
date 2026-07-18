package graph

import "testing"

func TestReadSnippet(t *testing.T) {
	got := ReadSnippet("testdata/sample.go", 3)
	expected := "func Hello() string {"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestReadSnippet_LastLine(t *testing.T) {
	got := ReadSnippet("testdata/sample.go", 8)
	expected := "return a + b"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestReadSnippet_FileNotFound(t *testing.T) {
	got := ReadSnippet("testdata/nonexistent.go", 1)
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

func TestReadSnippet_LineOutOfRange(t *testing.T) {
	got := ReadSnippet("testdata/sample.go", 999)
	if got != "" {
		t.Errorf("expected empty string for out-of-range line, got %q", got)
	}
}

func TestReadSnippet_LineZero(t *testing.T) {
	got := ReadSnippet("testdata/sample.go", 0)
	if got != "" {
		t.Errorf("expected empty string for line 0, got %q", got)
	}
}
