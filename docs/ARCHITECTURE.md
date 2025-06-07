# Architecture Overview

## System Design

The flash sale microservice follows **Domain-Driven Design (DDD)** principles with clean architecture separation. Built using Go standard library only, optimized for high-throughput flash sales.

### Core Architecture Layers

```
┌─────────────────────────────────────────┐
│                 HTTP Layer              │
│            (Standard Library)           │
├─────────────────────────────────────────┤
│              Application                │
│         (Commands & Use Cases)          │
├─────────────────────────────────────────┤
│                Domain                   │
│           (Pure Business Logic)         │
├─────────────────────────────────────────┤
│             Infrastructure              │
│        (PostgreSQL, Redis, HTTP)       │
└─────────────────────────────────────────┘
```

## Directory Structure

```
flash-sale-service/
├── cmd/server/                    # Application entry point
├── internal/
│   ├── domain/                    # Pure business logic
│   │   ├── sale/                  # Sale aggregate
│   │   ├── user/                  # User limits
│   │   └── errors/                # Domain errors
│   ├── application/               # Use cases & commands
│   │   ├── commands/              # Command handlers
│   │   ├── use_cases/             # Business workflows
│   │   └── ports/                 # Interface definitions
│   ├── infrastructure/            # External concerns
│   │   ├── persistence/           # Data layer
│   │   ├── http/                  # Web layer
│   │   ├── monitoring/            # Observability
│   │   └── scheduler/             # Background jobs
│   └── pkg/                       # Shared utilities
├── migrations/                    # Database schemas
└── monitoring/                    # Observability stack
```

## Key Design Principles

### 1. Domain-Driven Design
- **Pure domain logic** with no external dependencies
- **Aggregate roots** (Sale, Item, Checkout) enforce business rules
- **Value objects** for user limits and purchase results
- **Domain services** for complex business operations

### 2. Hexagonal Architecture
- **Ports & Adapters** pattern for testability
- **Repository interfaces** in application layer
- **Concrete implementations** in infrastructure layer

### 3. CQRS Pattern
- **Commands** for state-changing operations
- **Separate read/write models** for optimization
- **Event-driven** state updates via Redis

## Core Components

### Sale Aggregate
```go
type Sale struct {
    ID         string    // Format: YYYYMMDDHH  
    StartedAt  time.Time
    EndedAt    time.Time
    TotalItems int       // Always 10,000
    ItemsSold  int       // Atomic counter
}
```

### Purchase Service
- Validates business rules (limits, timing)
- Coordinates atomic operations
- Calculates purchase results

### Repository Pattern
- Abstracted data access via interfaces
- Transaction support for consistency
- Prepared statements for performance

## Data Flow

```
HTTP Request → Handler → Command → Use Case → Domain Service → Repository
     ↓
Domain Events → Cache Updates → Monitoring
```

## Concurrency Model

- **Optimistic locking** via conditional database updates
- **Redis Lua scripts** for atomic counters
- **Distributed locks** for critical sections
- **Connection pooling** (100 DB, 200 Redis connections)

## Dependencies

**Minimal external dependencies:**
- `lib/pq` - PostgreSQL driver
- `go-redis/v9` - Redis client  
- `prometheus/client_golang` - Metrics
- Standard library for everything else

## Deployment

- **Single binary** deployment
- **Docker containers** with multi-stage builds
- **Docker Compose** for local development
- **Health checks** and graceful shutdown