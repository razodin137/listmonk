package main

import (
	"bytes"

	"github.com/gorilla/feeds"
)

const archiveFeedFile = "archive.xml"

// archiveFeed builds the public RSS feed from the archive dataset entries.
// The link callback supplies each entry's public address so the live feed can
// use absolute archive URLs while the static export uses relative page names.
func archiveFeed(entries []campArchiveEntry, title, siteURL, description string,
	fullContent bool, link func(index int, entry campArchiveEntry) string) *feeds.Feed {
	items := make([]*feeds.Item, 0, len(entries))
	for i, entry := range entries {
		item := &feeds.Item{
			Title:   entry.Title,
			Link:    &feeds.Link{Href: link(i, entry)},
			Id:      entry.UUID,
			Created: entry.Date.Time,
		}

		if fullContent {
			item.Content = entry.Body
		}

		items = append(items, item)
	}

	return &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: siteURL},
		Description: description,
		Items:       items,
	}
}

// writeArchiveFeed renders a feed as RSS.
func writeArchiveFeed(feed *feeds.Feed) (string, error) {
	var buf bytes.Buffer
	if err := feed.WriteRss(&buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
