package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yuzvak/flashsale-service/internal/config"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/server"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/postgres"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/redis"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/scheduler"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	log := logger.NewLogger()
	log.Info("Starting Flash Sale Service")

	cfg, configErr := config.LoadConfig(*configPath)
	if configErr != nil {
		log.Fatal("Failed to load configuration", "error", configErr)
	}

	db, dbErr := postgres.NewConnection(cfg.Database)
	if dbErr != nil {
		log.Fatal("Failed to connect to database", "error", dbErr)
	}
	defer db.Close()

	if migrationErr := postgres.RunMigrations(cfg.Database); migrationErr != nil {
		log.Fatal("Failed to run migrations", "error", migrationErr)
	}

	redisClient, err := redis.NewConnection(cfg.Redis)
	if err != nil {
		log.Fatal("Failed to connect to Redis", "error", err)
	}
	defer redisClient.Close()

	dbMetricsCollector := monitoring.NewDBMetricsCollector(db.GetDB())
	dbMetricsCollector.StartCollecting(context.Background(), 30*time.Second)

	saleRepo := postgres.NewSaleRepository(db)
	saleScheduler := scheduler.NewSaleScheduler(saleRepo, log, 10000)

	httpServer := server.NewServer(cfg, db.GetDB(), redisClient, log)

	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	go saleScheduler.Start(serverCtx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sigChan
		shutdownCtx, _ := context.WithTimeout(serverCtx, 30*time.Second)

		log.Info("Shutting down server...")
		saleScheduler.Stop()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("Server shutdown error", "error", err)
		}

		serverStopCtx()
	}()

	log.Info("Server starting", "address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server failed", "error", err)
	}

	<-serverCtx.Done()
	log.Info("Server stopped")
}
