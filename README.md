# Strangler Pattern Demo

A comprehensive demonstration of the strangler pattern for gradually migrating from a monolithic SAP system to event-driven microservices architecture.

## Overview

This project demonstrates how to implement the strangler pattern by building a proxy service that sits between an e-commerce system and an SAP backend. The proxy gradually takes over functionality while maintaining backward compatibility.

## Current Implementation Status

✅ **Phase 1 Complete**: Basic proxy that passes requests to SAP with logging  
✅ **Phase 2 Complete**: Dual-write pattern with new order service, PostgreSQL, and Kafka events

## Architecture

**Phase 2: Dual Write Pattern**

```
                    ┌─────────────────┐
                    │   PostgreSQL    │
                    │    Database     │
                    └─────────────────┘
                             ▲
                             │
┌─────────────┐    ┌─────────┴───────┐    ┌─────────────┐
│  E-commerce │───▶│     Proxy       │───▶│  SAP Mock   │
│   System    │    │    Service      │    │   Service   │
└─────────────┘    └─────────┬───────┘    └─────────────┘
                             │
                             ▼
                    ┌─────────────────┐    ┌─────────────┐
                    │ Order Service   │───▶│    Kafka    │
                    │ (Port 8081)     │    │   Events    │
                    └─────────────────┘    └─────────────┘
```

## Tech Stack

- **Go 1.21+** - Main programming language
- **PostgreSQL** - Order service database
- **Kafka** - Event streaming platform
- **Gorilla Mux** - HTTP routing
- **Logrus** - Structured logging
- **Docker & Docker Compose** - Containerization
- **Sarama** - Kafka client library

## Project Structure

```
strangler-demo/
├── cmd/
│   ├── proxy/          # Main proxy service
│   ├── order-service/  # New order microservice  
│   └── sap-mock/       # Mock SAP service
├── internal/
│   ├── orders/         # Order handling logic
│   ├── events/         # Kafka event publishing
│   └── sap/            # SAP client integration
├── pkg/
│   └── models/         # Shared data models
├── scripts/            # Test and demo scripts
└── docker-compose.yml  # Service orchestration
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for local development)
- jq (for JSON formatting in scripts)

### 1. Start All Services

```bash
# Start the complete Phase 2 infrastructure
./scripts/start-phase2.sh
```

Or manually:
```bash
docker-compose up --build
```

### 2. Test the Implementation

```bash
# Basic functionality test
./scripts/test-order.sh

# Phase 2 comprehensive test
./scripts/test-phase2.sh

# Data comparison verification
./scripts/compare-data.sh

# Interactive demonstration
./scripts/demo-comparison.sh
```

### 3. Load Testing

```bash
# Light load test (development)
./scripts/advanced-load-test.sh -s light

# Medium load test (integration)  
./scripts/advanced-load-test.sh -s medium

# Performance benchmark
./scripts/performance-benchmark.sh
```

## Service Endpoints

| Service | Port | Description | Health Check |
|---------|------|-------------|--------------|
| Proxy | 8080 | Main API endpoint | `GET /health` |
| Order Service | 8081 | New microservice | `GET /health` |
| SAP Mock | 8082 | Legacy system simulation | `GET /health` |
| Kafka UI | 8090 | Event monitoring | Web interface |
| PostgreSQL | 5432 | Database | Internal |

## Key Features

### ✅ Dual-Write Pattern
- Orders written to **both** Order Service (PostgreSQL) and SAP Mock
- Graceful degradation if one system fails
- Maintains backward compatibility
- Real-time data consistency verification

### ✅ Event Streaming
- Kafka events published for all order operations
- `order.created` events with full order details
- Event monitoring via Kafka UI

### ✅ Data Comparison
- Real-time consistency verification between systems
- Individual order comparison endpoints
- Automated data consistency testing

### ✅ Performance Monitoring
- Comprehensive load testing suite
- Performance comparison between systems
- Detailed metrics and reporting

## API Documentation

### Core Order API

**Create Order**: `POST /orders`
```json
{
  "customer_id": "CUST-12345",
  "items": [{
    "product_id": "WIDGET-001",
    "quantity": 10,
    "unit_price": 25.99,
    "specifications": {
      "color": "blue",
      "finish": "matte"
    }
  }],
  "total_amount": 259.90,
  "delivery_date": "2025-06-20T00:00:00Z"
}
```

### Data Comparison API (Phase 2)

**Compare All Orders**: `GET /compare/orders`
- Returns comprehensive comparison between Order Service and SAP Mock
- Includes sync status, missing orders, and detailed analysis

**Compare Specific Order**: `GET /compare/orders/{id}`
- Field-by-field comparison of individual orders
- Perfect match verification

### Service-Specific APIs

**Order Service**: `GET /orders` - List all orders from PostgreSQL  
**SAP Mock**: `GET /orders` - List all orders from in-memory storage

For complete API documentation, see [API.md](API.md).

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | 8080 | Proxy service port |
| `ORDER_SERVICE_PORT` | 8081 | Order service port |
| `SAP_URL` | http://sap-mock:8082 | SAP service endpoint |
| `ORDER_SERVICE_URL` | http://order-service:8081 | Order service endpoint |
| `DB_HOST` | postgres | Database host |
| `KAFKA_BROKERS` | kafka:29092 | Kafka broker list |

## Testing & Verification

### Automated Testing
```bash
# Complete verification suite
./scripts/compare-data.sh --create-test-data

