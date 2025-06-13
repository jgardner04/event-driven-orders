#!/bin/bash

echo "Advanced Strangler Pattern Load Test"
echo "===================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/load-test-config.yaml"

# Default values (can be overridden by config or command line)
ORDER_COUNT=50
CONCURRENCY=10
BATCH_SIZE=5
SCENARIO=""
ENABLE_DETAILED_LOGGING=false
ENABLE_REAL_TIME_STATS=false

# Global tracking variables
declare -A PERFORMANCE_METRICS
declare -A ERROR_COUNTS
TEMP_DIR=""
START_TIME=""
END_TIME=""

# Function to show usage
show_usage() {
    cat <<EOF
Advanced Load Test for Strangler Pattern Demo

Usage: $0 [OPTIONS] [SCENARIO]

OPTIONS:
  -c, --count NUM         Number of orders to create
  -p, --concurrency NUM   Number of concurrent requests  
  -b, --batch NUM         Batch size for concurrency control
  -s, --scenario NAME     Use predefined scenario (light|medium|heavy|stress)
  -d, --detailed         Enable detailed logging
  -r, --real-time        Enable real-time statistics
  -h, --help             Show this help message

SCENARIOS:
  light    20 orders, 5 concurrent   - Development testing
  medium   100 orders, 15 concurrent - Integration testing  
  heavy    500 orders, 25 concurrent - Performance testing
  stress   1000 orders, 50 concurrent - Stress testing

EXAMPLES:
  $0                          # Default: 50 orders, 10 concurrent
  $0 -s medium               # Use medium scenario
  $0 -c 200 -p 20            # Custom: 200 orders, 20 concurrent
  $0 -s heavy -d -r          # Heavy scenario with detailed logging

The test will measure performance, verify data consistency, and generate
comprehensive reports comparing the new Order Service vs SAP Mock.
EOF
}

# Function to parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--count)
                ORDER_COUNT="$2"
                shift 2
                ;;
            -p|--concurrency)
                CONCURRENCY="$2"
                shift 2
                ;;
            -b|--batch)
                BATCH_SIZE="$2"
                shift 2
                ;;
            -s|--scenario)
                SCENARIO="$2"
                shift 2
                ;;
            -d|--detailed)
                ENABLE_DETAILED_LOGGING=true
                shift
                ;;
            -r|--real-time)
                ENABLE_REAL_TIME_STATS=true
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                if [[ -z "$SCENARIO" ]]; then
                    SCENARIO="$1"
                else
                    echo -e "${RED}Unknown option: $1${NC}"
                    show_usage
                    exit 1
                fi
                shift
                ;;
        esac
    done
}

# Function to apply scenario configuration
apply_scenario() {
    case "$SCENARIO" in
        light)
            ORDER_COUNT=20
            CONCURRENCY=5
            BATCH_SIZE=2
            echo -e "${CYAN}Applied 'light' scenario: $ORDER_COUNT orders, $CONCURRENCY concurrent${NC}"
            ;;
        medium)
            ORDER_COUNT=100
            CONCURRENCY=15
            BATCH_SIZE=5
            echo -e "${CYAN}Applied 'medium' scenario: $ORDER_COUNT orders, $CONCURRENCY concurrent${NC}"
            ;;
        heavy)
            ORDER_COUNT=500
            CONCURRENCY=25
            BATCH_SIZE=10
            echo -e "${CYAN}Applied 'heavy' scenario: $ORDER_COUNT orders, $CONCURRENCY concurrent${NC}"
            ;;
        stress)
            ORDER_COUNT=1000
            CONCURRENCY=50
            BATCH_SIZE=20
            echo -e "${CYAN}Applied 'stress' scenario: $ORDER_COUNT orders, $CONCURRENCY concurrent${NC}"
            ;;
        "")
            # No scenario specified, use defaults or command line values
            ;;
        *)
            echo -e "${RED}Unknown scenario: $SCENARIO${NC}"
            echo "Available scenarios: light, medium, heavy, stress"
            exit 1
            ;;
    esac
}

