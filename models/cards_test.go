package models

import (
	"testing"

	"github.com/jmoiron/sqlx/types"
)

func TestMakeSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"My Card", "my-card"},
		{"  My   Card  ", "my-card"},
		{"Café News!", "café-news"},
		{"2026 Kickoff", "2026-kickoff"},
		{"a_b/c", "a-b-c"},
		{"hello--world", "hello-world"},
		{"---", ""},
		{"日本語", "日本語"},
	}

	for _, tc := range cases {
		if got := MakeSlug(tc.in); got != tc.want {
			t.Errorf("MakeSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCardParsedLists(t *testing.T) {
	c := Card{Lists: types.JSONText(`[
		{"id": 1, "uuid": "u1", "name": "News", "type": "public", "optin": "single", "status": "active", "description": "d1"},
		{"id": 2, "uuid": "u2", "name": "Events", "type": "private", "optin": "double", "status": "active", "description": "d2"}
	]`)}

	lists := c.ParsedLists()
	if len(lists) != 2 {
		t.Fatalf("ParsedLists() returned %d lists, want 2", len(lists))
	}

	if got, want := lists[0].Name, "News"; got != want {
		t.Errorf("lists[0].Name = %q, want %q", got, want)
	}
	if got, want := lists[1].ID, 2; got != want {
		t.Errorf("lists[1].ID = %d, want %d", got, want)
	}
}

func TestCardParsedListsEmpty(t *testing.T) {
	if lists := (Card{}).ParsedLists(); lists != nil {
		t.Errorf("ParsedLists() = %v, want nil", lists)
	}
}
