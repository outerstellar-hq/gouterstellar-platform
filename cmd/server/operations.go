package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo"
)

func localhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	if hostport == "" {
		return true
	}
	host := hostport
	if parsed, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func livenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "UP",
			"build":     buildinfo.Current(),
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		})
	})
}

func readinessHandler(ping func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		body := map[string]any{
			"status":    "UP",
			"build":     buildinfo.Current(),
			"database":  map[string]string{"status": "UP"},
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err := ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "DOWN"
			body["database"] = map[string]string{
				"status": "DOWN",
				"error":  "Database connection failed",
			}
		}
		writeJSON(w, status, body)
	})
}

func robotsHandler() http.Handler {
	const body = `User-agent: *
Allow: /
Allow: /contacts
Allow: /search
Disallow: /api/
Disallow: /admin/
Disallow: /ws/
Disallow: /auth/
Disallow: /errors/
Disallow: /components/
Disallow: /messages/
Disallow: /notifications/
Disallow: /settings/

Sitemap: /sitemap.xml
`
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})
}

func sitemapHandler(appBaseURL string) http.Handler {
	base := strings.TrimRight(appBaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		today := time.Now().Format(time.DateOnly)
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url>
        <loc>%s/</loc>
        <lastmod>%s</lastmod>
        <changefreq>weekly</changefreq>
        <priority>1.0</priority>
    </url>
    <url>
        <loc>%s/auth</loc>
        <lastmod>%s</lastmod>
        <changefreq>monthly</changefreq>
        <priority>0.5</priority>
    </url>
    <url>
        <loc>%s/search</loc>
        <lastmod>%s</lastmod>
        <changefreq>weekly</changefreq>
        <priority>0.8</priority>
    </url>
</urlset>
`, base, today, base, today, base, today)
	})
}

func routeDiagnosticsHandler(catalog *extplatform.Catalog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"build":              buildinfo.Current(),
			"routes":             catalog.Routes(),
			"excludedPageSets":   []string{},
			"extensionReadiness": catalog.Readiness(),
			"timestamp":          time.Now().UTC().Format(time.RFC3339Nano),
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
