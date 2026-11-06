package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	archiveDatasetFile      = "archive.json"
	archivePostExt          = ".md"
	defaultArchiveExportDir = "content/posts"
)

// archiveExportFile is one campaign body destined for the export directory.
type archiveExportFile struct {
	Name string
	Body string
}

// archiveFinishedMut serialises the publish-and-export step so a later
// finishing campaign never overwrites the export with an older snapshot.
var archiveFinishedMut sync.Mutex

// archiveFinishedCampaign publishes a finished campaign to the public archive
// and refreshes the exported files in the same step.
func (a *App) archiveFinishedCampaign(c *models.Campaign) error {
	archiveFinishedMut.Lock()
	defer archiveFinishedMut.Unlock()

	if _, err := a.publishCampaignToArchive(c); err != nil {
		return err
	}

	if err := a.ExportArchive(defaultArchiveExportDir); err != nil {
		return fmt.Errorf("exporting archive: %w", err)
	}

	return nil
}

// BackfillArchive publishes every finished campaign that is not already in the
// archive and then writes the dataset and files, leaving up-to-date files
// untouched.
func (a *App) BackfillArchive(dir string) error {
	camps, _, err := a.core.QueryCampaigns("", []string{models.CampaignStatusFinished}, nil, "",
		models.FieldCreatedAt, models.OrderAsc, true, nil, 0, 0)
	if err != nil {
		return fmt.Errorf("fetching finished campaigns: %w", err)
	}

	var published, skipped int
	for i := range camps {
		wrote, err := a.publishCampaignToArchive(&camps[i])
		if err != nil {
			return fmt.Errorf("archiving campaign %s: %w", camps[i].UUID, err)
		}

		if wrote {
			published++
		} else {
			skipped++
		}
	}

	a.log.Printf("archive: backfilled %d campaigns, %d already archived", published, skipped)

	if err := a.ExportArchive(dir); err != nil {
		return fmt.Errorf("exporting archive: %w", err)
	}

	return nil
}

// publishCampaignToArchive marks a campaign as archived using its stored
// archive metadata. It reports whether the campaign was newly published.
func (a *App) publishCampaignToArchive(c *models.Campaign) (bool, error) {
	if c.Archive && c.ArchiveSlug.Valid && c.ArchiveSlug.String != "" {
		return false, nil
	}

	meta := models.JSON{}
	if len(c.ArchiveMeta) > 0 {
		if err := json.Unmarshal(c.ArchiveMeta, &meta); err != nil {
			return false, fmt.Errorf("decoding campaign archive meta: %w", err)
		}
		if meta == nil {
			meta = models.JSON{}
		}
	}

	slug := c.ArchiveSlug.String
	if slug == "" {
		used, err := a.archivedSlugs()
		if err != nil {
			return false, err
		}
		slug = models.MakeArchiveSlug(c.Subject, used)
	}

	if err := a.core.UpdateCampaignArchive(c.ID, true, 0, meta, slug); err != nil {
		return false, fmt.Errorf("publishing campaign to archive: %w", err)
	}

	published := !c.Archive
	c.Archive = true
	c.ArchiveSlug = null.NewString(slug, true)

	return published, nil
}

// archivedSlugs returns the slugs already taken by public archive entries so
// a new post never renames an older one.
func (a *App) archivedSlugs() (map[string]struct{}, error) {
	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return nil, fmt.Errorf("fetching archived campaigns: %w", err)
	}

	used := make(map[string]struct{}, len(camps))
	for _, camp := range camps {
		if camp.ArchiveSlug.Valid && camp.ArchiveSlug.String != "" {
			used[camp.ArchiveSlug.String] = struct{}{}
		}
	}

	return used, nil
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

	posts, err := archiveExportFiles(entries)
	if err != nil {
		return fmt.Errorf("rendering archive posts: %w", err)
	}

	var (
		files              = append(posts, archiveStaticFiles(entries)...)
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
func archiveExportFiles(entries []campArchiveEntry) ([]archiveExportFile, error) {
	var (
		out  = make([]archiveExportFile, 0, len(entries))
		used = make(map[string]struct{}, len(entries))
	)

	for _, entry := range entries {
		body, err := archivePostFile(entry)
		if err != nil {
			return nil, err
		}

		name := uniqueArchiveFileName(models.ArchiveFileName(entry.Slug), used)
		out = append(out, archiveExportFile{
			Name: name + archivePostExt,
			Body: body,
		})
	}

	return out, nil
}

// archivePostFile renders one campaign as a Hugo post with YAML front matter
// followed by the rendered HTML body.
func archivePostFile(entry campArchiveEntry) (string, error) {
	post := models.ArchivePost{
		Title:       entry.Title,
		Slug:        entry.Slug,
		Description: models.ArchiveDescription(entry.Body),
		Tags:        entry.Tags,
	}
	if entry.Date.Valid {
		post.Date = entry.Date.Time
	}

	return models.RenderArchivePost(post, entry.Body)
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
