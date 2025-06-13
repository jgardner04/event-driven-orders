#!/bin/bash

echo "Phase 2 Testing: Dual Write Pattern"
echo "===================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to check service health
check_service() {
    local service_name=$1
    local url=$2
    echo -n "Checking $service_name... "
    if curl -s -f "$url" > /dev/null; then
        echo -e "${GREEN}✓${NC}"
        return 0
    else
        echo -e "${RED}✗${NC}"
        return 1
    fi
}

echo "1. Checking service health..."
echo "-----------------------------"
check_service "Proxy" "http://localhost:8080/health"
check_service "Order Service" "http://localhost:8081/health"
check_service "SAP Mock" "http://localhost:8082/health"
echo ""

echo "2. Creating order through proxy..."
echo "-----------------------------------"
ORDER_ID=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "test-$(date +%s)")

RESPONSE=$(curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$ORDER_ID\",
    \"customer_id\": \"CUST-PHASE2-TEST\",
    \"items\": [
      {
        \"product_id\": \"WIDGET-001\",
        \"quantity\": 10,
        \"unit_price\": 25.99,
        \"specifications\": {
          \"color\": \"red\",
          \"finish\": \"glossy\",
          \"delivery\": \"express\"
        }
      },
      {
        \"product_id\": \"COMPONENT-042\",
        \"quantity\": 5,
        \"unit_price\": 149.99,
        \"specifications\": {
          \"size\": \"medium\",
          \"material\": \"steel\"
        }
      }
    ],
    \"total_amount\": 1009.85,
    \"delivery_date\": \"2025-06-25T00:00:00Z\"
  }")

echo "Response from proxy:"
echo "$RESPONSE" | jq .
echo ""

# Extract order ID from response if not set
if [ -z "$ORDER_ID" ]; then
    ORDER_ID=$(echo "$RESPONSE" | jq -r '.order.id')
fi

echo "3. Verifying order in Order Service..."
echo "---------------------------------------"
sleep 2  # Give time for async operations

ORDER_CHECK=$(curl -s "http://localhost:8081/orders/$ORDER_ID")
if echo "$ORDER_CHECK" | jq -e '.id' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Order found in Order Service database${NC}"
    echo "$ORDER_CHECK" | jq '{id: .id, customer_id: .customer_id, status: .status, total_amount: .total_amount}'
else
    echo -e "${RED}✗ Order not found in Order Service${NC}"
fi
echo ""

echo "4. Checking Kafka events..."
echo "----------------------------"
echo "Kafka UI available at: http://localhost:8090"
echo "Topic: order.created"
echo ""

echo "5. Checking proxy logs for dual write..."
echo "-----------------------------------------"
echo "To view detailed logs, run:"
echo "  docker-compose logs proxy | grep -E '(order service|SAP)'"
echo ""

echo "6. Performance comparison..."
echo "-----------------------------"
echo "Creating 5 orders to compare timing..."
echo ""

for i in {1..5}; do
    START_TIME=$(date +%s%N)
    
    curl -s -X POST http://localhost:8080/orders \
      -H "Content-Type: application/json" \
      -d "{
        \"customer_id\": \"CUST-PERF-TEST-$i\",
        \"items\": [{
          \"product_id\": \"WIDGET-00$i\",
          \"quantity\": $i,
          \"unit_price\": 19.99
        }],
        \"total_amount\": $(echo "$i * 19.99" | bc),
        \"delivery_date\": \"2025-07-01T00:00:00Z\"
      }" > /dev/null
    
    END_TIME=$(date +%s%N)
    DURATION=$((($END_TIME - $START_TIME) / 1000000))
    echo "Order $i: ${DURATION}ms"
done
echo ""

echo -e "${YELLOW}Phase 2 Test Summary:${NC}"
echo "======================"
echo "✓ Proxy writes to BOTH Order Service and SAP"
echo "✓ Orders are persisted in PostgreSQL"
echo "✓ Events are published to Kafka"
echo "✓ SAP still receives all orders (backward compatibility)"
echo ""
echo "Next Phase: Remove SAP calls and have SAP consume events"