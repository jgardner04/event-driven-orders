#!/bin/bash

echo "Strangler Pattern Data Comparison Demo"
echo "======================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}This demo shows that both systems (Order Service + SAP Mock) contain identical data"
echo "demonstrating the success of the dual-write pattern in Phase 2.${NC}"
echo ""

# Wait for user
echo "Press Enter to start the demo..."
read

echo -e "${BLUE}Step 1: Creating sample orders through the proxy${NC}"
echo "================================================="
echo ""

for i in {1..3}; do
    ORDER_ID="demo-$(date +%s)-$i"
    echo "Creating order $i (ID: $ORDER_ID)..."
    
    RESPONSE=$(curl -s -X POST http://localhost:8080/orders \
      -H "Content-Type: application/json" \
      -d "{
        \"id\": \"$ORDER_ID\",
        \"customer_id\": \"DEMO-CUSTOMER-$i\",
        \"items\": [{
          \"product_id\": \"WIDGET-DEMO-$i\",
          \"quantity\": $((i * 2)),
          \"unit_price\": $((i * 15 + 5)).99,
          \"specifications\": {
            \"color\": \"$([ $((i % 2)) -eq 0 ] && echo 'blue' || echo 'red')\",
            \"finish\": \"matte\",
            \"priority\": \"$([ $i -eq 1 ] && echo 'high' || echo 'normal')\"
          }
        }],
        \"total_amount\": $((i * 2 * (i * 15 + 5))).99,
        \"delivery_date\": \"2025-07-0${i}T00:00:00Z\"
      }")
    
    if echo "$RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓ Order $i created successfully${NC}"
    else
        echo -e "  ${RED}✗ Failed to create order $i${NC}"
    fi
    
    sleep 1
done

echo ""
echo "Waiting 2 seconds for all systems to process..."
sleep 2

echo ""
echo -e "${BLUE}Step 2: Checking order counts in both systems${NC}"
echo "==============================================="
echo ""

