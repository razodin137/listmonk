package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExportDir(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		baseDir string
		want    string
	}{
		{
			name:    "default against config directory",
			baseDir: "/etc/listmonk",
			want:    "/etc/listmonk/content/posts",
		},
		{
			name:    "relative against config directory",
			setting: "posts",
			baseDir: "/etc/listmonk",
			want:    "/etc/listmonk/posts",
		},
		{
			name:    "nested relative against config directory",
			setting: "./out/posts",
			baseDir: "/etc/listmonk",
			want:    "/etc/listmonk/out/posts",
		},
		{
			name:    "absolute stays absolute",
			setting: "/srv/posts",
			baseDir: "/etc/listmonk",
			want:    "/srv/posts",
		},
		{
			name:    "empty base uses working directory",
			setting: "content/posts",
			want:    "content/posts",
		},
		{
			name:    "surrounding space is ignored",
			setting: "  content/posts  ",
			baseDir: "/etc/listmonk",
			want:    "/etc/listmonk/content/posts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveExportDir(tt.setting, tt.baseDir); got != tt.want {
				t.Fatalf("ResolveExportDir(%q, %q) = %q, want %q", tt.setting, tt.baseDir, got, tt.want)
			}
		})
	}
}

func TestEnsureExportDirCreatesMissingParents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "posts")

	if err := EnsureExportDir(dir); err != nil {
		t.Fatalf("EnsureExportDir() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat export directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}

	if err := EnsureExportDir(dir); err != nil {
		t.Fatalf("EnsureExportDir() on existing directory error: %v", err)
	}
}
