package middleware

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

func NewRecoveryMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("Panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
						"method", r.Method,
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Internal server error",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
