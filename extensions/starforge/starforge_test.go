package starforge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type fakeClient struct {
	workers       []Worker
	sleepCatalog  SleepCatalog
	listErr       error
	sleepErr      error
	updateErr     error
	updatedID     string
	updatedLabels []string
}

func (f *fakeClient) ListWorkers(context.Context) ([]Worker, error) {
	return f.workers, f.listErr
}

func (f *fakeClient) SleepCatalog(context.Context) (SleepCatalog, error) {
	return f.sleepCatalog, f.sleepErr
}

func (f *fakeClient) UpdateWorkerLabel(_ context.Context, workerID, label string) error {
	f.updatedID = workerID
	f.updatedLabels = append(f.updatedLabels, label)
	return f.updateErr
}

type fakeRenderer struct{}

func (fakeRenderer) RegisterTemplates(string, fs.FS, string, string) error { return nil }
func (fakeRenderer) RenderPage(w http.ResponseWriter, _ *http.Request, page string, data any) error {
	_, _ = w.Write([]byte(page))
	switch view := data.(type) {
	case workerPage:
		if view.Unavailable {
			_, _ = w.Write([]byte(" Starforge is temporarily unavailable"))
		}
		for _, worker := range view.Workers {
			_, _ = w.Write([]byte(" " + worker.DisplayName + " " + worker.OperatorLabel))
		}
		_, _ = w.Write([]byte(fmt.Sprintf(" total=%d online=%d sessions=%d", view.Summary.Total, view.Summary.Online, view.Summary.ActiveSessions)))
	case sleepSeriesPage:
		if view.Unavailable {
			_, _ = w.Write([]byte(" Sleep catalog is temporarily unavailable"))
		}
		for _, story := range view.Stories {
			_, _ = w.Write([]byte(" " + story.Title))
			for _, episode := range story.Episodes {
				_, _ = w.Write([]byte(" " + episode.Title + " " + episode.PublicationTitle + " " + episode.PublicationDescription))
				for _, artifact := range episode.Artifacts {
					_, _ = w.Write([]byte(" " + artifact.Label + " " + artifact.StateClass))
					if !artifact.HasLink {
						_, _ = w.Write([]byte(" No active link"))
					}
				}
			}
		}
	}
	return nil
}

func TestStarforgeContract(t *testing.T) {
	t.Parallel()

	extension := New(&fakeClient{})
	diagnostics, err := extplatform.CheckExtension(extension, extplatform.TestHostContext(extplatform.ServiceBag{Pages: fakeRenderer{}}))
	require.NoError(t, err)
	assert.Equal(t, "starforge", extension.Manifest().ID)
	assert.Equal(t, []string{
		"GET /starforge",
		"GET /starforge/pipelines/sleep-series",
		"GET /api/starforge/workers",
		"PUT /api/starforge/workers/{uuid}/label",
	}, diagnostics.RoutePatterns())
	assert.Contains(t, diagnostics.NavigationLabels(), "Starforge")
}

func TestPipelineTemplateRegistryFailsClosed(t *testing.T) {
	t.Parallel()

	registry := newPipelineTemplateRegistry()
	template, err := registry.selectTemplate(pipelineKindSleepSeries, sleepSeriesSchema)
	require.NoError(t, err)
	assert.Equal(t, "starforge_sleep_series", template.Page)

	_, err = registry.selectTemplate("bounty-broadcast", sleepSeriesSchema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported Starforge pipeline template")

	_, err = registry.selectTemplate(pipelineKindSleepSeries, sleepSeriesSchema+1)
	require.Error(t, err)
}

func TestPageRendersWorkersAndExplicitUnavailableState(t *testing.T) {
	t.Parallel()

	workerClient := &fakeClient{workers: []Worker{{UUID: uuid.NewString(), DisplayName: "Agent Mac", OperatorLabel: "Nova", State: "online"}}}
	app := newStarforgeTestApp(t, workerClient)
	request := authenticatedRequest(http.MethodGet, "/starforge", nil)
	response := httptest.NewRecorder()
	app.Handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Agent Mac Nova")
	assert.Contains(t, response.Body.String(), "total=1 online=1 sessions=0")

	unavailableClient := &fakeClient{listErr: ErrUnavailable}
	app = newStarforgeTestApp(t, unavailableClient)
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/starforge", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "temporarily unavailable")
}

func TestBuildFleetViewUsesTruthfulStatesAndMissingTimestamps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 16, 12, 30, 0, 0, time.UTC)
	workers := []Worker{
		{UUID: uuid.NewString(), DisplayName: "Mac mini", OperatorLabel: "Nova", State: "online", LastSeenAt: &now, Heartbeat: Heartbeat{SessionID: "session-1", LastHeartbeatAt: &now}},
		{UUID: uuid.NewString(), DisplayName: "Linux server", State: "offline"},
		{UUID: uuid.NewString(), State: "pending"},
	}

	views, summary := buildFleetView(workers)
	assert.Equal(t, fleetSummary{Total: 3, Online: 1, ActiveSessions: 1, NotOnline: 2}, summary)
	assert.Equal(t, "Nova", views[0].Name)
	assert.Equal(t, "status-online", views[0].StatusClass)
	assert.Equal(t, "2026-07-16 12:30:00 UTC", views[0].LastSeenLabel)
	assert.Equal(t, "Not reported", views[1].LastHeartbeatLabel)
	assert.Equal(t, "Unnamed worker", views[2].Name)
	assert.Equal(t, "status-pending", views[2].StatusClass)
}

