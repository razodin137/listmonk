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
