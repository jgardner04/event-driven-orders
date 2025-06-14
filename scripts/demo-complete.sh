#!/bin/bash

# Complete Strangler Pattern Demo with Dashboard
# This script demonstrates the complete strangler pattern workflow with monitoring

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
PROXY_URL="http://localhost:8080"
ORDER_SERVICE_URL="http://localhost:8081"
SAP_URL="http://localhost:8082"
DASHBOARD_URL="http://localhost:3000"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${CYAN}ℹ️  $1${NC}"
}

demo_step() {
    echo ""
    echo -e "${PURPLE}=== $1 ===${NC}"
    echo ""
}

wait_for_service() {
    local url=$1
    local service_name=$2
    local max_attempts=30
    local attempt=1

    log "Waiting for $service_name to be available at $url..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$url/health" > /dev/null 2>&1; then
            success "$service_name is available"
            return 0
        fi
        
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    error "$service_name failed to start after $max_attempts attempts"
    return 1
}

create_sample_orders() {
    local count=$1
    local endpoint=$2
    local description=$3
    
    log "Creating $count sample orders via $description..."
    
    for i in $(seq 1 $count); do
        local customer_id="customer-demo-$i"
        local order_data=$(cat <<EOF
{
    "customer_id": "$customer_id",
    "items": [
        {
            "product_id": "widget-demo-$i",
            "quantity": $((i % 5 + 1)),
            "unit_price": $((10 + i * 5)).99,
            "specifications": {
                "color": "$([ $((i % 2)) -eq 0 ] && echo "blue" || echo "red")",
                "finish": "$([ $((i % 3)) -eq 0 ] && echo "matte" || echo "glossy")",
                "size": "$([ $((i % 4)) -eq 0 ] && echo "large" || echo "medium")"
            }
        }
    ],
    "total_amount": $((20 + i * 10)).99,
    "delivery_date": "$(date -d "+$((i * 7)) days" -Iseconds)",
    "status": "pending"
}
EOF
        )
        
        if curl -s -X POST "$endpoint" \
           -H "Content-Type: application/json" \
           -d "$order_data" > /dev/null; then
            echo -n "✓"
        else
            echo -n "✗"
        fi
        
        # Small delay to see real-time updates
        sleep 0.5
    done
    
    echo ""
    success "Created $count orders via $description"
}

check_service_health() {
    local url=$1
    local name=$2
    
    local response=$(curl -s "$url/health" || echo "failed")
    
    if [[ $response == *"healthy"* ]]; then
        success "$name: HEALTHY"
    else
        warning "$name: UNHEALTHY"
    fi
}

show_data_comparison() {
    log "Comparing data between Order Service and SAP..."
    
    local os_count=$(curl -s "$ORDER_SERVICE_URL/orders" | jq '.count // 0' 2>/dev/null || echo "0")
    local sap_count=$(curl -s "$SAP_URL/orders" | jq 'length // 0' 2>/dev/null || echo "0")
    
    echo "📊 Data Comparison:"
    echo "   Order Service: $os_count orders"
    echo "   SAP Mock:      $sap_count orders"
    echo "   Sync Status:   $([ "$os_count" -eq "$sap_count" ] && echo "✅ SYNCED" || echo "⚠️  OUT OF SYNC")"
}

