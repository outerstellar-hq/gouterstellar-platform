package starforge

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type fakeClient struct {
	workers       []Worker
	listErr       error
	updateErr     error
	updatedID     string
	updatedLabels []string
}

func (f *fakeClient) ListWorkers(context.Context) ([]Worker, error) {
	return f.workers, f.listErr
}

func (f *fakeClient) UpdateWorkerLabel(_ context.Context, workerID, label string) error {
	f.updatedID = workerID
	f.updatedLabels = append(f.updatedLabels, label)
	return f.updateErr
}

type fakeRenderer struct{}

func (fakeRenderer) RegisterTemplates(string, fs.FS, string, string) error { return nil }
func (fakeRenderer) RenderPage(w http.ResponseWriter, _ *http.Request, page string, data any) error {
	view := data.(workerPage)
	_, _ = w.Write([]byte(page))
	if view.Unavailable {
		_, _ = w.Write([]byte(" Starforge is temporarily unavailable"))
	}
	for _, worker := range view.Workers {
		_, _ = w.Write([]byte(" " + worker.DisplayName + " " + worker.OperatorLabel))
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
		"GET /api/starforge/workers",
		"PUT /api/starforge/workers/{uuid}/label",
	}, diagnostics.RoutePatterns())
	assert.Contains(t, diagnostics.NavigationLabels(), "Starforge")
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

	unavailableClient := &fakeClient{listErr: ErrUnavailable}
	app = newStarforgeTestApp(t, unavailableClient)
	response = httptest.NewRecorder()
	app.Handler.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/starforge", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "temporarily unavailable")
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
