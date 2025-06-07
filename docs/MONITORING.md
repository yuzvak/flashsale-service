# Monitoring & Observability

## Overview

The flash sale service implements comprehensive monitoring using Prometheus metrics and Grafana dashboards, following the Four Golden Signals methodology.

## Metrics Architecture

### Four Golden Signals Implementation

#### 1. Latency
```go
HTTPRequestDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "Duration of HTTP requests in seconds",
        Buckets: prometheus.DefBuckets,
    },
    []string{"handler", "method", "status_code"},
)
```

#### 2. Traffic
```go
HTTPRequestsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"handler", "method", "status_code"},
)
```

#### 3. Errors
- HTTP error rates by status code (4xx, 5xx)
- Business logic errors (checkout failures, purchase failures)
- Database and Redis operation failures

#### 4. Saturation
- Database connection pool utilization
- Memory usage and goroutine counts
- Redis connection pool metrics

## Business Metrics

### Flash Sale Specific Metrics
```go
// Sale progress tracking
SaleItemsTotal = promauto.NewGauge(prometheus.GaugeOpts{
    Name: "sale_items_total",
    Help: "Total number of items in sale",
})

SaleItemsSold = promauto.NewGauge(prometheus.GaugeOpts{
    Name: "sale_items_sold", 
    Help: "Number of items sold in sale",
})

// Checkout flow metrics
CheckoutAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
    Name: "checkout_attempts_total",
    Help: "Total number of checkout attempts",
})

CheckoutSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
    Name: "checkout_success_total",
    Help: "Total number of successful checkouts", 
})

// Purchase flow metrics  
PurchaseAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
    Name: "purchase_attempts_total",
    Help: "Total number of purchase attempts",
})

PurchaseSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
    Name: "purchase_success_total",
    Help: "Total number of successful purchases",
})
```

## Database Monitoring

### Query Performance
```go
DBQueryDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "db_query_duration_seconds",
        Help:    "Duration of database queries in seconds",
        Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
    },
    []string{"query_type", "table"},
)
```

### Connection Pool Monitoring
```go
func (c *DBMetricsCollector) collectMetrics() {
    stats := c.db.Stats()
    DBConnectionsActive.Set(float64(stats.InUse))
    DBConnectionsIdle.Set(float64(stats.Idle))
}
```

## Redis Monitoring

### Command Performance
```go
RedisCommandDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "redis_command_duration_seconds", 
        Help:    "Duration of Redis commands in seconds",
        Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
    },
    []string{"command"},
)
```

### Distributed Lock Metrics
```go
RedisLockAttemptsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "redis_lock_attempts_total",
        Help: "Total number of distributed lock attempts",
    },
    []string{"lock_type"},
)

RedisLockDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "redis_lock_duration_seconds",
        Help:    "Duration of lock hold time in seconds", 
        Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
    },
    []string{"lock_type"},
)
```

## Health Monitoring

### Health Check Endpoint
```go
type HealthData struct {
    ServicesStatus ServicesStatus `json:"services_status"`
    Uptime         string         `json:"uptime"`
    Memory         MemoryMetrics  `json:"memory"`
    Goroutines     int            `json:"goroutines"`
}

type ServicesStatus struct {
    App      string `json:"app"`
    Database string `json:"database"`
    Redis    string `json:"redis"`
}
```

### Memory Metrics
```go
type MemoryMetrics struct {
    Alloc      uint64 `json:"alloc"`
    TotalAlloc uint64 `json:"total_alloc"`
    Sys        uint64 `json:"sys"`
    NumGC      uint32 `json:"num_gc"`
}
```

## Instrumentation Patterns

### HTTP Middleware
```go
type HTTPMetricsMiddleware struct {
    next http.Handler
}

func (m *HTTPMetricsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
    
    m.next.ServeHTTP(wrapped, r)
    
    duration := time.Since(start).Seconds()
    statusCode := strconv.Itoa(wrapped.statusCode)
    handlerName := extractHandlerName(r.URL.Path)
    
    HTTPRequestDuration.WithLabelValues(handlerName, r.Method, statusCode).Observe(duration)
    HTTPRequestsTotal.WithLabelValues(handlerName, r.Method, statusCode).Inc()
}
```

### Database Instrumentation
```go
func InstrumentQuery(ctx context.Context, db *sql.DB, queryType, table, query string, args ...interface{}) (*sql.Rows, error) {
    end := TimeDBQuery(queryType, table)
    defer end()
    return db.QueryContext(ctx, query, args...)
}
```

### Redis Hook Implementation
```go
type RedisHook struct{}

func (RedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
    return context.WithValue(ctx, "start_time", time.Now()), nil
}

func (RedisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
    start, ok := ctx.Value("start_time").(time.Time)
    if !ok {
        return nil
    }
    duration := time.Since(start).Seconds()
    RedisCommandDuration.WithLabelValues(cmd.Name()).Observe(duration)
    return nil
}
```

## Grafana Dashboard

### Dashboard Structure
1. **Four Golden Signals**: Latency, Traffic, Errors, Saturation
2. **Flash Sale Specific**: Items sold rate, checkout/purchase rates
3. **Database & Cache**: Query performance, connection pools
4. **System Resources**: Memory, CPU, goroutines

### Key Panels
- P95/P50 response time trends
- Request rate by handler
- Error rate breakdown
- Sale progress visualization
- Lock contention metrics

## Alerting Strategy

### Critical Alerts
- Service health check failures
- High error rates (>5%)
- Database connection exhaustion
- Redis connectivity issues

### Performance Alerts  
- P95 response time >2s
- High lock contention
- Memory usage >80%
- Goroutine count anomalies

## Deployment Configuration

### Docker Compose Setup
```yaml
services:
  prometheus:
    image: prom/prometheus:v2.30.3
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:8.2.2
    volumes:
      - ./grafana-dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana-datasources:/etc/grafana/provisioning/datasources
    ports:
      - "3000:3000"
```

### Prometheus Configuration
```yaml
scrape_configs:
  - job_name: "flashsale"
    metrics_path: /metrics
    scrape_interval: 5s
    static_configs:
      - targets: ["host.docker.internal:8080"]
        labels:
          instance: "flashsale-service"
          environment: "development"
```