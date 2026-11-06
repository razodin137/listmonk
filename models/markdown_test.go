package models

import (
	"errors"
	"strings"
	"testing"
)

func TestMarkdownResolvesMediaImages(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "![A logo](logo.png)\n\n![Remote](https://example.com/remote.png)",
		TemplateBody: `{{ template "content" . }}`,
		MediaURLResolver: func(filename string) (string, error) {
			if filename != "logo.png" {
				t.Fatalf("unexpected media lookup %q", filename)
			}

			return "https://cdn.example.com/media/logo.png", nil
		},
	}

	out := renderTestCampaign(t, &c, c.CompileTemplate)
	if !strings.Contains(out, `src="https://cdn.example.com/media/logo.png" alt="A logo"`) {
		t.Fatalf("media image was not resolved: %q", out)
	}
	if !strings.Contains(out, `src="https://example.com/remote.png" alt="Remote"`) {
		t.Fatalf("remote image was rewritten: %q", out)
	}
	if len(c.MediaWarnings) != 0 {
		t.Fatalf("unexpected media warnings: %v", c.MediaWarnings)
	}
}

func TestMarkdownReportsMissingMediaImages(t *testing.T) {
	c := Campaign{
		ContentType:  CampaignContentTypeMarkdown,
		Body:         "![Missing](missing.png)",
		TemplateBody: `{{ template "content" . }}`,
		MediaURLResolver: func(string) (string, error) {
			return "", errors.New("not found")
		},
	}

	out := renderTestCampaign(t, &c, c.CompileTemplate)
	if !strings.Contains(out, `src="missing.png" alt="Missing"`) {
		t.Fatalf("missing image was dropped from the output: %q", out)
	}
	if len(c.MediaWarnings) != 1 || c.MediaWarnings[0] != "missing.png" {
		t.Fatalf("missing image was not reported: %v", c.MediaWarnings)
	}
}

func TestMediaFilename(t *testing.T) {
	for input, want := range map[string]string{
		"logo.png":                  "logo.png",
		"team%20photo.png":          "team photo.png",
		"https://example.com/a.png": "",
		"/uploads/a.png":            "",
		"cid:inline":                "",
		"":                          "",
	} {
		if got := mediaFilename(input); got != want {
			t.Fatalf("mediaFilename(%q) = %q, want %q", input, got, want)
		}
	}
}
