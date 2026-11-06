package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/archive"
	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
)

// previewBothTpl is the single overlay that shows the e-mail and its archive
// page side by side.
var previewBothTpl = template.Must(template.New("preview-both").Parse(`<!doctype html>
<html lang="{{ .Lang }}">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<style>
		html,
		body {
			height: 100%;
			margin: 0;
			background: #f4f4f4;
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
		}
		.preview-split {
			display: grid;
			grid-template-columns: 1fr 1fr;
			height: 100%;
		}
		.preview-pane {
			display: flex;
			flex-direction: column;
			min-width: 0;
			min-height: 0;
		}
		.preview-pane + .preview-pane {
			border-left: 1px solid #d9d9d9;
		}
		.preview-pane-head {
			flex: none;
			padding: 0.6rem 0.9rem;
			border-bottom: 1px solid #d9d9d9;
			background: #fff;
			color: #555;
			font-size: 0.75rem;
			font-weight: 600;
			letter-spacing: 0.04em;
			text-transform: uppercase;
		}
		.preview-pane iframe {
			flex: 1;
			width: 100%;
			min-height: 0;
			border: 0;
			background: #fff;
		}
	</style>
</head>
<body>
	<div class="preview-split" data-preview-split>
		<section class="preview-pane">
			<header class="preview-pane-head">{{ .EmailLabel }}</header>
			<iframe name="email-preview" title="{{ .EmailLabel }}" sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox" srcdoc="{{ .Email }}"></iframe>
		</section>
		<section class="preview-pane">
			<header class="preview-pane-head">{{ .ArchiveLabel }}</header>
			<iframe name="archive-preview" title="{{ .ArchiveLabel }}" sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox" srcdoc="{{ .Archive }}"></iframe>
		</section>
	</div>
	<script>
		(() => {
			const frames = Array.from(document.querySelectorAll("[data-preview-split] iframe"));

			window.addEventListener("message", (e) => {
				if (!e.data || e.data.type !== "campaign-preview-scroll") {
					return;
				}

				const source = frames.find((f) => f.contentWindow === e.source);
				if (!source) {
					return;
				}

				for (const frame of frames) {
					if (frame !== source) {
						frame.contentWindow.postMessage({ type: "campaign-preview-scroll-to", ratio: e.data.ratio }, "*");
					}
				}
			});
		})();
	</script>
</body>
</html>`))

const previewScrollScript = `<script>
	(() => {
		let locked = false;

		const send = () => {
			if (locked) {
				return;
			}

			const max = document.documentElement.scrollHeight - window.innerHeight;
			parent.postMessage({
				type: "campaign-preview-scroll",
				ratio: max > 0 ? window.scrollY / max : 0,
			}, "*");
		};

		window.addEventListener("scroll", () => window.requestAnimationFrame(send), { passive: true });
		window.addEventListener("message", (e) => {
			if (!e.data || e.data.type !== "campaign-preview-scroll-to") {
				return;
			}

			locked = true;
			const max = document.documentElement.scrollHeight - window.innerHeight;
			window.scrollTo(0, Math.max(0, Math.min(1, e.data.ratio)) * max);
			window.setTimeout(() => {
				locked = false;
			}, 80);
		});
		window.addEventListener("load", send);
	})();
</script>`

type previewBothData struct {
	Lang         string
	Title        string
	EmailLabel   string
	ArchiveLabel string
	Email        string
	Archive      string
}

