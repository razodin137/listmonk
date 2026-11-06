package main

import (
	"fmt"
	"html"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/captcha"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
)

const (
	// cardEmbedBaseHeight is the iframe height that fits a card with no
	// description or captcha. Extra content grows the frame from there.
	cardEmbedBaseHeight = 400

	// cardEmbedDescriptionHeight is the space taken by a one-line description.
	cardEmbedDescriptionHeight = 38

	// cardEmbedLineHeight is the extra height per additional description line.
	cardEmbedLineHeight = 26

	// cardEmbedCaptchaHeight is the space taken by a captcha widget.
	cardEmbedCaptchaHeight = 110

	// cardEmbedDescLineLen is the approximate number of characters that fit
	// on one line of a card description.
	cardEmbedDescLineLen = 55
)

// cardsView is the admin page view for the signup cards list.
type cardsView struct {
	adminView

	Cards []models.Card
	Page  models.PageProps
}

// cardView is the admin page view for a single signup card.
type cardView struct {
	adminView

	Card     models.Card
	AllLists []models.List
	IsNew    bool
	Tab      string
	Embed    cardEmbed
}

// cardEmbed is the iframe snippet shown on a card's overview page.
type cardEmbed struct {
	URL    string
	Height int
	Code   string
}

// makeCardEmbed builds the iframe snippet for a card.
func (a *App) makeCardEmbed(card models.Card, captcha bool) cardEmbed {
	path := fmt.Sprintf("%s/signup/%s/embed", strings.TrimSuffix(a.urlCfg.RootURL, "/"), url.PathEscape(card.Slug))
	height := cardEmbedHeight(card, captcha)

	return cardEmbed{
		URL:    path,
		Height: height,
		Code: fmt.Sprintf(
			`<iframe src="%s" title="%s" width="100%%" height="%d" style="border:0;max-width:100%%;" loading="lazy"></iframe>`,
			html.EscapeString(path), html.EscapeString(card.Name), height),
	}
}

// cardEmbedHeight returns a fixed iframe height that fits the card content.
func cardEmbedHeight(card models.Card, captcha bool) int {
	height := cardEmbedBaseHeight

	if card.Description != "" {
		lines := (len([]rune(card.Description)) + cardEmbedDescLineLen - 1) / cardEmbedDescLineLen
		if lines < 1 {
			lines = 1
		}
		height += cardEmbedDescriptionHeight + (lines-1)*cardEmbedLineHeight
	}

	if captcha {
		height += cardEmbedCaptchaHeight
	}

	return height
}

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

// ViewCards renders the HTML view listing signup cards.
func (a *App) ViewCards(c *echo.Context) error {
	q := makeQuery(c.Request().URL.Query(), cardsQueryDefaults)
	pg := a.pg.NewFromURL(q)

	cards, total, err := a.core.QueryCards(q.Get("query"), q.Get("order_by"), q.Get("order"), pg.Offset, pg.Limit)
	if err != nil {
		return err
	}

	data := cardsView{
		adminView: newAdminView(c, a.i18n.T("globals.terms.cards"), "", "lists.cards"),
		Cards:     cards,
		Page:      models.NewPageProps(q, total, pg.Page, pg.PerPage),
	}

	return c.Render(http.StatusOK, "admin-cards", data)
}

// ViewNewCard renders the new signup card page.
func (a *App) ViewNewCard(c *echo.Context) error {
	lists, err := a.cardLists(c)
	if err != nil {
		return err
	}

	data := cardView{
		adminView: newAdminView(c, fmt.Sprintf("%s / %s", a.i18n.T("cards.newCard"), a.i18n.T("globals.terms.cards")), "", "lists.cards"),
		AllLists:  lists,
		IsNew:     true,
	}

	return c.Render(http.StatusOK, "admin-card", data)
}

// ViewCard renders the signup card overview and editor.
func (a *App) ViewCard(c *echo.Context) error {
	card, err := a.core.GetCard(getID(c), "", "")
	if err != nil {
		return err
	}

	tab := "card"
	if strings.HasSuffix(c.Path(), "/edit") {
		tab = "edit"
	}

	data := cardView{
		adminView: newAdminView(c, fmt.Sprintf("%s / %s", card.Name, a.i18n.T("globals.terms.cards")), "", "lists.cards"),
		Card:      card,
		Tab:       tab,
		Embed:     a.makeCardEmbed(card, a.captcha != nil && a.captcha.IsEnabled()),
	}

	if tab == "edit" {
		data.AllLists, err = a.cardLists(c)
		if err != nil {
			return err
		}
	}

	return c.Render(http.StatusOK, "admin-card", data)
}

// cardLists returns the public lists that can be pinned to a card.
func (a *App) cardLists(c *echo.Context) ([]models.List, error) {
	user := auth.GetUser(c)
	hasAllPerm, permittedIDs := user.GetPermittedLists(auth.PermTypeGet)

	return a.core.GetLists(models.ListTypePublic, models.ListStatusActive, hasAllPerm, permittedIDs)
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

// DuplicateCard handles copying a signup card and its labels.
func (a *App) DuplicateCard(c *echo.Context) error {
	out, err := a.core.DuplicateCard(getID(c))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// cardTpl is the public view of a single signup card.
type cardTpl struct {
	publicTpl
	Card     models.Card
	Captcha  captchaTpl
	Action   string
	Labels   []models.CardList
	AltchaJS template.JS
}

// cardExportTpl is a card rendered as a single self-contained HTML document.
type cardExportTpl struct {
	cardTpl
	Styles template.CSS
}

// CardPage renders a card as a standalone public signup page.
func (a *App) CardPage(c *echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.invalidFeature")))
	}

	card, err := a.core.GetCard(0, "", c.Param("slug"))
	if err != nil {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("globals.messages.notFound", "name", "card")))
	}

	if len(cardListUUIDs(card)) == 0 {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.noListsAvailable")))
	}

	out := cardTpl{
		publicTpl: publicTpl{Title: card.Name, Description: card.Description},
		Card:      card,
		Captcha:   a.captchaTplData(),
	}

	return c.Render(http.StatusOK, "card", out)
}

// CardEmbedPage renders a card as an embeddable signup form with no page
// chrome around it. The iframe boundary keeps the host page's styles out.
func (a *App) CardEmbedPage(c *echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.invalidFeature")))
	}

	card, err := a.core.GetCard(0, "", c.Param("slug"))
	if err != nil {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("globals.messages.notFound", "name", "card")))
	}

	if len(cardListUUIDs(card)) == 0 {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.noListsAvailable")))
	}

	out := cardTpl{
		publicTpl: publicTpl{Title: card.Name, Description: card.Description},
		Card:      card,
		Captcha:   a.captchaTplData(),
	}

	return c.Render(http.StatusOK, "card-embed", out)
}

// cardExportAction returns the absolute URL that a downloaded card posts to.
func cardExportAction(rootURL string, card models.Card) string {
	return fmt.Sprintf("%s/signup/%s", strings.TrimSuffix(rootURL, "/"), url.PathEscape(card.Slug))
}

// cardExportFilename returns the download filename for a card.
func cardExportFilename(card models.Card) string {
	slug := card.Slug
	if slug == "" {
		slug = models.MakeSlug(card.Name)
	}
	if slug == "" {
		slug = "card"
	}

	return slug + ".html"
}

// cardRootURL returns the absolute root URL for links back to this install.
func (a *App) cardRootURL(c *echo.Context) string {
	if root := strings.TrimSuffix(a.urlCfg.RootURL, "/"); root != "" {
		return root
	}

	return fmt.Sprintf("%s://%s", c.Scheme(), c.Request().Host)
}

// makeCardExport builds the self-contained export view for a card.
func (a *App) makeCardExport(c *echo.Context, card models.Card) (cardExportTpl, error) {
	css, err := a.fs.Read("/public/static/style.css")
	if err != nil {
		return cardExportTpl{}, fmt.Errorf("error reading public styles: %w", err)
	}

	if len(a.cfg.Appearance.PublicCSS) > 0 {
		css = append(css, '\n')
		css = append(css, a.cfg.Appearance.PublicCSS...)
	}

	out := cardExportTpl{
		cardTpl: cardTpl{
			publicTpl: publicTpl{Title: card.Name, Description: card.Description},
			Card:      card,
			Captcha:   a.captchaTplData(),
			Action:    cardExportAction(a.cardRootURL(c), card),
			Labels:    card.ParsedLists(),
		},
		Styles: template.CSS(css),
	}

	if out.Captcha.Provider == captcha.ProviderAltcha {
		if js, err := a.fs.Read("/public/static/altcha.umd.js"); err == nil {
			out.AltchaJS = template.JS(js)
		}
	}

	return out, nil
}

