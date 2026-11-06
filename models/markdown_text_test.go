package models

import (
	"strings"
	"testing"

	null "gopkg.in/volatiletech/null.v6"
)

func TestRenderMarkdownText(t *testing.T) {
	body := []byte(`# First heading

Intro with a [link](https://example.com/docs) and **bold** text.

- one
- two

1. first
2. second

> A quote

` + "```go\nfmt.Println(\"hi\")\n```" + `

| Name | Value |
| :--- | ----: |
| One  | 1     |

Text with a footnote[^1].

[^1]: The footnote text.
`)

	out, err := renderMarkdownText(body)
	if err != nil {
		t.Fatalf("renderMarkdownText() error: %v", err)
	}

	for name, want := range map[string]string{
		"heading":        "First heading",
		"paragraph":      "Intro with a link (https://example.com/docs) and bold text.",
		"bullet list":    "- one\n- two",
		"ordered list":   "1. first\n2. second",
		"quote":          "> A quote",
		"code":           `fmt.Println("hi")`,
		"table":          "Name | Value",
		"footnote":       "[1] The footnote text.",
		"footnote link":  "[1]",
		"table cell":     "One | 1",
		"ordered prefix": "2. second",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("%s missing from plain text: %q", name, out)
		}
	}

	for _, markup := range []string{"# First", "[link](", "**bold**", "```", "| :---"} {
		if strings.Contains(out, markup) {
			t.Fatalf("Markdown markup %q leaked into plain text: %q", markup, out)
		}
	}
}

func TestConvertContentToPlainText(t *testing.T) {
	c := Campaign{
		ContentType: CampaignContentTypeMarkdown,
		Body:        "# Join us\n\nRead [the docs](https://example.com/docs).\n\n- one\n- two",
	}

	out, err := c.ConvertContent(CampaignContentTypeMarkdown, CampaignContentTypePlain)
	if err != nil {
		t.Fatalf("ConvertContent() error: %v", err)
	}

	for _, want := range []string{
		"Join us",
		"Read the docs (https://example.com/docs).",
		"- one\n- two",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("plain text is missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "# Join us") || strings.Contains(out, "[the docs](") {
		t.Fatalf("Markdown markup leaked into plain text: %q", out)
	}
}

func TestMarkdownCampaignGeneratesAltBody(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "# Join us\n\nRead [the docs](https://example.com/docs).\n\n- one\n- two",
		TemplateBody: `{{ template "content" . }}`,
	}

	if err := c.CompileTemplate(nil); err != nil {
		t.Fatalf("CompileTemplate() error: %v", err)
	}

	if !c.AltBody.Valid {
		t.Fatal("Markdown campaign did not get an alt body")
	}

	for _, want := range []string{
		"Join us",
		"Read the docs (https://example.com/docs).",
		"- one\n- two",
	} {
		if !strings.Contains(c.AltBody.String, want) {
			t.Fatalf("alt body is missing %q: %q", want, c.AltBody.String)
		}
	}
}

func TestMarkdownCampaignKeepsExplicitAltBody(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "# Join us",
		AltBody:      null.NewString("Custom alt body", true),
		TemplateBody: `{{ template "content" . }}`,
	}

	if err := c.CompileTemplate(nil); err != nil {
		t.Fatalf("CompileTemplate() error: %v", err)
	}

	if c.AltBody.String != "Custom alt body" {
		t.Fatalf("explicit alt body was replaced: %q", c.AltBody.String)
	}
}

func TestNonMarkdownCampaignHasNoGeneratedAltBody(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeHTML,
		Body:         "<p>Hello</p>",
		TemplateBody: `{{ template "content" . }}`,
	}

	if err := c.CompileTemplate(nil); err != nil {
		t.Fatalf("CompileTemplate() error: %v", err)
	}

	if c.AltBody.Valid {
		t.Fatalf("HTML campaign got a generated alt body: %q", c.AltBody.String)
	}
}

func TestMarkdownAltBodyDropsFrontMatter(t *testing.T) {
	c := Campaign{
		ContentType: CampaignContentTypeMarkdown,
		Body: `---
title: Front matter title
---

Body text.`,
		TemplateBody: `{{ template "content" . }}`,
	}

	if err := c.CompileTemplate(nil); err != nil {
		t.Fatalf("CompileTemplate() error: %v", err)
	}

	if strings.Contains(c.AltBody.String, "Front matter title") || strings.Contains(c.AltBody.String, "title:") {
		t.Fatalf("front matter leaked into alt body: %q", c.AltBody.String)
	}
	if c.AltBody.String != "Body text." {
		t.Fatalf("unexpected alt body: %q", c.AltBody.String)
	}
}

func TestMarkdownAltBodyKeepsTemplateExpressions(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "# Hello\n\nVisit {{ .Subscriber.Email }}",
		TemplateBody: `{{ template "content" . }}`,
	}

	if err := c.CompileTemplate(nil); err != nil {
		t.Fatalf("CompileTemplate() error: %v", err)
	}

	if !strings.Contains(c.AltBody.String, "{{ .Subscriber.Email }}") {
		t.Fatalf("template expression missing from alt body: %q", c.AltBody.String)
	}
}