main() {
    echo "============================================"
    echo "🎯 Complete Strangler Pattern Demo"
    echo "============================================"
    echo ""
    echo "This demo showcases:"
    echo "• Real-time order processing and monitoring"
    echo "• WebSocket-based live updates"
    echo "• Performance metrics visualization"
    echo "• Load testing capabilities"
    echo "• Data synchronization monitoring"
    echo ""
    
    # Check prerequisites
    demo_step "Checking Prerequisites"
    
    if ! command -v curl > /dev/null 2>&1; then
        error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq > /dev/null 2>&1; then
        warning "jq is not installed - some features will be limited"
    fi
    
    success "Prerequisites checked"
    
    # Wait for all services
    demo_step "Service Health Check"
    
    wait_for_service "$PROXY_URL" "Proxy Service" || exit 1
    wait_for_service "$ORDER_SERVICE_URL" "Order Service" || exit 1
    wait_for_service "$SAP_URL" "SAP Mock" || exit 1
    
    # Check if dashboard is running
    if curl -s "$DASHBOARD_URL" > /dev/null 2>&1; then
        success "Dashboard is available at $DASHBOARD_URL"
        info "🎯 Open $DASHBOARD_URL in your browser to monitor in real-time"
    else
        warning "Dashboard is not running. Start it with: ./scripts/start-dashboard.sh"
    fi
    
    echo ""
    read -p "Press Enter to continue with the demo..."
    
    # Phase 1: Initial System State
    demo_step "Phase 1: Initial System State"
    
    check_service_health "$PROXY_URL" "Proxy Service"
    check_service_health "$ORDER_SERVICE_URL" "Order Service"
    check_service_health "$SAP_URL" "SAP Mock"
    
    show_data_comparison
    
    # Phase 2: Order Creation Demo
    demo_step "Phase 2: Order Creation Demo"
    
    info "Watch the dashboard for real-time updates!"
    echo ""
    
    # Create orders through different endpoints
    create_sample_orders 5 "$PROXY_URL/orders" "Proxy (Full Flow)"
    
    sleep 2
    show_data_comparison
    
    echo ""
    read -p "Press Enter to continue with load testing demo..."
    
    # Phase 3: Load Testing Demo
    demo_step "Phase 3: Load Testing Simulation"
    
    info "Simulating higher load - watch the performance metrics!"
    
    # Create a burst of orders to show performance impact
    create_sample_orders 10 "$PROXY_URL/orders" "Proxy (Load Test)"
    
    # Show final state
    demo_step "Final System State"
    
    show_data_comparison
    
    # Performance summary
    log "Getting system performance summary..."
    
    echo ""
    echo "📈 Demo Summary:"
    echo "=================="
    
    # Get final counts
    local final_os_count=$(curl -s "$ORDER_SERVICE_URL/orders" | jq '.count // 0' 2>/dev/null || echo "0")
    local final_sap_count=$(curl -s "$SAP_URL/orders" | jq 'length // 0' 2>/dev/null || echo "0")
    
    echo "• Total Orders Created: $((final_os_count))"
    echo "• Order Service Orders: $final_os_count"
    echo "• SAP Mock Orders: $final_sap_count"
    echo "• Data Synchronization: $([ "$final_os_count" -eq "$final_sap_count" ] && echo "✅ Perfect" || echo "⚠️  Needs Attention")"
    
    echo ""
    echo "🎯 Dashboard Features Demonstrated:"
    echo "======================================"
    echo "• Real-time order tracking with WebSocket updates"
    echo "• Service health monitoring"
    echo "• Performance metrics visualization"
    echo "• Data synchronization status"
    echo "• Load testing capabilities"
    
    echo ""
    echo "🔗 Useful Links:"
    echo "================"
    echo "• Dashboard:    $DASHBOARD_URL"
    echo "• Proxy API:    $PROXY_URL"
    echo "• Order Service: $ORDER_SERVICE_URL"
    echo "• SAP Mock:     $SAP_URL"
    echo "• Kafka UI:     http://localhost:8090"
    
    echo ""
    success "Demo completed successfully! 🎉"
    
    echo ""
    info "Next steps:"
    echo "• Explore the dashboard's different tabs"
    echo "• Try the load testing features"
    echo "• Run data comparison and migration tools"
    echo "• Monitor real-time metrics during order creation"
    
    echo ""
    log "To create more test orders manually:"
    echo "curl -X POST $PROXY_URL/orders -H 'Content-Type: application/json' -d '{\"customer_id\":\"test\",\"items\":[{\"product_id\":\"widget\",\"quantity\":1,\"unit_price\":19.99}],\"total_amount\":19.99}'"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [--help]"
        echo ""
        echo "This script runs a complete demonstration of the strangler pattern"
        echo "with real-time monitoring dashboard."
        echo ""
        echo "Prerequisites:"
        echo "• All Go services must be running (proxy, order-service, sap-mock)"
        echo "• Dashboard should be running for best experience"
        echo "• curl must be installed"
        echo "• jq is recommended for enhanced output"
        echo ""
        echo "To start all services:"
        echo "  docker-compose up -d"
        echo ""
        echo "To start the dashboard:"
        echo "  ./scripts/start-dashboard.sh"
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac