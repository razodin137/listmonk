package models

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	gast "github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	plainHTMLIgnored = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
	plainHTMLBreak   = regexp.MustCompile(`(?i)<br\s*/?>`)
	plainHTMLTags    = regexp.MustCompile(`(?s)<[^>]*>`)
)

// renderMarkdownText converts Markdown to a readable plain-text alternative.
// The output keeps the words, link destinations, and list structure while
// dropping the Markdown and HTML markup that a text-only reader cannot use.
func renderMarkdownText(body []byte) (string, error) {
	doc := markdown.Parser().Parse(text.NewReader(body))
	r := plainTextRenderer{source: body}

	return strings.TrimSpace(r.renderBlocks(doc)), nil
}

type plainTextRenderer struct {
	source []byte
}

func (r *plainTextRenderer) renderBlocks(n gast.Node) string {
	var blocks []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if block := strings.Trim(r.renderBlock(c), "\n"); block != "" {
			blocks = append(blocks, block)
		}
	}

	return strings.Join(blocks, "\n\n")
}

func (r *plainTextRenderer) renderBlock(n gast.Node) string {
	switch node := n.(type) {
	case *gast.Paragraph:
		return strings.TrimSpace(r.renderInlineChildren(node))
	case *gast.Heading:
		return strings.TrimSpace(r.renderInlineChildren(node))
	case *gast.TextBlock:
		return strings.TrimSpace(r.renderInlineChildren(node))
	case *gast.ThematicBreak:
		return "---"
	case *gast.CodeBlock:
		return strings.TrimRight(r.renderLines(node.Lines()), "\n")
	case *gast.FencedCodeBlock:
		return strings.TrimRight(r.renderLines(node.Lines()), "\n")
	case *gast.Blockquote:
		return quotePlainText(r.renderBlocks(node))
	case *gast.HTMLBlock:
		return plainHTMLText(r.renderLines(node.Lines()))
	case *gast.List:
		return r.renderList(node)
	case *extast.Table:
		return r.renderTable(node)
	case *extast.FootnoteList:
		return r.renderBlocks(node)
	case *extast.Footnote:
		return fmt.Sprintf("[%d] %s", node.Index, r.renderBlocks(node))
	}

	if n.Type() == gast.TypeInline {
		return r.renderInline(n)
	}

	return r.renderBlocks(n)
}

func (r *plainTextRenderer) renderInlineChildren(n gast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		b.WriteString(r.renderInline(c))
	}

	return b.String()
}

func (r *plainTextRenderer) renderInline(n gast.Node) string {
	switch node := n.(type) {
	case *gast.Text:
		value := node.Segment.Value(r.source)
		if !node.IsRaw() {
			value = util.ResolveNumericReferences(util.ResolveEntityNames(value))
			value = util.UnescapePunctuations(value)
		}

		out := string(value)
		if node.HardLineBreak() {
			return out + "\n"
		}
		if node.SoftLineBreak() {
			return out + " "
		}

		return out
	case *gast.String:
		return string(node.Value)
	case *gast.CodeSpan, *gast.Emphasis:
		return r.renderInlineChildren(node)
	case *extast.Strikethrough:
		return r.renderInlineChildren(node)
	case *gast.Link:
		return renderLink(r.renderInlineChildren(node), node.Destination)
	case *gast.AutoLink:
		return string(node.Label(r.source))
	case *gast.Image:
		return r.renderInlineChildren(node)
	case *extast.FootnoteLink:
		return fmt.Sprintf("[%d]", node.Index)
	case *extast.FootnoteBacklink, *gast.RawHTML:
		return ""
	case *extast.TaskCheckBox:
		if node.IsChecked {
			return "[x] "
		}
		return "[ ] "
	}

	if n.Type() == gast.TypeInline {
		return r.renderInlineChildren(n)
	}

	return ""
}

func (r *plainTextRenderer) renderList(n *gast.List) string {
	var items []string
	number := n.Start
	if number == 0 {
		number = 1
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		item, ok := c.(*gast.ListItem)
		if !ok {
			continue
		}

		prefix := "- "
		if n.IsOrdered() {
			prefix = fmt.Sprintf("%d. ", number)
			number++
		}

		items = append(items, prefixPlainText(r.renderBlocks(item), prefix))
	}

	return strings.Join(items, "\n")
}

func (r *plainTextRenderer) renderTable(n *extast.Table) string {
	var rows []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		var cells []string
		for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, strings.TrimSpace(r.renderInlineChildren(cell)))
		}
		if len(cells) == 0 {
			continue
		}

		rows = append(rows, strings.Join(cells, " | "))
		if c.Kind() == extast.KindTableHeader {
			rule := make([]string, len(cells))
			for i := range rule {
				rule[i] = "---"
			}
			rows = append(rows, strings.Join(rule, " | "))
		}
	}

	return strings.Join(rows, "\n")
}

func (r *plainTextRenderer) renderLines(lines *text.Segments) string {
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(r.source))
	}

	return b.String()
}

func renderLink(label string, destination []byte) string {
	label = strings.TrimSpace(label)
	dest := plainTextValue(destination)

	switch {
	case dest == "":
		return label
	case label == "", label == dest:
		return dest
	default:
		return label + " (" + dest + ")"
	}
}

func plainTextValue(v []byte) string {
	v = util.ResolveNumericReferences(util.ResolveEntityNames(v))
	v = util.UnescapePunctuations(v)

	return string(v)
}

func quotePlainText(s string) string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}

	return strings.Join(lines, "\n")
}

func prefixPlainText(s, prefix string) string {
	lines := strings.Split(s, "\n")
	indent := strings.Repeat(" ", len(prefix))

	for i := range lines {
		switch {
		case i == 0:
			lines[i] = prefix + lines[i]
		case lines[i] == "":
			lines[i] = ""
		default:
			lines[i] = indent + lines[i]
		}
	}

	return strings.Join(lines, "\n")
}

func plainHTMLText(s string) string {
	s = plainHTMLIgnored.ReplaceAllString(s, "")
	s = plainHTMLBreak.ReplaceAllString(s, "\n")
	s = plainHTMLTags.ReplaceAllString(s, "")

	return strings.TrimSpace(html.UnescapeString(s))
}
