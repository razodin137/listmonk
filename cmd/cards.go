package main

import (
	"net/http"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
)

// cardReq wraps a card with the list IDs (labels) sent by the client.
// Both "list_ids" and "lists" are accepted.
type cardReq struct {
	models.Card
	ListIDs []int `json:"list_ids"`
	Lists   []int `json:"lists"`
}

// ids returns the list IDs sent in the request.
func (r cardReq) ids() []int {
	if r.ListIDs != nil {
		return r.ListIDs
	}

	return r.Lists
}

// cardsQueryDefaults is the allow list of filter query params.
var cardsQueryDefaults = map[string]string{
	"page":     "",
	"per_page": "",
	"query":    "",
	"order_by": models.FieldCreatedAt,
	"order":    models.OrderDesc,
}

// GetCards retrieves signup cards.
func (a *App) GetCards(c *echo.Context) error {
	q := makeQuery(c.Request().URL.Query(), cardsQueryDefaults)
	pg := a.pg.NewFromURL(q)

	cards, total, err := a.core.QueryCards(q.Get("query"), q.Get("order_by"), q.Get("order"), pg.Offset, pg.Limit)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{models.PageResults{
		Results:   cards,
		PageProps: models.NewPageProps(q, total, pg.Page, pg.PerPage),
	}})
}

// GetCard retrieves a single signup card by ID.
func (a *App) GetCard(c *echo.Context) error {
	card, err := a.core.GetCard(getID(c), "", "")
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{card})
}

// CreateCard handles signup card creation.
func (a *App) CreateCard(c *echo.Context) error {
	var req cardReq
	if err := c.Bind(&req); err != nil {
		return err
	}

	if !strHasLen(req.Name, 1, stdInputMaxLen) {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidName"))
	}

	out, err := a.core.CreateCard(req.Card, req.ids())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// UpdateCard handles signup card modification.
func (a *App) UpdateCard(c *echo.Context) error {
	var req cardReq
	if err := c.Bind(&req); err != nil {
		return err
	}

	out, err := a.core.UpdateCard(getID(c), req.Card, req.ids())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// DeleteCard handles signup card deletion.
func (a *App) DeleteCard(c *echo.Context) error {
	if err := a.core.DeleteCard(getID(c)); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}
