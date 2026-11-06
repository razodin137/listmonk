package main

import (
	"bytes"
	"html"
	"html/template"
	"strings"

	null "gopkg.in/volatiletech/null.v6"
)

const (
	archiveIndexFile      = "index.html"
	archivePageExt        = ".html"
	archiveStylesheetFile = "style.css"
	archiveSiteTitle      = "Mailing list archive"
)

type archiveStaticPage struct {
	Name     string
	Title    string
	Date     string
	DateTime string
	Body     template.HTML
}

type archiveStaticIndexEntry struct {
	Title    string
	Href     string
	Date     string
	DateTime string
	Excerpt  string
}

var archiveIndexTemplate = template.Must(template.New("archive-index").Parse(`<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<link rel="stylesheet" href="style.css">
</head>
<body>
	<main class="archive">
		<h1>{{ .Title }}</h1>
		{{ if .Entries }}
			<ul class="archive-list">
				{{ range .Entries }}
					<li>
						<a href="{{ .Href }}">{{ .Title }}</a>
						{{ if .DateTime }}<time class="date" datetime="{{ .DateTime }}">{{ .Date }}</time>{{ end }}
						{{ if .Excerpt }}<p class="excerpt">{{ .Excerpt }}</p>{{ end }}
					</li>
				{{ end }}
			</ul>
		{{ else }}
			<p class="empty">No archived messages yet.</p>
		{{ end }}
	</main>
</body>
</html>
`))

var archivePageTemplate = template.Must(template.New("archive-page").Parse(`<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<link rel="stylesheet" href="style.css">
</head>
<body>
	<main class="archive-campaign">
		<p class="back"><a href="index.html">&larr; {{ .SiteTitle }}</a></p>
		<h1>{{ .Title }}</h1>
		{{ if .DateTime }}<p class="date"><time datetime="{{ .DateTime }}">{{ .Date }}</time></p>{{ end }}
		<div class="body">{{ .Body }}</div>
	</main>
</body>
</html>
`))

const archiveStylesheet = `:root {
	color-scheme: light dark;
}
* {
	box-sizing: border-box;
}
body {
	margin: 0;
	padding: 2rem 1.25rem 4rem;
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
	font-size: 16px;
	line-height: 1.6;
	color: #222;
	background: #fff;
}
a {
	color: #1c5d99;
}
h1 {
	margin: 0 0 1.5rem;
	font-size: 1.75rem;
	line-height: 1.25;
}
.archive,
.archive-campaign {
	max-width: 42rem;
	margin: 0 auto;
}
.archive-list {
	margin: 0;
	padding: 0;
	list-style: none;
}
.archive-list li {
	margin: 0 0 1.5rem;
}
.archive-list a {
	font-size: 1.125rem;
	font-weight: 600;
	text-decoration: none;
}
.archive-list a:hover {
	text-decoration: underline;
}
.archive .date,
.archive-campaign .date {
	display: block;
	margin-top: 0.25rem;
	color: #666;
	font-size: 0.875rem;
}
.archive .excerpt {
	margin: 0.25rem 0 0;
	color: #444;
	font-size: 0.9375rem;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}
.archive-campaign .back {
	margin: 0 0 2rem;
	font-size: 0.9375rem;
}
.archive-campaign .body {
	margin-top: 2rem;
	line-height: 1.7;
	overflow-wrap: break-word;
}
.archive-campaign .body img {
	max-width: 100%;
	height: auto;
}
.archive-campaign .body pre {
	overflow-x: auto;
	padding: 0.75rem;
	background: #f4f4f4;
}
.archive-campaign .body table {
	width: 100%;
	border-collapse: collapse;
}
.archive-campaign .body td,
.archive-campaign .body th {
	padding: 0.4rem;
	border: 1px solid #ddd;
}
.archive-campaign .body blockquote {
	margin: 0;
	padding-left: 1rem;
	border-left: 3px solid #eee;
	color: #555;
}
.empty {
	color: #666;
}
@media (prefers-color-scheme: dark) {
	body {
		color: #e6e6e6;
		background: #111;
	}
	a {
		color: #8ab4f8;
	}
	.archive .date,
	.archive-campaign .date,
	.empty {
		color: #aaa;
	}
	.archive .excerpt,
	.archive-campaign .body blockquote {
		color: #ccc;
	}
	.archive-campaign .body pre {
		background: #222;
	}
	.archive-campaign .body td,
	.archive-campaign .body th {
		border-color: #444;
	}
	.archive-campaign .body blockquote {
		border-color: #444;
	}
}
`

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
		Body: archiveStylesheet,
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
	entries := make([]archiveStaticIndexEntry, 0, len(pages))
	for _, page := range pages {
		entries = append(entries, archiveStaticIndexEntry{
			Title:    page.Title,
			Href:     page.Name,
			Date:     page.Date,
			DateTime: page.DateTime,
			Excerpt:  archiveStaticExcerpt(page.Body),
		})
	}

	var buf bytes.Buffer
	if err := archiveIndexTemplate.Execute(&buf, struct {
		Title   string
		Entries []archiveStaticIndexEntry
	}{archiveSiteTitle, entries}); err != nil {
		return ""
	}

	return buf.String()
}

// archiveStaticPageHTML renders one campaign as a standalone reading page.
func archiveStaticPageHTML(page archiveStaticPage) string {
	var buf bytes.Buffer
	if err := archivePageTemplate.Execute(&buf, struct {
		SiteTitle string
		archiveStaticPage
	}{archiveSiteTitle, page}); err != nil {
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
