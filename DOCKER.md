# Docker Configuration Guide

## Overview

This project uses Docker and Docker Compose to orchestrate the services required for the strangler pattern demo.

## Services

### 1. Proxy Service

The main proxy service that forwards requests to the Order Service (Phase 3).

**Build Configuration**:
- **Context**: Project root directory
- **Dockerfile**: `cmd/proxy/Dockerfile`
- **Port**: 8080 (host) → 8080 (container)
- **Dependencies**: order-service

**Environment Variables**:
- `PROXY_PORT`: Port for the proxy service (default: 8080)
- `ORDER_SERVICE_URL`: URL of the Order Service (default: http://order-service:8081)

### 2. SAP Mock Service

Simulates the legacy SAP system. In Phase 3, consumes orders from Kafka events.

**Build Configuration**:
- **Context**: Project root directory
- **Dockerfile**: `cmd/sap-mock/Dockerfile`
- **Port**: 8082 (host) → 8082 (container)
- **Dependencies**: kafka

**Environment Variables**:
- `SAP_PORT`: Port for the SAP service (default: 8082)
- `KAFKA_BROKERS`: Kafka broker addresses (default: kafka:29092)

### 3. Order Service

The new microservice that handles order processing and publishes events to Kafka.

**Build Configuration**:
- **Context**: Project root directory
- **Dockerfile**: `cmd/order-service/Dockerfile`
- **Port**: 8081 (host) → 8081 (container)
- **Dependencies**: postgres, kafka

**Environment Variables**:
- `ORDER_SERVICE_PORT`: Service port (default: 8081)
- `DB_HOST`: PostgreSQL host (default: postgres)
- `DB_NAME`: Database name (default: orderservice)
- `KAFKA_BROKERS`: Kafka broker addresses (default: kafka:29092)

### 4. PostgreSQL Database

Stores orders for the Order Service.

**Configuration**:
- **Image**: postgres:15-alpine
- **Port**: 5432 (host) → 5432 (container)
- **Database**: orderservice
- **Init Script**: `scripts/db/init.sql`

### 5. Kafka & Zookeeper

Event streaming platform for asynchronous communication.

**Kafka Configuration**:
- **Port**: 9092 (host) → 9092 (container)
- **Topic**: order.created (auto-created)

**Kafka UI**:
- **Port**: 8090 (host) → 8080 (container)
- **URL**: http://localhost:8090

## Network Configuration

All services communicate through a custom bridge network named `strangler-net`. This allows:
- Service discovery by container name
- Network isolation from other Docker containers
- Inter-service communication without exposing all ports

## Phase 3 Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│     Proxy       │────▶│  Order Service  │────▶│   PostgreSQL    │
│   Port 8080     │     │   Port 8081     │     │   Port 5432     │
│                 │     │                 │     │                 │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 │ Publishes
                                 ▼
                        ┌─────────────────┐
                        │                 │
                        │     Kafka       │
                        │   Port 9092     │
                        │                 │
                        └────────┬────────┘
                                 │
                                 │ Consumes
                                 ▼
                        ┌─────────────────┐
                        │                 │
                        │   SAP Mock      │
                        │   Port 8082     │
                        │                 │
                        └─────────────────┘
```

## Quick Start Commands

### Start All Services

```bash
# Start with build
docker-compose up --build

# Start in background
docker-compose up -d

# Start with forced rebuild
docker-compose up --build --force-recreate
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f proxy
docker-compose logs -f sap-mock

# Last 100 lines
docker-compose logs --tail=100
```

### Stop Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Stop and remove images
docker-compose down --rmi all
```

### Service Management

```bash
# Check service status
docker-compose ps

# Restart a service
docker-compose restart proxy

# Stop a specific service
docker-compose stop sap-mock

# Start a stopped service
docker-compose start sap-mock
```

## Dockerfile Details

### Proxy Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o proxy cmd/proxy/main.go

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/proxy .
EXPOSE 8080
CMD ["./proxy"]
```

### SAP Mock Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o sap-mock cmd/sap-mock/main.go

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/sap-mock .
EXPOSE 8082
CMD ["./sap-mock"]
```

## Development Workflow

### Local Development with Docker

1. **Make code changes**
2. **Rebuild specific service**:
   ```bash
   docker-compose build proxy
   ```
3. **Restart the service**:
   ```bash
   docker-compose up -d proxy
   ```

### Debugging

**Access container shell**:
```bash
docker-compose exec proxy /bin/sh
docker-compose exec sap-mock /bin/sh
```

**View real-time logs**:
```bash
docker-compose logs -f proxy | jq .
```

**Check service health**:
```bash
# From host
curl http://localhost:8080/health | jq .

# From within network
docker-compose exec proxy wget -qO- http://sap-mock:8082/health
```

## Environment Overrides

Create a `.env` file in the project root to override default values:

```env
# .env
PROXY_PORT=9090
SAP_URL=http://sap-mock:8082
```

Or use environment-specific compose files:

```bash
# Development
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Production
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up
```

## Troubleshooting

### Container Won't Start

1. Check logs:
   ```bash
   docker-compose logs proxy
   ```

2. Verify build:
   ```bash
   docker-compose build --no-cache proxy
   ```

3. Check port availability:
   ```bash
   lsof -i :8080
   ```

### Network Issues

1. Verify network exists:
   ```bash
   docker network ls | grep strangler-net
   ```

2. Inspect network:
   ```bash
   docker network inspect strangler-demo_strangler-net
   ```

3. Test connectivity:
   ```bash
   docker-compose exec proxy ping sap-mock
   ```

### Performance Issues

1. Check resource usage:
   ```bash
   docker stats
   ```

2. Increase resources in Docker Desktop settings

3. Use production builds:
   ```bash
   CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo
   ```

## Best Practices

1. **Multi-stage builds**: Reduces image size by separating build and runtime
2. **Alpine Linux**: Minimal base image for security and size
3. **Health checks**: Add Docker health checks for better orchestration
4. **Resource limits**: Set memory and CPU limits in production
5. **Named volumes**: Use for persistent data in future phases

## Future Enhancements

When adding Kafka and PostgreSQL in Phase 2:

```yaml
services:
  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
    networks:
      - strangler-net

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: orders
      POSTGRES_USER: orderservice
      POSTGRES_PASSWORD: changeme
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - strangler-net

volumes:
  postgres-data:
```