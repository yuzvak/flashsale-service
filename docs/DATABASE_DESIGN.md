# Database Design & Schema

## Overview

The flash sale service uses PostgreSQL with a normalized schema designed for high concurrency and data consistency. The design prioritizes atomic operations and performance under heavy load.

## Core Schema

### Sales Table
```sql
CREATE TABLE sales (
    id VARCHAR(20) PRIMARY KEY,           -- Format: S-{uuid}
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP NOT NULL,
    total_items INTEGER NOT NULL DEFAULT 10000,
    items_sold INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sales_active ON sales(ended_at);
```

**Design Rationale:**
- `id` uses UUID format for uniqueness across distributed systems
- `items_sold` tracks current progress for quick limit checking
- Active sale index enables fast lookup of current sale

### Items Table
```sql
CREATE TABLE items (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    image_url VARCHAR(500) NOT NULL,
    sold BOOLEAN NOT NULL DEFAULT FALSE,
    sold_to_user_id VARCHAR(255),
    sold_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Performance indexes
CREATE INDEX idx_items_sale_sold ON items(sale_id) WHERE sold = FALSE;
CREATE INDEX idx_items_sale_user ON items(sale_id, sold_to_user_id) WHERE sold = TRUE;
CREATE INDEX idx_items_sold_at ON items(sold_at) WHERE sold = TRUE;
```

**Design Rationale:**
- Partial indexes for performance (only unsold items, only sold items)
- `sold_to_user_id` enables user purchase tracking
- `sold_at` provides audit trail

### Checkout System (Normalized)

#### Checkout Attempts
```sql
CREATE TABLE checkout_attempts (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    checkout_code VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_checkout_user_sale ON checkout_attempts(sale_id, user_id);
CREATE INDEX idx_checkout_code ON checkout_attempts(checkout_code);
```

#### Checkout Items (Many-to-Many)
```sql
CREATE TABLE checkout_items (
    id VARCHAR(255) PRIMARY KEY,
    checkout_attempt_id VARCHAR(255) NOT NULL REFERENCES checkout_attempts(id) ON DELETE CASCADE,
    item_id VARCHAR(255) NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (item_id) REFERENCES items(id),
    UNIQUE(checkout_attempt_id, item_id)
);

CREATE INDEX idx_checkout_items_attempt ON checkout_items(checkout_attempt_id);
CREATE INDEX idx_checkout_items_item ON checkout_items(item_id);
```

**Design Rationale:**
- Normalized structure avoids JSONB for better query performance
- Unique constraint prevents duplicate items in same checkout
- Foreign key constraints ensure data integrity

### Purchase Records
```sql
CREATE TABLE purchases (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    checkout_code VARCHAR(64) NOT NULL,
    purchased_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_purchases_user_sale ON purchases(user_id, sale_id);
CREATE INDEX idx_purchases_sale ON purchases(sale_id);
```

### Idempotency Support
```sql
CREATE TABLE purchase_results (
    checkout_code VARCHAR(64) PRIMARY KEY,
    result JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_purchase_results_created ON purchase_results(created_at);
```

**Design Rationale:**
- Prevents duplicate purchase processing
- JSONB stores complete purchase result for client response
- TTL-like cleanup via created_at index

## Transaction Patterns

### Atomic Item Purchase
```go
func (r *SaleRepository) MarkItemAsSold(ctx context.Context, id string, userID string) (bool, error) {
    query := `
        UPDATE items
        SET sold = TRUE, sold_to_user_id = $2, sold_at = NOW()
        WHERE id = $1 AND sold = FALSE
    `
    
    result, err := r.tx.ExecContext(ctx, query, id, userID)
    if err != nil {
        return false, err
    }
    
    rowsAffected, err := result.RowsAffected()
    return rowsAffected > 0, err
}
```

### Transaction Isolation
```go
func (r *SaleRepository) BeginTx(ctx context.Context) (ports.SaleRepository, error) {
    tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable, // Highest isolation for critical operations
    })
    if err != nil {
        return nil, err
    }
    
    return &SaleRepository{db: r.db, tx: tx, isTx: true}, nil
}
```

## Performance Optimizations

### Connection Pooling
```go
func NewConnection(cfg config.DatabaseConfig) (*Connection, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    
    db.SetMaxOpenConns(100)     // Max concurrent connections
    db.SetMaxIdleConns(50)      // Idle connection pool
    db.SetConnMaxLifetime(time.Hour)
    db.SetConnMaxIdleTime(30 * time.Minute)
    
    return &Connection{db: db}, nil
}
```

### Batch Operations
```go
func (r *SaleRepository) CreateItems(ctx context.Context, items []*sale.Item) error {
    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO items (id, sale_id, name, image_url, sold, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, item := range items {
        _, err = stmt.ExecContext(ctx, item.ID, item.SaleID, 
            item.Name, item.ImageURL, item.Sold, item.CreatedAt)
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}
```

### Query Optimization Patterns

#### Conditional Updates
```sql
-- Atomic item marking (prevents race conditions)
UPDATE items 
SET sold = TRUE, sold_to_user_id = $1, sold_at = CURRENT_TIMESTAMP
WHERE id = $2 AND sale_id = $3 AND sold = FALSE
```

#### Partial Indexes
```sql
-- Only index unsold items for performance
CREATE INDEX idx_items_sale_available ON items(sale_id, sold) WHERE sold = FALSE;

-- Only index sold items for analytics
CREATE INDEX idx_items_sale_user ON items(sale_id, sold_to_user_id) WHERE sold = TRUE;
```

## Migration Strategy

### Migration Runner
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
    
    // Apply migrations in order
    files, err := os.ReadDir(cfg.MigrationsPath)
    for _, file := range files {
        if strings.HasSuffix(file.Name(), ".up.sql") {
            if err := applyMigration(db, file.Name()); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

### File Structure
```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
```

## Data Consistency Guarantees

### Race Condition Prevention
- Conditional updates with WHERE clauses
- Serializable transaction isolation
- Optimistic locking patterns

### Referential Integrity
- Foreign key constraints
- Cascade deletes for cleanup
- NOT NULL constraints on critical fields

### Business Rule Enforcement
- Check constraints for valid states
- Partial indexes for performance
- Unique constraints preventing duplicates

## Monitoring Integration

### Query Performance Tracking
```go
func InstrumentQuery(ctx context.Context, db *sql.DB, queryType, table, query string, args ...interface{}) (*sql.Rows, error) {
    end := TimeDBQuery(queryType, table)
    defer end()
    return db.QueryContext(ctx, query, args...)
}
```

### Connection Pool Metrics
```go
func (c *DBMetricsCollector) collectMetrics() {
    stats := c.db.Stats()
    DBConnectionsActive.Set(float64(stats.InUse))
    DBConnectionsIdle.Set(float64(stats.Idle))
}
```

## Backup & Recovery Considerations

### Point-in-Time Recovery
- WAL archiving enabled
- Regular automated backups
- Transaction log retention

### Data Retention Policies
- Sale data retention (configurable)
- Checkout attempt cleanup
- Purchase result archival