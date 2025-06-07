package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
)

func SetupMetrics(mux *http.ServeMux, db *sql.DB, redisClient *redis.Client) *monitoring.MetricsServer {
	mux.Handle("/metrics", promhttp.Handler())

	dbCollector := monitoring.NewDBMetricsCollector(db)
	ctx := context.Background()
	dbCollector.StartCollecting(ctx, 15*time.Second)

	monitoring.InstrumentRedisClient(redisClient)

	metricsServer := monitoring.NewMetricsServer(":9090")

	return metricsServer
}

func WrapHandlers(mux *http.ServeMux, handlers map[string]http.Handler) {
	for path, handler := range handlers {
		mux.Handle(path, monitoring.WrapHandler(handler))
	}
}

func ExampleBusinessMetricsIntegration(checkoutUseCase interface{}, purchaseUseCase interface{}) {

	/*
		businessMetrics := monitoring.NewBusinessMetricsMiddleware()

		originalCheckoutHandler := checkoutUseCase.HandleCheckout
		checkoutUseCase.HandleCheckout = businessMetrics.WrapCheckoutHandler(originalCheckoutHandler)

		originalPurchaseHandler := purchaseUseCase.HandlePurchase
		purchaseUseCase.HandlePurchase = businessMetrics.WrapPurchaseHandler(originalPurchaseHandler)
	*/
}

func ExampleDatabaseMetricsIntegration(db *sql.DB) {

	/*
		ctx := context.Background()

		rows, err := monitoring.InstrumentQuery(
			ctx,
			db,
			"SELECT",
			"sales",
			"SELECT * FROM sales WHERE active = true",
		)

		result, err := monitoring.InstrumentExec(
			ctx,
			db,
			"INSERT",
			"items",
			"INSERT INTO items (id, name, sale_id) VALUES ($1, $2, $3)",
			"item-123",
			"Product Name",
			"sale-456",
		)
	*/
}

func ExampleRedisMetricsIntegration(redisClient *redis.Client) {

	/*
		bloomMetrics := monitoring.NewBloomFilterMetrics("items_sold")

		bloomMetrics.RecordAdd()

		bloomMetrics.RecordCheck()

		lockMetrics := monitoring.NewDistributedLockMetrics("purchase_lock")

		lockMetrics.RecordAttempt()

		lockMetrics.RecordSuccess()

		lockMetrics.RecordFailure("timeout")

		end := lockMetrics.TimeOperation()
		end() // Record duration
	*/
}
