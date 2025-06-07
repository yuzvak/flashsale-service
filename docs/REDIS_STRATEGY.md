# Redis Strategy

## Overview

Redis serves as the primary cache layer and distributed coordination system for high-performance operations, handling atomic operations, bloom filters, and distributed locking.

## Key Design Patterns

### 1. Atomic Operations with Lua Scripts

**Purchase Validation Script**
```lua
-- Atomic purchase check with user and sale limits
local sale_key = KEYS[1]     -- sale:{sale_id}:items_sold
local user_key = KEYS[2]     -- user:{user_id}:sale:{sale_id}:count
local item_count = ARGV[1]
local max_sale_items = ARGV[2]
local max_user_items = ARGV[3]

-- Check and increment counters atomically
if current_sale_count + item_count > max_sale_items then
    return 0  -- Sale limit exceeded
end
if current_user_count + item_count > max_user_items then
    return 0  -- User limit exceeded
end

-- Increment both counters
redis.call('INCRBY', sale_key, item_count)
redis.call('INCRBY', user_key, item_count)
return 1  -- Success
```

### 2. Key Patterns

**Counters**
- `sale:{sale_id}:items_sold` - Total items sold in sale
- `user:{user_id}:sale:{sale_id}:count` - Items purchased by user

**Checkout Management**
- `user:{user_id}:sale:{sale_id}:checkout` - User's checkout code
- `checkout:{code}` - Checkout code validation
- `user:{user_id}:sale:{sale_id}:checked_items` - Set of checked out items

**Bloom Filter**
- `bloom:sold_items` - Fast rejection of sold items
- Parameters: 100,000 items, 0.01 false positive rate

**Distributed Locks**
- `lock:{key}` - Purchase operation locks with 3-second timeout

### 3. Connection Pooling

```go
// Redis connection configuration
PoolSize: 100
MaxIdleConns: 50
Timeout: 30 * time.Second
```

## Performance Optimizations

### 1. Bloom Filter Integration
- **Purpose**: Fast rejection of sold items before database queries
- **Update Frequency**: Background process every 10 seconds
- **Size**: 128KB for 10,000 items with 0.1% false positive rate

### 2. Pipeline Operations
- Batch multiple Redis commands for reduced network round trips
- Used in checkout operations and counter updates

### 3. TTL Strategy
- All keys expire 1 hour after sale ends
- Automatic cleanup prevents memory leaks
- Checkout codes expire with sale end time

## Error Handling

### 1. Graceful Degradation
- Bloom filter failures don't block operations
- Cache misses fall back to database queries
- Lock timeouts return user-friendly errors

### 2. Retry Logic
- Connection failures: 3 retries with exponential backoff
- Lock acquisition: Single attempt with clear error message
- Counter operations: Atomic with rollback on failure

## Monitoring

### 1. Metrics Tracked
- `redis_command_duration_seconds` - Command latency
- `redis_lock_attempts_total` - Lock operation attempts
- `redis_lock_success_total` - Successful lock acquisitions
- `redis_lock_failure_total` - Failed lock attempts with reasons

### 2. Health Checks
- Connection verification in `/health` endpoint
- Pool statistics monitoring
- Command success rate tracking

## Failover Strategy

### 1. Single Instance Design
- Current implementation uses single Redis instance
- Suitable for contest requirements and development
- For production: Redis Sentinel or Cluster recommended

### 2. Recovery Procedures
- Bloom filter rebuilds automatically from PostgreSQL
- Counter synchronization from database on startup
- Checkout code regeneration on cache miss