# Function to initialize logging and metrics
initialize_logging() {
    TEMP_DIR=$(mktemp -d)
    
    # Create log files
    RESULTS_LOG="$TEMP_DIR/results.log"
    PERFORMANCE_LOG="$TEMP_DIR/performance.log"
    ERROR_LOG="$TEMP_DIR/errors.log"
    METRICS_CSV="$TEMP_DIR/metrics.csv"
    
    # Initialize CSV headers
    echo "timestamp,order_id,operation,duration_ms,status,system" > "$METRICS_CSV"
    
    echo -e "${BLUE}Logging initialized:${NC}"
    echo "• Results: $RESULTS_LOG"
    echo "• Performance: $PERFORMANCE_LOG"
    echo "• Errors: $ERROR_LOG"
    echo "• Metrics: $METRICS_CSV"
    echo ""
}

# Function to log performance metric
log_metric() {
    local timestamp="$1"
    local order_id="$2"
    local operation="$3"
    local duration="$4"
    local status="$5"
    local system="$6"
    
    echo "$timestamp,$order_id,$operation,$duration,$status,$system" >> "$METRICS_CSV"
    
    if $ENABLE_DETAILED_LOGGING; then
        echo "$(date -Iseconds) [$system] $operation for $order_id: ${duration}ms ($status)" >> "$PERFORMANCE_LOG"
    fi
}

# Function to log error
log_error() {
    local message="$1"
    local order_id="$2"
    local system="$3"
    
    echo "$(date -Iseconds) ERROR [$system] $message (Order: $order_id)" >> "$ERROR_LOG"
    
    # Update error counts
    ERROR_COUNTS["$system"]=$((${ERROR_COUNTS["$system"]:-0} + 1))
}

# Function to check service health with detailed reporting
check_services_detailed() {
    echo -e "${BLUE}Performing detailed service health checks...${NC}"
    
    local services=(
        "http://localhost:8080/health:Proxy"
        "http://localhost:8081/health:Order-Service" 
        "http://localhost:8082/health:SAP-Mock"
    )
    
    local all_healthy=true
    
    for service_info in "${services[@]}"; do
        IFS=':' read -r url name <<< "$service_info"
        echo -n "  $name: "
        
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "%{http_code}" "$url" 2>/dev/null)
        local end_time=$(date +%s%N)
        local duration=$(( (end_time - start_time) / 1000000 ))
        
        local http_code=${response: -3}
        
        if [ "$http_code" -eq 200 ]; then
            echo -e "${GREEN}✓ Healthy (${duration}ms)${NC}"
            log_metric "$(date -Iseconds)" "health-check" "health" "$duration" "success" "$name"
        else
            echo -e "${RED}✗ Unhealthy (HTTP $http_code)${NC}"
            log_error "Health check failed with HTTP $http_code" "health-check" "$name"
            all_healthy=false
        fi
    done
    
    if ! $all_healthy; then
        echo -e "${RED}Not all services are healthy. Please check service status.${NC}"
        exit 1
    fi
    
    echo ""
}

