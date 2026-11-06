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

func TestMakeArchiveSlugSubjectEdgeCases(t *testing.T) {
	longTitle := strings.Repeat("A long subject ", 12)
	longSlug := strings.TrimSuffix(strings.Repeat("a-long-subject-", 12), "-")

	cases := []struct {
		name  string
		title string
		used  []string
		want  string
	}{
		{
			name:  "punctuation",
			title: "Hello, World! -- 2026",
			want:  "hello-world-2026",
		},
		{
			name:  "emoji between words",
			title: "Launch 🚀 day",
			want:  "launch-day",
		},
		{
			name:  "emoji only",
			title: "🚀🔥",
			want:  "untitled",
		},
		{
			name:  "quotes",
			title: `"Quoted" and 'single'`,
			want:  "quoted-and-single",
		},
		{
			name:  "curly quotes",
			title: "“Curly” and ‘single’",
			want:  "curly-and-single",
		},
		{
			name:  "apostrophe",
			title: "It's here",
			want:  "it-s-here",
		},
		{
			name:  "collision with taken slug",
			title: "Hello, World!",
			used:  []string{"hello-world"},
			want:  "hello-world-2",
		},
		{
			name:  "empty subject",
			title: "",
			want:  "untitled",
		},
		{
			name:  "only punctuation",
			title: "!!! ??? ...",
			want:  "untitled",
		},
		{
			name:  "long subject",
			title: longTitle,
			want:  longSlug,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			used := make(map[string]struct{}, len(tc.used))
			for _, slug := range tc.used {
				used[slug] = struct{}{}
			}

			if got := MakeArchiveSlug(tc.title, used); got != tc.want {
				t.Fatalf("MakeArchiveSlug(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

func TestMakeArchiveSlugSuffixesEveryCollision(t *testing.T) {
	used := make(map[string]struct{})

	got := make([]string, 4)
	for i := range got {
		got[i] = MakeArchiveSlug("Hot deal", used)
	}

	want := []string{"hot-deal", "hot-deal-2", "hot-deal-3", "hot-deal-4"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collision %d = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestMakeArchiveTitle(t *testing.T) {
	cases := map[string]struct {
		subject string
		want    string
	}{
		"surrounding spaces":     {subject: "  Hello, World!  ", want: "Hello, World!"},
		"decoded entities":       {subject: "Fish &amp; Chips", want: "Fish & Chips"},
		"decoded leading prefix": {subject: "&#82;e: Hello", want: "Hello"},
		"doubled spaces":         {subject: "Hello  World", want: "Hello World"},
		"mixed whitespace":       {subject: "Hello\n\tWorld", want: "Hello World"},
		"reply prefix":           {subject: "Re: Hello", want: "Hello"},
		"reply prefix no space":  {subject: "Re:Hello", want: "Hello"},
		"repeated prefixes":      {subject: "Re: Re: Hello", want: "Hello"},
		"other case prefix":      {subject: "RE: Hello", want: "Hello"},
		"prefix after entity":    {subject: "&nbsp;Re: Hello", want: "Hello"},
		"similar word":           {subject: "Reminder: Hello", want: "Reminder: Hello"},
		"only prefix":            {subject: "Re:   ", want: ""},
		"keeps case and words":   {subject: "mixed CASE stays", want: "mixed CASE stays"},
		"empty":                  {subject: "", want: ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := MakeArchiveTitle(tc.subject); got != tc.want {
				t.Errorf("MakeArchiveTitle(%q) = %q, want %q", tc.subject, got, tc.want)
			}
		})
	}
}

func TestCampaignArchiveTitleUsesNameNotSubject(t *testing.T) {
	c := Campaign{
		Name:    "  The post title  ",
		Subject: "Re: A different inbox subject",
	}

	if got, want := c.ArchiveTitle(), "The post title"; got != want {
		t.Fatalf("ArchiveTitle() = %q, want %q", got, want)
	}
}

func TestCampaignArchiveTitleFallsBackToSubject(t *testing.T) {
	c := Campaign{Subject: "Fallback subject"}

	if got, want := c.ArchiveTitle(), "Fallback subject"; got != want {
		t.Fatalf("ArchiveTitle() = %q, want %q", got, want)
	}
}

func TestDefaultSubjectFillsOnlyWhenEmpty(t *testing.T) {
	cases := map[string]struct {
		c       Campaign
		subject string
	}{
		"empty subject":       {c: Campaign{Name: "Post title"}, subject: "Post title"},
		"whitespace subject":  {c: Campaign{Name: "Post title", Subject: "   "}, subject: "Post title"},
		"subject already set": {c: Campaign{Name: "Post title", Subject: "Inbox subject"}, subject: "Inbox subject"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.c.DefaultSubject()
			if tc.c.Subject != tc.subject {
				t.Fatalf("subject = %q, want %q", tc.c.Subject, tc.subject)
			}
		})
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
