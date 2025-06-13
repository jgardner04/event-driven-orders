#!/bin/bash

echo "Strangler Pattern Load Test"
echo "==========================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default configuration
DEFAULT_ORDER_COUNT=50
DEFAULT_CONCURRENCY=10
DEFAULT_BATCH_SIZE=5

# Parse command line arguments
ORDER_COUNT=${1:-$DEFAULT_ORDER_COUNT}
CONCURRENCY=${2:-$DEFAULT_CONCURRENCY}
BATCH_SIZE=${3:-$DEFAULT_BATCH_SIZE}

# Validate parameters
if ! [[ "$ORDER_COUNT" =~ ^[0-9]+$ ]] || [ "$ORDER_COUNT" -lt 1 ]; then
    echo -e "${RED}Error: ORDER_COUNT must be a positive integer${NC}"
    exit 1
fi

if ! [[ "$CONCURRENCY" =~ ^[0-9]+$ ]] || [ "$CONCURRENCY" -lt 1 ]; then
    echo -e "${RED}Error: CONCURRENCY must be a positive integer${NC}"
    exit 1
fi

if ! [[ "$BATCH_SIZE" =~ ^[0-9]+$ ]] || [ "$BATCH_SIZE" -lt 1 ]; then
    echo -e "${RED}Error: BATCH_SIZE must be a positive integer${NC}"
    exit 1
fi

echo -e "${CYAN}Configuration:${NC}"
echo "• Order Count: $ORDER_COUNT"
echo "• Concurrency: $CONCURRENCY"
echo "• Batch Size: $BATCH_SIZE"
echo ""

# Performance tracking variables
PROXY_TIMES=()
ORDER_SERVICE_TIMES=()
SAP_TIMES=()
FAILED_ORDERS=()
SUCCESSFUL_ORDERS=()

