package models

import (
	"strings"
	"testing"
	"time"
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
