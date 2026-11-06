package models

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx/types"
)

var reCardSlug = regexp.MustCompile(`[^\p{L}\p{M}\p{N}]+`)

// MakeSlug normalizes a string into a URL-safe slug, keeping Unicode
// letters and numbers and collapsing everything else into dashes.
func MakeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reCardSlug.ReplaceAllString(s, "-")

	return strings.Trim(s, "-")
}

// Cards represents a slice of Card.
type Cards []Card

// Card is a named public signup form that pins a fixed set of lists
// (labels). The lists remain ordinary lists underneath.
type Card struct {
	Base

	UUID        string         `db:"uuid" json:"uuid"`
	Name        string         `db:"name" json:"name"`
	Slug        string         `db:"slug" json:"slug"`
	Description string         `db:"description" json:"description"`
	Lists       types.JSONText `db:"lists" json:"lists"`

	// ListIDs is accepted when creating or updating a card and is not stored
	// on the card row itself.
	ListIDs []int `db:"-" json:"list_ids,omitempty"`

	// Pseudofield for getting the total number of cards in searches and queries.
	Total int `db:"total" json:"-"`
}

// CardList is a minimal representation of a list attached to a card,
// parsed from the raw Lists JSON for use in HTML views.
type CardList struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Optin       string `json:"optin"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// ParsedLists unmarshals the card's raw Lists JSON into a slice for
// rendering in HTML views.
func (c Card) ParsedLists() []CardList {
	if len(c.Lists) == 0 {
		return nil
	}

	var out []CardList
	if err := json.Unmarshal(c.Lists, &out); err != nil {
		return nil
	}

	return out
}

// GetIDs returns the list of card IDs.
func (c Cards) GetIDs() []int {
	ids := make([]int, len(c))
	for i, card := range c {
		ids[i] = card.ID
	}

	return ids
}
