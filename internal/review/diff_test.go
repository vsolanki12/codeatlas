package review

import (
	"testing"
)

func TestParseDiff_ModifiedFile(t *testing.T) {
	diff := `diff --git a/pkg/controller.go b/pkg/controller.go
index abc1234..def5678 100644
--- a/pkg/controller.go
+++ b/pkg/controller.go
@@ -10,5 +10,7 @@ func Reconcile() {
 context
-old line
+new line
+added line
 context
`

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Path != "pkg/controller.go" {
		t.Errorf("path = %q, want %q", f.Path, "pkg/controller.go")
	}
	if f.Status != FileModified {
		t.Errorf("status = %q, want %q", f.Status, FileModified)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.OldStart != 10 || h.OldCount != 5 {
		t.Errorf("old range = %d,%d, want 10,5", h.OldStart, h.OldCount)
	}
	if h.NewStart != 10 || h.NewCount != 7 {
		t.Errorf("new range = %d,%d, want 10,7", h.NewStart, h.NewCount)
	}
	if h.Header != "func Reconcile() {" {
		t.Errorf("header = %q, want %q", h.Header, "func Reconcile() {")
	}
	if f.AddedLines != 2 {
		t.Errorf("added = %d, want 2", f.AddedLines)
	}
	if f.DeletedLines != 1 {
		t.Errorf("deleted = %d, want 1", f.DeletedLines)
	}
}

func TestParseDiff_NewFile(t *testing.T) {
	diff := `diff --git a/pkg/new.go b/pkg/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/pkg/new.go
@@ -0,0 +1,5 @@
+package pkg
+
+func New() {
+	return
+}
`

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Status != FileAdded {
		t.Errorf("status = %q, want %q", f.Status, FileAdded)
	}
	if f.Path != "pkg/new.go" {
		t.Errorf("path = %q, want %q", f.Path, "pkg/new.go")
	}
	if f.AddedLines != 5 {
		t.Errorf("added = %d, want 5", f.AddedLines)
	}
}

func TestParseDiff_DeletedFile(t *testing.T) {
	diff := `diff --git a/pkg/old.go b/pkg/old.go
deleted file mode 100644
index abc1234..0000000
--- a/pkg/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package pkg
-
-func Old() {}
`

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Status != FileDeleted {
		t.Errorf("status = %q, want %q", f.Status, FileDeleted)
	}
	if f.DeletedLines != 3 {
		t.Errorf("deleted = %d, want 3", f.DeletedLines)
	}
}

func TestParseDiff_Rename(t *testing.T) {
	diff := `diff --git a/old/path.go b/new/path.go
similarity index 95%
rename from old/path.go
rename to new/path.go
index abc1234..def5678 100644
--- a/old/path.go
+++ b/new/path.go
@@ -5,3 +5,4 @@ func Foo() {
 context
+added
 context
`

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Status != FileRenamed {
		t.Errorf("status = %q, want %q", f.Status, FileRenamed)
	}
	if f.OldPath != "old/path.go" {
		t.Errorf("old path = %q, want %q", f.OldPath, "old/path.go")
	}
	if f.Path != "new/path.go" {
		t.Errorf("new path = %q, want %q", f.Path, "new/path.go")
	}
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
index abc..def 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,4 @@
 line
+added
 line
diff --git a/b.go b/b.go
index abc..def 100644
--- a/b.go
+++ b/b.go
@@ -10,3 +10,3 @@
-old
+new
`

	files := ParseDiff(diff)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != "a.go" {
		t.Errorf("file 0 path = %q, want %q", files[0].Path, "a.go")
	}
	if files[1].Path != "b.go" {
		t.Errorf("file 1 path = %q, want %q", files[1].Path, "b.go")
	}
}

func TestParseDiff_MultipleHunks(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
index abc..def 100644
--- a/file.go
+++ b/file.go
@@ -5,3 +5,4 @@ func A() {
 context
+added
 context
@@ -50,3 +51,4 @@ func B() {
 context
+added
 context
`

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if len(files[0].Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(files[0].Hunks))
	}

	if files[0].Hunks[0].OldStart != 5 {
		t.Errorf("hunk 0 old start = %d, want 5", files[0].Hunks[0].OldStart)
	}
	if files[0].Hunks[1].OldStart != 50 {
		t.Errorf("hunk 1 old start = %d, want 50", files[0].Hunks[1].OldStart)
	}
	if files[0].Hunks[0].Header != "func A() {" {
		t.Errorf("hunk 0 header = %q, want %q", files[0].Hunks[0].Header, "func A() {")
	}
	if files[0].Hunks[1].Header != "func B() {" {
		t.Errorf("hunk 1 header = %q, want %q", files[0].Hunks[1].Header, "func B() {")
	}
}

func TestParseRange_Single(t *testing.T) {
	start, count := parseRange("42")
	if start != 42 || count != 1 {
		t.Errorf("parseRange(\"42\") = %d,%d, want 42,1", start, count)
	}
}

func TestParseRange_WithCount(t *testing.T) {
	start, count := parseRange("10,5")
	if start != 10 || count != 5 {
		t.Errorf("parseRange(\"10,5\") = %d,%d, want 10,5", start, count)
	}
}

func TestParseDiff_Empty(t *testing.T) {
	files := ParseDiff("")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
