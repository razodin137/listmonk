package models

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownMediaResolver resolves a media library filename to its public URL.
var MarkdownMediaResolver func(string) (string, error)

// renderMarkdown converts Markdown to HTML, resolving image references that
// name a media library file and collecting the names that cannot be resolved.
func renderMarkdown(body []byte, resolve func(string) (string, error)) (string, []string, error) {
	doc := markdown.Parser().Parse(text.NewReader(body))
	warnings := resolveMarkdownImages(doc, resolve)

	var b bytes.Buffer
	if err := markdown.Renderer().Render(&b, body, doc); err != nil {
		return "", warnings, fmt.Errorf("error rendering markdown: %w", err)
	}

	return b.String(), warnings, nil
}

func resolveMarkdownImages(doc ast.Node, resolve func(string) (string, error)) []string {
	if resolve == nil {
		return nil
	}

	var warnings []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		img, ok := n.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}

		filename := mediaFilename(string(img.Destination))
		if filename == "" {
			return ast.WalkContinue, nil
		}

		resolved, err := resolve(filename)
		if err != nil || resolved == "" {
			warnings = append(warnings, filename)
			return ast.WalkContinue, nil
		}

		img.Destination = []byte(resolved)
		return ast.WalkContinue, nil
	})

	return warnings
}

// mediaFilename returns the bare filename in a Markdown image destination, or
// an empty string when the destination is a URL or an absolute path.
func mediaFilename(dest string) string {
	if dest == "" || strings.ContainsAny(dest, "/\\?#:") {
		return ""
	}

	if name, err := url.PathUnescape(dest); err == nil {
		return name
	}

	return dest
}
