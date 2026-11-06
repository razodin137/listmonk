package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/knadh/listmonk/internal/archive"
	"github.com/knadh/listmonk/models"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	archiveDatasetFile      = "archive.json"
	archiveContentIndexFile = "index.md"
	archivePostExt          = ".md"
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

	if err := a.ExportArchive(a.cfg.ArchiveExportDir); err != nil {
		return fmt.Errorf("exporting archive: %w", err)
	}

	return nil
}

// BackfillArchive publishes every finished campaign that is not already in the
// archive and then writes the dataset and files, leaving up-to-date files
// untouched.
func (a *App) BackfillArchive(dir string) error {
	return a.BackfillArchiveYears(dir, 0)
}

// BackfillArchiveYears publishes finished campaigns from the last years and
// then writes them, newest first. A year count of zero or less means every
// campaign. Each campaign left out is logged with the reason it was skipped.
func (a *App) BackfillArchiveYears(dir string, years int) error {
	camps, _, err := a.core.QueryCampaigns("", []string{models.CampaignStatusFinished}, nil, "",
		models.FieldCreatedAt, models.OrderAsc, true, nil, 0, 0)
	if err != nil {
		return fmt.Errorf("fetching finished campaigns: %w", err)
	}

	var (
		window             = archive.Window{Years: years, Now: time.Now()}
		published, skipped int
	)

	for i := range camps {
		camp := &camps[i]
		title := camp.ArchiveTitle()

		if reason := archiveCampaignSkipReason(*camp, window); reason != "" {
			skipped++
			a.log.Printf("archive: skipped campaign %d %q: %s", camp.ID, title, reason)
			continue
		}

		wrote, err := a.publishCampaignToArchive(camp)
		if err != nil {
			return fmt.Errorf("archiving campaign %s: %w", camp.UUID, err)
		}

		if wrote {
			published++
		} else {
			skipped++
			a.log.Printf("archive: skipped campaign %d %q: already archived", camp.ID, title)
		}
	}

	a.log.Printf("archive: backfilled %d campaigns, skipped %d", published, skipped)

	if err := a.ExportArchiveYears(dir, years); err != nil {
		return fmt.Errorf("exporting archive: %w", err)
	}

	return nil
}

// archiveCampaignSkipReason explains why a campaign is outside the window, or
// returns an empty string when it belongs in the export.
func archiveCampaignSkipReason(c models.Campaign, window archive.Window) string {
	if c.Type != models.CampaignTypeRegular && c.Type != models.CampaignTypePost {
		return fmt.Sprintf("type %q is not part of the public archive", c.Type)
	}

	date := campaignArchiveDate(c)

	return window.SkipReason(date.Time, date.Valid)
}

// archiveEntriesWithin keeps the entries inside the window, preserving their
// newest-first order. It reports how many entries were left out.
func archiveEntriesWithin(entries []campArchiveEntry, window archive.Window) ([]campArchiveEntry, int) {
	if window.Years <= 0 {
		return entries, 0
	}

	out := entries[:0]
	skipped := 0
	for _, entry := range entries {
		if window.SkipReason(entry.Date.Time, entry.Date.Valid) != "" {
			skipped++
			continue
		}

		out = append(out, entry)
	}

	return out, skipped
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

	slug, err := a.archiveSlugFor(c)
	if err != nil {
		return false, err
	}

	if err := a.core.UpdateCampaignArchive(c.ID, true, 0, meta, slug); err != nil {
		return false, fmt.Errorf("publishing campaign to archive: %w", err)
	}

	published := !c.Archive
	c.Archive = true
	c.ArchiveSlug = null.NewString(slug, true)

	return published, nil
}

// archiveSlugFor returns c's slug using the same assignment as the dataset,
// export, and live archive, so publishing never picks a different suffix.
func (a *App) archiveSlugFor(c *models.Campaign) (string, error) {
	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return "", fmt.Errorf("fetching archived campaigns: %w", err)
	}

	idx := -1
	for i := range camps {
		if camps[i].ID == c.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		camps = append(camps, *c)
		idx = len(camps) - 1
	} else {
		camps[idx] = *c
	}

	titles := make([]string, len(camps))
	for i := range camps {
		titles[i] = camps[i].ArchiveTitle()
	}

	return archiveSlugs(camps, titles)[idx], nil
}

