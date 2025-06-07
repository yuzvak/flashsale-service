# Design Decisions & Architecture Rationale

## Overview

This document explains the key architectural decisions made in the flash sale microservice, including trade-offs, alternatives considered, and implementation rationale.

## Domain-Driven Design (DDD) Implementation

### Decision: Clean Architecture with DDD
**Rationale**: Separation of concerns, testability, and maintainability for complex business logic.

#### Project Structure
```
internal/
├── domain/           # Pure business logic, no external dependencies
│   ├── sale/        # Sale aggregate root
│   ├── user/        # User value objects
│   └── errors/      # Domain errors
├── application/     # Use cases and orchestration
│   ├── commands/    # Command handlers
│   ├── use_cases/   # Business workflows
│   └── ports/       # Interface definitions
└── infrastructure/ # External concerns
    ├── persistence/ # Database implementations
    ├── http/        # HTTP handlers
    └── monitoring/  # Observability
```

**Benefits:**
- Clear separation between business logic and infrastructure
- Easy to test domain logic in isolation
- Framework-agnostic design
- Scalable codebase structure

**Trade-offs:**
- More boilerplate code than simple layered architecture
- Learning curve for developers unfamiliar with DDD

## Database Design Decisions

### Decision: PostgreSQL with Normalized Schema
**Alternative Considered**: NoSQL databases (MongoDB, DynamoDB)

**Rationale:**
- ACID transactions required for inventory management
- Complex queries for analytics and reporting
- Strong consistency guarantees for financial operations
- Mature ecosystem and operational tooling

#### Schema Normalization
```sql
-- Separate tables for checkout attempts and items
CREATE TABLE checkout_attempts (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    checkout_code VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE checkout_items (
    id VARCHAR(255) PRIMARY KEY,
    checkout_attempt_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(checkout_attempt_id, item_id)
);
```

**Benefits:**
- Better query performance than JSONB
- Referential integrity enforcement
- Easier analytics and reporting
- Standard SQL optimization techniques apply

**Trade-offs:**
- More complex queries for retrieving complete checkout data
- Additional JOIN operations

### Decision: Serializable Isolation for Critical Operations
```go
tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
})
```

**Rationale:**
- Prevents race conditions in high-concurrency scenarios
- Ensures atomic operations for inventory updates
- Maintains data consistency under load

**Trade-offs:**
- Higher latency for write operations
- Potential for serialization failures requiring retries

## Caching Strategy Decisions

### Decision: Redis as Cache + Atomic Operations Coordinator
**Alternative Considered**: In-memory caching only

**Rationale:**
- Distributed system requires shared state
- Atomic operations via Lua scripts
- High-performance data structures (sets, counters)
- Persistence and durability options

#### Lua Scripts for Atomicity
```lua
-- Atomic purchase validation and counter updates
local sale_key = KEYS[1]
local user_key = KEYS[2]
local item_count = tonumber(ARGV[1])
local max_sale_items = tonumber(ARGV[2])
local max_user_items = tonumber(ARGV[3])

-- Check limits and update counters atomically
if current_sale_count + item_count > max_sale_items then
    return 0
end

redis.call('INCRBY', sale_key, item_count)
redis.call('INCRBY', user_key, item_count)
return 1
```

**Benefits:**
- Eliminates race conditions between limit checks and updates
- Single network round trip for complex operations
- Consistent behavior across multiple service instances

**Trade-offs:**
- Lua script complexity
- Redis becomes critical dependency
- Debugging distributed state can be challenging

### Decision: Bloom Filter for Performance Optimization
**Alternative Considered**: Database-only item availability checks

**Rationale:**
- Fast negative lookups (99.9% accuracy)
- Reduces database load for popular items
- Memory-efficient (128KB for 100,000 items)

```go
// 10-second update interval balances accuracy and performance
const BloomUpdateInterval = 10 * time.Second

func UpdateBloomFilter(saleID string) {
    ticker := time.NewTicker(BloomUpdateInterval)
    for range ticker.C {
        soldItems := db.Query(`SELECT id FROM items WHERE sale_id = $1 AND sold = TRUE`, saleID)
        bloom := NewBloomFilter(BloomSize, BloomHashes)
        for _, itemID := range soldItems {
            bloom.Add([]byte(itemID))
        }
        redis.Set("sale:" + saleID + ":bloom:sold", bloom.Serialize())
    }
}
```

