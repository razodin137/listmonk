package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
	"github.com/lib/pq"
)

var cardQuerySortFields = []string{"name", "slug", models.FieldCreatedAt, "updated_at"}

// QueryCards gets cards based on multiple query params. Along with the
// paginated results, the total number of cards in the DB is returned.
func (c *Core) QueryCards(searchStr, orderBy, order string, offset, limit int) ([]models.Card, int, error) {
	var out = []models.Card{}

	queryStr, stmt := makeSearchQuery(searchStr, orderBy, order, c.q.QueryCards, cardQuerySortFields)
	if err := c.db.Select(&out, stmt, 0, "", queryStr, offset, limit); err != nil {
		c.log.Printf("error fetching cards: %v", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "cards", "error", pqErrMsg(err)))
	}

	total := 0
	if len(out) > 0 {
		total = out[0].Total
	}

	return out, total, nil
}

// GetCard gets a card by its ID, UUID, or slug.
func (c *Core) GetCard(id int, uuid, slug string) (models.Card, error) {
	var uu any
	if uuid != "" {
		uu = uuid
	}

	var out []models.Card
	if err := c.q.GetCard.Select(&out, id, uu, slug); err != nil {
		c.log.Printf("error fetching card: %v", err)
		return models.Card{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "card", "error", pqErrMsg(err)))
	}

	if len(out) == 0 {
		return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.notFound", "name", "card"))
	}

	return out[0], nil
}

// CreateCard creates a card and pins the given lists (labels) to it.
func (c *Core) CreateCard(card models.Card, listIDs []int) (models.Card, error) {
	uu, err := uuid.NewV4()
	if err != nil {
		c.log.Printf("error generating UUID: %v", err)
		return models.Card{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUUID", "error", err.Error()))
	}

	name := strings.TrimSpace(card.Name)
	if name == "" {
		return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.T("globals.messages.invalidName"))
	}

	slug := models.MakeSlug(card.Slug)
	if slug == "" {
		slug = models.MakeSlug(name)
	}
	if slug == "" {
		return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.invalidFields", "name", "slug"))
	}

	slug, err = c.uniqueCardSlug(slug, 0)
	if err != nil {
		return models.Card{}, err
	}

	listIDs = normalizeCardListIDs(listIDs)
	if err := c.validateCardListIDs(listIDs); err != nil {
		return models.Card{}, err
	}

	captcha, ok := normalizeCardCaptcha(card.Captcha)
	if !ok {
		return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.invalidFields", "name", "captcha"))
	}

	var newID int
	if err := c.q.CreateCard.Get(&newID, uu.String(), name, slug, card.Description,
		normalizeCardSuccessMessage(card.SuccessMessage), normalizeCardRedirectURL(card.RedirectURL),
		captcha, pq.Array(listIDs)); err != nil {
		c.log.Printf("error creating card: %v", err)
		return models.Card{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "card", "error", pqErrMsg(err)))
	}

	return c.GetCard(newID, "", "")
}

// UpdateCard updates a card and reconciles the lists (labels) pinned to it.
// A nil listIDs slice leaves the existing lists untouched.
func (c *Core) UpdateCard(id int, card models.Card, listIDs []int) (models.Card, error) {
	cur, err := c.GetCard(id, "", "")
	if err != nil {
		return models.Card{}, err
	}

	name := strings.TrimSpace(card.Name)
	if name == "" {
		name = cur.Name
	}

	slug := cur.Slug
	if strings.TrimSpace(card.Slug) != "" {
		slug = models.MakeSlug(card.Slug)
		if slug == "" {
			return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
				c.i18n.Ts("globals.messages.invalidFields", "name", "slug"))
		}

		slug, err = c.uniqueCardSlug(slug, id)
		if err != nil {
			return models.Card{}, err
		}
	}

	if listIDs == nil {
		listIDs = cur.ParsedListIDs()
	} else {
		listIDs = normalizeCardListIDs(listIDs)
		if err := c.validateCardListIDs(listIDs); err != nil {
			return models.Card{}, err
		}
	}

	captcha := cur.Captcha
	if strings.TrimSpace(card.Captcha) != "" {
		var ok bool
		captcha, ok = normalizeCardCaptcha(card.Captcha)
		if !ok {
			return models.Card{}, echo.NewHTTPError(http.StatusBadRequest,
				c.i18n.Ts("globals.messages.invalidFields", "name", "captcha"))
		}
	}

	if _, err := c.q.UpdateCard.Exec(id, name, slug, card.Description,
		normalizeCardSuccessMessage(card.SuccessMessage), normalizeCardRedirectURL(card.RedirectURL),
		captcha, pq.Array(listIDs)); err != nil {
		c.log.Printf("error updating card: %v", err)
		return models.Card{}, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "card", "error", pqErrMsg(err)))
	}

	return c.GetCard(id, "", "")
}