// ExportArchive writes the public archive dataset, one Markdown file per
// campaign, an index of those files, and a static HTML site to dir. Files
// whose contents have not changed are left untouched, and hand-edited files
// are kept with the new export written to a numeric variant beside them.
func (a *App) ExportArchive(dir string) error {
	return a.ExportArchiveYears(dir, 0)
}

// ExportArchiveYears writes the same files as ExportArchive, limited to
// campaigns from the last years. A year count of zero or less exports every
// campaign. Entries are written newest first.
func (a *App) ExportArchiveYears(dir string, years int) error {
	if dir == "" {
		dir = archive.DefaultExportDir
	}

	if err := archive.EnsureExportDir(dir); err != nil {
		return err
	}

	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return fmt.Errorf("fetching archived campaigns: %w", err)
	}

	entries, err := a.buildCampaignArchiveEntries(camps)
	if err != nil {
		return fmt.Errorf("building archive entries: %w", err)
	}

	window := archive.Window{Years: years, Now: time.Now()}
	entries, filtered := archiveEntriesWithin(entries, window)

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
		files                    = append(posts, archiveStaticFiles(entries)...)
		index                    = make([]archive.IndexEntry, len(posts))
		written, unchanged, kept int
	)

	record := func(res archive.WriteResult) {
		if res.Written {
			written++
		} else {
			unchanged++
		}

		if res.Edited != "" {
			kept++
			a.log.Printf("archive: kept hand-edited %s, export content is in %s", res.Edited, res.Path)
		}
	}

	res, err := writeArchiveFile(filepath.Join(dir, archiveDatasetFile), data)
	if err != nil {
		return err
	}
	record(res)

	for i, file := range files {
		res, err := writeArchiveFile(filepath.Join(dir, file.Name), []byte(file.Body))
		if err != nil {
			return err
		}
		record(res)

		if i < len(posts) {
			index[i] = archive.IndexEntry{
				Slug:  entries[i].Slug,
				Title: entries[i].Title,
				Date:  archiveIndexDate(entries[i].Date),
				File:  filepath.Base(res.Path),
			}
		}
	}

	wrote, err := archive.WriteFileIfChanged(
		filepath.Join(dir, archiveContentIndexFile), []byte(archive.RenderIndex(index)))
	if err != nil {
		return fmt.Errorf("writing archive index: %w", err)
	}
	if wrote {
		written++
	} else {
		unchanged++
	}

	a.log.Printf("archive: exported %d campaigns to %s", len(entries), dir)
	if filtered > 0 {
		a.log.Printf("archive: skipped %d posts outside the %d-year window", filtered, years)
	}
	a.log.Printf("archive: wrote %d files, left %d unchanged, kept %d hand-edited", written, unchanged, kept)

	return nil
}

// archiveExportFiles names each campaign file from the same slug that the
// archive URL and front matter use.
func archiveExportFiles(entries []campArchiveEntry) ([]archiveExportFile, error) {
	out := make([]archiveExportFile, 0, len(entries))

	for _, entry := range entries {
		body, err := archivePostFile(entry)
		if err != nil {
			return nil, err
		}

		out = append(out, archiveExportFile{
			Name: models.ArchiveFileName(entry.Slug) + archivePostExt,
			Body: body,
		})
	}

	return out, nil
}

// archiveIndexDate formats a campaign date as the post front matter does.
func archiveIndexDate(t null.Time) string {
	if !t.Valid {
		return ""
	}

	return t.Time.UTC().Format(time.RFC3339)
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

// writeArchiveFile writes an export file without ever replacing a hand edit.
func writeArchiveFile(path string, data []byte) (archive.WriteResult, error) {
	return archive.WriteFilePreservingEdits(path, data)
}
