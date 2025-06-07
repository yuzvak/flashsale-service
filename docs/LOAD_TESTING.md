# Load Testing Strategy & Results

## Overview

The flash sale service includes comprehensive load testing capabilities designed to validate system performance under high concurrency scenarios typical of flash sales.

## Testing Framework

### Load Test Configuration
```go
type LoadTestConfig struct {
    BaseURL             string
    ConcurrentUsers     int
    TestDurationSeconds int
    RampUpSeconds       int
    ItemCount           int
}
```

### Test Profiles

#### Standard Load Test
- **Light**: 50 users, 30 seconds
- **Medium**: 100 users, 60 seconds  
- **Heavy**: 500 users, 300 seconds
- **Stress**: 1000 users, 600 seconds

#### Realistic Load Test
- **User Distribution**: 
  - 10% Aggressive buyers (90% checkout, 80% purchase probability)
  - 60% Normal buyers (60% checkout, 40% purchase probability)
  - 30% Browsers (30% checkout, 10% purchase probability)

## Test Scenarios

### 1. Concurrent Checkout Testing
- Multiple users attempting to checkout same items
- Validates race condition handling
- Tests user limit enforcement (10 items per user)

### 2. Purchase Flow Testing
- End-to-end checkout → purchase flow
- Atomic purchase validation
- Sale limit enforcement (10,000 items total)

### 3. Realistic User Behavior
```go
var UserProfiles = []UserBehaviorProfile{
    {
        Name:                "aggressive_buyer",
        CheckoutProbability: 0.9,
        PurchaseProbability: 0.8,
        ItemsPerSession:     3,
        SessionDelay:        100 * time.Millisecond,
        PopularItemBias:     0.7,
    },
    // ... other profiles
}
```

## Key Metrics Tracked

### Performance Metrics
- **Throughput**: Requests per second (RPS)
- **Response Times**: P50, P95, P99 percentiles
- **Error Rates**: Failed vs successful requests
- **Business Metrics**: Checkout/purchase success rates

### Monitoring During Tests
```go
func (lt *LoadTester) monitorProgress(ctx context.Context, startTime time.Time) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Real-time metrics logging
            fmt.Printf("[%s] Total: %d, Success: %d, RPS: %.1f\n",
                elapsed, totalReqs, successReqs, currentRPS)
        }
    }
}
```

## Database Integration Testing

### Realistic Load Tester
- Direct database connectivity for item distribution
- Popular vs normal item bias simulation
- Periodic item availability updates

```go
func (rlt *RealisticLoadTester) LoadItemsFromDB() error {
    // Load available items directly from database
    rows, err := rlt.db.Query(`
        SELECT id FROM items 
        WHERE sale_id = $1 AND sold = FALSE 
        ORDER BY created_at
    `, saleID)
    // Process items with popularity bias
}
```

## Test Execution

### Makefile Targets
```makefile
load-test:          # Standard load test
load-test-light:    # Light load test  
load-test-heavy:    # Heavy load test
load-test-stress:   # Stress test
realistic-test:     # Realistic user behavior test
```

### Result Persistence
- JSON format result files with timestamps
- Performance metrics preservation
- Error categorization and analysis

## Expected Performance Characteristics

### Target Metrics
- **Target RPS**: 10,000+ requests per second
- **P95 Response Time**: < 1 second for checkout
- **P95 Response Time**: < 2 seconds for purchase
- **Error Rate**: < 5% under normal load
- **Checkout Success Rate**: > 95%
- **Purchase Success Rate**: > 90%

## Stress Testing Focus Areas

1. **Race Conditions**: Multiple users, same items
2. **User Limits**: 10 item per user enforcement
3. **Sale Limits**: 10,000 total items enforcement  
4. **Cache Performance**: Redis under high load
5. **Database Transactions**: Atomic operations under pressure
6. **Memory Usage**: Goroutine and connection pool behavior

## Test Infrastructure

### HTTP Client Configuration
```go
client: &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        1000,
        MaxIdleConnsPerHost: 100,
        MaxConnsPerHost:     200,
    },
}
```

### Monitoring Integration
- Real-time progress tracking
- Performance degradation detection
- Resource usage monitoring during tests