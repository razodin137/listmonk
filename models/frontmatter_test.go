package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

func TestParseFrontMatter(t *testing.T) {
	body := "---\n" +
		"title: A post about front matter\n" +
		"slug: a-post-about-front-matter\n" +
		"description: The short version.\n" +
		"tags:\n" +
		"  - go\n" +
		"  - markdown\n" +
		"date: 2026-11-06T10:30:00Z\n" +
		"---\n" +
		"# Body\n"

	fm, rest, ok, err := ParseFrontMatter(body)
	if err != nil {
		t.Fatalf("ParseFrontMatter() error: %v", err)
	}
	if !ok {
		t.Fatal("ParseFrontMatter() did not find a block")
	}
	if rest != "# Body\n" {
		t.Fatalf("body = %q, want %q", rest, "# Body\n")
	}
	if fm.Title != "A post about front matter" {
		t.Fatalf("title = %q", fm.Title)
	}
	if fm.Slug != "a-post-about-front-matter" {
		t.Fatalf("slug = %q", fm.Slug)
	}
	if fm.Description != "The short version." {
		t.Fatalf("description = %q", fm.Description)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "go" || fm.Tags[1] != "markdown" {
		t.Fatalf("tags = %v", fm.Tags)
	}
	if want := time.Date(2026, 11, 6, 10, 30, 0, 0, time.UTC); !fm.Date.Equal(want) {
		t.Fatalf("date = %v, want %v", fm.Date, want)
	}
}

func TestParseFrontMatterAcceptsInlineTagsAndQuotedDate(t *testing.T) {
	body := "---\n" +
		"title: \"Quoted title\"\n" +
		"tags: [one, two]\n" +
		"date: \"2026-11-06\"\n" +
		"---\n" +
		"Body\n"

	fm, rest, ok, err := ParseFrontMatter(body)
	if err != nil {
		t.Fatalf("ParseFrontMatter() error: %v", err)
	}
	if !ok {
		t.Fatal("ParseFrontMatter() did not find a block")
	}
	if rest != "Body\n" {
		t.Fatalf("body = %q", rest)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "one" || fm.Tags[1] != "two" {
		t.Fatalf("tags = %v", fm.Tags)
	}
	if !fm.Date.Equal(time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date = %v", fm.Date)
	}
}

func TestParseFrontMatterIgnoresBodiesWithoutOne(t *testing.T) {
	for name, body := range map[string]string{
		"plain body":      "# Heading\n\nText\n",
		"unclosed block":  "---\ntitle: Not front matter\n",
		"horizontal rule": "---\n\nText\n",
		"rule with text":  "---\nSome text\n---\n",
		"delimiter later": "# Heading\n\n---\n\nText\n",
		"empty body":      "",
	} {
		t.Run(name, func(t *testing.T) {
			_, rest, ok, err := ParseFrontMatter(body)
			if err != nil {
				t.Fatalf("ParseFrontMatter() error: %v", err)
			}
			if ok {
				t.Fatalf("ParseFrontMatter() found front matter in %q", body)
			}
			if rest != body {
				t.Fatalf("body changed from %q to %q", body, rest)
			}
		})
	}
}

func TestParseFrontMatterRejectsMalformedYAML(t *testing.T) {
	body := "---\ntitle: [unclosed\n---\nBody\n"

	if _, _, _, err := ParseFrontMatter(body); err == nil {
		t.Fatal("ParseFrontMatter() did not reject malformed YAML")
	}
}

func TestApplyFrontMatterWinsOverCampaignFields(t *testing.T) {
	c := Campaign{
		ContentType: CampaignContentTypeMarkdown,
		Name:        "Form name",
		Subject:     "Form subject",
		Body: "---\n" +
			"title: Front matter title\n" +
			"slug: front-matter-slug\n" +
			"description: A description from the file.\n" +
			"tags: [go, markdown]\n" +
			"date: 2026-11-06\n" +
			"---\n" +
			"# Body\n",
		Tags:        pq.StringArray{"form-tag"},
		ArchiveSlug: null.NewString("form-slug", true),
		ArchiveMeta: json.RawMessage(`{"email":"reader@example.com"}`),
	}

	ok, err := c.ApplyFrontMatter()
	if err != nil {
		t.Fatalf("ApplyFrontMatter() error: %v", err)
	}
	if !ok {
		t.Fatal("ApplyFrontMatter() did not find a block")
	}
	if c.Name != "Front matter title" || c.Subject != "Front matter title" {
		t.Fatalf("name = %q, subject = %q", c.Name, c.Subject)
	}
	if c.ArchiveSlug.String != "front-matter-slug" {
		t.Fatalf("archive slug = %q", c.ArchiveSlug.String)
	}
	if len(c.Tags) != 2 || c.Tags[0] != "go" || c.Tags[1] != "markdown" {
		t.Fatalf("tags = %v", c.Tags)
	}
	if c.Body != "# Body\n" {
		t.Fatalf("body = %q", c.Body)
	}

	var meta map[string]any
	if err := json.Unmarshal(c.ArchiveMeta, &meta); err != nil {
		t.Fatalf("archive metadata is not valid JSON: %v", err)
	}
	if meta["email"] != "reader@example.com" {
		t.Fatalf("existing archive metadata was lost: %v", meta)
	}
	if meta["description"] != "A description from the file." {
		t.Fatalf("description metadata = %v", meta["description"])
	}
	if meta["date"] != "2026-11-06T00:00:00Z" {
		t.Fatalf("date metadata = %v", meta["date"])
	}
}

func TestApplyFrontMatterLeavesBodyAloneWithoutOne(t *testing.T) {
	c := Campaign{
		ContentType: CampaignContentTypeMarkdown,
		Name:        "Form name",
		Subject:     "Form subject",
		Body:        "# Body\n",
		Tags:        pq.StringArray{"form-tag"},
		ArchiveSlug: null.NewString("form-slug", true),
		ArchiveMeta: json.RawMessage(`{}`),
	}

	ok, err := c.ApplyFrontMatter()
	if err != nil {
		t.Fatalf("ApplyFrontMatter() error: %v", err)
	}
	if ok {
		t.Fatal("ApplyFrontMatter() found a block")
	}
	if c.Name != "Form name" || c.Subject != "Form subject" || c.Body != "# Body\n" {
		t.Fatalf("campaign changed: %+v", c)
	}
	if len(c.Tags) != 1 || c.Tags[0] != "form-tag" || c.ArchiveSlug.String != "form-slug" {
		t.Fatalf("campaign metadata changed: %+v", c)
	}
	if string(c.ArchiveMeta) != "{}" {
		t.Fatalf("archive metadata changed: %s", c.ArchiveMeta)
	}
}

func TestMarkdownRenderingDropsFrontMatter(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "---\ntitle: Hidden title\n---\n# Visible body\n",
		TemplateBody: `{{ template "content" . }}`,
	}

	out := renderTestCampaign(t, &c, c.CompileTemplate)
	if strings.Contains(out, "Hidden title") || strings.Contains(out, "title:") {
		t.Fatalf("front matter was rendered: %q", out)
	}
	if !strings.Contains(out, "Visible body") {
		t.Fatalf("body was not rendered: %q", out)
	}
}
