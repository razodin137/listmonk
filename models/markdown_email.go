package models

import (
	"fmt"

	gast "github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const (
	emailHeadingStyle    = "font-family: Arial, Helvetica, sans-serif; font-weight: 600; line-height: 1.25; color: #1f2328; margin: 24px 0 12px;"
	emailBlockquoteStyle = "margin: 16px 0; padding: 8px 16px; border-left: 4px solid #d0d7de; color: #57606a;"
	emailCodeSpanStyle   = "font-family: Consolas, 'Courier New', monospace; font-size: 13px; background-color: #f6f8fa; border: 1px solid #d0d7de; border-radius: 3px; padding: 1px 4px;"
	emailPreStyle        = "font-family: Consolas, 'Courier New', monospace; font-size: 13px; line-height: 1.5; color: #1f2328; background-color: #f6f8fa; border: 1px solid #d0d7de; border-radius: 4px; padding: 12px 16px; margin: 16px 0; overflow-x: auto; white-space: pre-wrap; word-break: break-word;"
	emailCodeStyle       = "font-family: Consolas, 'Courier New', monospace; font-size: 13px; line-height: 1.5; color: #1f2328; background-color: transparent; padding: 0; border: 0; white-space: pre-wrap; word-break: break-word;"
	emailTableStyle      = "border-collapse: collapse; width: 100%; margin: 16px 0; font-family: Arial, Helvetica, sans-serif; font-size: 14px; line-height: 1.5;"
	emailTableCellStyle  = "border: 1px solid #d0d7de; padding: 8px 12px; vertical-align: top;"
	emailTableHeadStyle  = "background-color: #f6f8fa; font-weight: 600;"
	emailRuleStyle       = "border: 0; border-top: 1px solid #d0d7de; height: 0; margin: 24px 0;"

	emailFootnoteStyle         = "margin: 32px 0 0; padding-top: 16px; border-top: 1px solid #d0d7de; font-family: Arial, Helvetica, sans-serif; font-size: 13px; color: #57606a;"
	emailFootnoteRuleStyle     = "border: 0; border-top: 1px solid #d0d7de; height: 0; margin: 0 0 12px;"
	emailFootnoteListStyle     = "margin: 0; padding-left: 20px;"
	emailFootnoteItemStyle     = "margin: 4px 0;"
	emailFootnoteRefStyle      = "font-size: 12px; line-height: 1; vertical-align: super;"
	emailFootnoteLinkStyle     = "color: #0969da; text-decoration: none;"
	emailFootnoteBacklinkStyle = "color: #0969da; text-decoration: none;"
)

func emailHeadingStyleFor(level int) string {
	switch level {
	case 1:
		return emailHeadingStyle + " font-size: 28px;"
	case 2:
		return emailHeadingStyle + " font-size: 24px;"
	case 3:
		return emailHeadingStyle + " font-size: 20px;"
	case 4:
		return emailHeadingStyle + " font-size: 16px;"
	case 5:
		return emailHeadingStyle + " font-size: 14px;"
	default:
		return emailHeadingStyle + " font-size: 12px;"
	}
}

func emailTableCellStyleFor(align extast.Alignment, header bool) string {
	style := emailTableCellStyle
	if header {
		style += " " + emailTableHeadStyle
	}

	switch align {
	case extast.AlignLeft:
		return style + " text-align: left;"
	case extast.AlignRight:
		return style + " text-align: right;"
	case extast.AlignCenter:
		return style + " text-align: center;"
	default:
		return style
	}
}

type emailHTMLStyles struct{}

