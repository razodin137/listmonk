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
	"sort"
	"strings"

	"github.com/knadh/listmonk/internal/archive"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
	null "gopkg.in/volatiletech/null.v6"
)

const (
	excerptLength         = 160
	archiveLiveStylesheet = "/public/archive.css"
)

var reHTMLTag = regexp.MustCompile(`(?s)<[^>]*>`)

type campArchive struct {
	UUID      string    `json:"uuid"`
	Title     string    `json:"title"`
	Subject   string    `json:"subject"`
	Content   string    `json:"content"`
	Excerpt   string    `json:"excerpt"`
	CreatedAt null.Time `json:"created_at"`
	StartedAt null.Time `json:"started_at"`
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

// campaignArchiveDate returns the date an archived campaign was started,
// published, or created.
func campaignArchiveDate(c models.Campaign) null.Time {
	return archiveDateFrom(c.StartedAt, c.SendAt, c.CreatedAt)
}

// archiveDateFrom picks the first meaningful timestamp for an archived
// campaign. Published posts have a start time but no send time.
func archiveDateFrom(startedAt, sendAt, createdAt null.Time) null.Time {
	if startedAt.Valid {
		return startedAt
	}
	if sendAt.Valid {
		return sendAt
	}

	return createdAt
}

// campaignArchiveSlug returns a campaign's stable archive slug. A slug that
// has already been written wins; otherwise it is derived from the title.
func campaignArchiveSlug(c models.Campaign, title string, used map[string]struct{}) string {
	if c.ArchiveSlug.Valid && c.ArchiveSlug.String != "" {
		used[c.ArchiveSlug.String] = struct{}{}
		return c.ArchiveSlug.String
	}

	return models.MakeArchiveSlug(title, used)
}

// campaignArchiveTags returns the campaign tags as a non-nil slice so the
// dataset always emits a JSON array.
func campaignArchiveTags(c models.Campaign) []string {
	return append([]string{}, c.Tags...)
}

// archiveListData builds the shared archive list view model.
func (a *App) archiveListData(camps []campArchive) archive.ListData {
	entries := make([]archive.Entry, 0, len(camps))
	for _, camp := range camps {
		date := archiveDateFrom(camp.StartedAt, camp.SendAt, camp.CreatedAt)

		entries = append(entries, archive.Entry{
			Title:    camp.Title,
			Href:     camp.URL,
			Date:     a.archiveDate(date),
			DateTime: archiveDateTime(date),
			Excerpt:  camp.Excerpt,
		})
	}

	title := a.i18n.T("public.archiveTitle")
	data := archive.ListData{
		Lang:        a.cfg.Lang,
		SiteTitle:   a.cfg.SiteName,
		Title:       title,
		Description: title,
		Stylesheet:  archiveLiveStylesheet,
		FeedURL:     strings.TrimSuffix(a.urlCfg.RootURL, "/") + "/archive.xml",
		Entries:     entries,
		Empty:       a.i18n.T("public.archiveEmpty"),
	}

	if a.cfg.EnablePublicSubPage {
		data.SubscribeURL = strings.TrimSuffix(a.urlCfg.RootURL, "/") + "/subscription/form"
		data.SubscribeLabel = a.i18n.T("globals.buttons.subscribe")
	}

	return data
}

// archivePageData builds the shared single-campaign view model.
func (a *App) archivePageData(title string, date null.Time, body template.HTML) archive.PageData {
	return archive.PageData{
		Lang:       a.cfg.Lang,
		SiteTitle:  a.cfg.SiteName,
		Title:      title,
		Stylesheet: archiveLiveStylesheet,
		BackHref:   a.urlCfg.ArchiveURL,
		BackLabel:  a.i18n.T("public.archiveTitle"),
		Date:       a.archiveDate(date),
		DateTime:   archiveDateTime(date),
		Body:       body,
	}
}

// archiveDate formats a date for display in the archive.
func (a *App) archiveDate(t null.Time) string {
	return niceDate(t, a.i18n.T("_.date_short"), a.i18n.FormatDate)
}

// archiveDateTime formats a date as an ISO date for the datetime attribute.
func archiveDateTime(t null.Time) string {
	if !t.Valid {
		return ""
	}

	return t.Time.Format("2006-01-02")
}

// renderArchiveList renders the shared archive list template.
func (a *App) renderArchiveList(c *echo.Context, data archive.ListData) error {
	var buf bytes.Buffer
	if err := archive.RenderList(&buf, data); err != nil {
		a.log.Printf("error rendering archive list: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.errorFetchingCampaign")))
	}

	return c.HTML(http.StatusOK, buf.String())
}

