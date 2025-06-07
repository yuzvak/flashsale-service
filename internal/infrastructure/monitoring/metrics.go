package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler", "method", "status_code"},
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"handler", "method", "status_code"},
	)
)

var (
	SaleItemsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sale_items_total",
			Help: "Total number of items in sale",
		},
	)

	SaleItemsSold = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sale_items_sold",
			Help: "Number of items sold in sale",
		},
	)

	SaleItemsSoldTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sale_items_sold_total",
			Help: "Total number of items sold",
		},
	)

	CheckoutAttemptsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "checkout_attempts_total",
			Help: "Total number of checkout attempts",
		},
	)

	CheckoutSuccessTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "checkout_success_total",
			Help: "Total number of successful checkouts",
		},
	)

	CheckoutFailureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "checkout_failure_total",
			Help: "Total number of failed checkouts",
		},
		[]string{"reason"},
	)

	PurchaseAttemptsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "purchase_attempts_total",
			Help: "Total number of purchase attempts",
		},
	)

	PurchaseSuccessTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "purchase_success_total",
			Help: "Total number of successful purchases",
		},
	)

	PurchaseFailureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "purchase_failure_total",
			Help: "Total number of failed purchases",
		},
		[]string{"reason"},
	)
)

var (
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"query_type", "table"},
	)

	DBConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)
)

var (
	RedisCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "redis_command_duration_seconds",
			Help:    "Duration of Redis commands in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"command"},
	)

	RedisLockAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_lock_attempts_total",
			Help: "Total number of distributed lock attempts",
		},
		[]string{"lock_type"},
	)

	RedisLockSuccessTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_lock_success_total",
			Help: "Total number of successful lock acquisitions",
		},
		[]string{"lock_type"},
	)

	RedisLockFailureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_lock_failure_total",
			Help: "Total number of failed lock acquisitions",
		},
		[]string{"lock_type", "reason"},
	)

	RedisLockDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "redis_lock_duration_seconds",
			Help:    "Duration of lock hold time in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"lock_type"},
	)
)

func TimeHTTPRequest(handler, method string) func(statusCode string) {
	start := time.Now()
	return func(statusCode string) {
		duration := time.Since(start).Seconds()
		HTTPRequestDuration.WithLabelValues(handler, method, statusCode).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(handler, method, statusCode).Inc()
	}
}

func TimeDBQuery(queryType, table string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start).Seconds()
		DBQueryDuration.WithLabelValues(queryType, table).Observe(duration)
	}
}

func TimeRedisCommand(command string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start).Seconds()
		RedisCommandDuration.WithLabelValues(command).Observe(duration)
	}
}

func TimeRedisLock(lockKey string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start).Seconds()
		lockType := getLockType(lockKey)
		RedisLockDuration.WithLabelValues(lockType).Observe(duration)
	}
}

func RecordCheckoutAttempt(userID, itemID string) {
	CheckoutAttemptsTotal.Inc()
}

func RecordCheckoutSuccess(userID, itemID string) {
	CheckoutSuccessTotal.Inc()
}

func RecordCheckoutFailure(userID, itemID, reason string) {
	CheckoutFailureTotal.WithLabelValues(reason).Inc()
}

func RecordPurchaseAttempt(checkoutCode string) {
	PurchaseAttemptsTotal.Inc()
}

func RecordPurchaseSuccess(checkoutCode string) {
	PurchaseSuccessTotal.Inc()
}

func RecordPurchaseFailure(checkoutCode, reason string) {
	PurchaseFailureTotal.WithLabelValues(reason).Inc()
}

func RecordItemSold(saleID, itemID string) {
	SaleItemsSoldTotal.Inc()
}

func UpdateSaleItemsCount(saleID string, total, sold int) {
	SaleItemsTotal.Set(float64(total))
	SaleItemsSold.Set(float64(sold))
}

func RecordLockAttempt(lockKey string) {
	lockType := getLockType(lockKey)
	RedisLockAttemptsTotal.WithLabelValues(lockType).Inc()
}

func RecordLockSuccess(lockKey string) {
	lockType := getLockType(lockKey)
	RedisLockSuccessTotal.WithLabelValues(lockType).Inc()
}

func RecordLockFailure(lockKey, reason string) {
	lockType := getLockType(lockKey)
	RedisLockFailureTotal.WithLabelValues(lockType, reason).Inc()
}

func getLockType(lockKey string) string {
	if len(lockKey) >= 4 {
		prefix := lockKey[:4]
		switch prefix {
		case "sale":
			return "sale"
		case "item":
			return "item"
		case "user":
			return "user"
		default:
			return "other"
		}
	}
	return "unknown"
}