# Function to generate realistic test data
generate_realistic_order() {
    local order_num=$1
    local timestamp=$(date +%s)
    
    # Randomization based on order number for repeatability
    local customer_id="LOAD-CUST-$((order_num % 100 + 1))"
    local product_types=("WIDGET" "COMPONENT" "ASSEMBLY" "MODULE" "GADGET")
    local product_type=${product_types[$((order_num % ${#product_types[@]}))]}
    local product_id="$product_type-$((order_num % 50 + 1))"
    
    # Realistic quantities and prices
    local quantity=$((order_num % 5 + 1))
    local base_price=$((order_num % 100 + 20))
    local unit_price="${base_price}.99"
    local total_amount=$(echo "$quantity * $base_price" | bc).99
    
    # Realistic specifications
    local colors=("red" "blue" "green" "yellow" "black" "white")
    local sizes=("small" "medium" "large" "xl")
    local materials=("plastic" "metal" "wood" "composite")
    local priorities=("low" "normal" "high" "urgent")
    
    local color=${colors[$((order_num % ${#colors[@]}))]}
    local size=${sizes[$((order_num % ${#sizes[@]}))]}
    local material=${materials[$((order_num % ${#materials[@]}))]}
    local priority=${priorities[$((order_num % ${#priorities[@]}))]}
    
    # Future delivery date
    local delivery_day=$((order_num % 30 + 1))
    local delivery_date="2025-07-$(printf "%02d" $delivery_day)T00:00:00Z"
    
    cat <<EOF
{
  "id": "advanced-load-${timestamp}-${order_num}",
  "customer_id": "$customer_id",
  "items": [{
    "product_id": "$product_id",
    "quantity": $quantity,
    "unit_price": $unit_price,
    "specifications": {
      "color": "$color",
      "size": "$size", 
      "material": "$material",
      "priority": "$priority",
      "test_batch": "advanced-load-${timestamp}",
      "order_sequence": $order_num
    }
  }],
  "total_amount": $total_amount,
  "delivery_date": "$delivery_date"
}
EOF
}

# Function to send order with comprehensive logging
send_order_with_logging() {
    local order_num=$1
    local order_data=$(generate_realistic_order $order_num)
    local order_id=$(echo "$order_data" | jq -r '.id')
    local timestamp=$(date -Iseconds)
    
    # Send to proxy (dual-write)
    local start_time=$(date +%s%N)
    local response=$(curl -s -w "%{http_code}" -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "$order_data" 2>/dev/null)
    local end_time=$(date +%s%N)
    
    local http_code=${response: -3}
    local proxy_duration=$(( (end_time - start_time) / 1000000 ))
    
    if [ "$http_code" -eq 201 ]; then
        log_metric "$timestamp" "$order_id" "create" "$proxy_duration" "success" "proxy"
        echo "SUCCESS,$order_num,$order_id,$proxy_duration" >> "$RESULTS_LOG"
        
        # Measure individual service performance after a brief delay
        sleep 0.5
        measure_individual_services "$order_id" "$timestamp"
        
        if $ENABLE_REAL_TIME_STATS; then
            echo -e "${GREEN}Order $order_num: $proxy_duration ms${NC}"
        fi
    else
        log_metric "$timestamp" "$order_id" "create" "$proxy_duration" "failed" "proxy"
        log_error "Order creation failed with HTTP $http_code" "$order_id" "proxy"
        echo "FAILED,$order_num,$order_id,$proxy_duration,$http_code" >> "$RESULTS_LOG"
        
        if $ENABLE_REAL_TIME_STATS; then
            echo -e "${RED}Order $order_num: FAILED ($http_code)${NC}"
        fi
    fi
}

# Function to measure individual service performance
measure_individual_services() {
    local order_id="$1"
    local timestamp="$2"
    
    # Measure Order Service
    local start_time=$(date +%s%N)
    local os_response=$(curl -s -w "%{http_code}" "http://localhost:8081/orders/$order_id" 2>/dev/null)
    local end_time=$(date +%s%N)
    local os_duration=$(( (end_time - start_time) / 1000000 ))
    local os_http_code=${os_response: -3}
    
    if [ "$os_http_code" -eq 200 ]; then
        log_metric "$timestamp" "$order_id" "retrieve" "$os_duration" "success" "order-service"
        PERFORMANCE_METRICS["os_total"]=$((${PERFORMANCE_METRICS["os_total"]:-0} + os_duration))
        PERFORMANCE_METRICS["os_count"]=$((${PERFORMANCE_METRICS["os_count"]:-0} + 1))
    else
        log_metric "$timestamp" "$order_id" "retrieve" "$os_duration" "failed" "order-service"
        log_error "Order retrieval failed with HTTP $os_http_code" "$order_id" "order-service"
    fi
    
    # Measure SAP Mock
    start_time=$(date +%s%N)
    local sap_response=$(curl -s -w "%{http_code}" "http://localhost:8082/orders/$order_id" 2>/dev/null)
    end_time=$(date +%s%N)
    local sap_duration=$(( (end_time - start_time) / 1000000 ))
    local sap_http_code=${sap_response: -3}
    
    if [ "$sap_http_code" -eq 200 ]; then
        log_metric "$timestamp" "$order_id" "retrieve" "$sap_duration" "success" "sap-mock"
        PERFORMANCE_METRICS["sap_total"]=$((${PERFORMANCE_METRICS["sap_total"]:-0} + sap_duration))
        PERFORMANCE_METRICS["sap_count"]=$((${PERFORMANCE_METRICS["sap_count"]:-0} + 1))
    else
        log_metric "$timestamp" "$order_id" "retrieve" "$sap_duration" "failed" "sap-mock"
        log_error "Order retrieval failed with HTTP $sap_http_code" "$order_id" "sap-mock"
    fi
}

# Function to run the advanced load test
run_advanced_load_test() {
    echo -e "${BLUE}Starting advanced load test...${NC}"
    echo "Configuration: $ORDER_COUNT orders, $CONCURRENCY concurrency, $BATCH_SIZE batch size"
    echo ""
    
    START_TIME=$(date +%s)
    local pids=()
    local active_jobs=0
    local completed=0
    
    # Progress tracking
    if $ENABLE_REAL_TIME_STATS; then
        echo -e "${CYAN}Real-time results (Order #: Response Time):${NC}"
    else
        echo -n "Progress: "
    fi
    
    for ((i=1; i<=ORDER_COUNT; i++)); do
        # Send order in background
        send_order_with_logging $i &
        pids+=($!)
        ((active_jobs++))
        
        # Progress indicator
        if ! $ENABLE_REAL_TIME_STATS; then
            if [ $((i % 10)) -eq 0 ]; then
                echo -n "."
            fi
        fi
        
        # Control concurrency
        if [ $active_jobs -ge $CONCURRENCY ]; then
            # Wait for some jobs to complete
            for ((j=0; j<$BATCH_SIZE; j++)); do
                if [ ${#pids[@]} -gt 0 ]; then
                    wait ${pids[0]}
                    pids=("${pids[@]:1}")
                    ((active_jobs--))
                    ((completed++))
                    
                    if $ENABLE_REAL_TIME_STATS && [ $((completed % 10)) -eq 0 ]; then
                        echo -e "${YELLOW}Completed: $completed/$ORDER_COUNT${NC}"
                    fi
                fi
            done
        fi
        
        # Small delay to control load
        sleep 0.05
    done
    
    # Wait for all remaining jobs
    if ! $ENABLE_REAL_TIME_STATS; then
        echo ""
        echo -e "${YELLOW}Waiting for remaining orders to complete...${NC}"
    fi
    
    for pid in "${pids[@]}"; do
        wait $pid
    done
    
    END_TIME=$(date +%s)
    local total_duration=$((END_TIME - START_TIME))
    
    echo -e "${GREEN}✓ Advanced load test completed in ${total_duration}s${NC}"
    echo ""
}

# Function to generate comprehensive analysis
generate_comprehensive_analysis() {
    echo -e "${CYAN}Comprehensive Performance Analysis${NC}"
    echo "=================================="
    
    if [ ! -f "$RESULTS_LOG" ]; then
        echo -e "${RED}No results available for analysis${NC}"
        return
    fi
    
    # Basic statistics
    local successful_count=$(grep "^SUCCESS" "$RESULTS_LOG" | wc -l)
    local failed_count=$(grep "^FAILED" "$RESULTS_LOG" | wc -l)
    local success_rate=$(( successful_count * 100 / ORDER_COUNT ))
    local total_duration=$((END_TIME - START_TIME))
    local throughput=$(( successful_count / total_duration ))
    
    echo "Test Summary:"
    echo "• Duration: ${total_duration}s"
    echo "• Total Orders: $ORDER_COUNT"
    echo -e "• Successful: ${GREEN}$successful_count${NC} (${success_rate}%)"
    echo -e "• Failed: ${RED}$failed_count${NC}"
    echo "• Throughput: ~${throughput} orders/second"
    echo ""
    
    # Proxy performance analysis
    if [ $successful_count -gt 0 ]; then
        local proxy_times=($(grep "^SUCCESS" "$RESULTS_LOG" | cut -d',' -f4))
        local proxy_avg=$(( $(IFS=+; echo "$((${proxy_times[*]}))" ) / ${#proxy_times[@]} ))
        local proxy_min=$(printf '%s\n' "${proxy_times[@]}" | sort -n | head -1)
        local proxy_max=$(printf '%s\n' "${proxy_times[@]}" | sort -n | tail -1)
        
        echo "Proxy Performance (Dual-Write):"
        echo "• Average Response Time: ${proxy_avg}ms"
        echo "• Min Response Time: ${proxy_min}ms"
        echo "• Max Response Time: ${proxy_max}ms"
        
        # Calculate percentiles
        local sorted_times=($(printf '%s\n' "${proxy_times[@]}" | sort -n))
        local p50_idx=$(( ${#sorted_times[@]} * 50 / 100 ))
        local p95_idx=$(( ${#sorted_times[@]} * 95 / 100 ))
        local p99_idx=$(( ${#sorted_times[@]} * 99 / 100 ))
        
        echo "• P50: ${sorted_times[$p50_idx]}ms"
        echo "• P95: ${sorted_times[$p95_idx]}ms"
        echo "• P99: ${sorted_times[$p99_idx]}ms"
        echo ""
    fi
    
    # Individual service performance
    if [ ${PERFORMANCE_METRICS["os_count"]:-0} -gt 0 ] && [ ${PERFORMANCE_METRICS["sap_count"]:-0} -gt 0 ]; then
        local os_avg=$(( ${PERFORMANCE_METRICS["os_total"]} / ${PERFORMANCE_METRICS["os_count"]} ))
        local sap_avg=$(( ${PERFORMANCE_METRICS["sap_total"]} / ${PERFORMANCE_METRICS["sap_count"]} ))
        
        echo "Individual Service Performance:"
        echo "• Order Service (PostgreSQL): ${os_avg}ms average"
        echo "• SAP Mock (In-Memory): ${sap_avg}ms average"
        
        if [ $os_avg -lt $sap_avg ]; then
            local improvement=$(( (sap_avg - os_avg) * 100 / sap_avg ))
            echo -e "• ${GREEN}Order Service is ${improvement}% faster than SAP Mock${NC}"
        elif [ $os_avg -gt $sap_avg ]; then
            local degradation=$(( (os_avg - sap_avg) * 100 / sap_avg ))
            echo -e "• ${YELLOW}Order Service is ${degradation}% slower than SAP Mock${NC}"
        fi
        echo ""
    fi
    
    # Error analysis
    if [ ${#ERROR_COUNTS[@]} -gt 0 ]; then
        echo "Error Summary:"
        for system in "${!ERROR_COUNTS[@]}"; do
            echo "• $system: ${ERROR_COUNTS[$system]} errors"
        done
        echo ""
    fi
}

# Function to verify data consistency after load test
verify_data_consistency_comprehensive() {
    echo -e "${BLUE}Comprehensive Data Consistency Verification${NC}"
    echo "==========================================="
    
    # Wait for all async operations to complete
    echo "Waiting 5 seconds for all async operations to complete..."
    sleep 5
    
    # Use comparison endpoint
    echo "Fetching comparison data..."
    local comparison_result=$(curl -s http://localhost:8080/compare/orders 2>/dev/null)
    
    if [ $? -eq 0 ] && [ ! -z "$comparison_result" ]; then
        local sync_status=$(echo "$comparison_result" | jq -r '.analysis.sync_status // false')
        local os_count=$(echo "$comparison_result" | jq -r '.order_service.count // 0')
        local sap_count=$(echo "$comparison_result" | jq -r '.sap.count // 0')
        local missing_in_sap=$(echo "$comparison_result" | jq -r '.analysis.missing_in_sap | length // 0')
        local missing_in_os=$(echo "$comparison_result" | jq -r '.analysis.missing_in_order_service | length // 0')
        
        echo "Data Consistency Results:"
        echo "• Order Service Count: $os_count"
        echo "• SAP Mock Count: $sap_count"
        echo "• Missing in SAP: $missing_in_sap orders"
        echo "• Missing in Order Service: $missing_in_os orders"
        echo -e "• Systems Synchronized: $([ "$sync_status" = "true" ] && echo "${GREEN}✓ YES${NC}" || echo "${RED}✗ NO${NC}")"
        
        if [ "$sync_status" = "true" ]; then
            echo -e "${GREEN}✓ Perfect data consistency achieved${NC}"
        else
            echo -e "${RED}✗ Data consistency issues detected${NC}"
            echo "  This may indicate issues with the dual-write pattern implementation"
        fi
    else
        echo -e "${RED}✗ Could not fetch comparison data${NC}"
    fi
    
    echo ""
}

# Function to save comprehensive report
save_comprehensive_report() {
    local timestamp=$(date +%Y%m%d-%H%M%S)
    local report_file="advanced-load-test-report-$timestamp.txt"
    local csv_file="advanced-load-test-metrics-$timestamp.csv"
    
    echo -e "${BLUE}Saving comprehensive report...${NC}"
    
    # Save main report
    {
        echo "Advanced Strangler Pattern Load Test Report"
        echo "==========================================="
        echo "Timestamp: $(date)"
        echo "Scenario: ${SCENARIO:-custom}"
        echo "Configuration: $ORDER_COUNT orders, $CONCURRENCY concurrency, $BATCH_SIZE batch"
        echo "Duration: $((END_TIME - START_TIME))s"
        echo ""
        echo "=== RESULTS SUMMARY ==="
        cat "$RESULTS_LOG" 2>/dev/null | head -20
        echo ""
        echo "=== ERROR LOG ==="
        cat "$ERROR_LOG" 2>/dev/null
        echo ""
        echo "=== PERFORMANCE METRICS ==="
        echo "Order Service Average: ${PERFORMANCE_METRICS["os_total"]:-0} / ${PERFORMANCE_METRICS["os_count"]:-1} = $(( ${PERFORMANCE_METRICS["os_total"]:-0} / ${PERFORMANCE_METRICS["os_count"]:-1} ))ms"
        echo "SAP Mock Average: ${PERFORMANCE_METRICS["sap_total"]:-0} / ${PERFORMANCE_METRICS["sap_count"]:-1} = $(( ${PERFORMANCE_METRICS["sap_total"]:-0} / ${PERFORMANCE_METRICS["sap_count"]:-1} ))ms"
    } > "$report_file"
    
    # Copy metrics CSV
    cp "$METRICS_CSV" "$csv_file" 2>/dev/null
    
    echo -e "${GREEN}Reports saved:${NC}"
    echo "• Summary: $report_file"
    echo "• Metrics CSV: $csv_file"
    echo "• Temp files in: $TEMP_DIR (will be cleaned up)"
    echo ""
}

# Cleanup function
cleanup() {
    if [ ! -z "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        echo -e "${YELLOW}Cleaning up temporary files...${NC}"
        rm -rf "$TEMP_DIR"
    fi
}
trap cleanup EXIT

# Main execution
main() {
    # Parse command line arguments
    parse_arguments "$@"
    
    # Apply scenario if specified
    apply_scenario
    
    # Validate configuration
    if [ $ORDER_COUNT -lt 1 ] || [ $CONCURRENCY -lt 1 ] || [ $BATCH_SIZE -lt 1 ]; then
        echo -e "${RED}Invalid configuration parameters${NC}"
        exit 1
    fi
    
    # Check dependencies
    for cmd in curl jq bc; do
        if ! command -v $cmd &> /dev/null; then
            echo -e "${RED}Error: $cmd is required but not installed${NC}"
            exit 1
        fi
    done
    
    # Initialize logging
    initialize_logging
    
    # Show configuration
    echo -e "${CYAN}Advanced Load Test Configuration:${NC}"
    echo "• Orders: $ORDER_COUNT"
    echo "• Concurrency: $CONCURRENCY"
    echo "• Batch Size: $BATCH_SIZE"
    echo "• Detailed Logging: $ENABLE_DETAILED_LOGGING"
    echo "• Real-time Stats: $ENABLE_REAL_TIME_STATS"
    echo ""
    
    # Run the test
    check_services_detailed
    run_advanced_load_test
    generate_comprehensive_analysis
    verify_data_consistency_comprehensive
    save_comprehensive_report
    
    echo -e "${GREEN}Advanced load test completed successfully!${NC}"
    echo ""
    echo "Next steps:"
    echo "• Review the generated report files"
    echo "• Check Kafka UI at http://localhost:8090 for event streams"
    echo "• Run './scripts/performance-benchmark.sh' for detailed performance analysis"
}

# Run main function with all arguments
main "$@"