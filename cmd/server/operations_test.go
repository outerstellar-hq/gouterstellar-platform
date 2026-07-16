package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

func TestLocalhostOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "missing host", want: http.StatusNoContent},
		{name: "localhost", host: "localhost:8080", want: http.StatusNoContent},
		{name: "ipv4 loopback", host: "127.0.0.1:8080", want: http.StatusNoContent},
		{name: "ipv6 loopback", host: "[::1]:8080", want: http.StatusNoContent},
		{name: "localhost suffix attack", host: "localhost.attacker.example", want: http.StatusForbidden},
		{name: "ipv4 loopback range", host: "127.0.0.10:8080", want: http.StatusNoContent},
		{name: "remote host", host: "app.example.com", want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			req.Host = tt.host
			res := httptest.NewRecorder()
			localhostOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(res, req)
			assert.Equal(t, tt.want, res.Code)
		})
	}
}

func TestLivenessDoesNotProbeDependencies(t *testing.T) {
	t.Parallel()

	res := httptest.NewRecorder()
	livenessHandler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
	assert.Contains(t, res.Body.String(), `"status":"UP"`)
	assert.Contains(t, res.Body.String(), `"timestamp":`)
}

func TestReadinessReportsDatabaseStateWithoutLeakingErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ping       func(context.Context) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ready",
			ping:       func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   `"database":{"status":"UP"}`,
		},
		{
			name:       "database down",
			ping:       func(context.Context) error { return errors.New("secret connection string") },
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"error":"Database connection failed"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := httptest.NewRecorder()
			readinessHandler(tt.ping).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

			assert.Equal(t, tt.wantStatus, res.Code)
			assert.Contains(t, res.Body.String(), tt.wantBody)
			assert.NotContains(t, res.Body.String(), "secret connection string")
		})
	}
}

func TestStaticDiscoveryHandlers(t *testing.T) {
	t.Parallel()

	robots := httptest.NewRecorder()
	robotsHandler().ServeHTTP(robots, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	require.Equal(t, http.StatusOK, robots.Code)
	assert.Contains(t, robots.Body.String(), "Disallow: /api/")
	assert.Contains(t, robots.Body.String(), "Sitemap: /sitemap.xml")

	sitemap := httptest.NewRecorder()
	sitemapHandler("https://app.example.com/").ServeHTTP(sitemap, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	require.Equal(t, http.StatusOK, sitemap.Code)
	assert.Equal(t, "application/xml; charset=utf-8", sitemap.Header().Get("Content-Type"))
	assert.Contains(t, sitemap.Body.String(), "<loc>https://app.example.com/</loc>")
	assert.Contains(t, sitemap.Body.String(), "<loc>https://app.example.com/search</loc>")
	assert.False(t, strings.Contains(sitemap.Body.String(), "app.example.com//"))
}

func TestRouteDiagnosticsHandler(t *testing.T) {
	t.Parallel()

	res := httptest.NewRecorder()
	routeDiagnosticsHandler(extplatform.NewCatalog()).ServeHTTP(
		res,
		httptest.NewRequest(http.MethodGet, "/debug/routes", nil),
	)
	require.Equal(t, http.StatusOK, res.Code)
	assert.JSONEq(t, `{"excludedPageSets":[],"extensionReadiness":[],"routes":[],"timestamp":"ignored"}`,
		strings.ReplaceAll(res.Body.String(), extractTimestamp(res.Body.String()), "ignored"))
}

func extractTimestamp(body string) string {
	var payload struct {
		Timestamp string `json:"timestamp"`
	}
	_ = json.Unmarshal([]byte(body), &payload)
	return payload.Timestamp
}
