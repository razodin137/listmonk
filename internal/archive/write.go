package archive

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WriteResult reports where an exported file ended up.
type WriteResult struct {
	Path    string // file holding the exported content
	Edited  string // hand-edited file that was kept, empty when none
	Written bool   // whether Path was created or replaced
}

// WriteFileIfChanged writes data to path only when the file does not already
// contain it. It reports whether the file was written.
func WriteFileIfChanged(path string, data []byte) (bool, error) {
	exists, same, err := fileMatches(path, data)
	if err != nil {
		return false, err
	}
	if exists && same {
		return false, nil
	}

	if err := writeFileAtomic(path, data); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}

	return true, nil
}

// WriteFilePreservingEdits writes data to path, but never over a file that
// holds something else. A differing path is treated as a hand edit: it is left
// untouched and data goes to the first free numeric variant beside it, such as
// post-2.md for post.md.
func WriteFilePreservingEdits(path string, data []byte) (WriteResult, error) {
	exists, same, err := fileMatches(path, data)
	if err != nil {
		return WriteResult{}, err
	}
	if exists && same {
		return WriteResult{Path: path}, nil
	}
	if !exists {
		if _, err := WriteFileIfChanged(path, data); err != nil {
			return WriteResult{}, err
		}

		return WriteResult{Path: path, Written: true}, nil
	}

	for n := 2; ; n++ {
		variant := numericVariant(path, n)

		exists, same, err := fileMatches(variant, data)
		if err != nil {
			return WriteResult{}, err
		}
		if exists {
			if same {
				return WriteResult{Path: variant, Edited: path}, nil
			}

			continue
		}

		if _, err := WriteFileIfChanged(variant, data); err != nil {
			return WriteResult{}, err
		}

		return WriteResult{Path: variant, Edited: path, Written: true}, nil
	}
}

// fileMatches reads path and reports whether it exists and already holds data.
func fileMatches(path string, data []byte) (exists, same bool, err error) {
	old, err := os.ReadFile(path)
	if errors.Is(err, iofs.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("reading %s: %w", path, err)
	}

	return true, bytes.Equal(old, data), nil
}

// numericVariant inserts n before path's extension.
func numericVariant(path string, n int) string {
	ext := filepath.Ext(path)

	return fmt.Sprintf("%s-%d%s", strings.TrimSuffix(path, ext), n, ext)
}

// writeFileAtomic writes data to a temporary file beside the destination and
// renames it into place, so a crash or a full disk never leaves a partial file
// behind.
func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tmp := f.Name()
	defer func() {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	tmp = ""

	return nil
}
