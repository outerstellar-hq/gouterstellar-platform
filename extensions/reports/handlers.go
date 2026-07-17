package reports

import (
	"encoding/json"
	"net/http"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

// Contribute registers the reports extension's routes and navigation through
// the platform's contribution context. Every route registration is stamped with
// the extension's owner ID and validated against the manifest's ownership
// declaration by the platform's route registry.
func (e *Extension) Contribute(ctx *extplatform.ContributionContext) error {
	if err := ctx.Pages.Register(extplatform.TemplateSource{
		FS:          templatesFS,
		PagesDir:    "templates/pages",
		PartialsDir: "templates/partials",
	}); err != nil {
		return err
	}
	e.pages = ctx.Pages
	ctx.Routes.Protected(http.MethodGet, "/reports", "Reports home", http.HandlerFunc(e.home))
	ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "Message count summary", http.HandlerFunc(e.summary))
	if err := ctx.Routes.StaticAssets("/extensions/reports/assets", extplatform.AssetSource{
		FS: assetsFS, Directory: "assets",
	}); err != nil {
		return err
	}
	ctx.Navigation.Add("Reports", "/reports", "bar-chart")
	return nil
}

// home renders the report through the shared platform shell. It accesses data
// exclusively through public platform capabilities.
func (e *Extension) home(w http.ResponseWriter, r *http.Request) {
	count, err := e.messages.CountMessages(r.Context())
	if err != nil {
		http.Error(w, "Failed to load report data", http.StatusInternalServerError)
		return
	}
	if err := e.pages.Render(w, r, "reports", reportPage{
		MessageCount: count,
		Request:      extplatform.RequestContextFrom(r),
	}); err != nil {
		http.Error(w, "Failed to render reports", http.StatusInternalServerError)
	}
}

type reportPage struct {
	MessageCount int64
	Request      extplatform.RequestContext
}

// summary returns a JSON object with the message count.
func (e *Extension) summary(w http.ResponseWriter, r *http.Request) {
	count, err := e.messages.CountMessages(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "count failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message_count": count,
	})
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
