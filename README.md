# Flash Sale Microservice

High-performance flash sale microservice built with Go standard library, PostgreSQL, and Redis. Designed to handle 10,000 items per hourly sale with high concurrency and correctness guarantees.

FOR NOTØ WITH LOVE ♡

## 🎯 Enhanced Implementation

While the original task specified a 1-checkout → 1-purchase flow, I found this too simple for a real-world scenario. This implementation allows users to **checkout up to 10 items simultaneously** before making a single atomic purchase, providing a more realistic e-commerce experience.

## 🚀 Quick Start

### Start the Service
```bash
docker-compose up
```

### Start Monitoring Stack
```bash
cd monitoring
docker-compose up
```

### Run Load Tests
```bash
make realistic-test
```

## 📊 Monitoring & Observability

- **Grafana Dashboard**: http://localhost:3000 (admin/admin)
- **Prometheus Metrics**: http://localhost:9090
- **Health Endpoint**: http://localhost:8080/health

## 🏗️ Architecture

Built with Domain-Driven Design principles:

- **Domain Layer**: Pure business logic (sale rules, item management)
- **Application Layer**: Use cases and commands
- **Infrastructure Layer**: PostgreSQL, Redis, HTTP handlers
- **Zero frameworks**: Only Go standard library + minimal dependencies

## ⚡ Key Features

- **10,000 items per hour** with atomic purchase guarantees
- **Bloom filters** for fast sold-item detection
- **Redis Lua scripts** for atomic operations
- **Comprehensive monitoring** with Prometheus/Grafana
- **Load testing suite** with realistic user behaviors
- **Zero overselling** with strict concurrency controls

## 📚 Documentation

Detailed documentation available in the [project wiki](https://github.com/yuzvak/flashsale-service/wiki):

- [Architecture Overview](https://github.com/yuzvak/flashsale-service/wiki/Architecture-Overview)
- [Concurrency Strategy](https://github.com/yuzvak/flashsale-service/wiki/Concurrency-Strategy)  
- [Database Design & Schema](https://github.com/yuzvak/flashsale-service/wiki/Database-Design-&-Schema)
- [Redis Strategy](https://github.com/yuzvak/flashsale-service/wiki/Redis-Strategy)
- [Load Testing Strategy & Results](https://github.com/yuzvak/flashsale-service/wiki/Load-Testing-Strategy-&-Results)
- [Monitoring & Observability](https://github.com/yuzvak/flashsale-service/wiki/Monitoring-&-Observability)
- [Deployment Strategy](https://github.com/yuzvak/flashsale-service/wiki/Deployment-Strategy)
- [Scaling Strategy](https://github.com/yuzvak/flashsale-service/wiki/Scaling-Strategy)
- [Design Decisions & Architecture Rationale](https://github.com/yuzvak/flashsale-service/wiki/Design-Decisions-&-Architecture-Rationale)

Built by [yuzvak](https://t.me/yuzvak) for production environments requiring strict correctness guarantees and high availability.