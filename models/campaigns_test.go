package models

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

const testMarkdownBody = `# First

Intro paragraph.

- one
- two

## Second

Closing paragraph.
`

func TestMarkdownRendersInEmailAndArchive(t *testing.T) {
	c := Campaign{
		ContentType:         CampaignContentTypeMarkdown,
		Body:                testMarkdownBody,
		TemplateBody:        `EMAIL<html><body>{{ template "content" . }}</body></html>`,
		ArchiveTemplateBody: `ARCHIVE<html><body>{{ template "content" . }}</body></html>`,
	}

	email := renderTestCampaign(t, &c, c.CompileTemplate)
	archive := renderTestCampaign(t, &c, c.CompileArchiveTemplate)

	if !strings.Contains(email, "EMAIL") {
		t.Fatalf("email output did not use the campaign template: %q", email)
	}
	if !strings.Contains(archive, "ARCHIVE") {
		t.Fatalf("archive output did not use the archive template: %q", archive)
	}

	emailContent := testBodyOf(t, email)
	archiveContent := testBodyOf(t, archive)
	if emailContent != archiveContent {
		t.Fatalf("email content = %q, archive content = %q", emailContent, archiveContent)
	}

	if !strings.Contains(emailContent, "<h1") || !strings.Contains(emailContent, "<li>one</li>") {
		t.Fatalf("markdown was not rendered: %q", emailContent)
	}
}

func TestMakeArchiveSlug(t *testing.T) {
	used := make(map[string]struct{})

	cases := []struct {
		in   string
		want string
	}{
		{"Hello, World!", "hello-world"},
		{"  Go -- Go  ", "go-go"},
		{"!!!", "untitled"},
		{"Hello World", "hello-world-2"},
		{"Hello World", "hello-world-3"},
		{"", "untitled-2"},
	}

	for _, tc := range cases {
		if got := MakeArchiveSlug(tc.in, used); got != tc.want {
			t.Errorf("MakeArchiveSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCompileContentTemplateOmitsWrapper(t *testing.T) {
	c := Campaign{
		ContentType:         CampaignContentTypeMarkdown,
		Body:                testMarkdownBody,
		TemplateBody:        `EMAIL<html><body>{{ template "content" . }}</body></html>`,
		ArchiveTemplateBody: `ARCHIVE<html><body>{{ template "content" . }}</body></html>`,
	}

	content := renderTestCampaign(t, &c, c.CompileContentTemplate)
	if strings.Contains(content, "EMAIL") || strings.Contains(content, "ARCHIVE") {
		t.Fatalf("content template included a wrapper: %q", content)
	}
	if !strings.Contains(content, "<h1") || !strings.Contains(content, "<li>one</li>") {
		t.Fatalf("markdown was not rendered: %q", content)
	}
}

func TestArchiveTemplateFallsBackToCampaignTemplate(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         testMarkdownBody,
		TemplateBody: `CAMPAIGN<html><body>{{ template "content" . }}</body></html>`,
	}

	archive := renderTestCampaign(t, &c, c.CompileArchiveTemplate)
	if !strings.Contains(archive, "CAMPAIGN") {
		t.Fatalf("archive output did not fall back to the campaign template: %q", archive)
	}
}

func TestArchivedForPublic(t *testing.T) {
	cases := []struct {
		name string
		c    Campaign
		want bool
	}{
		{"draft stays hidden", Campaign{Archive: true, Status: CampaignStatusDraft, Type: CampaignTypeRegular}, false},
		{"finished mail is public", Campaign{Archive: true, Status: CampaignStatusFinished, Type: CampaignTypeRegular}, true},
		{"finished post is public", Campaign{Archive: true, Status: CampaignStatusFinished, Type: CampaignTypePost}, true},
		{"running mail is public", Campaign{Archive: true, Status: CampaignStatusRunning, Type: CampaignTypeRegular}, true},
		{"unarchived post is hidden", Campaign{Archive: false, Status: CampaignStatusFinished, Type: CampaignTypePost}, false},
		{"optin is hidden", Campaign{Archive: true, Status: CampaignStatusFinished, Type: CampaignTypeOptin}, false},
	}

	for _, tc := range cases {
		if got := tc.c.ArchivedForPublic(); got != tc.want {
			t.Errorf("%s: ArchivedForPublic() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func renderTestCampaign(t *testing.T, c *Campaign, compile func(template.FuncMap) error) string {
	t.Helper()

	if err := compile(template.FuncMap{}); err != nil {
		t.Fatalf("error compiling template: %v", err)
	}

	var out bytes.Buffer
	if err := c.Tpl.ExecuteTemplate(&out, BaseTpl, nil); err != nil {
		t.Fatalf("error rendering template: %v", err)
	}

	return out.String()
}

func testBodyOf(t *testing.T, html string) string {
	t.Helper()

	start := strings.Index(html, "<body>")
	end := strings.LastIndex(html, "</body>")
	if start < 0 || end < start {
		t.Fatalf("output has no body: %q", html)
	}

	return html[start+len("<body>") : end]
}
