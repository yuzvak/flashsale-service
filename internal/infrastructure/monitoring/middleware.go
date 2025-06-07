package monitoring

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPMetricsMiddleware struct {
	next http.Handler
}

func NewHTTPMetricsMiddleware(next http.Handler) *HTTPMetricsMiddleware {
	return &HTTPMetricsMiddleware{
		next: next,
	}
}

func (m *HTTPMetricsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	wrapped := &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default to 200
	}

	handlerName := extractHandlerName(r.URL.Path)

	m.next.ServeHTTP(wrapped, r)

	duration := time.Since(start).Seconds()
	statusCode := strconv.Itoa(wrapped.statusCode)

	HTTPRequestDuration.WithLabelValues(handlerName, r.Method, statusCode).Observe(duration)
	HTTPRequestsTotal.WithLabelValues(handlerName, r.Method, statusCode).Inc()
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func extractHandlerName(path string) string {
	path = strings.TrimPrefix(path, "/")

	switch {
	case strings.HasPrefix(path, "api/v1/admin/sales"):
		return "admin_sales"
	case strings.HasPrefix(path, "api/v1/admin/items"):
		return "admin_items"
	case strings.HasPrefix(path, "api/v1/checkout"):
		return "checkout"
	case strings.HasPrefix(path, "api/v1/purchase"):
		return "purchase"
	case strings.HasPrefix(path, "metrics"):
		return "metrics"
	case strings.HasPrefix(path, "health"):
		return "health"
	default:
		parts := strings.Split(path, "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
		return "unknown"
	}
}

func MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Metrics endpoint - use promhttp.Handler() in production"))
	})
}
