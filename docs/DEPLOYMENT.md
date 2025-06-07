# Deployment Strategy

## Overview

The flash sale service is containerized using Docker with a multi-stage build process and includes comprehensive orchestration for development, testing, and production environments.

## Docker Configuration

### Multi-Stage Dockerfile
```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy dependency files
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o flashsale ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder stage
COPY --from=builder /app/flashsale /app/

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Copy config file
COPY --from=builder /app/config.json /app/

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/flashsale"]
```

**Build Optimizations:**
- Multi-stage build reduces final image size
- Static binary compilation (CGO_ENABLED=0)
- Alpine Linux base for security and size
- Essential runtime dependencies only

## Docker Compose Orchestration

### Core Services
```yaml
services:
  app:
    build: .
    ports:
      - "${SERVER_PORT:-8080}:${SERVER_PORT:-8080}"
    environment:
      - SERVER_HOST=${SERVER_HOST:-0.0.0.0}
      - SERVER_PORT=${SERVER_PORT:-8080}
      - DB_HOST=${DB_HOST:-postgres}
      - DB_PORT=${DB_PORT:-5432}
      - DB_USER=${DB_USER:-postgres}
      - DB_PASSWORD=${DB_PASSWORD:-postgres}
      - DB_NAME=${DB_NAME:-flashsale}
      - DB_SSLMODE=${DB_SSLMODE:-disable}
      - REDIS_HOST=${REDIS_HOST:-redis}
      - REDIS_PORT=${REDIS_PORT:-6379}
      - REDIS_PASSWORD=${REDIS_PASSWORD:-}
      - REDIS_DB=${REDIS_DB:-0}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - flashsale-network

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=${DB_USER:-postgres}
      - POSTGRES_PASSWORD=${DB_PASSWORD:-postgres}
      - POSTGRES_DB=${DB_NAME:-flashsale}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    networks:
      - flashsale-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-postgres}"]
      interval: 30s
      timeout: 10s
      retries: 3
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass "${REDIS_PASSWORD:-}"
    volumes:
      - redis-data:/data
    networks:
      - flashsale-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3
    ports:
      - "6379:6379"

volumes:
  postgres-data:
  redis-data:

networks:
  flashsale-network:
    driver: bridge
```

## Configuration Management

### Environment Variables
```bash
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_secure_password_here
DB_NAME=flashsale
DB_SSLMODE=disable

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password_here
REDIS_DB=0
```

### JSON Configuration
```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "host": "postgres",
    "port": 5432,
    "user": "postgres",
    "password": "postgres",
    "dbname": "flashsale",
    "sslmode": "disable",
    "migrations_path": "migrations"
  },
  "redis": {
    "host": "redis",
    "port": 6379,
    "password": "",
    "db": 0
  }
}
```

### Configuration Loading
```go
func LoadConfig(path string) (*Config, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var config Config
    decoder := json.NewDecoder(file)
    if err := decoder.Decode(&config); err != nil {
        return nil, err
    }

    return &config, nil
}
```

## Database Migration Strategy

### Automated Migration Runner
```go
func RunMigrations(cfg config.DatabaseConfig) error {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return err
    }
    defer db.Close()

    // Create migrations tracking table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS migrations (
            id SERIAL PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
    if err != nil {
        return err
    }

    // Get applied migrations
    rows, err := db.Query("SELECT name FROM migrations")
    if err != nil {
        return err
    }
    defer rows.Close()

    appliedMigrations := make(map[string]bool)
    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err != nil {
            return err
        }
        appliedMigrations[name] = true
    }

    // Apply new migrations
    files, err := os.ReadDir(cfg.MigrationsPath)
    if err != nil {
        return err
    }

    var migrations []string
    for _, file := range files {
        if !file.IsDir() && strings.HasSuffix(file.Name(), ".up.sql") {
            migrations = append(migrations, file.Name())
        }
    }
    sort.Strings(migrations)

    for _, migration := range migrations {
        if appliedMigrations[migration] {
            continue
        }

        filePath := filepath.Join(cfg.MigrationsPath, migration)
        content, err := os.ReadFile(filePath)
        if err != nil {
            return err
        }

        tx, err := db.Begin()
        if err != nil {
            return err
        }

        _, err = tx.Exec(string(content))
        if err != nil {
            tx.Rollback()
            return err
        }

        _, err = tx.Exec("INSERT INTO migrations (name) VALUES ($1)", migration)
        if err != nil {
            tx.Rollback()
            return err
        }

        if err := tx.Commit(); err != nil {
            return err
        }

        fmt.Printf("Applied migration: %s\n", migration)
    }

    return nil
}
```

## Health Checks & Readiness

