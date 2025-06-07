package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/middleware"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
)

func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/health", s.healthHandler.HandleHealth())

	mux.HandleFunc("/sales/active", s.saleHandler.HandleGetActiveSale)
	mux.HandleFunc("/sales/", s.handleSaleRoutes)
	mux.HandleFunc("/checkout", s.checkoutHandler.HandleCheckout())
	mux.HandleFunc("/purchase", s.purchaseHandler.HandlePurchase())
	mux.HandleFunc("/admin/sales", s.adminHandler.HandleCreateSale)

	handler := middleware.NewRecoveryMiddleware(s.logger)(mux)
	handler = middleware.NewLoggingMiddleware(s.logger)(handler)
	handler = monitoring.WrapHandler(handler)
	handler = s.corsMiddleware(handler)
	handler = s.timeoutMiddleware(handler)

	return handler
}

func (s *Server) handleSaleRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sales/")
	parts := strings.Split(path, "/")

	if len(parts) == 1 && parts[0] != "" {
		if r.Method == http.MethodGet {
			s.saleHandler.HandleGetSale(w, r)
			return
		}
	} else if len(parts) == 2 && parts[1] == "items" {
		if r.Method == http.MethodGet {
			s.saleHandler.HandleGetSaleItems(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Expose-Headers", "Link")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) timeoutMiddleware(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, 90*time.Second, "Request timeout")
}
