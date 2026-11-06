package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	archiveDatasetFile = "archive.json"
	archivePostExt     = ".md"
)

// archiveExportFile is one campaign body destined for the export directory.
type archiveExportFile struct {
	Name string
	Body string
}

// ExportArchive writes the public archive dataset, one Markdown file per
// campaign, and a static HTML site to dir. Files whose contents have not
// changed are left untouched.
func (a *App) ExportArchive(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}

	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return fmt.Errorf("fetching archived campaigns: %w", err)
	}

	entries, err := a.buildCampaignArchiveEntries(camps)
	if err != nil {
		return fmt.Errorf("building archive entries: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding archive dataset: %w", err)
	}
	data = append(data, '\n')

	var (
		files              = append(archiveExportFiles(entries), archiveStaticFiles(entries)...)
		written, unchanged int
	)

	wrote, err := writeArchiveFile(filepath.Join(dir, archiveDatasetFile), data)
	if err != nil {
		return err
	}
	if wrote {
		written++
	} else {
		unchanged++
	}

	for _, file := range files {
		wrote, err := writeArchiveFile(filepath.Join(dir, file.Name), []byte(file.Body))
		if err != nil {
			return err
		}
		if wrote {
			written++
		} else {
			unchanged++
		}
	}

	a.log.Printf("archive: exported %d campaigns to %s", len(entries), dir)
	a.log.Printf("archive: wrote %d files, left %d unchanged", written, unchanged)

	return nil
}

// archiveExportFiles names each campaign file and appends a numeric suffix
// when two campaigns resolve to the same name.
func archiveExportFiles(entries []campArchiveEntry) []archiveExportFile {
	var (
		out  = make([]archiveExportFile, 0, len(entries))
		used = make(map[string]struct{}, len(entries))
	)

	for _, entry := range entries {
		name := uniqueArchiveFileName(archiveFileName(entry.Slug), used)
		out = append(out, archiveExportFile{
			Name: name + archivePostExt,
			Body: entry.Body,
		})
	}

	return out
}

// archiveFileName turns a slug into a single safe file name component.
func archiveFileName(slug string) string {
	name := strings.TrimSpace(slug)
	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return '-'
		case unicode.IsControl(r):
			return -1
		}

		return r
	}, name)
	name = strings.Trim(name, ". ")

	if name == "" {
		return "untitled"
	}

	return name
}

// uniqueArchiveFileName returns name, or the first free name-N when it is
// already taken.
func uniqueArchiveFileName(name string, used map[string]struct{}) string {
	if _, ok := used[name]; !ok {
		used[name] = struct{}{}
		return name
	}

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if _, ok := used[candidate]; !ok {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

// writeArchiveFile writes data only when the destination differs from it.
func writeArchiveFile(path string, data []byte) (bool, error) {
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}

	return true, nil
}