### Health Check Endpoint
```go
type HealthData struct {
    ServicesStatus ServicesStatus `json:"services_status"`
    Uptime         string         `json:"uptime"`
    Memory         MemoryMetrics  `json:"memory"`
    Goroutines     int            `json:"goroutines"`
}

func (h *HealthHandler) HandleHealth() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        dbStatus := "UP"
        if err := h.db.Ping(); err != nil {
            dbStatus = "DOWN"
        }

        redisStatus := "UP"
        if err := h.redis.Ping(r.Context()).Err(); err != nil {
            redisStatus = "DOWN"
        }

        var mem runtime.MemStats
        runtime.ReadMemStats(&mem)

        data := HealthData{
            ServicesStatus: ServicesStatus{
                App:      "UP",
                Database: dbStatus,
                Redis:    redisStatus,
            },
            Uptime: time.Since(h.startTime).String(),
            Memory: MemoryMetrics{
                Alloc:      mem.Alloc,
                TotalAlloc: mem.TotalAlloc,
                Sys:        mem.Sys,
                NumGC:      mem.NumGC,
            },
            Goroutines: runtime.NumGoroutine(),
        }

        response.WriteSuccess(w, data)
    }
}
```

### Docker Health Checks
```yaml
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-postgres}"]
  interval: 30s
  timeout: 10s
  retries: 3
```

## Monitoring Stack Deployment

### Prometheus & Grafana
```yaml
# monitoring/docker-compose.yml
services:
  prometheus:
    image: prom/prometheus:v2.30.3
    container_name: prometheus
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    ports:
      - "9090:9090"
    restart: unless-stopped
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:8.2.2
    container_name: grafana
    volumes:
      - ./grafana-dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana-datasources:/etc/grafana/provisioning/datasources
      - grafana_data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    ports:
      - "3000:3000"
    restart: unless-stopped
    networks:
      - monitoring
    depends_on:
      - prometheus
```

## Build & Deployment Automation

### Makefile Commands
```makefile
# Build commands
build:
	go build -o flashsale ./cmd/server

docker-build:
	docker build -t flashsale-service .

# Deployment commands
docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Monitoring
run-monitoring:
	cd monitoring && docker-compose up -d

# Load testing
load-test:
	go run ./scripts/load-testing/run_test_load.go

realistic-test:
	go run ./scripts/load-testing/run_test_realistic_load.go
```

## Application Startup Sequence

### Main Application Bootstrap
```go
func main() {
    configPath := flag.String("config", "config.json", "Path to configuration file")
    flag.Parse()

    log := logger.NewLogger()
    log.Info("Starting Flash Sale Service")

    // Load configuration
    cfg, err := config.LoadConfig(*configPath)
    if err != nil {
        log.Fatal("Failed to load configuration", "error", err)
    }

    // Database connection
    db, err := postgres.NewConnection(cfg.Database)
    if err != nil {
        log.Fatal("Failed to connect to database", "error", err)
    }
    defer db.Close()

    // Run migrations
    if err := postgres.RunMigrations(cfg.Database); err != nil {
        log.Fatal("Failed to run migrations", "error", err)
    }

    // Redis connection
    redisClient, err := redis.NewConnection(cfg.Redis)
    if err != nil {
        log.Fatal("Failed to connect to Redis", "error", err)
    }
    defer redisClient.Close()

    // Start metrics collection
    dbMetricsCollector := monitoring.NewDBMetricsCollector(db.GetDB())
    dbMetricsCollector.StartCollecting(context.Background(), 30*time.Second)

    // Initialize repositories and services
    saleRepo := postgres.NewSaleRepository(db)
    saleScheduler := scheduler.NewSaleScheduler(saleRepo, log, 10000)

    // Start HTTP server
    httpServer := server.NewServer(cfg, db.GetDB(), redisClient, log)

    // Graceful shutdown handling
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
```

## Security Considerations

### Container Security
- Non-root user in containers
- Read-only filesystem where possible
- Minimal attack surface with Alpine Linux
- Regular security updates for base images

### Network Security
- Internal Docker networks for service communication
- Exposed ports only where necessary
- Environment variable injection for secrets
- No hardcoded credentials in images

### Data Security
- TLS connections for database (configurable)
- Redis AUTH when required
- Input validation and sanitization
- SQL injection prevention with prepared statements

## Production Deployment Checklist

- [ ] Environment variables configured
- [ ] Database migrations applied
- [ ] Health checks responding
- [ ] Monitoring stack deployed
- [ ] Log aggregation configured
- [ ] Backup strategy implemented
- [ ] Load testing completed
- [ ] Security scan passed
- [ ] Performance benchmarks met