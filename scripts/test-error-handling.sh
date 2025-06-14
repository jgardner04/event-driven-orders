#!/bin/bash

echo "Error Handling and Dead Letter Queue Demo"
echo "========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Function to check service health
check_services() {
    echo -e "${BLUE}Checking service health...${NC}"
    
    local services=("http://localhost:8080/health|Proxy" "http://localhost:8081/health|Order-Service" "http://localhost:8082/health|SAP-Mock")
    
    for service_info in "${services[@]}"; do
        IFS='|' read -r url name <<< "$service_info"
        echo -n "  $name: "
        if curl -s -f "$url" > /dev/null; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗${NC}"
            echo -e "${RED}Error: $name is not healthy. Please start services first.${NC}"
            exit 1
        fi
    done
    echo ""
}

# Function to create a test order
create_order() {
    local customer_id=$1
    local amount=$2
    
    local response=$(curl -s -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d '{
            "customer_id": "'$customer_id'",
            "items": [{
                "product_id": "TEST-PRODUCT",
                "quantity": 1,
                "unit_price": '$amount',
                "specifications": {"test": "error-handling"}
            }],
            "total_amount": '$amount',
            "delivery_date": "2025-08-01T00:00:00Z"
        }')
    
    echo "$response"
}

# Function to check metrics
check_metrics() {
    echo -e "${CYAN}SAP Consumer Metrics:${NC}"
    local metrics=$(curl -s http://localhost:8082/admin/metrics)
    
    echo "  Processed: $(echo "$metrics" | jq -r '.consumer_metrics.processed_count // 0')"
    echo "  Success: $(echo "$metrics" | jq -r '.consumer_metrics.success_count // 0')"
    echo "  Failed: $(echo "$metrics" | jq -r '.consumer_metrics.failure_count // 0')"
    echo "  Retries: $(echo "$metrics" | jq -r '.consumer_metrics.retry_count // 0')"
    echo "  DLQ: $(echo "$metrics" | jq -r '.consumer_metrics.dlq_count // 0')"
    echo ""
}

# Main demo flow
demo_error_handling() {
    echo -e "${PURPLE}Scenario 1: Normal Operation${NC}"
    echo "==============================="
    
    # Check initial metrics
    echo -e "${YELLOW}Initial state:${NC}"
    check_metrics
    
    # Create a normal order
    echo -e "${YELLOW}Creating a normal order...${NC}"
    local order=$(create_order "NORMAL-CUSTOMER" "99.99")
    local order_id=$(echo "$order" | jq -r '.order.id // "unknown"')
    echo "Order created: ${order_id:0:8}..."
    
    # Wait for processing
    echo "Waiting for event processing..."
    sleep 5
    
    # Check metrics
    echo -e "${YELLOW}After normal processing:${NC}"
    check_metrics
    
    echo -e "${PURPLE}Scenario 2: Simulated Failures with Retry${NC}"
    echo "=========================================="
    
    # Enable 50% failure rate
    echo -e "${YELLOW}Setting 50% failure rate...${NC}"
    curl -s -X POST http://localhost:8082/admin/failure-rate \
        -H "Content-Type: application/json" \
        -d '{"failure_rate": 0.5}' | jq .
    echo ""
    
    # Create multiple orders
    echo -e "${YELLOW}Creating 5 orders with 50% failure rate...${NC}"
    for i in {1..5}; do
        echo -n "Order $i: "
        create_order "RETRY-CUSTOMER-$i" "$((50 + i * 10)).00" > /dev/null
        echo -e "${GREEN}✓${NC}"
        sleep 1
    done
    
    # Wait for retries
    echo "Waiting for retries and processing..."
    sleep 15
    
    # Check metrics
    echo -e "${YELLOW}After retry processing:${NC}"
    check_metrics
    
    echo -e "${PURPLE}Scenario 3: Complete Outage (DLQ Test)${NC}"
    echo "======================================="
    
    # Enable outage
    echo -e "${YELLOW}Simulating complete SAP outage...${NC}"
    curl -s -X POST http://localhost:8082/admin/simulate-outage \
        -H "Content-Type: application/json" \
        -d '{"outage": true}' | jq .
    echo ""
    
    # Create orders during outage
    echo -e "${YELLOW}Creating 3 orders during outage...${NC}"
    for i in {1..3}; do
        echo -n "Order $i: "
        create_order "DLQ-CUSTOMER-$i" "$((100 + i * 25)).00" > /dev/null
        echo -e "${GREEN}✓${NC}"
        sleep 1
    done
    
    # Wait for DLQ processing
    echo "Waiting for messages to be sent to DLQ..."
    sleep 20
    
    # Check metrics
    echo -e "${YELLOW}After DLQ processing:${NC}"
    check_metrics
    
    # Restore service
    echo -e "${PURPLE}Scenario 4: Service Recovery${NC}"
    echo "============================="
    
    echo -e "${YELLOW}Restoring SAP service...${NC}"
    # Disable outage
    curl -s -X POST http://localhost:8082/admin/simulate-outage \
        -H "Content-Type: application/json" \
        -d '{"outage": false}' | jq .
    
    # Reset failure rate
    curl -s -X POST http://localhost:8082/admin/failure-rate \
        -H "Content-Type: application/json" \
        -d '{"failure_rate": 0.0}' | jq .
    echo ""
    
    echo -e "${YELLOW}Service restored. DLQ messages will be replayed after delay...${NC}"
    echo "Waiting for DLQ replay (30 seconds)..."
    sleep 30
    
    # Final metrics
    echo -e "${YELLOW}Final metrics after recovery:${NC}"
    check_metrics
    
    # Verify data consistency
    echo -e "${CYAN}Data Consistency Check:${NC}"
    local comparison=$(curl -s http://localhost:8080/compare/orders)
    local sync_status=$(echo "$comparison" | jq -r '.analysis.sync_status // false')
    echo -e "Systems synchronized: $([ "$sync_status" = "true" ] && echo "${GREEN}✓ YES${NC}" || echo "${RED}✗ NO${NC}")"
}

# Function to show Kafka topics
show_kafka_info() {
    echo -e "${CYAN}Kafka Topics Information:${NC}"
    echo "• Main topic: order.created"
    echo "• DLQ topic: order.created.dlq"
    echo "• View in Kafka UI: http://localhost:8090"
    echo ""
}

# Main execution
main() {
    check_services
    
    echo -e "${CYAN}Error Handling Features:${NC}"
    echo "• Retry with exponential backoff (max 3 retries)"
    echo "• Dead Letter Queue for failed messages"
    echo "• Automatic replay from DLQ after delay"
    echo "• Metrics tracking for monitoring"
    echo ""
    
    demo_error_handling
    
    echo ""
    echo -e "${GREEN}Error Handling Demo Complete!${NC}"
    echo ""
    
    show_kafka_info
    
    echo -e "${CYAN}Key Takeaways:${NC}"
    echo "• Orders are retried automatically on transient failures"
    echo "• Failed messages go to DLQ after max retries"
    echo "• DLQ messages can be replayed when service recovers"
    echo "• System maintains eventual consistency"
}

# Run main function
main "$@"