// renderArchivePage renders the shared single-campaign template.
func (a *App) renderArchivePage(c *echo.Context, data archive.PageData) error {
	var buf bytes.Buffer
	if err := archive.RenderPage(&buf, data); err != nil {
		a.log.Printf("error rendering archive page: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("globals.terms.error"), "", a.i18n.T("public.errorFetchingCampaign")))
	}

	return c.HTML(http.StatusOK, buf.String())
}

// ArchiveStylesheet serves the stylesheet shared by the live archive and the
// static export.
func (a *App) ArchiveStylesheet(c *echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=86400")

	return c.Blob(http.StatusOK, "text/css; charset=utf-8", []byte(archive.Stylesheet()))
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
			Title: camp.ArchiveTitle(),
			Date:  campaignArchiveDate(camp),
			Tags:  campaignArchiveTags(camp),
			Body:  string(msg.Body()),
		})
	}

	assignArchiveSlugs(camps, out)

	return out, nil
}

// archiveSlugs returns the stable archive slug for each campaign, in the
// same order as camps. Every caller that needs a campaign's address, file
// name, or front matter slug uses this one rule.
func archiveSlugs(camps models.Campaigns, titles []string) []string {
	entries := make([]campArchiveEntry, len(camps))
	for i := range camps {
		entries[i].Title = titles[i]
	}

	assignArchiveSlugs(camps, entries)

	slugs := make([]string, len(entries))
	for i := range entries {
		slugs[i] = entries[i].Slug
	}

	return slugs
}

// assignArchiveSlugs fills in each entry's slug from the campaign's stored
// slug or title. Stored slugs are reserved first, and derived slugs are
// assigned oldest first so a new campaign never renames an older one.
func assignArchiveSlugs(camps models.Campaigns, entries []campArchiveEntry) {
	// index.html is the archive's list page, so no post may take that slug.
	used := map[string]struct{}{
		strings.TrimSuffix(archiveIndexFile, archivePageExt): {},
	}
	derived := make([]int, 0, len(camps))

	for i, camp := range camps {
		if camp.ArchiveSlug.Valid && camp.ArchiveSlug.String != "" {
			entries[i].Slug = camp.ArchiveSlug.String
			used[entries[i].Slug] = struct{}{}
			continue
		}

		derived = append(derived, i)
	}

	sort.SliceStable(derived, func(i, j int) bool {
		a, b := camps[derived[i]], camps[derived[j]]
		if !a.CreatedAt.Time.Equal(b.CreatedAt.Time) {
			return a.CreatedAt.Time.Before(b.CreatedAt.Time)
		}

		return a.ID < b.ID
	})

	for _, i := range derived {
		entries[i].Slug = campaignArchiveSlug(camps[i], entries[i].Title, used)
	}
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

// GetCampaignArchivesFeed renders the public campaign archives RSS feed from
// the same entries as the dataset and the static export.
func (a *App) GetCampaignArchivesFeed(c *echo.Context) error {
	camps, _, err := a.core.GetArchivedCampaigns(0, 0)
	if err != nil {
		return err
	}

	entries, err := a.buildCampaignArchiveEntries(camps)
	if err != nil {
		a.log.Printf("error compiling archive feed entry: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorFetchingCampaign"))
	}

	feed := archiveFeed(entries, a.cfg.SiteName, a.urlCfg.RootURL, a.i18n.T("public.archiveTitle"),
		a.cfg.EnablePublicArchiveRSSContent, func(_ int, entry campArchiveEntry) string {
			href, _ := url.JoinPath(a.urlCfg.ArchiveURL, entry.Slug)
			return href
		})

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

	data := a.archiveListData(out)
	data.Pagination = template.HTML(pg.HTML("", nil))

	return a.renderArchiveList(c, data)
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
	if err != nil || !pubCamp.ArchivedForPublic() {
		notFound := false

		if er, ok := err.(*echo.HTTPError); ok {
			if er.Code == http.StatusBadRequest {
				notFound = true
			}
		} else if err == nil {
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

	return a.renderArchivePage(c, a.archivePageData(pubCamp.ArchiveTitle(),
		campaignArchiveDate(pubCamp), template.HTML(msg.Body())))
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

	camps := make(models.Campaigns, len(msgs))
	titles := make([]string, len(msgs))
	for i, m := range msgs {
		camps[i] = *m.Campaign
		titles[i] = m.Campaign.ArchiveTitle()
	}

	slugs := archiveSlugs(camps, titles)

	out := make([]campArchive, 0, len(msgs))
	for i, m := range msgs {
		camp := m.Campaign

		archive := campArchive{
			UUID:      camp.UUID,
			Title:     camp.ArchiveTitle(),
			Subject:   camp.Subject,
			Excerpt:   campaignExcerpt(*camp),
			CreatedAt: camp.CreatedAt,
			StartedAt: camp.StartedAt,
			SendAt:    camp.SendAt,
		}

		archive.URL, _ = url.JoinPath(a.urlCfg.ArchiveURL, slugs[i])

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
