package filter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics returns a middleware that records Prometheus metrics for every HTTP
// request. Labels use the matched Chi route pattern (e.g. "/contacts/{syncId}")
// rather than the raw URL path to avoid label cardinality explosion from
// path-embedded IDs.
func Metrics(reg *prometheus.Registry) func(http.Handler) http.Handler {
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route_pattern", "status"})

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "route_pattern", "status"})

	reg.MustRegister(duration, requests)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rc, r)

			elapsed := time.Since(start).Seconds()
			status := strconv.Itoa(rc.status)
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = "not_found"
			}

			duration.WithLabelValues(r.Method, routePattern, status).Observe(elapsed)
			requests.WithLabelValues(r.Method, routePattern, status).Inc()
		})
	}
}
