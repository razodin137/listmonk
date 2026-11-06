package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileIfChangedWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")

	wrote, err := WriteFileIfChanged(path, []byte("first"))
	if err != nil {
		t.Fatalf("WriteFileIfChanged() error: %v", err)
	}
	if !wrote {
		t.Fatal("WriteFileIfChanged() reported an unchanged file on first write")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("file contents = %q, want %q", got, "first")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Fatalf("file mode = %o, want %o", mode, 0o644)
	}

	assertNoTempFiles(t, dir, path)

	wrote, err = WriteFileIfChanged(path, []byte("first"))
	if err != nil {
		t.Fatalf("WriteFileIfChanged() on unchanged file error: %v", err)
	}
	if wrote {
		t.Fatal("WriteFileIfChanged() rewrote an unchanged file")
	}

	wrote, err = WriteFileIfChanged(path, []byte("second"))
	if err != nil {
		t.Fatalf("WriteFileIfChanged() on changed file error: %v", err)
	}
	if !wrote {
		t.Fatal("WriteFileIfChanged() reported an unchanged file on changed write")
	}

	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading rewritten file: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("file contents = %q, want %q", got, "second")
	}

	assertNoTempFiles(t, dir, path)
}

func TestWriteFileAtomicCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("creating blocking directory: %v", err)
	}

	if err := writeFileAtomic(path, []byte("data")); err == nil {
		t.Fatal("writeFileAtomic() succeeded over an existing directory")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat destination: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("failed write replaced the destination directory")
	}

	assertNoTempFiles(t, dir, path)
}

func TestWriteFilePreservingEditsWritesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")

	res, err := WriteFilePreservingEdits(path, []byte("generated"))
	if err != nil {
		t.Fatalf("WriteFilePreservingEdits() error: %v", err)
	}
	if res.Path != path {
		t.Fatalf("WriteFilePreservingEdits() path = %q, want %q", res.Path, path)
	}
	if res.Edited != "" {
		t.Fatalf("WriteFilePreservingEdits() reported edited file %q on new file", res.Edited)
	}
	if !res.Written {
		t.Fatal("WriteFilePreservingEdits() did not report a new file")
	}

	assertFileContents(t, path, "generated")
	assertNoTempFiles(t, dir, path)
}

func TestWriteFilePreservingEditsLeavesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")

	if err := os.WriteFile(path, []byte("generated"), 0o644); err != nil {
		t.Fatalf("writing existing file: %v", err)
	}

	res, err := WriteFilePreservingEdits(path, []byte("generated"))
	if err != nil {
		t.Fatalf("WriteFilePreservingEdits() error: %v", err)
	}
	if res.Path != path || res.Edited != "" || res.Written {
		t.Fatalf("WriteFilePreservingEdits() = %+v, want an unchanged %s", res, path)
	}
}

func TestWriteFilePreservingEditsKeepsHandEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	variant := filepath.Join(dir, "post-2.md")

	if err := os.WriteFile(path, []byte("hand edited"), 0o644); err != nil {
		t.Fatalf("writing hand edit: %v", err)
	}

	res, err := WriteFilePreservingEdits(path, []byte("generated"))
	if err != nil {
		t.Fatalf("WriteFilePreservingEdits() error: %v", err)
	}
	if res.Path != variant {
		t.Fatalf("WriteFilePreservingEdits() path = %q, want %q", res.Path, variant)
	}
	if res.Edited != path {
		t.Fatalf("WriteFilePreservingEdits() edited = %q, want %q", res.Edited, path)
	}
	if !res.Written {
		t.Fatal("WriteFilePreservingEdits() did not report the variant write")
	}

	assertFileContents(t, path, "hand edited")
	assertFileContents(t, variant, "generated")
	assertNoTempFiles(t, dir, variant)
}

func TestWriteFilePreservingEditsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	variant := filepath.Join(dir, "post-2.md")

	if err := os.WriteFile(path, []byte("hand edited"), 0o644); err != nil {
		t.Fatalf("writing hand edit: %v", err)
	}

	if _, err := WriteFilePreservingEdits(path, []byte("generated")); err != nil {
		t.Fatalf("WriteFilePreservingEdits() error: %v", err)
	}

	res, err := WriteFilePreservingEdits(path, []byte("generated"))
	if err != nil {
		t.Fatalf("WriteFilePreservingEdits() second call error: %v", err)
	}
	if res.Path != variant || res.Edited != path || res.Written {
		t.Fatalf("WriteFilePreservingEdits() second call = %+v, want unchanged %s", res, variant)
	}

	if _, err := os.Stat(filepath.Join(dir, "post-3.md")); !os.IsNotExist(err) {
		t.Fatalf("second export created post-3.md: %v", err)
	}
}

func TestWriteFilePreservingEditsSkipsEditedVariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	variant2 := filepath.Join(dir, "post-2.md")
	variant3 := filepath.Join(dir, "post-3.md")

	for _, file := range []struct {
		path string
		body string
	}{
		{path, "hand edited"},
		{variant2, "hand edited twice"},
	} {
		if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", file.path, err)
		}
	}

	res, err := WriteFilePreservingEdits(path, []byte("generated"))
	if err != nil {
		t.Fatalf("WriteFilePreservingEdits() error: %v", err)
	}
	if res.Path != variant3 {
		t.Fatalf("WriteFilePreservingEdits() path = %q, want %q", res.Path, variant3)
	}

	assertFileContents(t, path, "hand edited")
	assertFileContents(t, variant2, "hand edited twice")
	assertFileContents(t, variant3, "generated")
}

func TestNumericVariant(t *testing.T) {
	tests := []struct {
		path string
		n    int
		want string
	}{
		{path: "post.md", n: 2, want: "post-2.md"},
		{path: "archive.json", n: 3, want: "archive-3.json"},
		{path: "index.html", n: 2, want: "index-2.html"},
		{path: "README", n: 2, want: "README-2"},
		{path: "/tmp/out/post.md", n: 4, want: "/tmp/out/post-4.md"},
	}

	for _, tt := range tests {
		if got := numericVariant(tt.path, tt.n); got != tt.want {
			t.Fatalf("numericVariant(%q, %d) = %q, want %q", tt.path, tt.n, got, tt.want)
		}
	}
}

// assertFileContents checks a file's exact contents.
func assertFileContents(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s contents = %q, want %q", path, got, want)
	}
}

// assertNoTempFiles checks that an atomic write did not leave its scratch file
// behind.
func assertNoTempFiles(t *testing.T, dir, path string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}

	prefix := "." + filepath.Base(path) + ".tmp-"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}
