package archive

import "strings"

// IndexEntry is one exported post in the content index.
type IndexEntry struct {
	Slug  string
	Title string
	Date  string
	File  string
}

// RenderIndex returns a Markdown table of exported posts. The export rewrites
// it every run, so it always describes the files currently in the directory.
func RenderIndex(entries []IndexEntry) string {
	var b strings.Builder

	b.WriteString("# Archive index\n\n")
	b.WriteString("| Slug | Title | Date | File |\n")
	b.WriteString("| --- | --- | --- | --- |\n")

	for _, entry := range entries {
		b.WriteString("| ")
		b.WriteString(indexCell(entry.Slug))
		b.WriteString(" | ")
		b.WriteString(indexCell(entry.Title))
		b.WriteString(" | ")
		b.WriteString(indexCell(entry.Date))
		b.WriteString(" | ")
		b.WriteString(indexCell(entry.File))
		b.WriteString(" |\n")
	}

	return b.String()
}

// indexCell flattens a value onto one table row and escapes its pipes.
func indexCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")

	return strings.ReplaceAll(s, "|", `\|`)
}