// DuplicateCard creates a copy of a card with the same labels and a unique
// name and slug.
func (c *Core) DuplicateCard(id int) (models.Card, error) {
	cur, err := c.GetCard(id, "", "")
	if err != nil {
		return models.Card{}, fmt.Errorf("fetching card %d: %w", id, err)
	}

	listIDs := make([]int, 0, len(cur.ParsedLists()))
	for _, l := range cur.ParsedLists() {
		if l.Type == models.ListTypePublic {
			listIDs = append(listIDs, l.ID)
		}
	}

	name := fmt.Sprintf("%s (copy)", cur.Name)
	return c.CreateCard(models.Card{
		Name:           name,
		Slug:           models.MakeSlug(name),
		Description:    cur.Description,
		SuccessMessage: cur.SuccessMessage,
		RedirectURL:    cur.RedirectURL,
		Captcha:        cur.Captcha,
	}, listIDs)
}

// RecordCardJoin records one successful signup that came through a card. The
// subscriber is new, or an existing subscriber who gained the card's labels.
func (c *Core) RecordCardJoin(cardID, subscriberID int, isNew bool) error {
	if _, err := c.q.InsertCardJoin.Exec(cardID, subscriberID, isNew); err != nil {
		c.log.Printf("error recording card join: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "card", "error", pqErrMsg(err)))
	}

	return nil
}

// DeleteCard deletes a card along with its list associations.
func (c *Core) DeleteCard(id int) error {
	if _, err := c.q.DeleteCard.Exec(id); err != nil {
		c.log.Printf("error deleting card: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "card", "error", pqErrMsg(err)))
	}

	return nil
}

// validateCardListIDs ensures the given list IDs exist and are public. Cards
// may only pin public lists.
func (c *Core) validateCardListIDs(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	listTypes, err := c.GetListTypes(ids, nil)
	if err != nil {
		return fmt.Errorf("fetching list types: %w", err)
	}

	if len(listTypes) != len(ids) {
		return echo.NewHTTPError(http.StatusBadRequest,
			c.i18n.Ts("globals.messages.invalidFields", "name", "lists"))
	}

	for _, typ := range listTypes {
		if typ == models.ListTypePrivate {
			return echo.NewHTTPError(http.StatusBadRequest,
				c.i18n.Ts("globals.messages.invalidFields", "name", "lists"))
		}
	}

	return nil
}

// normalizeCardListIDs returns a de-duplicated copy of ids with invalid
// entries dropped. A nil slice is returned as nil so callers can distinguish
// an omitted list from an explicit empty one.
func normalizeCardListIDs(ids []int) []int {
	if ids == nil {
		return nil
	}

	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id < 1 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

// normalizeCardCaptcha returns a valid per-card captcha mode. An empty value
// falls back to inheriting the global setting.
func normalizeCardCaptcha(mode string) (string, bool) {
	switch strings.TrimSpace(mode) {
	case "", models.CardCaptchaInherit:
		return models.CardCaptchaInherit, true
	case models.CardCaptchaOn:
		return models.CardCaptchaOn, true
	case models.CardCaptchaOff:
		return models.CardCaptchaOff, true
	default:
		return "", false
	}
}

// normalizeCardSuccessMessage trims the card's success message.
func normalizeCardSuccessMessage(msg string) string {
	return strings.TrimSpace(msg)
}

// normalizeCardRedirectURL trims the card's redirect URL.
func normalizeCardRedirectURL(raw string) string {
	return strings.TrimSpace(raw)
}

// uniqueCardSlug returns slug if no other card uses it, otherwise it appends
// an incrementing suffix until it finds a free one.
func (c *Core) uniqueCardSlug(slug string, excludeID int) (string, error) {
	base := slug

	for i := 2; ; i++ {
		var exists bool
		if err := c.q.CardSlugExists.Get(&exists, slug, excludeID); err != nil {
			c.log.Printf("error checking card slug: %v", err)
			return "", echo.NewHTTPError(http.StatusInternalServerError,
				c.i18n.Ts("globals.messages.errorFetching", "name", "card slug", "error", pqErrMsg(err)))
		}

		if !exists {
			return slug, nil
		}

		slug = fmt.Sprintf("%s-%d", base, i)
	}
}