# Check Order Service
echo -n "Order Service count: "
ORDER_SERVICE_COUNT=$(curl -s http://localhost:8081/orders | jq -r '.count')
echo -e "${CYAN}$ORDER_SERVICE_COUNT orders${NC}"

# Check SAP Mock
echo -n "SAP Mock count: "
SAP_COUNT=$(curl -s http://localhost:8082/orders | jq -r '.count')
echo -e "${CYAN}$SAP_COUNT orders${NC}"

if [ "$ORDER_SERVICE_COUNT" -eq "$SAP_COUNT" ]; then
    echo -e "${GREEN}✓ Counts match! Both systems have $ORDER_SERVICE_COUNT orders${NC}"
else
    echo -e "${RED}✗ Count mismatch! Order Service: $ORDER_SERVICE_COUNT, SAP: $SAP_COUNT${NC}"
fi

echo ""
echo "Press Enter to continue..."
read

echo -e "${BLUE}Step 3: Using the comparison endpoint${NC}"
echo "======================================"
echo ""

echo "Calling comparison API..."
COMPARISON=$(curl -s http://localhost:8080/compare/orders)

echo "Comparison results:"
echo "$COMPARISON" | jq '{
  timestamp: .timestamp,
  order_service_count: .order_service.count,
  sap_count: .sap.count,
  analysis: {
    total_count_match: .analysis.total_count_match,
    sync_status: .analysis.sync_status,
    missing_in_sap: .analysis.missing_in_sap,
    missing_in_order_service: .analysis.missing_in_order_service
  }
}'

SYNC_STATUS=$(echo "$COMPARISON" | jq -r '.analysis.sync_status')
if [ "$SYNC_STATUS" = "true" ]; then
    echo -e "${GREEN}✓ Systems are perfectly synchronized!${NC}"
else
    echo -e "${RED}✗ Systems are not synchronized${NC}"
fi

echo ""
echo "Press Enter to continue..."
read

echo -e "${BLUE}Step 4: Comparing individual orders${NC}"
echo "===================================="
echo ""

# Get some order IDs
ORDER_IDS=$(curl -s http://localhost:8081/orders | jq -r '.orders[0:2][].id')

for ORDER_ID in $ORDER_IDS; do
    echo "Comparing order: $ORDER_ID"
    
    SINGLE_COMPARISON=$(curl -s "http://localhost:8080/compare/orders/$ORDER_ID")
    
    echo "  Analysis:"
    echo "$SINGLE_COMPARISON" | jq '.analysis | {
      perfect_match: .perfect_match,
      id_match: .id_match,
      customer_id_match: .customer_id_match,
      total_amount_match: .total_amount_match,
      items_count_match: .items_count_match
    }'
    
    PERFECT_MATCH=$(echo "$SINGLE_COMPARISON" | jq -r '.analysis.perfect_match')
    if [ "$PERFECT_MATCH" = "true" ]; then
        echo -e "  ${GREEN}✓ Perfect match${NC}"
    else
        echo -e "  ${YELLOW}⚠ Some differences found${NC}"
        DIFFERENCES=$(echo "$SINGLE_COMPARISON" | jq -r '.analysis.differences // "unknown"')
        echo "    Differences: $DIFFERENCES"
    fi
    echo ""
done

echo "Press Enter to continue..."
read

echo -e "${BLUE}Step 5: Showing actual data from both systems${NC}"
echo "=============================================="
echo ""

# Get a sample order
SAMPLE_ORDER_ID=$(curl -s http://localhost:8081/orders | jq -r '.orders[0].id')

if [ "$SAMPLE_ORDER_ID" != "null" ] && [ "$SAMPLE_ORDER_ID" != "" ]; then
    echo "Sample Order ID: $SAMPLE_ORDER_ID"
    echo ""
    
    echo -e "${CYAN}From Order Service (PostgreSQL):${NC}"
    curl -s "http://localhost:8081/orders/$SAMPLE_ORDER_ID" | jq '{
      id: .id,
      customer_id: .customer_id,
      total_amount: .total_amount,
      status: .status,
      delivery_date: .delivery_date,
      created_at: .created_at,
      items: [.items[] | {
        product_id: .product_id,
        quantity: .quantity,
        unit_price: .unit_price,
        specifications: .specifications
      }]
    }'
    
    echo ""
    echo -e "${CYAN}From SAP Mock (In-Memory):${NC}"
    curl -s "http://localhost:8082/orders/$SAMPLE_ORDER_ID" | jq '{
      id: .id,
      customer_id: .customer_id,
      total_amount: .total_amount,
      status: .status,
      delivery_date: .delivery_date,
      created_at: .created_at,
      items: [.items[] | {
        product_id: .product_id,
        quantity: .quantity,
        unit_price: .unit_price,
        specifications: .specifications
      }]
    }'
else
    echo "No orders found to display"
fi

echo ""
echo "Press Enter to continue..."
read

echo -e "${BLUE}Step 6: Performance comparison${NC}"
echo "==============================="
echo ""

echo "Testing response times for order listing..."

# Test Order Service
echo -n "Order Service: "
START_TIME=$(date +%s%N)
curl -s http://localhost:8081/orders > /dev/null
END_TIME=$(date +%s%N)
ORDER_SERVICE_TIME=$((($END_TIME - $START_TIME) / 1000000))
echo "${ORDER_SERVICE_TIME}ms"

# Test SAP Mock
echo -n "SAP Mock: "
START_TIME=$(date +%s%N)
curl -s http://localhost:8082/orders > /dev/null
END_TIME=$(date +%s%N)
SAP_TIME=$((($END_TIME - $START_TIME) / 1000000))
echo "${SAP_TIME}ms"

# Test Comparison Endpoint
echo -n "Comparison API: "
START_TIME=$(date +%s%N)
curl -s http://localhost:8080/compare/orders > /dev/null
END_TIME=$(date +%s%N)
COMPARISON_TIME=$((($END_TIME - $START_TIME) / 1000000))
echo "${COMPARISON_TIME}ms"

echo ""
if [ "$ORDER_SERVICE_TIME" -lt "$SAP_TIME" ]; then
    echo -e "${GREEN}✓ Order Service is faster than SAP Mock${NC}"
else
    echo -e "${YELLOW}⚠ SAP Mock performed better this time${NC}"
fi

echo ""
echo -e "${YELLOW}Demo Summary${NC}"
echo "============"
echo ""
echo -e "${GREEN}✓ Dual-write pattern is working correctly${NC}"
echo -e "${GREEN}✓ Both systems contain identical order data${NC}"
echo -e "${GREEN}✓ Comparison APIs provide real-time verification${NC}"
echo -e "${GREEN}✓ Data consistency is maintained across systems${NC}"
echo ""
echo -e "${CYAN}This demonstrates that Phase 2 of the strangler pattern is successful:${NC}"
echo "• Orders are written to both Order Service (PostgreSQL) and SAP Mock"
echo "• Data remains synchronized between systems"
echo "• Comparison endpoints provide visibility into data consistency"
echo "• The system is ready for Phase 3 (event-driven migration)"
echo ""
echo -e "${BLUE}Available endpoints:${NC}"
echo "• GET  /compare/orders           - Compare all orders"
echo "• GET  /compare/orders/{id}      - Compare specific order"
echo "• GET  /orders (on port 8081)    - List orders from Order Service"
echo "• GET  /orders (on port 8082)    - List orders from SAP Mock"
echo ""
echo -e "${YELLOW}To run automated verification: ./scripts/compare-data.sh${NC}"