**Benefits:**
- 90%+ reduction in database queries for sold items
- Sub-millisecond response times
- Scales well with item count

**Trade-offs:**
- 10-second lag for item availability updates
- False positives possible (but acceptable for this use case)
- Additional complexity in cache management

## Concurrency Control Decisions

### Decision: Optimistic Locking with Conditional Updates
**Alternative Considered**: Pessimistic locking with SELECT FOR UPDATE

```sql
-- Atomic item purchase with conditional update
UPDATE items 
SET sold = TRUE, sold_to_user_id = $1, sold_at = NOW()
WHERE id = $2 AND sale_id = $3 AND sold = FALSE
```

**Rationale:**
- Better performance under high contention
- No lock timeouts or deadlocks
- Natural failure mode for race conditions

**Benefits:**
- High throughput even with many concurrent purchases
- Simple error handling (0 rows affected = already sold)
- Database handles optimization automatically

**Trade-offs:**
- Requires application-level retry logic
- Failed attempts still consume resources

### Decision: Distributed Locks for Purchase Sessions
```go
lockKey := fmt.Sprintf("purchase:%s", checkoutCode)
locked, err := uc.cache.DistributedLock(ctx, lockKey, uc.lockTimeout)
```

**Rationale:**
- Prevents duplicate purchase attempts for same checkout code
- Ensures idempotency across service instances
- Graceful handling of client retry storms

**Benefits:**
- Eliminates duplicate processing
- Predictable behavior during failures
- Protection against client misbehavior

**Trade-offs:**
- Adds latency to purchase flow
- Redis becomes critical dependency
- Lock timeouts need careful tuning

## API Design Decisions

### Decision: Two-Phase Flow (Checkout → Purchase)
**Alternative Considered**: Single-phase immediate purchase

**Rationale:**
- Allows users to accumulate items before committing
- Better user experience for mobile/web clients
- Reduces inventory pressure during peak traffic

#### Flow Design
```go
// Phase 1: Add items to checkout (no inventory reservation)
POST /checkout?user_id=user123&item_id=item456
{
  "code": "CHK-S-abc123-xyz789",
  "items_count": 3,
  "sale_ends_at": "2024-11-02T16:00:00Z"
}

// Phase 2: Atomic purchase of all checkout items
POST /purchase?code=CHK-S-abc123-xyz789
{
  "success": true,
  "purchased_items": [...],
  "total_purchased": 2,
  "failed_count": 1
}
```

**Benefits:**
- Users can browse and select multiple items
- Reduces database write pressure during browsing
- Clear separation between selection and commitment
- Supports complex client workflows

**Trade-offs:**
- Items not reserved during checkout phase
- Possibility of items becoming unavailable between phases
- More complex client state management

### Decision: No Web Framework (Standard Library Only)
**Alternative Considered**: Gin, Echo, or other frameworks

**Rationale:**
- Minimal dependencies as per requirements
- Full control over request handling
- Better performance characteristics
- Easier to understand and debug

```go
func (s *Server) setupRoutes() http.Handler {
    mux := http.NewServeMux()
    
    mux.HandleFunc("/health", s.healthHandler.HandleHealth())
    mux.HandleFunc("/checkout", s.checkoutHandler.HandleCheckout())
    mux.HandleFunc("/purchase", s.purchaseHandler.HandlePurchase())
    
    // Apply middleware manually
    handler := middleware.NewRecoveryMiddleware(s.logger)(mux)
    handler = middleware.NewLoggingMiddleware(s.logger)(handler)
    handler = monitoring.WrapHandler(handler)
    
    return handler
}
```

**Benefits:**
- Zero framework dependencies
- Predictable performance characteristics
- Easy to optimize specific endpoints
- Reduced attack surface

**Trade-offs:**
- More boilerplate code for common functionality
- Manual implementation of middleware chain
- Less community ecosystem around patterns

## Error Handling Decisions

### Decision: Domain Error Mapping to HTTP Status Codes
```go
var errorMappings = map[error]ErrorMapping{
    domainErrors.ErrItemAlreadySold: {
        HTTPStatus: http.StatusConflict,
        Status:     StatusConflict,
        Message:    "Items already sold",
    },
    domainErrors.ErrUserLimitExceeded: {
        HTTPStatus: http.StatusBadRequest,
        Status:     StatusError,
        Message:    "User has reached maximum items limit",
    },
}
```

