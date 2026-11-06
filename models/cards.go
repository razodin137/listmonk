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

	UUID           string         `db:"uuid" json:"uuid"`
	Name           string         `db:"name" json:"name"`
	Slug           string         `db:"slug" json:"slug"`
	Description    string         `db:"description" json:"description"`
	SuccessMessage string         `db:"success_message" json:"success_message"`
	RedirectURL    string         `db:"redirect_url" json:"redirect_url"`
	Lists          types.JSONText `db:"lists" json:"lists"`

	// Joins is the number of signups that came through the card, with the
	// new and existing subscriber split.
	Joins         int `db:"joins" json:"joins"`
	NewJoins      int `db:"new_joins" json:"new_joins"`
	ExistingJoins int `db:"existing_joins" json:"existing_joins"`

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

// HasPrivateList reports whether any list pinned to the card has become
// private.
func (c Card) HasPrivateList() bool {
	for _, l := range c.ParsedLists() {
		if l.Type == ListTypePrivate {
			return true
		}
	}

	return false
}

// ParsedListIDs returns the IDs of the lists pinned to the card.
func (c Card) ParsedListIDs() []int {
	lists := c.ParsedLists()
	ids := make([]int, 0, len(lists))
	for _, l := range lists {
		ids = append(ids, l.ID)
	}

	return ids
}

// GetIDs returns the list of card IDs.
func (c Cards) GetIDs() []int {
	ids := make([]int, len(c))
	for i, card := range c {
		ids[i] = card.ID
	}

	return ids
}
