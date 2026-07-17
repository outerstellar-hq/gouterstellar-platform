package starforge

import (
	"embed"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

//go:embed templates/pages/*.html
var templatesFS embed.FS

type Extension struct {
	client            Client
	pages             *extplatform.PageRegistry
	pipelineTemplates pipelineTemplateRegistry
}

func New(client Client) *Extension {
	return &Extension{client: client, pipelineTemplates: newPipelineTemplateRegistry()}
}

func (e *Extension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "starforge",
		Label: "Starforge",
		Mode:  extplatform.ExtensionHost,
		Ownership: extplatform.RouteOwnership{
			UI:  []string{"/starforge"},
			API: []string{"/api/starforge"},
		},
	}
}

func (e *Extension) Contribute(ctx *extplatform.ContributionContext) error {
	if e.client == nil {
		return errors.New("Starforge client is required")
	}
	if err := ctx.Pages.Register(extplatform.TemplateSource{FS: templatesFS, PagesDir: "templates/pages"}); err != nil {
		return err
	}
	e.pages = ctx.Pages
	ctx.Routes.Protected(http.MethodGet, "/starforge", "Starforge worker inventory", http.HandlerFunc(e.home))
	ctx.Routes.Protected(http.MethodGet, "/starforge/pipelines/sleep-series", "Sleep Series production ledger", http.HandlerFunc(e.sleepSeries))
	ctx.Routes.API(http.MethodGet, "/api/starforge/workers", "List Starforge workers", http.HandlerFunc(e.listWorkers))
	ctx.Routes.API(http.MethodPut, "/api/starforge/workers/{uuid}/label", "Update Starforge worker label", http.HandlerFunc(e.updateWorkerLabel))
	ctx.Navigation.Add("Starforge", "/starforge", "server")
	return nil
}

type workerPage struct {
	Workers     []workerView
	Summary     fleetSummary
	Unavailable bool
	Request     extplatform.RequestContext
}

type fleetSummary struct {
	Total          int
	Online         int
	ActiveSessions int
	NotOnline      int
}

type workerView struct {
	Worker
	Name               string
	StatusClass        string
	LastSeenLabel      string
	ConnectedAtLabel   string
	LastHeartbeatLabel string
	HasSession         bool
}

func (e *Extension) home(w http.ResponseWriter, r *http.Request) {
	page := workerPage{Request: extplatform.RequestContextFrom(r)}
	workers, err := e.client.ListWorkers(r.Context())
	if err != nil {
		page.Unavailable = true
	} else {
		page.Workers, page.Summary = buildFleetView(workers)
	}
	if err := e.pages.Render(w, r, "starforge", page); err != nil {
		http.Error(w, "Failed to render Starforge", http.StatusInternalServerError)
	}
}

func buildFleetView(workers []Worker) ([]workerView, fleetSummary) {
	views := make([]workerView, 0, len(workers))
	summary := fleetSummary{Total: len(workers)}
	for _, worker := range workers {
		state := strings.ToLower(strings.TrimSpace(worker.State))
		if state == "online" {
			summary.Online++
		} else {
			summary.NotOnline++
		}
		hasSession := strings.TrimSpace(worker.Heartbeat.SessionID) != ""
		if hasSession {
			summary.ActiveSessions++
		}
		name := strings.TrimSpace(worker.OperatorLabel)
		if name == "" {
			name = strings.TrimSpace(worker.DisplayName)
		}
		if name == "" {
			name = "Unnamed worker"
		}
		views = append(views, workerView{
			Worker:             worker,
			Name:               name,
			StatusClass:        workerStatusClass(state),
			LastSeenLabel:      formatWorkerTime(worker.LastSeenAt),
			ConnectedAtLabel:   formatWorkerTime(worker.Heartbeat.ConnectedAt),
			LastHeartbeatLabel: formatWorkerTime(worker.Heartbeat.LastHeartbeatAt),
			HasSession:         hasSession,
		})
	}
	return views, summary
}

func workerStatusClass(state string) string {
	switch state {
	case "online":
		return "status-online"
	case "offline":
		return "status-offline"
	case "pending", "approved":
		return "status-pending"
	case "rejected", "revoked":
		return "status-blocked"
	default:
		return "status-unknown"
	}
}

func formatWorkerTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "Not reported"
	}
	return value.UTC().Format("2006-01-02 15:04:05 UTC")
}

func (e *Extension) listWorkers(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	workers, err := e.client.ListWorkers(r.Context())
	if err != nil {
		writeClientError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workers": workers})
}

func (e *Extension) updateWorkerLabel(w http.ResponseWriter, r *http.Request) {
	if !authenticated(r) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	workerID := chi.URLParam(r, "uuid")
	if _, err := uuid.Parse(workerID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid worker UUID")
		return
	}
	label, err := readLabel(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid worker label")
		return
	}
	if err := e.client.UpdateWorkerLabel(r.Context(), workerID, label); err != nil {
		writeClientError(w, err)
		return
	}
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func authenticated(r *http.Request) bool {
	return extplatform.RequestContextFrom(r).User != nil
}

func readLabel(w http.ResponseWriter, r *http.Request) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	contentType := r.Header.Get("Content-Type")
	var label string
	if strings.Contains(contentType, "application/json") {
		var body struct {
			Label string `json:"label"`
		}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil || ensureJSONEOF(decoder) != nil {
			return "", ErrInvalidLabel
		}
		label = body.Label
	} else {
		if err := r.ParseForm(); err != nil {
			return "", ErrInvalidLabel
		}
		label = r.FormValue("label")
	}
	return ValidateLabel(label)
}

func writeClientError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidLabel):
		writeError(w, http.StatusBadRequest, "invalid worker label")
	case errors.Is(err, ErrMalformedResponse):
		writeError(w, http.StatusBadGateway, "Starforge returned an invalid response")
	default:
		writeError(w, http.StatusServiceUnavailable, "Starforge is temporarily unavailable")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
