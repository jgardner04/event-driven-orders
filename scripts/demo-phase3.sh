#!/bin/bash

echo "Strangler Pattern Phase 3 Demo"
echo "==============================="
echo ""
echo "Phase 3: Complete Event-Driven Architecture"
echo "• Proxy writes ONLY to Order Service (no direct SAP calls)"
echo "• SAP consumes order events from Kafka"
echo "• Complete decoupling achieved"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

# Function to demonstrate Phase 3 flow
demo_phase3_flow() {
    echo -e "${CYAN}Phase 3 Demo: Event-Driven Order Processing${NC}"
    echo "============================================="
    echo ""
    
    # Show initial state
    echo -e "${YELLOW}1. Initial state (should be empty):${NC}"
    local initial_comparison=$(curl -s http://localhost:8080/compare/orders)
    local initial_os_count=$(echo "$initial_comparison" | jq -r '.order_service.count // 0')
    local initial_sap_count=$(echo "$initial_comparison" | jq -r '.sap.count // 0')
    
    echo "   Order Service: $initial_os_count orders"
    echo "   SAP Mock: $initial_sap_count orders"
    echo ""
    
    # Create test orders
    echo -e "${YELLOW}2. Creating orders via Proxy (writes to Order Service only):${NC}"
    
    local orders=(
        '{"customer_id": "PHASE3-CUST-001", "items": [{"product_id": "WIDGET-A", "quantity": 3, "unit_price": 25.99, "specifications": {"color": "red", "size": "large"}}], "total_amount": 77.97, "delivery_date": "2025-07-20T00:00:00Z"}'
        '{"customer_id": "PHASE3-CUST-002", "items": [{"product_id": "GADGET-B", "quantity": 2, "unit_price": 45.00, "specifications": {"color": "blue", "material": "plastic"}}], "total_amount": 90.00, "delivery_date": "2025-07-25T00:00:00Z"}'
        '{"customer_id": "PHASE3-CUST-003", "items": [{"product_id": "MODULE-C", "quantity": 1, "unit_price": 199.99, "specifications": {"priority": "high", "warranty": "extended"}}], "total_amount": 199.99, "delivery_date": "2025-07-30T00:00:00Z"}'
    )
    
    local order_ids=()
    
    for i in "${!orders[@]}"; do
        echo -n "   Creating order $((i+1)): "
        local response=$(curl -s -X POST http://localhost:8080/orders \
            -H "Content-Type: application/json" \
            -d "${orders[$i]}")
        
        local success=$(echo "$response" | jq -r '.success // false')
        if [ "$success" = "true" ]; then
            local order_id=$(echo "$response" | jq -r '.order.id')
            order_ids+=("$order_id")
            echo -e "${GREEN}✓ Created (ID: ${order_id:0:8}...)${NC}"
        else
            echo -e "${RED}✗ Failed${NC}"
            echo "$response" | jq .
        fi
        
        # Small delay between orders
        sleep 1
    done
    echo ""
    
    # Wait for Kafka events to be processed
    echo -e "${YELLOW}3. Waiting for Kafka events to be processed by SAP...${NC}"
    echo -n "   Processing"
    for i in {1..10}; do
        echo -n "."
        sleep 1
    done
    echo -e " ${GREEN}Done!${NC}"
    echo ""
    
    # Check final state
    echo -e "${YELLOW}4. Final state verification:${NC}"
    local final_comparison=$(curl -s http://localhost:8080/compare/orders)
    local final_os_count=$(echo "$final_comparison" | jq -r '.order_service.count // 0')
    local final_sap_count=$(echo "$final_comparison" | jq -r '.sap.count // 0')
    local sync_status=$(echo "$final_comparison" | jq -r '.analysis.sync_status // false')
    
    echo "   Order Service: $final_os_count orders"
    echo "   SAP Mock: $final_sap_count orders"
    echo -e "   Systems Synchronized: $([ "$sync_status" = "true" ] && echo "${GREEN}✓ YES${NC}" || echo "${RED}✗ NO${NC}")"
    echo ""
    
    # Show event flow analysis
    echo -e "${YELLOW}5. Event Flow Analysis:${NC}"
    echo "   ┌─────────────┐    HTTP POST     ┌──────────────┐"
    echo "   │   Client    │ ─────────────────> │    Proxy     │"
    echo "   │ (eCommerce) │                   │ (Phase 3)    │"
    echo "   └─────────────┘                   └──────────────┘"
    echo "                                             │"
    echo "                                             │ HTTP POST"
    echo "                                             ▼"
    echo "                                     ┌──────────────┐"
    echo "                                     │    Order     │"
    echo "                                     │   Service    │ ─┐"
    echo "                                     │              │  │"
    echo "                                     └──────────────┘  │"
    echo "                                             │         │"
    echo "                                             │         │ Kafka Event"
    echo "                                             │         │ Publish"
    echo "                                             │         │"
    echo "                                             ▼         ▼"
    echo "                                     ┌──────────────────────────┐"
    echo "                                     │         Kafka            │"
    echo "                                     │   (order.created topic)  │"
    echo "                                     └──────────────────────────┘"
    echo "                                             │"
    echo "                                             │ Event"
    echo "                                             │ Consumption"
    echo "                                             ▼"
    echo "                                     ┌──────────────┐"
    echo "                                     │  SAP Mock    │"
    echo "                                     │ (Consumer)   │"
    echo "                                     └──────────────┘"
    echo ""
    
    # Performance comparison
    if [ ${#order_ids[@]} -gt 0 ]; then
        echo -e "${YELLOW}6. Sample order verification:${NC}"
        local sample_id="${order_ids[0]}"
        local comparison=$(curl -s "http://localhost:8080/compare/orders/$sample_id")
        local perfect_match=$(echo "$comparison" | jq -r '.analysis.perfect_match // false')
        local os_found=$(echo "$comparison" | jq -r '.order_service.found // false')
        local sap_found=$(echo "$comparison" | jq -r '.sap.found // false')
        
        echo "   Sample Order ID: ${sample_id:0:8}..."
        echo "   Order Service: $([ "$os_found" = "true" ] && echo "${GREEN}✓ Found${NC}" || echo "${RED}✗ Not Found${NC}")"
        echo "   SAP Mock: $([ "$sap_found" = "true" ] && echo "${GREEN}✓ Found${NC}" || echo "${RED}✗ Not Found${NC}")"
        echo "   Data Match: $([ "$perfect_match" = "true" ] && echo "${GREEN}✓ Perfect${NC}" || echo "${YELLOW}⚠ Partial${NC}")"
        echo ""
    fi
    
    # Show Kafka UI link
    echo -e "${CYAN}Kafka Events:${NC}"
    echo "   View Kafka events at: http://localhost:8090"
    echo "   Topic: order.created"
    echo ""
    
    echo -e "${GREEN}✓ Phase 3 Demo Complete!${NC}"
    echo ""
    echo -e "${CYAN}Key Achievements:${NC}"
    echo "• ✅ Proxy completely decoupled from SAP"
    echo "• ✅ Orders flow through event-driven architecture"
    echo "• ✅ SAP receives orders via Kafka events"
    echo "• ✅ No direct HTTP calls to SAP"
    echo "• ✅ Strangler Pattern migration complete"
}

# Function to show architecture evolution
show_architecture_evolution() {
    echo -e "${CYAN}Strangler Pattern Evolution Summary${NC}"
    echo "=================================="
    echo ""
    
    echo -e "${YELLOW}Phase 1: Legacy Proxy${NC}"
    echo "Client → Proxy → SAP (direct calls)"
    echo ""
    
    echo -e "${YELLOW}Phase 2: Dual Write${NC}"
    echo "Client → Proxy → Order Service + SAP (parallel writes)"
    echo "                ↓"
    echo "              Kafka Events"
    echo ""
    
    echo -e "${YELLOW}Phase 3: Event-Driven (Current)${NC}"
    echo "Client → Proxy → Order Service → Kafka → SAP"
    echo ""
    
    echo -e "${GREEN}Migration Complete: Legacy SAP transformed into event consumer!${NC}"
    echo ""
}

# Main execution
main() {
    check_services
    demo_phase3_flow
    show_architecture_evolution
    
    echo "Next steps:"
    echo "• Explore Kafka UI: http://localhost:8090"
    echo "• Run load tests: ./scripts/load-test.sh"
    echo "• Compare with Phase 2: ./scripts/test-phase2.sh"
}

# Run main function
main "$@"