func TestStarforgeTemplateRendersFleetSummaryAndHonestMissingData(t *testing.T) {
	t.Parallel()

	workers, summary := buildFleetView([]Worker{{
		UUID:        uuid.NewString(),
		DisplayName: "Linux render node",
		State:       "offline",
	}})
	page := struct{ BodyData workerPage }{BodyData: workerPage{
		Workers: workers,
		Summary: summary,
		Request: extplatform.RequestContext{CSRFToken: "test-csrf"},
	}}
	parsed, err := template.ParseFS(templatesFS, "templates/pages/starforge.html")
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, parsed.ExecuteTemplate(&output, "content", page))

	html := output.String()
	assert.Contains(t, html, "Starforge fleet")
	assert.Contains(t, html, "Production ledgers")
	assert.Contains(t, html, `href="/starforge/pipelines/sleep-series"`)
	assert.Contains(t, html, "Linux render node")
	assert.Contains(t, html, "Not reported")
	assert.Contains(t, html, "No active session")
	assert.Contains(t, html, `value="test-csrf"`)
	assert.NotContains(t, html, "0001-01-01")
}

func TestSleepSeriesPageRendersTypedCatalogAndUnavailableState(t *testing.T) {
	t.Parallel()

	app := newStarforgeTestApp(t, &fakeClient{sleepCatalog: SleepCatalog{Stories: []SleepStory{{
		ID:    "story-1",
		Title: "Rain over Europa",
		Order: 1,
		Episodes: []SleepEpisode{{
			ID:            "episode-1",
			Title:         "Harbor lights",
			Order:         1,
			Status:        "running",
			LastUpdatedAt: ptrTime(time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)),
			PublicationMetadata: PublicationMetadata{
				Title:       "Sleep story title",
				Description: "Publication-ready description",
			},
			Stages: []SleepStage{
				{Name: "text", Status: "complete"},
				{Name: "voice", Status: "running"},
			},
			Artifacts: []SleepArtifact{
				{Label: "720p preview", URL: "https://starline.invalid/preview.mp4", State: "preview"},
				{Label: "4K master", URL: "https://starline.invalid/master.mp4", State: "durable"},
				{Label: "expired preview", URL: "https://starline.invalid/old.mp4", State: "expired"},
				{Label: "missing subtitles", State: "missing"},
			},
		}},
	}}}})
	response := httptest.NewRecorder()
	app.Handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/starforge/pipelines/sleep-series", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Rain over Europa")
	assert.Contains(t, body, "Harbor lights")
	assert.Contains(t, body, "Sleep story title")
	assert.Contains(t, body, "Publication-ready description")
	assert.Contains(t, body, "720p preview")
	assert.Contains(t, body, "artifact-durable")
	assert.Contains(t, body, "artifact-expired")
	assert.Contains(t, body, "No active link")

	app = newStarforgeTestApp(t, &fakeClient{sleepErr: ErrUnavailable})
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/starforge/pipelines/sleep-series", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Sleep catalog is temporarily unavailable")
}

