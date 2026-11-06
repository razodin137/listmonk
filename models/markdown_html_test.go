package models

import (
	"strings"
	"testing"
)

const testRawHTMLBody = `# Join the list

Intro paragraph with <span class="badge">inline HTML</span>.

<form action="/subscribe" method="post" class="signup-form">
  <label for="email">Email</label>
  <input id="email" type="email" name="email" required>
  <button class="button" type="submit">Subscribe</button>
</form>

<table class="pricing">
  <thead>
    <tr><th>Plan</th><th>Price</th></tr>
  </thead>
  <tbody>
    <tr><td>Basic</td><td>$5</td></tr>
  </tbody>
</table>

Closing paragraph.
`

func TestMarkdownPreservesRawHTML(t *testing.T) {
	out, warnings, err := renderMarkdown([]byte(testRawHTMLBody), nil)
	if err != nil {
		t.Fatalf("renderMarkdown() error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	for name, want := range map[string]string{
		"inline HTML":    `<span class="badge">inline HTML</span>`,
		"form":           `<form action="/subscribe" method="post" class="signup-form">`,
		"input":          `<input id="email" type="email" name="email" required>`,
		"button":         `<button class="button" type="submit">Subscribe</button>`,
		"table":          `<table class="pricing">`,
		"table row":      `<tr><td>Basic</td><td>$5</td></tr>`,
		"markdown still": `<h1 style="`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("%s did not survive rendering: %q", name, out)
		}
	}

	for _, escaped := range []string{"&lt;form", "&lt;table", "&lt;button", "&lt;span"} {
		if strings.Contains(out, escaped) {
			t.Fatalf("raw HTML was escaped (%s): %q", escaped, out)
		}
	}
}

func TestMarkdownRawHTMLIsIdenticalInEmailAndArchive(t *testing.T) {
	c := Campaign{
		ContentType:         CampaignContentTypeMarkdown,
		Body:                testRawHTMLBody,
		TemplateBody:        `EMAIL<html><body>{{ template "content" . }}</body></html>`,
		ArchiveTemplateBody: `ARCHIVE<html><body>{{ template "content" . }}</body></html>`,
	}

	email := testBodyOf(t, renderTestCampaign(t, &c, c.CompileTemplate))
	archive := testBodyOf(t, renderTestCampaign(t, &c, c.CompileArchiveTemplate))

	if email != archive {
		t.Fatalf("email content = %q, archive content = %q", email, archive)
	}

	if !strings.Contains(email, `<button class="button" type="submit">Subscribe</button>`) {
		t.Fatalf("raw HTML missing from rendered body: %q", email)
	}
	if strings.Contains(email, "&lt;form") || strings.Contains(email, "&lt;table") {
		t.Fatalf("raw HTML was escaped in the rendered body: %q", email)
	}
}

func TestConvertContentPreservesRawHTML(t *testing.T) {
	c := Campaign{
		ContentType: CampaignContentTypeMarkdown,
		Body:        testRawHTMLBody,
	}

	out, err := c.ConvertContent(CampaignContentTypeMarkdown, CampaignContentTypeHTML)
	if err != nil {
		t.Fatalf("ConvertContent() error: %v", err)
	}

	if !strings.Contains(out, `<form action="/subscribe" method="post" class="signup-form">`) {
		t.Fatalf("raw HTML missing from converted body: %q", out)
	}
	if strings.Contains(out, "&lt;form") {
		t.Fatalf("raw HTML was escaped in the converted body: %q", out)
	}
}