# Temporary files for concurrent processing
TEMP_DIR=$(mktemp -d)
RESULTS_FILE="$TEMP_DIR/results.txt"
ORDERS_FILE="$TEMP_DIR/orders.txt"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up temporary files...${NC}"
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Function to check service health
check_services() {
    echo -e "${BLUE}Checking service health...${NC}"
    
    local services=("http://localhost:8080/health:Proxy" "http://localhost:8081/health:Order-Service" "http://localhost:8082/health:SAP-Mock")
    
    for service_info in "${services[@]}"; do
        IFS=':' read -r url name <<< "$service_info"
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

# Function to generate order data
generate_order() {
    local order_id=$1
    local timestamp=$(date +%s)
    local customer_num=$((order_id % 100 + 1))
    local product_count=$((order_id % 3 + 1))
    local base_price=$((order_id % 50 + 10))
    
    cat <<EOF
{
  "id": "load-test-${timestamp}-${order_id}",
  "customer_id": "LOAD-CUST-${customer_num}",
  "items": [
    {
      "product_id": "LOAD-WIDGET-${product_count}",
      "quantity": $((product_count * 2)),
      "unit_price": ${base_price}.99,
      "specifications": {
        "test_batch": "load-test-${timestamp}",
        "order_sequence": "${order_id}",
        "color": "$([ $((order_id % 2)) -eq 0 ] && echo 'blue' || echo 'red')",
        "priority": "$([ $((order_id % 3)) -eq 0 ] && echo 'high' || echo 'normal')"
      }
    }
  ],
  "total_amount": $((product_count * 2 * base_price)).99,
  "delivery_date": "2025-07-$(printf "%02d" $((order_id % 28 + 1)))T00:00:00Z"
}
EOF
}

# Function to send single order and measure performance
send_order() {
    local order_num=$1
    local order_data=$(generate_order $order_num)
    local order_id=$(echo "$order_data" | jq -r '.id')
    
    # Send to proxy (which writes to both systems)
    local start_time=$(date +%s%N)
    local response=$(curl -s -w "%{http_code}" -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "$order_data")
    local end_time=$(date +%s%N)
    
    local http_code=${response: -3}
    local response_body=${response%???}
    local proxy_time=$(( (end_time - start_time) / 1000000 ))
    
    # Check if order was successful
    if [ "$http_code" -eq 201 ]; then
        echo "SUCCESS,$order_num,$order_id,$proxy_time" >> "$RESULTS_FILE"
        echo "$order_id" >> "$ORDERS_FILE"
        
        # Measure direct service times for comparison
        measure_direct_service_times "$order_id"
    else
        echo "FAILED,$order_num,$order_id,$proxy_time,$http_code" >> "$RESULTS_FILE"
        echo -e "${RED}Order $order_num failed: HTTP $http_code${NC}" >&2
    fi
}

# Function to measure direct service response times
measure_direct_service_times() {
    local order_id=$1
    
    # Wait a moment for order to be processed
    sleep 0.5
    
    # Measure Order Service response time
    local start_time=$(date +%s%N)
    local order_service_response=$(curl -s -w "%{http_code}" "http://localhost:8081/orders/$order_id")
    local end_time=$(date +%s%N)
    local order_service_time=$(( (end_time - start_time) / 1000000 ))
    local os_http_code=${order_service_response: -3}
    
    # Measure SAP Mock response time
    start_time=$(date +%s%N)
    local sap_response=$(curl -s -w "%{http_code}" "http://localhost:8082/orders/$order_id")
    end_time=$(date +%s%N)
    local sap_time=$(( (end_time - start_time) / 1000000 ))
    local sap_http_code=${sap_response: -3}
    
    # Log individual service times
    echo "DIRECT,$order_id,$order_service_time,$sap_time,$os_http_code,$sap_http_code" >> "$RESULTS_FILE"
}

# Function to run load test batch
run_load_test() {
    echo -e "${BLUE}Starting load test...${NC}"
    echo "Creating $ORDER_COUNT orders with concurrency $CONCURRENCY"
    echo ""
    
    local start_time=$(date +%s)
    local pids=()
    local active_jobs=0
    
    for ((i=1; i<=ORDER_COUNT; i++)); do
        # Send order in background
        send_order $i &
        pids+=($!)
        ((active_jobs++))
        
        # Progress indicator
        if [ $((i % 10)) -eq 0 ]; then
            echo -n "."
        fi
        
        # Control concurrency
        if [ $active_jobs -ge $CONCURRENCY ]; then
            # Wait for some jobs to complete
            for ((j=0; j<$BATCH_SIZE; j++)); do
                if [ ${#pids[@]} -gt 0 ]; then
                    wait ${pids[0]}
                    pids=("${pids[@]:1}")
                    ((active_jobs--))
                fi
            done
        fi
        
        # Small delay to avoid overwhelming services
        sleep 0.1
    done
    
    # Wait for all remaining jobs
    echo ""
    echo -e "${YELLOW}Waiting for remaining orders to complete...${NC}"
    for pid in "${pids[@]}"; do
        wait $pid
    done
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))
    
    echo -e "${GREEN}✓ Load test completed in ${total_duration}s${NC}"
    echo ""
}

# Function to analyze results
analyze_results() {
    echo -e "${BLUE}Analyzing Results${NC}"
    echo "=================="
    
    if [ ! -f "$RESULTS_FILE" ]; then
        echo -e "${RED}No results file found${NC}"
        return
    fi
    
    # Count successful and failed orders
    local successful_count=$(grep "^SUCCESS" "$RESULTS_FILE" | wc -l)
    local failed_count=$(grep "^FAILED" "$RESULTS_FILE" | wc -l)
    local success_rate=$(( successful_count * 100 / ORDER_COUNT ))
    
    echo "Order Statistics:"
    echo "• Total Orders: $ORDER_COUNT"
    echo -e "• Successful: ${GREEN}$successful_count${NC}"
    echo -e "• Failed: ${RED}$failed_count${NC}"
    echo -e "• Success Rate: ${GREEN}${success_rate}%${NC}"
    echo ""
    
    if [ $successful_count -gt 0 ]; then
        # Analyze proxy response times
        local proxy_times=($(grep "^SUCCESS" "$RESULTS_FILE" | cut -d',' -f4))
        local proxy_avg=$(( $(IFS=+; echo "$((${proxy_times[*]}))" ) / ${#proxy_times[@]} ))
        local proxy_min=$(printf '%s\n' "${proxy_times[@]}" | sort -n | head -1)
        local proxy_max=$(printf '%s\n' "${proxy_times[@]}" | sort -n | tail -1)
        
        echo "Proxy Response Times (ms):"
        echo "• Average: ${proxy_avg}ms"
        echo "• Min: ${proxy_min}ms"
        echo "• Max: ${proxy_max}ms"
        echo ""
        
        # Analyze direct service times
        if grep -q "^DIRECT" "$RESULTS_FILE"; then
            local os_times=($(grep "^DIRECT" "$RESULTS_FILE" | cut -d',' -f3))
            local sap_times=($(grep "^DIRECT" "$RESULTS_FILE" | cut -d',' -f4))
            
            if [ ${#os_times[@]} -gt 0 ]; then
                local os_avg=$(( $(IFS=+; echo "$((${os_times[*]}))" ) / ${#os_times[@]} ))
                local os_min=$(printf '%s\n' "${os_times[@]}" | sort -n | head -1)
                local os_max=$(printf '%s\n' "${os_times[@]}" | sort -n | tail -1)
                
                echo "Order Service Direct Times (ms):"
                echo "• Average: ${os_avg}ms"
                echo "• Min: ${os_min}ms"
                echo "• Max: ${os_max}ms"
            fi
            
            if [ ${#sap_times[@]} -gt 0 ]; then
                local sap_avg=$(( $(IFS=+; echo "$((${sap_times[*]}))" ) / ${#sap_times[@]} ))
                local sap_min=$(printf '%s\n' "${sap_times[@]}" | sort -n | head -1)
                local sap_max=$(printf '%s\n' "${sap_times[@]}" | sort -n | tail -1)
                
                echo "SAP Mock Direct Times (ms):"
                echo "• Average: ${sap_avg}ms"
                echo "• Min: ${sap_min}ms"
                echo "• Max: ${sap_max}ms"
            fi
            
            # Performance comparison
            if [ ${#os_times[@]} -gt 0 ] && [ ${#sap_times[@]} -gt 0 ]; then
                echo ""
                echo -e "${CYAN}Performance Comparison:${NC}"
                if [ $os_avg -lt $sap_avg ]; then
                    local speedup=$(( sap_avg * 100 / os_avg ))
                    echo -e "• Order Service is ${GREEN}${speedup}%${NC} the speed of SAP Mock"
                    echo -e "• Order Service is ${GREEN}$((speedup - 100))% faster${NC} than SAP Mock"
                else
                    local slowdown=$(( os_avg * 100 / sap_avg ))
                    echo -e "• Order Service is ${YELLOW}${slowdown}%${NC} the speed of SAP Mock"
                fi
            fi
        fi
    fi
    echo ""
}

# Function to verify data consistency
verify_data_consistency() {
    echo -e "${BLUE}Verifying Data Consistency${NC}"
    echo "=========================="
    
    if [ ! -f "$ORDERS_FILE" ]; then
        echo -e "${RED}No orders file found for verification${NC}"
        return
    fi
    
    local order_ids=($(cat "$ORDERS_FILE"))
    local total_orders=${#order_ids[@]}
    
    if [ $total_orders -eq 0 ]; then
        echo -e "${RED}No successful orders to verify${NC}"
        return
    fi
    
    echo "Checking $total_orders orders for consistency..."
    echo ""
    
    # Use comparison endpoint for verification
    local comparison_result=$(curl -s http://localhost:8080/compare/orders)
    local sync_status=$(echo "$comparison_result" | jq -r '.analysis.sync_status')
    local os_count=$(echo "$comparison_result" | jq -r '.order_service.count')
    local sap_count=$(echo "$comparison_result" | jq -r '.sap.count')
    
    echo "System Comparison:"
    echo "• Order Service Count: $os_count"
    echo "• SAP Mock Count: $sap_count"
    echo -e "• Systems In Sync: $([ "$sync_status" = "true" ] && echo "${GREEN}✓${NC}" || echo "${RED}✗${NC}")"
    
    if [ "$sync_status" = "true" ]; then
        echo -e "${GREEN}✓ All systems contain consistent data${NC}"
    else
        echo -e "${RED}✗ Data inconsistency detected${NC}"
        
        # Show missing orders
        local missing_in_sap=$(echo "$comparison_result" | jq -r '.analysis.missing_in_sap[]?' 2>/dev/null | head -5)
        local missing_in_os=$(echo "$comparison_result" | jq -r '.analysis.missing_in_order_service[]?' 2>/dev/null | head -5)
        
        if [ ! -z "$missing_in_sap" ]; then
            echo "Sample orders missing in SAP:"
            echo "$missing_in_sap" | sed 's/^/  • /'
        fi
        
        if [ ! -z "$missing_in_os" ]; then
            echo "Sample orders missing in Order Service:"
            echo "$missing_in_os" | sed 's/^/  • /'
        fi
    fi
    
    echo ""
    
    # Sample individual order verification
    echo "Sample Order Verification:"
    local sample_orders=($(head -3 "$ORDERS_FILE"))
    
    for order_id in "${sample_orders[@]}"; do
        echo -n "  $order_id: "
        local comparison=$(curl -s "http://localhost:8080/compare/orders/$order_id")
        local perfect_match=$(echo "$comparison" | jq -r '.analysis.perfect_match')
        local os_found=$(echo "$comparison" | jq -r '.order_service.found')
        local sap_found=$(echo "$comparison" | jq -r '.sap.found')
        
        if [ "$os_found" = "true" ] && [ "$sap_found" = "true" ]; then
            if [ "$perfect_match" = "true" ]; then
                echo -e "${GREEN}✓ Perfect match${NC}"
            else
                echo -e "${YELLOW}⚠ Data mismatch${NC}"
            fi
        else
            echo -e "${RED}✗ Missing in one system${NC}"
        fi
    done
    echo ""
}

# Function to generate detailed report
generate_report() {
    echo -e "${CYAN}Load Test Summary Report${NC}"
    echo "========================"
    echo "Timestamp: $(date)"
    echo "Test Configuration: $ORDER_COUNT orders, $CONCURRENCY concurrency, $BATCH_SIZE batch size"
    echo ""
    
    # Save detailed results to file
    local report_file="load-test-report-$(date +%Y%m%d-%H%M%S).txt"
    
    {
        echo "Strangler Pattern Load Test Report"
        echo "================================="
        echo "Timestamp: $(date)"
        echo "Configuration: $ORDER_COUNT orders, $CONCURRENCY concurrency"
        echo ""
        echo "Raw Results:"
        cat "$RESULTS_FILE" 2>/dev/null
    } > "$report_file"
    
    echo -e "${GREEN}Detailed report saved to: $report_file${NC}"
    echo ""
    
    # Show throughput calculation
    if [ -f "$RESULTS_FILE" ]; then
        local successful_count=$(grep "^SUCCESS" "$RESULTS_FILE" | wc -l)
        local first_timestamp=$(head -1 "$RESULTS_FILE" | cut -d',' -f2)
        local last_timestamp=$(tail -1 "$RESULTS_FILE" | cut -d',' -f2)
        
        if [ $successful_count -gt 0 ] && [ ! -z "$first_timestamp" ] && [ ! -z "$last_timestamp" ]; then
            local duration=$((last_timestamp - first_timestamp + 1))
            local throughput=$(( successful_count / duration ))
            echo "Throughput: ~${throughput} orders/second"
        fi
    fi
}

# Function to show usage
show_usage() {
    cat <<EOF
Usage: $0 [ORDER_COUNT] [CONCURRENCY] [BATCH_SIZE]

Parameters:
  ORDER_COUNT   Number of orders to create (default: $DEFAULT_ORDER_COUNT)
  CONCURRENCY   Number of concurrent requests (default: $DEFAULT_CONCURRENCY)
  BATCH_SIZE    Batch size for concurrency control (default: $DEFAULT_BATCH_SIZE)

Examples:
  $0                    # Use defaults: 50 orders, 10 concurrent, 5 batch
  $0 100               # 100 orders with default concurrency
  $0 100 20           # 100 orders, 20 concurrent
  $0 100 20 10        # 100 orders, 20 concurrent, 10 batch size

The script will:
1. Check service health
2. Generate and send orders concurrently through the proxy
3. Measure response times for proxy, order service, and SAP mock
4. Verify data consistency between systems
5. Generate performance analysis and comparison
6. Create detailed report file

Requirements:
- Services must be running (proxy:8080, order-service:8081, sap-mock:8082)
- curl and jq must be installed
EOF
}

# Main execution
main() {
    # Show usage if help requested
    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        show_usage
        exit 0
    fi
    
    # Check dependencies
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: curl is required but not installed${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is required but not installed${NC}"
        exit 1
    fi
    
    # Run the load test
    check_services
    run_load_test
    sleep 2  # Allow time for final processing
    analyze_results
    verify_data_consistency
    generate_report
    
    echo -e "${GREEN}Load test completed successfully!${NC}"
    echo ""
    echo "Next steps:"
    echo "• Review the generated report file"
    echo "• Check Kafka UI at http://localhost:8090 for events"
    echo "• Run './scripts/compare-data.sh' for additional verification"
}

# Run main function with all arguments
main "$@"