package reports

import (
	"encoding/json"
	"fmt"
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// Contribute registers the reports extension's routes and navigation through
// the platform's contribution context. Every route registration is stamped with
// the extension's owner ID and validated against the manifest's ownership
// declaration by the platform's route registry.
func (e *Extension) Contribute(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/reports", "Reports home", http.HandlerFunc(e.home))
	ctx.Routes.API(http.MethodGet, "/api/v1/reports/summary", "Message count summary", http.HandlerFunc(e.summary))
	ctx.Navigation.Add("Reports", "/reports", "bar-chart")
	return nil
}

// home renders a minimal HTML page showing the message count. It accesses data
// exclusively through the MessageCounter capability interface.
func (e *Extension) home(w http.ResponseWriter, r *http.Request) {
	count, err := e.messages.CountMessages(r.Context())
	if err != nil {
		http.Error(w, "Failed to load report data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>Reports</title></head><body>")
	_, _ = fmt.Fprintf(w, "<h1>Reports</h1>")
	_, _ = fmt.Fprintf(w, "<p>Messages: %d</p>", count)
	_, _ = fmt.Fprintf(w, "</body></html>")
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
