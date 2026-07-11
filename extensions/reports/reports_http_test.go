package reports

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

func TestReportsHomeHTTP(t *testing.T) {
	ext := New(stubMessageCounter{count: 99})

	app, err := extplatform.NewTestApp(extplatform.TestOptions{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{ext},
	})
	require.NoError(t, err)
	t.Cleanup(app.Close)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Reports")
	assert.Contains(t, rec.Body.String(), "99")
}

func TestReportsSummaryAPI(t *testing.T) {
	ext := New(stubMessageCounter{count: 7})

	app, err := extplatform.NewTestApp(extplatform.TestOptions{
		Mode:       extplatform.FullPlatform,
		Extensions: []extplatform.Extension{ext},
	})
	require.NoError(t, err)
	t.Cleanup(app.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "message_count")
	assert.Contains(t, rec.Body.String(), "7")
}
