package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gorilla/feeds"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
	null "gopkg.in/volatiletech/null.v6"
)

const excerptLength = 160

var reHTMLTag = regexp.MustCompile(`(?s)<[^>]*>`)

type campArchive struct {
	UUID      string    `json:"uuid"`
	Subject   string    `json:"subject"`
	Content   string    `json:"content"`
	Excerpt   string    `json:"excerpt"`
	CreatedAt null.Time `json:"created_at"`
	SendAt    null.Time `json:"send_at"`
	URL       string    `json:"url"`
}

// campArchiveEntry is one sent campaign in the public archive dataset.
type campArchiveEntry struct {
	UUID  string    `json:"uuid"`
	Title string    `json:"title"`
	Slug  string    `json:"slug"`
	Date  null.Time `json:"date"`
	Tags  []string  `json:"tags"`
	Body  string    `json:"body"`
}

// campaignArchiveDate returns the date a campaign was sent. Scheduled
// campaigns fall back to their send time or creation time.
func campaignArchiveDate(c models.Campaign) null.Time {
	if c.StartedAt.Valid {
		return c.StartedAt
	}
	if c.SendAt.Valid {
		return c.SendAt
	}

	return c.CreatedAt
}

// campaignArchiveSlug returns the stable archive slug, falling back to the
// campaign UUID when no custom slug is set.
func campaignArchiveSlug(c models.Campaign) string {
	if c.ArchiveSlug.Valid && c.ArchiveSlug.String != "" {
		return c.ArchiveSlug.String
	}

	return c.UUID
}

// campaignArchiveTags returns the campaign tags as a non-nil slice so the
// dataset always emits a JSON array.
func campaignArchiveTags(c models.Campaign) []string {
	return append([]string{}, c.Tags...)
}

// campaignExcerpt returns a single-line plain text summary of a campaign body.
func campaignExcerpt(c models.Campaign) string {
	var text string

	if c.ContentType == models.CampaignContentTypeMarkdown {
		plain, err := c.ConvertContent(models.CampaignContentTypeMarkdown, models.CampaignContentTypePlain)
		if err == nil {
			text = plain
		} else {
			text = c.Body
		}
	} else {
		text = html.UnescapeString(reHTMLTag.ReplaceAllString(c.Body, " "))
	}

	text = strings.Join(strings.Fields(text), " ")

	runes := []rune(text)
	if len(runes) > excerptLength {
		text = strings.TrimSpace(string(runes[:excerptLength])) + "…"
	}

	return text
}

// GetCampaignArchives renders the public campaign archives page.
func (a *App) GetCampaignArchives(c *echo.Context) error {
	// Get archives from the DB.
	pg := a.pg.NewFromURL(c.Request().URL.Query())
	camps, total, err := a.getCampaignArchives(pg.Offset, pg.Limit, false)
	if err != nil {
		return err
	}

	if len(camps) == 0 {
		return c.JSON(http.StatusOK, okResp{models.PageResults{
			Results: []campArchive{},
		}})
	}

	// Meta.
	out := models.PageResults{
		Results: camps,
		PageProps: models.PageProps{
			Total:   total,
			Page:    pg.Page,
			PerPage: pg.PerPage,
		},
	}

	return c.JSON(200, okResp{out})
}

// GetCampaignArchiveDataset returns every archived campaign, newest first, as
// one JSON dataset for static site generators and other consumers.
func (a *App) GetCampaignArchiveDataset(c *echo.Context) error {
	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return err
	}

	out, err := a.buildCampaignArchiveEntries(camps)
	if err != nil {
		a.log.Printf("error compiling archive dataset entry: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorFetchingCampaign"))
	}

	return c.JSON(http.StatusOK, out)
}

// buildCampaignArchiveEntries compiles archived campaigns into dataset entries
// in the order they were fetched.
func (a *App) buildCampaignArchiveEntries(camps models.Campaigns) ([]campArchiveEntry, error) {
	out := make([]campArchiveEntry, 0, len(camps))
	for _, camp := range camps {
		msg, err := a.compileArchiveContent(camp)
		if err != nil {
			return nil, err
		}

		out = append(out, campArchiveEntry{
			UUID:  camp.UUID,
			Title: msg.Subject(),
			Slug:  campaignArchiveSlug(camp),
			Date:  campaignArchiveDate(camp),
			Tags:  campaignArchiveTags(camp),
			Body:  string(msg.Body()),
		})
	}

	return out, nil
}

