package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/yuzvak/flashsale-service/internal/application/use_cases"
	"github.com/yuzvak/flashsale-service/internal/config"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/handlers"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/postgres"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/redis"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type Server struct {
	server          *http.Server
	logger          *logger.Logger
	healthHandler   *handlers.HealthHandler
	saleHandler     *handlers.SaleHandler
	checkoutHandler *handlers.CheckoutHandler
	purchaseHandler *handlers.PurchaseHandler
	adminHandler    *handlers.AdminHandler
}

func NewServer(cfg *config.Config, db *sql.DB, redisConn *redis.Connection, logger *logger.Logger) *Server {
	conn, err := postgres.NewConnection(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	saleRepo := postgres.NewSaleRepository(conn)
	checkoutRepo := postgres.NewCheckoutRepository(conn)

	cache := redis.NewCache(redisConn, logger)

	purchaseUseCase := use_cases.NewPurchaseUseCase(
		saleRepo,
		checkoutRepo,
		cache,
		logger,
	)

	saleHandler := handlers.NewSaleHandler(saleRepo, logger)
	checkoutHandler := handlers.NewCheckoutHandler(saleRepo, checkoutRepo, cache, logger)
	purchaseHandler := handlers.NewPurchaseHandler(purchaseUseCase, logger)
	adminHandler := handlers.NewAdminHandler(saleRepo, logger)
	healthHandler := handlers.NewHealthHandler(db, redisConn.GetClient(), logger)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		server:          server,
		logger:          logger,
		healthHandler:   healthHandler,
		saleHandler:     saleHandler,
		checkoutHandler: checkoutHandler,
		purchaseHandler: purchaseHandler,
		adminHandler:    adminHandler,
	}
}

func (s *Server) ListenAndServe() error {
	s.server.Handler = s.setupRoutes()

	s.logger.Info("Starting HTTP server", map[string]interface{}{
		"address": s.server.Addr,
	})

	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server", nil)
	return s.server.Shutdown(ctx)
}
