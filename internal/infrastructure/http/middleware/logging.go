package middleware

import (
	"net/http"
	"time"

	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

func NewLoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()

			wrw := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrw, r)

			log.Info("HTTP Request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrw.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
