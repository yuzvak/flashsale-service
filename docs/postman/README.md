# Flash Sale Service - Smart Postman Collection

Comprehensive testing suite for the Flash Sale microservice with automated scenarios, load testing, and performance monitoring.

## 📋 Overview

This Postman collection provides:
- **Functional Testing**: Complete API validation
- **Load Testing**: High-throughput scenarios
- **Performance Monitoring**: Response time tracking
- **Edge Case Testing**: Race conditions, limits, error handling
- **Automated Reporting**: Metrics collection and analysis

## 🚀 Quick Start

### 1. Import Collection & Environment

```bash
# Import the collection
curl -o flash-sale-collection.json https://raw.githubusercontent.com/your-repo/postman/flash-sale-collection.json

# Import the environment
curl -o flash-sale-environment.json https://raw.githubusercontent.com/your-repo/postman/flash-sale-environment.json
```

### 2. Configure Environment

Update the environment variables in Postman:

```json
{
  "base_url": "http://localhost:8080",
  "verbose_logging": "false",
  "load_test_users": "100",
  "performance_threshold_ms": "1000"
}
```

### 3. Start Your Service

```bash
# Using Docker Compose
docker-compose up -d

# Or build and run manually
docker build -t flash-sale-service .
docker run -p 8080:8080 flash-sale-service
```

## 📁 Collection Structure

### 🏥 Health & Infrastructure
- **Health Check**: Verify service status and dependencies
- **Metrics Endpoint**: Check Prometheus metrics availability

### 🛡️ Admin Operations
- **Create New Sale**: Admin endpoint to create sales
- Validates timing and item count
- Automatic sale scheduling

### 📊 Sale Information
- **Get Active Sale**: Retrieve current sale details
- **Get Sale Items**: List available items with pagination
- Auto-populates test item IDs

### 🛒 Checkout Flow
- **Single Item Checkout**: Basic checkout functionality
- **Add Second Item**: Multi-item checkout testing
- **Duplicate Item Prevention**: Error handling validation

### 💳 Purchase Flow
- **Execute Purchase**: Complete purchase with checkout code
- **Duplicate Purchase Prevention**: Idempotency testing
- **Invalid Code Handling**: Error scenario testing

### 🚫 Edge Cases & Limits
- **User Limit Testing**: 10-item per user enforcement
- **Parameter Validation**: Missing/invalid parameters
- **Race Condition Handling**: Concurrent request testing

### ⚡ Load Testing Scenarios
- **Concurrent Checkout**: High-throughput checkout testing
- **Race Condition Simulation**: Multiple users, same item
- **Peak Load Simulation**: Flash sale opening rush

### 📈 Performance Monitoring
- **Response Time Baselines**: Performance benchmarking
- **Throughput Testing**: Requests per second measurement
- **Resource Usage Tracking**: Memory and CPU monitoring

## 📄 License
MIT License - see LICENSE file for details.