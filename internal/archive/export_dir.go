package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultExportDir is where archive posts are written when no directory is
// configured.
const DefaultExportDir = "content/posts"

// ResolveExportDir returns the configured export directory resolved against
// baseDir, which is normally the directory containing the config file. A
// relative setting is joined to baseDir; an absolute one is kept as is.
func ResolveExportDir(setting, baseDir string) string {
	dir := strings.TrimSpace(setting)
	if dir == "" {
		dir = DefaultExportDir
	}

	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}

	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}

	return filepath.Join(baseDir, dir)
}

// EnsureExportDir creates the export directory and any missing parents.
func EnsureExportDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating archive export directory %s: %w", dir, err)
	}

	return nil
}
