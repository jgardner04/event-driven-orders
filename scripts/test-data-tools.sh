#!/bin/bash

# Test Data Comparison and Migration Tools
# This script demonstrates the complete data migration and comparison workflow

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
PROXY_URL="http://localhost:8080"
ORDER_SERVICE_URL="http://localhost:8081"
SAP_URL="http://localhost:8082"
DATA_TOOLS_BIN="$PROJECT_ROOT/cmd/data-tools/data-tools"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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
    exit 1
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
}

build_data_tools() {
    log "Building data-tools CLI..."
    cd "$PROJECT_ROOT"
    
    if ! go build -o "$DATA_TOOLS_BIN" ./cmd/data-tools; then
        error "Failed to build data-tools CLI"
    fi
    
    success "Data-tools CLI built successfully"
}

create_test_data() {
    log "Creating test orders in both systems..."
    
    # Create orders in Order Service (via proxy)
    local orders_created=0
    for i in {1..5}; do
        local order_data=$(cat <<EOF
{
    "id": "order-os-$i",
    "customer_id": "customer-$i",
    "items": [
        {
            "product_id": "widget-$i",
            "quantity": $((i * 2)),
            "unit_price": $((i * 10.50)),
            "specifications": {
                "color": "blue",
                "finish": "matte"
            }
        }
    ],
    "total_amount": $((i * 21.00)),
    "delivery_date": "$(date -d "+$((i * 7)) days" -Iseconds)",
    "status": "pending"
}
EOF
        )
        
        if curl -s -X POST "$PROXY_URL/orders" \
           -H "Content-Type: application/json" \
           -d "$order_data" > /dev/null; then
            orders_created=$((orders_created + 1))
        fi
        
        sleep 1
    done
    
    # Create some orders directly in SAP (simulating historical data)
    for i in {6..8}; do
        local order_data=$(cat <<EOF
{
    "id": "order-sap-$i",
    "customer_id": "customer-$i",
    "items": [
        {
            "product_id": "legacy-widget-$i",
            "quantity": $((i * 3)),
            "unit_price": $((i * 15.75)),
            "specifications": {
                "color": "red",
                "finish": "glossy"
            }
        }
    ],
    "total_amount": $((i * 47.25)),
    "delivery_date": "$(date -d "+$((i * 5)) days" -Iseconds)",
    "status": "confirmed"
}
EOF
        )
        
        if curl -s -X POST "$SAP_URL/orders" \
           -H "Content-Type: application/json" \
           -d "$order_data" > /dev/null; then
            orders_created=$((orders_created + 1))
        fi
        
        sleep 1
    done
    
    success "Created $orders_created test orders"
    sleep 3  # Allow time for event processing
}

run_data_comparison() {
    log "Running data comparison analysis..."
    
    # JSON format comparison
    log "Generating detailed JSON comparison report..."
    if ! "$DATA_TOOLS_BIN" -command=compare -format=json -output="$PROJECT_ROOT/comparison-report.json" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        error "Data comparison failed"
    fi
    
    # Summary format comparison
    log "Generating summary comparison report..."
    if ! "$DATA_TOOLS_BIN" -command=compare -format=summary -output="$PROJECT_ROOT/comparison-summary.txt" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        error "Summary comparison failed"
    fi
    
    success "Data comparison completed"
    
    # Display summary
    if [ -f "$PROJECT_ROOT/comparison-summary.txt" ]; then
        echo ""
        log "=== COMPARISON SUMMARY ==="
        cat "$PROJECT_ROOT/comparison-summary.txt"
        echo ""
    fi
}

run_migration_dry_run() {
    log "Running migration dry run..."
    
    # Bidirectional dry run
    log "Testing bidirectional migration (dry run)..."
    if ! "$DATA_TOOLS_BIN" -command=migrate -direction=bidirectional -dry-run=true \
        -batch-size=10 -concurrency=2 -output="$PROJECT_ROOT/migration-dry-run.json" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        error "Migration dry run failed"
    fi
    
    success "Migration dry run completed successfully"
}