// PreviewCampaignBoth renders the campaign's e-mail and archive page in one
// sandboxed document. Both sides are built from the same campaign body.
func (a *App) PreviewCampaignBoth(c *echo.Context) error {
	id := getID(c)
	if err := a.checkCampaignPerm(auth.PermTypeGet, id, c); err != nil {
		return err
	}

	camp, msg, err := a.campaignPreviewMessage(c, id)
	if err != nil {
		return err
	}

	archivePage, err := a.campaignArchivePreviewPage(camp)
	if err != nil {
		a.log.Printf("error rendering archive preview: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			a.i18n.Ts("public.errorFetchingCampaign"))
	}

	emailDoc := string(msg.Body())
	if camp.ContentType == models.CampaignContentTypePlain {
		emailDoc = plainPreviewDocument(emailDoc)
	}

	out, err := renderPreviewBoth(previewBothData{
		Lang:         a.cfg.Lang,
		Title:        camp.Name,
		EmailLabel:   a.i18n.T("globals.fields.email"),
		ArchiveLabel: a.i18n.T("globals.terms.archive"),
		Email:        injectPreviewScrollScript(emailDoc),
		Archive:      injectPreviewScrollScript(archivePage),
	})
	if err != nil {
		a.log.Printf("error rendering campaign preview: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			a.i18n.Ts("public.errorFetchingCampaign"))
	}

	return renderSandboxedHTML(c, out)
}

// campaignPreview loads a campaign with the body and metadata from the
// preview form applied over the saved record.
func (a *App) campaignPreview(c *echo.Context, id int) (models.Campaign, error) {
	var (
		isPost      = c.Request().Method == http.MethodPost
		contentType = c.FormValue("content_type")
		tplID, _    = strconv.Atoi(c.FormValue("template_id"))
	)

	if contentType == models.CampaignContentTypeVisual || tplID < 1 {
		tplID = 0
	}

	camp, err := a.core.GetCampaignForPreview(id, tplID)
	if err != nil {
		return camp, err
	}

	if isPost {
		camp.ContentType = contentType
		camp.Body = c.FormValue("body")

		if meta := c.FormValue("archive_meta"); meta != "" {
			camp.ArchiveMeta = json.RawMessage(meta)
		}

		if contentType == models.CampaignContentTypeVisual {
			camp.TemplateBody = ""
		}
	}

	// Use a dummy campaign ID to prevent views and clicks from {{ TrackView }}
	// and {{ TrackLink }} being registered on preview.
	camp.UUID = dummySubscriber.UUID

	return camp, nil
}

// campaignPreviewMessage compiles a campaign's e-mail preview. The optional
// POST fields override the saved body so the preview can follow the editor.
func (a *App) campaignPreviewMessage(c *echo.Context, id int) (models.Campaign, manager.CampaignMessage, error) {
	camp, err := a.campaignPreview(c, id)
	if err != nil {
		return camp, manager.CampaignMessage{}, err
	}

	if err := camp.CompileTemplate(a.manager.TemplateFuncs(&camp, models.CampPreview)); err != nil {
		a.log.Printf("error compiling template: %v", err)
		return camp, manager.CampaignMessage{}, echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.errorCompiling", "error", err.Error()))
	}

	msg, err := a.manager.NewCampaignMessage(&camp, dummySubscriber)
	if err != nil {
		a.log.Printf("error rendering message: %v", err)
		return camp, manager.CampaignMessage{}, echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("templates.errorRendering", "error", err.Error()))
	}

	return camp, msg, nil
}

// campaignArchivePreviewPage renders the same reading page the public archive
// uses, from the campaign's content rather than its e-mail template.
func (a *App) campaignArchivePreviewPage(camp models.Campaign) (string, error) {
	msg, err := a.compileArchiveContent(camp)
	if err != nil {
		return "", fmt.Errorf("compiling archive preview: %w", err)
	}

	data := a.archivePageData(models.MakeArchiveTitle(msg.Subject()), campaignArchiveDate(camp), template.HTML(msg.Body()))

	var b bytes.Buffer
	if err := archive.RenderPage(&b, data); err != nil {
		return "", fmt.Errorf("rendering archive preview page: %w", err)
	}

	return inlineArchiveStylesheet(b.String()), nil
}

// renderPreviewBoth executes the side-by-side preview template.
func renderPreviewBoth(data previewBothData) (string, error) {
	var b bytes.Buffer
	if err := previewBothTpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("rendering preview template: %w", err)
	}

	return b.String(), nil
}

// injectPreviewScrollScript adds the scroll relay to a preview document so the
// two panes move together even though both are sandboxed.
func injectPreviewScrollScript(doc string) string {
	if i := strings.Index(strings.ToLower(doc), "</body>"); i >= 0 {
		return doc[:i] + previewScrollScript + doc[i:]
	}

	return doc + previewScrollScript
}

// inlineArchiveStylesheet swaps the archive stylesheet link for the same CSS
// inline, so the preview does not depend on the public archive being enabled.
func inlineArchiveStylesheet(page string) string {
	link := `<link rel="stylesheet" href="` + archiveLiveStylesheet + `">`
	if !strings.Contains(page, link) {
		return page
	}

	return strings.Replace(page, link, "<style>"+archive.Stylesheet()+"</style>", 1)
}

// plainPreviewDocument wraps a plain-text campaign in a minimal document.
func plainPreviewDocument(body string) string {
	return `<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		body {
			margin: 0;
			padding: 1.5rem;
			font-family: monospace;
			white-space: pre-wrap;
		}
	</style>
</head>
<body>` + html.EscapeString(body) + `</body>
</html>`
}
