package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpRequestsTotal counts the total number of HTTP requests
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	// httpRequestDuration tracks request latency in seconds
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"path", "method", "status"},
	)

	// httpRequestsInFlight tracks the number of active HTTP requests
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being served",
		},
	)

	// httpResponseSize tracks the size of HTTP responses in bytes
	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response sizes in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"path", "method", "status"},
	)
)

// PrometheusMetrics is a middleware that records HTTP metrics for Prometheus
func PrometheusMetrics(isMetricsEnabled bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// If metrics are disabled, skip all metric collection
			// Skip metrics collection for the metrics endpoint itself and health endpoint
			if !isMetricsEnabled || r.URL.Path == "/metrics" || r.URL.Path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Increment in-flight requests
			httpRequestsInFlight.Inc()
			defer httpRequestsInFlight.Dec()

			// Normalize the path BEFORE handling the request
			// This ensures we get the route pattern from chi's context early
			path := normalizePath(r)
			method := r.Method

			// Use chi's WrapResponseWriter to preserve interfaces (Flusher, etc.)
			// and automatically track status/size
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Handle the request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())

			// Record metrics
			httpRequestsTotal.WithLabelValues(path, method, status).Inc()
			httpRequestDuration.WithLabelValues(path, method, status).Observe(duration)
			httpResponseSize.WithLabelValues(path, method, status).Observe(float64(ww.BytesWritten()))
		})
	}
}

// Regex patterns for path normalization, compiled once for performance
var (
	// Matches UUID format (8-4-4-4-12 hex digits)
	// Example: /users/550e8400-e29b-41d4-a716-446655440000 → /users/{id}
	uuidPattern = regexp.MustCompile(`/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

	// Matches numeric IDs (pure digits)
	// Example: /users/123 → /users/{id}, /batches/456 → /batches/{id}
	numericIDPattern = regexp.MustCompile(`/\d+`)

	// Only matches if 16+ chars to avoid false positives with names like "admin" or "login"
	// Example: /sessions/a1b2c3d4e5f6g7h8i9j0 → /sessions/{id}
	alphanumericIDPattern = regexp.MustCompile(`/[a-zA-Z0-9_-]{16,}`)
)

// normalizePath extracts the route pattern to prevent metric cardinality explosion
// Strategy:
// 1. Try chi's RoutePattern() first (works for normal handlers)
// 2. Fallback to regex-based normalization (handles redirectHandler and proxy cases)
func normalizePath(r *http.Request) string {

	// First attempt: Get route pattern from chi's routing context
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		routePattern := rctx.RoutePattern()
		if routePattern != "" {
			return routePattern
		}
	}

	// Fallback: Use regex patterns to normalize IDs
	// This handles cases where RoutePattern() is empty (e.g., redirectHandler)
	path := r.URL.Path

	// Apply patterns in order of specificity (most specific first)
	path = uuidPattern.ReplaceAllString(path, "/{id}")

	path = alphanumericIDPattern.ReplaceAllString(path, "/{id}")

	path = numericIDPattern.ReplaceAllString(path, "/{id}")

	return path
}
