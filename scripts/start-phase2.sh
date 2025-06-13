#!/bin/bash

echo "Starting Strangler Pattern Demo - Phase 2"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Phase 2: Dual Write Pattern${NC}"
echo "- Proxy writes to BOTH new Order Service AND SAP"
echo "- PostgreSQL database for persistent storage"
echo "- Kafka for event streaming"
echo "- Backward compatibility maintained"
echo ""

echo "1. Pulling latest images..."
docker-compose pull

echo ""
echo "2. Starting services (this may take a few minutes)..."
docker-compose up -d

echo ""
echo "3. Waiting for services to be ready..."

# Wait for database
echo -n "Waiting for PostgreSQL... "
until docker-compose exec -T postgres pg_isready -U orderservice > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}✓${NC}"

# Wait for Kafka
echo -n "Waiting for Kafka... "
until docker-compose exec -T kafka kafka-topics --bootstrap-server localhost:9092 --list > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}✓${NC}"

# Wait for services
echo -n "Waiting for Order Service... "
until curl -s http://localhost:8081/health > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}✓${NC}"

echo -n "Waiting for SAP Mock... "
until curl -s http://localhost:8082/health > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}✓${NC}"

echo -n "Waiting for Proxy... "
until curl -s http://localhost:8080/health > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}✓${NC}"

echo ""
echo -e "${GREEN}All services are ready!${NC}"
echo ""

echo "Service URLs:"
echo "============="
echo "• Proxy Service:     http://localhost:8080"
echo "• Order Service:     http://localhost:8081"
echo "• SAP Mock:          http://localhost:8082"
echo "• Kafka UI:          http://localhost:8090"
echo ""

echo "Quick Commands:"
echo "==============="
echo "• Test Phase 2:      ./scripts/test-phase2.sh"
echo "• View logs:         docker-compose logs -f [service]"
echo "• Stop services:     docker-compose down"
echo ""

echo -e "${YELLOW}Ready to test Phase 2 of the Strangler Pattern!${NC}"
echo "Run './scripts/test-phase2.sh' to start testing."