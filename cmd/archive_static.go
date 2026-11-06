package main

import (
	"bytes"
	"html"
	"html/template"
	"strings"

	"github.com/knadh/listmonk/internal/archive"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	archiveIndexFile      = "index.html"
	archivePageExt        = ".html"
	archiveStylesheetFile = "style.css"
	archiveSiteTitle      = "Mailing list archive"
	archiveEmptyText      = "No archived messages yet."
	archiveLang           = "en"
)

type archiveStaticPage struct {
	Name     string
	Title    string
	Date     string
	DateTime string
	Body     template.HTML
}

// archiveStaticFiles renders the list page, one page per entry, and the
// stylesheet that sits beside them.
func archiveStaticFiles(entries []campArchiveEntry) []archiveExportFile {
	pages := archiveStaticPages(entries)

	out := make([]archiveExportFile, 0, len(pages)+2)
	out = append(out, archiveExportFile{
		Name: archiveIndexFile,
		Body: archiveStaticIndex(pages),
	})

	for _, page := range pages {
		out = append(out, archiveExportFile{
			Name: page.Name,
			Body: archiveStaticPageHTML(page),
		})
	}

	out = append(out, archiveExportFile{
		Name: archiveStylesheetFile,
		Body: archive.Stylesheet(),
	})

	return out
}

// archiveStaticPages names each entry's HTML page. The index name is reserved
// so a campaign cannot overwrite the site's list page.
func archiveStaticPages(entries []campArchiveEntry) []archiveStaticPage {
	var (
		out  = make([]archiveStaticPage, 0, len(entries))
		used = map[string]struct{}{
			strings.TrimSuffix(archiveIndexFile, archivePageExt): {},
		}
	)

	for _, entry := range entries {
		base := uniqueArchiveFileName(archiveFileName(entry.Slug), used)
		date, dateTime := archiveStaticDate(entry.Date)

		out = append(out, archiveStaticPage{
			Name:     base + archivePageExt,
			Title:    entry.Title,
			Date:     date,
			DateTime: dateTime,
			Body:     template.HTML(entry.Body),
		})
	}

	return out
}

// archiveStaticIndex renders the archive list with links relative to the
// exported directory.
func archiveStaticIndex(pages []archiveStaticPage) string {
	entries := make([]archive.Entry, 0, len(pages))
	for _, page := range pages {
		entries = append(entries, archive.Entry{
			Title:    page.Title,
			Href:     page.Name,
			Date:     page.Date,
			DateTime: page.DateTime,
			Excerpt:  archiveStaticExcerpt(page.Body),
		})
	}

	var buf bytes.Buffer
	if err := archive.RenderList(&buf, archive.ListData{
		Lang:       archiveLang,
		Title:      archiveSiteTitle,
		Stylesheet: archiveStylesheetFile,
		Entries:    entries,
		Empty:      archiveEmptyText,
	}); err != nil {
		return ""
	}

	return buf.String()
}

// archiveStaticPageHTML renders one campaign as a standalone reading page.
func archiveStaticPageHTML(page archiveStaticPage) string {
	var buf bytes.Buffer
	if err := archive.RenderPage(&buf, archive.PageData{
		Lang:       archiveLang,
		Title:      page.Title,
		Stylesheet: archiveStylesheetFile,
		BackHref:   archiveIndexFile,
		BackLabel:  archiveSiteTitle,
		Date:       page.Date,
		DateTime:   page.DateTime,
		Body:       page.Body,
	}); err != nil {
		return ""
	}

	return buf.String()
}

// archiveStaticDate returns a display date and an ISO date for a campaign.
func archiveStaticDate(t null.Time) (string, string) {
	if !t.Valid {
		return "", ""
	}

	return t.Time.Format("2 Jan 2006"), t.Time.Format("2006-01-02")
}

// archiveStaticExcerpt returns a one-line plain text summary of a page body.
func archiveStaticExcerpt(body template.HTML) string {
	text := html.UnescapeString(reHTMLTag.ReplaceAllString(string(body), " "))
	text = strings.Join(strings.Fields(text), " ")

	runes := []rune(text)
	if len(runes) > excerptLength {
		text = strings.TrimSpace(string(runes[:excerptLength])) + "…"
	}

	return text
}