run_actual_migration() {
    log "Running actual data migration..."
    
    # Migrate missing orders from SAP to Order Service
    log "Migrating SAP orders to Order Service..."
    if ! "$DATA_TOOLS_BIN" -command=migrate -direction=to_order_service \
        -batch-size=5 -concurrency=1 -skip-existing=true \
        -output="$PROJECT_ROOT/migration-to-os.json" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        warning "Migration to Order Service encountered issues (check logs)"
    fi
    
    # Migrate missing orders from Order Service to SAP
    log "Migrating Order Service orders to SAP..."
    if ! "$DATA_TOOLS_BIN" -command=migrate -direction=to_sap \
        -batch-size=5 -concurrency=1 -skip-existing=true \
        -output="$PROJECT_ROOT/migration-to-sap.json" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        warning "Migration to SAP encountered issues (check logs)"
    fi
    
    success "Data migration completed"
    
    # Wait for migration to complete
    sleep 2
}

validate_migration() {
    log "Validating migration results..."
    
    if ! "$DATA_TOOLS_BIN" -command=validate -output="$PROJECT_ROOT/migration-validation.json" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        error "Migration validation failed"
    fi
    
    success "Migration validation completed"
}

run_post_migration_comparison() {
    log "Running post-migration comparison..."
    
    if ! "$DATA_TOOLS_BIN" -command=compare -format=summary \
        -output="$PROJECT_ROOT/post-migration-comparison.txt" \
        -order-service="$ORDER_SERVICE_URL" -sap="$SAP_URL"; then
        error "Post-migration comparison failed"
    fi
    
    success "Post-migration comparison completed"
    
    # Display post-migration summary
    if [ -f "$PROJECT_ROOT/post-migration-comparison.txt" ]; then
        echo ""
        log "=== POST-MIGRATION COMPARISON ==="
        cat "$PROJECT_ROOT/post-migration-comparison.txt"
        echo ""
    fi
}

display_results() {
    log "=== DATA TOOLS TEST RESULTS ==="
    echo ""
    
    # Show generated files
    echo "Generated Reports:"
    echo "=================="
    ls -la "$PROJECT_ROOT"/*.json "$PROJECT_ROOT"/*.txt 2>/dev/null | grep -E '\.(json|txt)$' || true
    echo ""
    
    # Show key metrics from validation
    if [ -f "$PROJECT_ROOT/migration-validation.json" ]; then
        log "Migration Validation Summary:"
        if command -v jq > /dev/null 2>&1; then
            jq -r '"Order Service Orders: " + (.total_order_service | tostring) + 
                    "\nSAP Orders: " + (.total_sap | tostring) + 
                    "\nSync Percentage: " + (.sync_percentage | tostring) + "%" + 
                    "\nValidation Status: " + (if .is_valid then "PASSED" else "FAILED" end)' \
                "$PROJECT_ROOT/migration-validation.json"
        else
            cat "$PROJECT_ROOT/migration-validation.json"
        fi
        echo ""
    fi
    
    success "Data tools testing completed successfully!"
}

cleanup_test_files() {
    log "Cleaning up test files..."
    rm -f "$PROJECT_ROOT"/*.json "$PROJECT_ROOT"/*.txt "$DATA_TOOLS_BIN"
    success "Test files cleaned up"
}

main() {
    echo "============================================"
    echo "🔄 Data Tools Integration Test"
    echo "============================================"
    echo ""
    
    # Check if services are running
    wait_for_service "$PROXY_URL" "Proxy Service"
    wait_for_service "$ORDER_SERVICE_URL" "Order Service"
    wait_for_service "$SAP_URL" "SAP Mock"
    
    # Build and test
    build_data_tools
    create_test_data
    run_data_comparison
    run_migration_dry_run
    run_actual_migration
    validate_migration
    run_post_migration_comparison
    display_results
    
    echo ""
    log "All data tools tests completed successfully! 🎉"
    
    # Ask user if they want to clean up
    echo ""
    read -p "Clean up test files? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup_test_files
    else
        log "Test files preserved for inspection"
    fi
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [--help|--cleanup]"
        echo ""
        echo "Options:"
        echo "  --help     Show this help message"
        echo "  --cleanup  Only clean up test files"
        echo ""
        echo "This script tests the data comparison and migration tools by:"
        echo "1. Creating test data in both systems"
        echo "2. Running data comparison analysis"
        echo "3. Performing migration dry run"
        echo "4. Executing actual data migration"
        echo "5. Validating migration results"
        echo "6. Generating comprehensive reports"
        exit 0
        ;;
    --cleanup)
        cleanup_test_files
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac