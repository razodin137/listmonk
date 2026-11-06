package models

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRenderArchivePostWritesHugoFrontMatter(t *testing.T) {
	date := time.Date(2026, 11, 6, 10, 30, 0, 0, time.FixedZone("", 7*60*60))
	body := "<h1>Heading</h1>\n<p>First &amp; only paragraph.</p>\n<p>Second paragraph.</p>"

	out, err := RenderArchivePost(ArchivePost{
		Title:       "Hello: World",
		Slug:        "hello-world",
		Date:        date,
		Description: ArchiveDescription(body),
		Tags:        []string{"go", "news"},
	}, body)
	if err != nil {
		t.Fatalf("RenderArchivePost() error: %v", err)
	}

	fm, rest, ok, err := ParseFrontMatter(out)
	if err != nil {
		t.Fatalf("ParseFrontMatter() error: %v", err)
	}
	if !ok {
		t.Fatal("rendered post has no front matter")
	}
	if rest != "\n"+body {
		t.Fatalf("body = %q, want %q", rest, "\n"+body)
	}
	if fm.Title != "Hello: World" {
		t.Fatalf("title = %q", fm.Title)
	}
	if fm.Slug != "hello-world" {
		t.Fatalf("slug = %q", fm.Slug)
	}
	if fm.Description != "First & only paragraph." {
		t.Fatalf("description = %q", fm.Description)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "go" || fm.Tags[1] != "news" {
		t.Fatalf("tags = %v", fm.Tags)
	}
	if !fm.Date.Equal(date) {
		t.Fatalf("date = %v, want %v", fm.Date, date)
	}
	if !strings.Contains(out, "\ndraft: false\n") {
		t.Fatalf("draft is not false in %q", out)
	}
}

func TestRenderArchivePostKeepsEmptyTagsAndMissingDateHugoReady(t *testing.T) {
	out, err := RenderArchivePost(ArchivePost{
		Title:       "No tags",
		Slug:        "no-tags",
		Description: ArchiveDescription("<p>Only a paragraph.</p>"),
	}, "<p>Only a paragraph.</p>")
	if err != nil {
		t.Fatalf("RenderArchivePost() error: %v", err)
	}

	if !strings.Contains(out, "tags: []\n") {
		t.Fatalf("empty tags are not an empty list: %q", out)
	}
	if strings.Contains(out, "date:") {
		t.Fatalf("missing date was written: %q", out)
	}
}

func TestRenderArchivePostKeepsHTMLBodyUnchanged(t *testing.T) {
	const htmlBody = `<div class="email">
  <h1>Hello &amp; welcome</h1>
  <p>Use *asterisks* and # hashes literally, not as Markdown.</p>
  <a href="/unsubscribe?u=1&amp;c=2">Unsubscribe</a>
  <img src="https://example.com/banner.png" alt="A &amp; B">
</div>
`

	out, err := RenderArchivePost(ArchivePost{
		Title: "HTML body",
		Slug:  "html-body",
	}, htmlBody)
	if err != nil {
		t.Fatalf("RenderArchivePost() error: %v", err)
	}

	_, rest, ok, err := ParseFrontMatter(out)
	if err != nil {
		t.Fatalf("ParseFrontMatter() error: %v", err)
	}
	if !ok {
		t.Fatal("rendered post has no front matter")
	}
	if rest != "\n"+htmlBody {
		t.Fatalf("body = %q, want %q", rest, "\n"+htmlBody)
	}

	for _, want := range []string{
		`<h1>Hello &amp; welcome</h1>`,
		`<a href="/unsubscribe?u=1&amp;c=2">Unsubscribe</a>`,
		`<img src="https://example.com/banner.png" alt="A &amp; B">`,
		"Use *asterisks* and # hashes literally, not as Markdown.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML body was altered, missing %q in %q", want, out)
		}
	}

	for _, escaped := range []string{"&lt;div", "&lt;h1", "&lt;a ", "&lt;img", "<em>asterisks</em>"} {
		if strings.Contains(out, escaped) {
			t.Fatalf("HTML body was escaped or rendered as Markdown (%s): %q", escaped, out)
		}
	}
}

func TestExportedArchivePostParsesAsYAMLAndHTML(t *testing.T) {
	date := time.Date(2026, 11, 6, 10, 30, 0, 0, time.UTC)
	htmlBody := `<div><h1>Hello &amp; welcome</h1><p>First paragraph.</p></div>`

	slug := MakeArchiveSlug("Hello: World", map[string]struct{}{})
	if slug != "hello-world" {
		t.Fatalf("MakeArchiveSlug() = %q, want %q", slug, "hello-world")
	}

	post, err := RenderArchivePost(ArchivePost{
		Title:       "Hello: World",
		Slug:        slug,
		Date:        date,
		Description: "Hello & welcome",
		Tags:        []string{"go", "news"},
	}, htmlBody)
	if err != nil {
		t.Fatalf("RenderArchivePost() error: %v", err)
	}

	if name := ArchiveFileName(slug) + ".md"; name != "hello-world.md" {
		t.Fatalf("exported file name = %q, want %q", name, "hello-world.md")
	}

	block, body := splitExportedPost(t, post)

	var meta struct {
		Title       string   `yaml:"title"`
		Slug        string   `yaml:"slug"`
		Date        string   `yaml:"date"`
		Description string   `yaml:"description"`
		Tags        []string `yaml:"tags"`
		Draft       bool     `yaml:"draft"`
	}
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		t.Fatalf("front matter is not valid YAML: %v", err)
	}

	if meta.Title != "Hello: World" {
		t.Fatalf("title = %q", meta.Title)
	}
	if meta.Slug != "hello-world" {
		t.Fatalf("slug = %q", meta.Slug)
	}
	if meta.Date != "2026-11-06T10:30:00Z" {
		t.Fatalf("date = %q", meta.Date)
	}
	if meta.Description != "Hello & welcome" {
		t.Fatalf("description = %q", meta.Description)
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "go" || meta.Tags[1] != "news" {
		t.Fatalf("tags = %v", meta.Tags)
	}
	if meta.Draft {
		t.Fatal("draft = true, want false")
	}

	if body != "\n"+htmlBody {
		t.Fatalf("body = %q, want %q", body, "\n"+htmlBody)
	}

	doc := parseHTMLFragment(t, body)
	if doc.XMLName.Local != "div" {
		t.Fatalf("body root = %q, want div", doc.XMLName.Local)
	}
	if len(doc.Nodes) != 2 || doc.Nodes[0].XMLName.Local != "h1" || doc.Nodes[1].XMLName.Local != "p" {
		t.Fatalf("body elements = %+v, want h1 then p", doc.Nodes)
	}
	if doc.Nodes[0].Content != "Hello & welcome" {
		t.Fatalf("heading = %q, want %q", doc.Nodes[0].Content, "Hello & welcome")
	}
}

func splitExportedPost(t *testing.T, post string) (string, string) {
	t.Helper()

	const delimiter = "---\n"
	if !strings.HasPrefix(post, delimiter) {
		t.Fatalf("export does not start with front matter: %q", post)
	}

	rest := post[len(delimiter):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		t.Fatalf("export has no closing front matter delimiter: %q", post)
	}

	return rest[:end], rest[end+len("\n---\n"):]
}

type htmlNode struct {
	XMLName xml.Name
	Content string     `xml:",chardata"`
	Nodes   []htmlNode `xml:",any"`
}

func parseHTMLFragment(t *testing.T, body string) htmlNode {
	t.Helper()

	var node htmlNode
	if err := xml.Unmarshal([]byte(strings.TrimSpace(body)), &node); err != nil {
		t.Fatalf("body is not valid HTML markup: %v", err)
	}

	return node
}

func TestArchiveFileNameUsesSlugAndFallsBackToUntitled(t *testing.T) {
	cases := map[string]struct {
		slug string
		want string
	}{
		"plain slug":         {slug: "hello-world", want: "hello-world"},
		"surrounding spaces": {slug: "  hello-world  ", want: "hello-world"},
		"path separators":    {slug: "a/b\\c", want: "a-b-c"},
		"control characters": {slug: "hello\x00world\n", want: "hello-world"},
		"dots and spaces":    {slug: ".. ", want: "untitled"},
		"empty":              {slug: "", want: "untitled"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ArchiveFileName(tc.slug); got != tc.want {
				t.Fatalf("ArchiveFileName(%q) = %q, want %q", tc.slug, got, tc.want)
			}
		})
	}
}

func TestArchiveDescriptionUsesFirstParagraph(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"paragraph after heading": {
			body: "<h1>A heading</h1>\n<p>First paragraph.</p>\n<p>Second.</p>",
			want: "First paragraph.",
		},
		"entities and inline tags": {
			body: "<p>Hello <strong>there</strong> &amp; welcome.</p>",
			want: "Hello there & welcome.",
		},
		"no paragraph": {
			body: "<div>Plain block.</div>",
			want: "Plain block.",
		},
		"empty body": {
			body: "",
			want: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ArchiveDescription(tc.body); got != tc.want {
				t.Fatalf("ArchiveDescription() = %q, want %q", got, tc.want)
			}
		})
	}
}
