// Package archive holds the shared public archive theme: the list and page
// templates plus the stylesheet used by both the live site and the static
// export.
package archive

import (
	"embed"
	"html/template"
	"io"
)

const (
	ListTemplate = "archive-list"
	PageTemplate = "archive-page"
)

//go:embed theme/*.html theme/*.css
var files embed.FS

var (
	templates  = template.Must(template.ParseFS(files, "theme/*.html"))
	stylesheet = mustRead("theme/style.css")
)

// Entry is one campaign in the archive list.
type Entry struct {
	Title    string
	Href     string
	Date     string
	DateTime string
	Excerpt  string
}

// ListData is the view model for the archive list page.
type ListData struct {
	Lang           string
	SiteTitle      string
	Title          string
	Description    string
	Stylesheet     string
	FeedURL        string
	SubscribeURL   string
	SubscribeLabel string
	Pagination     template.HTML
	Entries        []Entry
	Empty          string
}

// PageData is the view model for a single archived campaign.
type PageData struct {
	Lang       string
	SiteTitle  string
	Title      string
	Stylesheet string
	BackHref   string
	BackLabel  string
	Date       string
	DateTime   string
	Body       template.HTML
}

// RenderList renders the archive list page.
func RenderList(w io.Writer, data ListData) error {
	return templates.ExecuteTemplate(w, ListTemplate, data)
}

// RenderPage renders a single archived campaign.
func RenderPage(w io.Writer, data PageData) error {
	return templates.ExecuteTemplate(w, PageTemplate, data)
}

// Stylesheet returns the archive stylesheet.
func Stylesheet() string {
	return stylesheet
}

func mustRead(name string) string {
	b, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return string(b)
}
