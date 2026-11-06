package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
	"gopkg.in/yaml.v3"
)

const frontMatterDelimiter = "---"

// FrontMatter is the metadata that may precede a Markdown body.
type FrontMatter struct {
	Title       string
	Slug        string
	Description string
	Tags        []string
	Date        time.Time
}

// ParseFrontMatter splits a leading YAML front matter block from a Markdown
// body. The body is returned unchanged when no block is present.
func ParseFrontMatter(body string) (FrontMatter, string, bool, error) {
	block, rest, ok := splitFrontMatter(body)
	if !ok {
		return FrontMatter{}, body, false, nil
	}

	raw, ok, err := decodeFrontMatter(block)
	if err != nil {
		return FrontMatter{}, body, false, err
	}
	if !ok {
		return FrontMatter{}, body, false, nil
	}

	fm := FrontMatter{
		Title:       frontMatterString(raw["title"]),
		Slug:        frontMatterString(raw["slug"]),
		Description: frontMatterString(raw["description"]),
	}

	if tags, ok := raw["tags"]; ok {
		fm.Tags, err = frontMatterTags(tags)
		if err != nil {
			return FrontMatter{}, body, false, err
		}
	}

	if date, ok := raw["date"]; ok {
		fm.Date, err = frontMatterDate(date)
		if err != nil {
			return FrontMatter{}, body, false, err
		}
	}

	return fm, rest, true, nil
}

// ApplyFrontMatter reads front matter from a Markdown campaign body and
// applies the values over the campaign's form fields.
func (c *Campaign) ApplyFrontMatter() (bool, error) {
	if c.ContentType != CampaignContentTypeMarkdown {
		return false, nil
	}

	fm, body, ok, err := ParseFrontMatter(c.Body)
	if err != nil || !ok {
		return false, err
	}

	c.Body = body

	if title := strings.TrimSpace(fm.Title); title != "" {
		c.Name = title
		c.Subject = title
	}

	if slug := strings.TrimSpace(fm.Slug); slug != "" {
		c.ArchiveSlug = null.NewString(slug, true)
	}

	if len(fm.Tags) > 0 {
		c.Tags = pq.StringArray(fm.Tags)
	}

	meta := map[string]any{}
	if len(c.ArchiveMeta) > 0 {
		if err := json.Unmarshal(c.ArchiveMeta, &meta); err != nil {
			return false, fmt.Errorf("error reading campaign archive metadata: %w", err)
		}
	}

	if description := strings.TrimSpace(fm.Description); description != "" {
		meta["description"] = description
	}
	if !fm.Date.IsZero() {
		meta["date"] = fm.Date.Format(time.RFC3339)
	}

	if len(meta) > 0 {
		out, err := json.Marshal(meta)
		if err != nil {
			return false, fmt.Errorf("error writing campaign archive metadata: %w", err)
		}
		c.ArchiveMeta = json.RawMessage(out)
	}

	return true, nil
}

// bodyWithoutFrontMatter returns a Markdown body with a leading YAML block
// removed. Bodies without a valid block are returned as-is.
func bodyWithoutFrontMatter(body string) string {
	_, rest, ok, err := ParseFrontMatter(body)
	if err != nil || !ok {
		return body
	}

	return rest
}

func splitFrontMatter(body string) (string, string, bool) {
	s := strings.TrimPrefix(body, "\ufeff")

	line, rest := nextLine(s)
	if strings.TrimRight(line, " \r\t") != frontMatterDelimiter {
		return "", body, false
	}

	var block strings.Builder
	for {
		if rest == "" {
			return "", body, false
		}

		line, rest = nextLine(rest)
		switch strings.TrimRight(line, " \r\t") {
		case frontMatterDelimiter, "...":
			return block.String(), rest, true
		}

		block.WriteString(line)
		block.WriteByte('\n')
	}
}

func nextLine(s string) (string, string) {
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return s, ""
	}

	return s[:i], s[i+1:]
}

func decodeFrontMatter(block string) (map[string]any, bool, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return nil, false, fmt.Errorf("error parsing front matter: %w", err)
	}

	if len(doc.Content) == 0 {
		return map[string]any{}, true, nil
	}

	node := doc.Content[0]
	if node.Kind != yaml.MappingNode {
		return nil, false, nil
	}

	raw := make(map[string]any, len(node.Content)/2)
	if err := node.Decode(&raw); err != nil {
		return nil, false, fmt.Errorf("error parsing front matter: %w", err)
	}

	return lowerFrontMatterKeys(raw), true, nil
}

func lowerFrontMatterKeys(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[strings.ToLower(key)] = value
	}

	return out
}

func frontMatterString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case time.Time:
		return val.Format(time.RFC3339)
	case fmt.Stringer:
		return val.String()
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprint(val)
	}
}

func frontMatterTags(v any) ([]string, error) {
	var tags []string

	switch val := v.(type) {
	case nil:
		return nil, nil
	case string:
		tags = strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == '\n'
		})
	case []any:
		tags = make([]string, 0, len(val))
		for _, tag := range val {
			tags = append(tags, frontMatterString(tag))
		}
	case []string:
		tags = val
	default:
		return nil, errors.New("front matter tags must be a list or a string")
	}

	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			out = append(out, tag)
		}
	}

	return out, nil
}

func frontMatterDate(v any) (time.Time, error) {
	switch val := v.(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		return val, nil
	case string:
		return parseFrontMatterDate(val)
	case int:
		return time.Unix(int64(val), 0), nil
	case int64:
		return time.Unix(val, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid front matter date %q", frontMatterString(val))
	}
}

func parseFrontMatterDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid front matter date %q", s)
}