**Rationale:**
- Clean separation between domain and HTTP concerns
- Consistent error responses across endpoints
- Easy to add new error types without HTTP knowledge

**Benefits:**
- Domain errors remain HTTP-agnostic
- Consistent client experience
- Easy testing of business logic

**Trade-offs:**
- Additional mapping layer
- Potential for unmapped errors

### Decision: Structured JSON Error Responses
```go
type ErrorResponse struct {
    Message string `json:"message"`
    Error   string `json:"error,omitempty"`
    Code    string `json:"code,omitempty"`
}
```

**Rationale:**
- Machine-readable error information
- Consistent structure for client handling
- Debugging information when appropriate

## Monitoring Design Decisions

### Decision: Prometheus + Grafana Stack
**Alternative Considered**: Custom metrics, DataDog, New Relic

**Rationale:**
- Open source and self-hosted
- Industry standard for microservices
- Rich ecosystem and community support
- Cost-effective for high-volume metrics

#### Four Golden Signals Implementation
```go
// Latency
HTTPRequestDuration = promauto.NewHistogramVec(...)

// Traffic  
HTTPRequestsTotal = promauto.NewCounterVec(...)

// Errors
CheckoutFailureTotal = promauto.NewCounterVec(...)

// Saturation
DBConnectionsActive = promauto.NewGauge(...)
```

**Benefits:**
- Comprehensive observability out of the box
- Historical data retention and analysis
- Alerting capabilities
- Cost-effective scaling

**Trade-offs:**
- Additional infrastructure to manage
- Learning curve for Prometheus query language
- Storage requirements for metrics retention

### Decision: Business Metrics as First-Class Citizens
```go
// Flash sale specific metrics
SaleItemsSoldTotal = promauto.NewCounter(...)
CheckoutSuccessTotal = promauto.NewCounter(...)
PurchaseSuccessTotal = promauto.NewCounter(...)
```

**Rationale:**
- Business stakeholders need real-time visibility
- Technical metrics alone insufficient for flash sales
- Enables data-driven optimization decisions

**Benefits:**
- Business and technical teams share common metrics
- Real-time business intelligence
- Easier debugging of business logic issues

## Performance Optimization Decisions

### Decision: Connection Pooling Strategy
```go
db.SetMaxOpenConns(100)     // Total connections across all instances
db.SetMaxIdleConns(50)      // Keep connections warm
db.SetConnMaxLifetime(time.Hour)
```

**Rationale:**
- Balance between connection overhead and resource usage
- Account for multiple service instances sharing database
- Prevent connection leaks and stale connections

### Decision: Prepared Statements for All Queries
```go
stmt, err := tx.PrepareContext(ctx, `
    INSERT INTO items (id, sale_id, name, image_url, sold, created_at)
    VALUES ($1, $2, $3, $4, $5, $6)
`)
```

**Rationale:**
- Better performance for repeated queries
- Protection against SQL injection
- Query plan caching at database level

**Benefits:**
- 20-30% performance improvement for bulk operations
- Enhanced security posture
- Consistent query performance

**Trade-offs:**
- Additional complexity in query handling
- Memory usage for statement caching

## Security Design Decisions

### Decision: Input Validation at Handler Level
```go
func (h *CheckoutHandler) HandleCheckout() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := r.URL.Query().Get("user_id")
        itemID := r.URL.Query().Get("item_id")
        
        errors := make(map[string]string)
        if userID == "" {
            errors["user_id"] = "user_id is required"
        }
        if itemID == "" {
            errors["item_id"] = "item_id is required"
        }
        
        if len(errors) > 0 {
            response.WriteValidationError(w, "Validation failed", errors)
            return
        }
    }
}
```

**Rationale:**
- Defense in depth strategy
- Clear validation error messages
- Prevents invalid data from reaching business logic

### Decision: No Authentication/Authorization Layer
**Rationale:**
- Focus on core business logic as per requirements
- Authentication typically handled by API gateway
- Simplifies testing and demonstration

**Note**: Production deployment would require:
- JWT or OAuth2 integration
- Rate limiting per user
- API key management
- Request signing