# Performance testing
./scripts/load-test.sh 100 20  # 100 orders, 20 concurrent

# Advanced scenarios
./scripts/advanced-load-test.sh -s heavy -d -r
```

### Manual Verification
```bash
# Create an order
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "TEST", "items": [{"product_id": "TEST-1", "quantity": 1, "unit_price": 10.00}], "total_amount": 10.00, "delivery_date": "2025-07-01T00:00:00Z"}'

# Verify in both systems
curl http://localhost:8081/orders | jq .  # Order Service
curl http://localhost:8082/orders | jq .  # SAP Mock

# Check consistency
curl http://localhost:8080/compare/orders | jq .analysis.sync_status
```

## Monitoring & Observability

### Logging
- Structured JSON logging with Logrus
- Request/response timing
- Error tracking and categorization
- Performance metrics

### Metrics
- Order creation response times
- System-specific performance measurements
- Data consistency verification results
- Load testing analytics

### Event Monitoring
- Kafka UI: http://localhost:8090
- Real-time event stream visualization
- Topic and partition monitoring

## Development

### Local Development
```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build all services
go build ./cmd/proxy
go build ./cmd/order-service  
go build ./cmd/sap-mock
```

### Docker Development
```bash
# Rebuild specific service
docker-compose build proxy
docker-compose up -d proxy

# View logs
docker-compose logs -f proxy
docker-compose logs -f order-service
```

## Troubleshooting

### Common Issues

**Services not starting:**
```bash
# Check service status
docker-compose ps

# Check logs
docker-compose logs [service-name]

# Restart all services
docker-compose restart
```

**Performance issues:**
```bash
# Check resource usage
docker stats

# Check service health
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health
```

**Data inconsistency:**
```bash
# Run comparison check
./scripts/compare-data.sh

# Check individual order
curl http://localhost:8080/compare/orders/{order-id}
```

For detailed troubleshooting, see [LOAD-TESTING.md](LOAD-TESTING.md).

## Documentation

- **[API.md](API.md)** - Complete API documentation
- **[DOCKER.md](DOCKER.md)** - Docker configuration and deployment
- **[LOAD-TESTING.md](LOAD-TESTING.md)** - Comprehensive load testing guide
- **[CLAUDE.md](CLAUDE.md)** - Development methodology and guidelines

## Implementation Phases

### ✅ Phase 1: Basic Proxy (Complete)
- HTTP proxy between e-commerce and SAP
- Request/response logging
- Basic health checks

### ✅ Phase 2: Dual Write (Complete)
- Order Service with PostgreSQL database
- Kafka event publishing
- Dual-write to both systems
- Data consistency verification
- Comprehensive load testing

### 🔄 Phase 3: Event-Driven (Next)
- SAP consumes events from Kafka
- Remove direct SAP calls from proxy
- Complete strangler pattern implementation

## Success Metrics

### Performance
- **Order Service**: ~50-200ms response time
- **Dual-write**: ~1500ms total (includes SAP simulation)
- **Throughput**: 20+ orders/second under load

### Reliability
- **Success Rate**: >95% under normal load
- **Data Consistency**: 100% synchronization between systems
- **Zero Data Loss**: All successful orders in both systems

### Migration Confidence
- ✅ Proven data consistency under load
- ✅ Performance monitoring and comparison
- ✅ Graceful degradation capabilities
- ✅ Ready for Phase 3 implementation

## Contributing

This is a demonstration project showcasing strangler pattern implementation. Feel free to:

- Fork and experiment with different approaches
- Add new features or testing scenarios
- Improve performance optimizations
- Extend to additional services

## License

MIT License - See LICENSE file for details

---

**Ready to see the strangler pattern in action?**

```bash
./scripts/start-phase2.sh
./scripts/demo-comparison.sh
```