// compileArchiveContent renders a campaign body as a standalone HTML fragment
// without the e-mail template wrapper.
func (a *App) compileArchiveContent(camp models.Campaign) (manager.CampaignMessage, error) {
	a.manager.ApplyArchiveImages(&camp)

	if err := camp.CompileContentTemplate(a.manager.TemplateFuncs(&camp)); err != nil {
		return manager.CampaignMessage{}, fmt.Errorf("compiling archive content: %w", err)
	}

	var sub models.Subscriber
	if err := json.Unmarshal(camp.ArchiveMeta, &sub); err != nil {
		return manager.CampaignMessage{}, fmt.Errorf("unmarshalling archive meta: %w", err)
	}

	msg, err := a.manager.NewCampaignMessage(&camp, sub)
	if err != nil {
		return manager.CampaignMessage{}, fmt.Errorf("rendering archive content: %w", err)
	}

	return msg, nil
}

// GetCampaignArchivesFeed renders the public campaign archives RSS feed.
func (a *App) GetCampaignArchivesFeed(c *echo.Context) error {
	var (
		pg              = a.pg.NewFromURL(c.Request().URL.Query())
		showFullContent = a.cfg.EnablePublicArchiveRSSContent
	)

	// Get archives from the DB.
	camps, _, err := a.getCampaignArchives(pg.Offset, pg.Limit, showFullContent)
	if err != nil {
		return err
	}

	// Format output for the feed.
	out := make([]*feeds.Item, 0, len(camps))
	for _, c := range camps {
		pubDate := c.CreatedAt.Time

		if c.SendAt.Valid {
			pubDate = c.SendAt.Time
		}

		out = append(out, &feeds.Item{
			Title:   c.Subject,
			Link:    &feeds.Link{Href: c.URL},
			Content: c.Content,
			Created: pubDate,
		})
	}

	// Generate the feed.
	feed := &feeds.Feed{
		Title:       a.cfg.SiteName,
		Link:        &feeds.Link{Href: a.urlCfg.RootURL},
		Description: a.i18n.T("public.archiveTitle"),
		Items:       out,
	}

	if err := feed.WriteRss(c.Response()); err != nil {
		a.log.Printf("error generating archive RSS feed: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.errorProcessingRequest"))
	}

	return nil
}

// CampaignArchivesPage renders the public campaign archives page.
func (a *App) CampaignArchivesPage(c *echo.Context) error {
	// Get archives from the DB.
	pg := a.pg.NewFromURL(c.Request().URL.Query())
	out, total, err := a.getCampaignArchives(pg.Offset, pg.Limit, false)
	if err != nil {
		return err
	}
	pg.SetTotal(total)

	title := a.i18n.T("public.archiveTitle")
	return c.Render(http.StatusOK, "archive", struct {
		Title       string
		Description string
		Campaigns   []campArchive
		TotalPages  int
		Pagination  template.HTML
	}{title, title, out, pg.TotalPages, template.HTML(pg.HTML("", nil))})
}

// CampaignArchivePage renders the public campaign archives page.
func (a *App) CampaignArchivePage(c *echo.Context) error {
	// ID can be the UUID or slug.
	var (
		idStr      = c.Param("id")
		uuid, slug string
	)
	if reUUID.MatchString(idStr) {
		uuid = idStr
	} else {
		slug = idStr
	}

	// Get the campaign from the DB.
	pubCamp, err := a.core.GetArchivedCampaign(0, uuid, slug)
	if err != nil || pubCamp.Type != models.CampaignTypeRegular {
		notFound := false

		// Camppaig doesn't exist.
		if er, ok := err.(*echo.HTTPError); ok {
			if er.Code == http.StatusBadRequest {
				notFound = true
			}
		} else if pubCamp.Type != models.CampaignTypeRegular {
			// Campaign isn't of regular type.
			notFound = true
		}

		// 404.
		if notFound {
			return c.Render(http.StatusNotFound, tplMessage,
				makeMsgTpl(a.i18n.T("globals.terms.notFound"), "", a.i18n.T("public.campaignNotFound")))
		}

		// Some other internal error.
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Resolve data-embed images and compile the campaign body without the
	// e-mail template wrapper so it can sit inside the reading layout.
	a.manager.ApplyArchiveImages(&pubCamp)
	if err := pubCamp.CompileContentTemplate(a.manager.TemplateFuncs(&pubCamp)); err != nil {
		a.log.Printf("error compiling campaign content: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Load the dummy subscriber meta used to render the campaign.
	var sub models.Subscriber
	if err := json.Unmarshal(pubCamp.ArchiveMeta, &sub); err != nil {
		a.log.Printf("error unmarshalling campaign archive meta: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	msg, err := a.manager.NewCampaignMessage(&pubCamp, sub)
	if err != nil {
		a.log.Printf("error rendering campaign: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Sent campaigns show the send time; fall back to the creation time.
	date := pubCamp.CreatedAt
	if pubCamp.SendAt.Valid {
		date = pubCamp.SendAt
	}

	return c.Render(http.StatusOK, "archive-campaign", struct {
		Title      string
		Date       null.Time
		Body       template.HTML
		ArchiveURL string
	}{
		Title:      msg.Subject(),
		Date:       date,
		Body:       template.HTML(msg.Body()),
		ArchiveURL: a.urlCfg.ArchiveURL,
	})
}

// CampaignArchivePageLatest renders the latest public campaign.
func (a *App) CampaignArchivePageLatest(c *echo.Context) error {
	// Get the latest campaign from the DB.
	camps, _, err := a.getCampaignArchives(0, 1, true)
	if err != nil {
		return err
	}

	if len(camps) == 0 {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.notFound"), "", a.i18n.T("public.campaignNotFound")))
	}
	camp := camps[0]

	return c.HTML(http.StatusOK, camp.Content)
}

// getCampaignArchives fetches the public campaign archives from the DB.
func (a *App) getCampaignArchives(offset, limit int, renderBody bool) ([]campArchive, int, error) {
	pubCamps, total, err := a.core.GetArchivedCampaigns(offset, limit)
	if err != nil {
		return []campArchive{}, total, echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorFetchingCampaign"))
	}

	msgs, err := a.compileArchiveCampaigns(pubCamps)
	if err != nil {
		return []campArchive{}, total, err
	}

	out := make([]campArchive, 0, len(msgs))
	for _, m := range msgs {
		camp := m.Campaign

		archive := campArchive{
			UUID:      camp.UUID,
			Subject:   camp.Subject,
			Excerpt:   campaignExcerpt(*camp),
			CreatedAt: camp.CreatedAt,
			SendAt:    camp.SendAt,
		}

		// The campaign may have a custom slug.
		if camp.ArchiveSlug.Valid {
			archive.URL, _ = url.JoinPath(a.urlCfg.ArchiveURL, camp.ArchiveSlug.String)
		} else {
			archive.URL, _ = url.JoinPath(a.urlCfg.ArchiveURL, camp.UUID)
		}

		// Render the full template body if requested.
		if renderBody {
			msg, err := a.manager.NewCampaignMessage(camp, m.Subscriber)
			if err != nil {
				return []campArchive{}, total, err
			}
			archive.Content = string(msg.Body())
		}

		out = append(out, archive)
	}

	return out, total, nil
}

// compileArchiveCampaigns compiles the campaign template with the subscriber data.
func (a *App) compileArchiveCampaigns(camps []models.Campaign, opts ...models.CampOpt) ([]manager.CampaignMessage, error) {

	var (
		b   = bytes.Buffer{}
		out = make([]manager.CampaignMessage, 0, len(camps))
	)
	for _, c := range camps {
		camp := c

		// Resolve data-embed images to media URLs as cid: cannot resolve on a web page.
		a.manager.ApplyArchiveImages(&camp)

		if err := camp.CompileArchiveTemplate(a.manager.TemplateFuncs(&camp, opts...)); err != nil {
			a.log.Printf("error compiling template: %v", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorFetchingCampaign"))
		}

		// Load the dummy subscriber meta.
		var sub models.Subscriber
		if err := json.Unmarshal([]byte(camp.ArchiveMeta), &sub); err != nil {
			a.log.Printf("error unmarshalling campaign archive meta: %v", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorFetchingCampaign"))
		}

		m := manager.CampaignMessage{
			Campaign:   &camp,
			Subscriber: sub,
		}

		// Render the subject if it's a template.
		if camp.SubjectTpl != nil {
			if err := camp.SubjectTpl.ExecuteTemplate(&b, models.ContentTpl, m); err != nil {
				return nil, err
			}
			camp.Subject = b.String()
			b.Reset()
		}

		out = append(out, m)
	}

	return out, nil
}
