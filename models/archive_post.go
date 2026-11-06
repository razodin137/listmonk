package models

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ArchivePost is one archived campaign rendered as a Hugo post.
type ArchivePost struct {
	Title       string
	Slug        string
	Date        time.Time
	Description string
	Tags        []string
}

// archivePostFrontMatter is the YAML block written above an exported body.
type archivePostFrontMatter struct {
	Title       string   `yaml:"title"`
	Slug        string   `yaml:"slug"`
	Date        string   `yaml:"date,omitempty"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Draft       bool     `yaml:"draft"`
}

var (
	reArchiveParagraph = regexp.MustCompile(`(?is)<p(?:\s[^>]*)?>(.*?)</p>`)
	reArchiveHTMLTag   = regexp.MustCompile(`(?s)<[^>]*>`)
)

// RenderArchivePost writes a campaign as a Markdown file with YAML front
// matter followed by the body unchanged.
func RenderArchivePost(post ArchivePost, body string) (string, error) {
	tags := post.Tags
	if tags == nil {
		tags = []string{}
	}

	var date string
	if !post.Date.IsZero() {
		date = post.Date.UTC().Format(time.RFC3339)
	}

	meta, err := yaml.Marshal(archivePostFrontMatter{
		Title:       post.Title,
		Slug:        post.Slug,
		Date:        date,
		Description: post.Description,
		Tags:        tags,
		Draft:       false,
	})
	if err != nil {
		return "", fmt.Errorf("encoding archive front matter: %w", err)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(meta)
	b.WriteString("---\n\n")
	b.WriteString(body)

	return b.String(), nil
}

// ArchiveFileName turns a post slug into a single safe file name component.
func ArchiveFileName(slug string) string {
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

// ArchiveDescription returns the plain text of a body's first paragraph.
func ArchiveDescription(body string) string {
	if m := reArchiveParagraph.FindStringSubmatch(body); m != nil {
		return archivePlainText(m[1])
	}

	return archivePlainText(body)
}

// archivePlainText strips tags and entities from an HTML fragment and
// collapses its whitespace.
func archivePlainText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(reArchiveHTMLTag.ReplaceAllString(s, " "))), " ")
}