// ExportCard downloads a card as one self-contained HTML file. The styles are
// inlined and the form posts back to this install with its labels baked in.
func (a *App) ExportCard(c *echo.Context) error {
	card, err := a.core.GetCard(getID(c), "", "")
	if err != nil {
		return err
	}

	return a.writeCardExport(c, card)
}

// CardExportPage downloads a card as a self-contained HTML file by slug.
func (a *App) CardExportPage(c *echo.Context) error {
	card, err := a.core.GetCard(0, "", c.Param("slug"))
	if err != nil {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("globals.messages.notFound", "name", "card")))
	}

	return a.writeCardExport(c, card)
}

// writeCardExport renders a card as a downloadable HTML file.
func (a *App) writeCardExport(c *echo.Context, card models.Card) error {
	if !a.cfg.EnablePublicSubPage {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.invalidFeature")))
	}

	if len(cardListUUIDs(card)) == 0 {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.noListsAvailable")))
	}

	out, err := a.makeCardExport(c, card)
	if err != nil {
		return err
	}

	hdr := c.Response().Header()
	hdr.Set(echo.HeaderContentType, "text/html; charset=utf-8")
	hdr.Set(echo.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{
		"filename": cardExportFilename(card),
	}))

	return c.Render(http.StatusOK, "card-export", out)
}

// CardSubscription handles submissions from a standalone card page. The
// subscriber is always added to the card's labels, never to lists sent by
// the client.
func (a *App) CardSubscription(c *echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return echo.NewHTTPError(http.StatusNotFound, a.i18n.T("public.invalidFeature"))
	}

	card, err := a.core.GetCard(0, "", c.Param("slug"))
	if err != nil {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("globals.messages.notFound", "name", "card")))
	}

	if c.FormValue("nonce") != "" {
		return echo.NewHTTPError(http.StatusBadGateway, a.i18n.T("public.invalidFeature"))
	}

	if err := a.verifyCaptcha(c); err != nil {
		e, ok := err.(*echo.HTTPError)
		if !ok {
			return err
		}

		return c.Render(e.Code, tplMessage, makeMsgTpl(a.i18n.T("globals.terms.error"), "", fmt.Sprintf("%s", e.Message)))
	}

	var req subFormReq
	if err := c.Bind(&req); err != nil {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("globals.messages.invalidData")))
	}

	listUUIDs := cardListUUIDs(card)
	if len(listUUIDs) == 0 {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.noListsSelected")))
	}

	out, err := a.subscribeSub(req.Name, req.Email, listUUIDs)
	if err != nil {
		e, ok := err.(*echo.HTTPError)
		if !ok {
			return err
		}

		return c.Render(e.Code, tplMessage, makeMsgTpl(a.i18n.T("globals.terms.error"), "", fmt.Sprintf("%s", e.Message)))
	}

	if out.New || out.GainedLabels {
		if err := a.core.RecordCardJoin(card.ID, out.Subscriber.ID, out.New); err != nil {
			return err
		}
	}

	if out.HasOptin {
		return c.Render(http.StatusOK, tplMessage,
			makeMsgTpl(a.i18n.T("globals.buttons.subscribe"), "", a.i18n.Ts("public.subOptinPending")))
	}

	if redirect := cardRedirectURL(card.RedirectURL); redirect != "" {
		return c.Redirect(http.StatusSeeOther, redirect)
	}

	return c.Render(http.StatusOK, tplMessage,
		makeMsgTpl(a.i18n.T("globals.buttons.subscribe"), "", cardSuccessMessage(card, a.i18n.Ts("public.subConfirmed"))))
}

// cardSuccessMessage returns the message shown after a successful card
// signup, falling back to the default confirmation message.
func cardSuccessMessage(card models.Card, fallback string) string {
	if msg := strings.TrimSpace(card.SuccessMessage); msg != "" {
		return msg
	}

	return fallback
}

// cardRedirectURL returns the URL to send a card's visitor to after a
// successful signup. Only http(s) and relative URLs are allowed.
func cardRedirectURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if u.IsAbs() {
		if (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return raw
		}

		return ""
	}

	// Reject protocol-relative URLs that point at another host.
	if strings.HasPrefix(raw, "//") {
		return ""
	}

	return raw
}

// cardListUUIDs returns the UUIDs of the lists (labels) pinned to a card.
func cardListUUIDs(card models.Card) []string {
	lists := card.ParsedLists()
	out := make([]string, 0, len(lists))
	for _, l := range lists {
		out = append(out, l.UUID)
	}

	return out
}