func (s *emailHTMLStyles) Transform(doc *gast.Document, reader text.Reader, pc parser.Context) {
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *gast.Heading:
			id, hasID := n.AttributeString("id")
			if hasID {
				n.RemoveAttributes()
			}
			n.SetAttributeString("style", emailHeadingStyleFor(node.Level))
			if hasID {
				n.SetAttributeString("id", id)
			}
		case *gast.Blockquote:
			n.SetAttributeString("style", emailBlockquoteStyle)
		case *gast.CodeSpan:
			n.SetAttributeString("style", emailCodeSpanStyle)
		case *gast.ThematicBreak:
			n.SetAttributeString("style", emailRuleStyle)
		case *extast.Table:
			n.SetAttributeString("style", emailTableStyle)
			n.SetAttributeString("cellpadding", "0")
			n.SetAttributeString("cellspacing", "0")
			n.SetAttributeString("border", "0")
		case *extast.TableHeader:
			n.SetAttributeString("style", emailTableHeadStyle)
		case *extast.TableCell:
			header := node.Parent() != nil && node.Parent().Kind() == extast.KindTableHeader
			n.SetAttributeString("style", emailTableCellStyleFor(node.Alignment, header))
			if node.Alignment != extast.AlignNone {
				n.SetAttributeString("align", node.Alignment.String())
			}
		}

		return gast.WalkContinue, nil
	})
}

type emailHTMLRenderer struct{}

func (r *emailHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(gast.KindFencedCodeBlock, r.renderFencedCodeBlock)
	reg.Register(extast.KindFootnote, r.renderFootnote)
	reg.Register(extast.KindFootnoteList, r.renderFootnoteList)
	reg.Register(extast.KindFootnoteLink, r.renderFootnoteLink)
	reg.Register(extast.KindFootnoteBacklink, r.renderFootnoteBacklink)
}

func (r *emailHTMLRenderer) renderCodeBlock(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<pre style="` + emailPreStyle + `"><code style="` + emailCodeStyle + `">`)
		writeMarkdownLines(w, source, n)
		return gast.WalkContinue, nil
	}

	_, _ = w.WriteString("</code></pre>\n")
	return gast.WalkContinue, nil
}

func (r *emailHTMLRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return gast.WalkContinue, nil
	}

	n := node.(*gast.FencedCodeBlock)
	_, _ = w.WriteString(`<pre style="` + emailPreStyle + `"><code style="` + emailCodeStyle + `"`)
	if language := n.Language(source); language != nil {
		_, _ = w.WriteString(` class="language-`)
		html.DefaultWriter.Write(w, language)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	writeMarkdownLines(w, source, n)

	return gast.WalkContinue, nil
}

func (r *emailHTMLRenderer) renderFootnoteList(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="footnotes" role="doc-endnotes" style="` + emailFootnoteStyle + `">` + "\n")
		_, _ = w.WriteString(`<hr style="` + emailFootnoteRuleStyle + `" />` + "\n")
		_, _ = w.WriteString(`<ol style="` + emailFootnoteListStyle + `">` + "\n")
	} else {
		_, _ = w.WriteString("</ol>\n</div>\n")
	}

	return gast.WalkContinue, nil
}

func (r *emailHTMLRenderer) renderFootnote(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*extast.Footnote)
	if entering {
		_, _ = fmt.Fprintf(w, `<li id="fn:%d" style="%s">`+"\n", n.Index, emailFootnoteItemStyle)
	} else {
		_, _ = w.WriteString("</li>\n")
	}

	return gast.WalkContinue, nil
}

func (r *emailHTMLRenderer) renderFootnoteLink(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}

	n := node.(*extast.FootnoteLink)
	_, _ = fmt.Fprintf(w, `<sup id="%s" style="%s"><a href="#fn:%d" style="%s" role="doc-noteref">%d</a></sup>`,
		emailFootnoteRefID(n.Index, n.RefIndex), emailFootnoteRefStyle, n.Index, emailFootnoteLinkStyle, n.Index)

	return gast.WalkContinue, nil
}

func (r *emailHTMLRenderer) renderFootnoteBacklink(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}

	n := node.(*extast.FootnoteBacklink)
	_, _ = fmt.Fprintf(w, `&#160;<a href="#%s" style="%s" role="doc-backlink">&#x21a9;&#xfe0e;</a>`,
		emailFootnoteRefID(n.Index, n.RefIndex), emailFootnoteBacklinkStyle)

	return gast.WalkContinue, nil
}

func emailFootnoteRefID(index, refIndex int) string {
	if refIndex > 0 {
		return fmt.Sprintf("fnref%d:%d", refIndex, index)
	}

	return fmt.Sprintf("fnref:%d", index)
}

func writeMarkdownLines(w util.BufWriter, source []byte, n gast.Node) {
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		html.DefaultWriter.RawWrite(w, line.Value(source))
	}
}