func TestSleepSeriesTemplateRendersKeyboardAndNarrowScreenLandmarks(t *testing.T) {
	t.Parallel()

	page := struct{ BodyData sleepSeriesPage }{BodyData: buildSleepSeriesPage(SleepCatalog{Stories: []SleepStory{{
		ID:    "story-1",
		Title: "Story",
		Episodes: []SleepEpisode{{
			ID:     "episode-1",
			Title:  "Episode",
			Status: "complete",
			PublicationMetadata: PublicationMetadata{
				Title:       "Copy title",
				Description: "Copy description",
			},
			Stages: []SleepStage{{Name: "text", Status: "complete"}},
			Artifacts: []SleepArtifact{
				{Label: "Durable master", URL: "https://starline.invalid/master.mp4", State: "durable"},
			},
		}},
	}}})}
	parsed, err := template.ParseFS(templatesFS, "templates/pages/starforge_sleep_series.html")
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, parsed.ExecuteTemplate(&output, "content", page))

	html := output.String()
	assert.Contains(t, html, `aria-label="Fixed production stages"`)
	assert.Contains(t, html, `aria-label="Preview and durable artifacts"`)
	assert.Contains(t, html, `data-copy-target="publication-title-episode-1"`)
	assert.Contains(t, html, `data-copy-target="publication-description-episode-1"`)
	assert.Contains(t, html, `aria-live="polite"`)
	assert.Contains(t, html, `<a class="btn btn-secondary" href="https://starline.invalid/master.mp4">Open artifact</a>`)
	assert.NotContains(t, html, "<script")
}

func TestProtectedPageAndBFFRejectAnonymousRequests(t *testing.T) {
	t.Parallel()

	app := newStarforgeTestApp(t, &fakeClient{})
	page := httptest.NewRecorder()
	app.Handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/starforge", nil))
	assert.Equal(t, http.StatusUnauthorized, page.Code)

	api := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/starforge/workers", nil)
	request.Header.Set("Accept", "application/json")
	app.Handler.ServeHTTP(api, request)
	assert.Equal(t, http.StatusUnauthorized, api.Code)
}

func TestWorkerBFFUsesStableCamelCaseContractAndNullMissingTimes(t *testing.T) {
	t.Parallel()

	workerID := uuid.NewString()
	app := newStarforgeTestApp(t, &fakeClient{workers: []Worker{{
		UUID:        workerID,
		DisplayName: "Linux render node",
		State:       "offline",
	}}})
	response := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/starforge/workers", nil)
	request.Header.Set("Accept", "application/json")
	app.Handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"workers":[{"uuid":"`+workerID+`","displayName":"Linux render node","operatorLabel":"","state":"offline","os":"","architecture":"","agentVersion":"","lastSeenAt":null,"heartbeat":{"serverId":"","sessionId":"","connectedAt":null,"lastHeartbeatAt":null}}]}`, response.Body.String())
}

func TestWorkerLabelValidationAndErrorPropagation(t *testing.T) {
	t.Parallel()

	workerID := uuid.NewString()
	client := &fakeClient{}
	app := newStarforgeTestApp(t, client)
	changed := authenticatedRequest(http.MethodPut, "/api/starforge/workers/"+workerID+"/label", strings.NewReader(`{"label":" Nova · Mac mini "}`))
	changed.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	app.Handler.ServeHTTP(response, changed)
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, workerID, client.updatedID)
	assert.Equal(t, []string{"Nova · Mac mini"}, client.updatedLabels)

	cleared := authenticatedRequest(http.MethodPut, "/api/starforge/workers/"+workerID+"/label", strings.NewReader(`{"label":""}`))
	cleared.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, cleared)
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "", client.updatedLabels[1])

	invalid := authenticatedRequest(http.MethodPut, "/api/starforge/workers/"+workerID+"/label", strings.NewReader(`{"label":"`+strings.Repeat("x", 121)+`"}`))
	invalid.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, invalid)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	require.Len(t, client.updatedLabels, 2)

	client.updateErr = errors.New("remote failure containing private detail")
	failed := authenticatedRequest(http.MethodPut, "/api/starforge/workers/"+workerID+"/label", strings.NewReader(`{"label":"Nova"}`))
	failed.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, failed)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.NotContains(t, response.Body.String(), "private detail")
}

func newStarforgeTestApp(t *testing.T, client Client) *extplatform.TestApp {
	t.Helper()
	requireAuthenticated := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if extplatform.RequestContextFrom(r).User == nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	app, err := extplatform.NewTestApp(extplatform.TestOptions{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{New(client)},
		Services:   extplatform.ServiceBag{Pages: fakeRenderer{}},
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			extplatform.GroupProtectedUI: {requireAuthenticated},
			extplatform.GroupAPI:         {requireAuthenticated},
		},
	})
	require.NoError(t, err)
	return app
}

func authenticatedRequest(method, target string, body *strings.Reader) *http.Request {
	var request *http.Request
	if body == nil {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, body)
	}
	return extplatform.WithRequestContext(request, extplatform.RequestContext{
		User:      &extplatform.RequestUser{ID: uuid.NewString(), Username: "operator", Role: "USER"},
		CSRFToken: "csrf-token",
	})
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
