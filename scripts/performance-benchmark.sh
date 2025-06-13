#!/bin/bash

echo "Strangler Pattern Performance Benchmark"
echo "======================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Performance test configuration
WARMUP_REQUESTS=5
BENCHMARK_REQUESTS=20
SAMPLE_ORDER_ID=""

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
            echo -e "${RED}Error: $name is not healthy${NC}"
            exit 1
        fi
    done
    echo ""
}

# Function to create a sample order and get its ID
create_sample_order() {
    echo -e "${BLUE}Creating sample order for benchmarking...${NC}"
    
    local order_data='{
        "customer_id": "BENCHMARK-CUSTOMER",
        "items": [{
            "product_id": "BENCHMARK-WIDGET",
            "quantity": 5,
            "unit_price": 29.99,
            "specifications": {
                "benchmark": "true",
                "created_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
            }
        }],
        "total_amount": 149.95,
        "delivery_date": "2025-07-15T00:00:00Z"
    }'
    
    local response=$(curl -s -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "$order_data")
    
    SAMPLE_ORDER_ID=$(echo "$response" | jq -r '.order.id')
    
    if [ "$SAMPLE_ORDER_ID" != "null" ] && [ ! -z "$SAMPLE_ORDER_ID" ]; then
        echo -e "${GREEN}✓ Sample order created: $SAMPLE_ORDER_ID${NC}"
        # Wait for order to be processed
        sleep 2
    else
        echo -e "${RED}✗ Failed to create sample order${NC}"
        exit 1
    fi
    echo ""
}

# Function to benchmark order creation (dual-write through proxy)
benchmark_order_creation() {
    echo -e "${BLUE}Benchmarking Order Creation (Proxy - Dual Write)${NC}"
    echo "================================================="
    
    local times=()
    
    # Warmup
    echo "Warming up with $WARMUP_REQUESTS requests..."
    for ((i=1; i<=WARMUP_REQUESTS; i++)); do
        local order_data='{
            "customer_id": "WARMUP-'$i'",
            "items": [{"product_id": "WARMUP-'$i'", "quantity": 1, "unit_price": 10.99}],
            "total_amount": 10.99,
            "delivery_date": "2025-07-20T00:00:00Z"
        }'
        
        curl -s -X POST http://localhost:8080/orders \
            -H "Content-Type: application/json" \
            -d "$order_data" > /dev/null
    done
    
    echo "Running $BENCHMARK_REQUESTS benchmark requests..."
    echo ""
    
    # Actual benchmark
    for ((i=1; i<=BENCHMARK_REQUESTS; i++)); do
        local order_data='{
            "customer_id": "BENCH-'$i'",
            "items": [{"product_id": "BENCH-'$i'", "quantity": 2, "unit_price": 25.99}],
            "total_amount": 51.98,
            "delivery_date": "2025-07-25T00:00:00Z"
        }'
        
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "%{http_code}" -X POST http://localhost:8080/orders \
            -H "Content-Type: application/json" \
            -d "$order_data")
        local end_time=$(date +%s%N)
        
        local http_code=${response: -3}
        local duration=$(( (end_time - start_time) / 1000000 ))
        
        if [ "$http_code" -eq 201 ]; then
            times+=($duration)
            echo -n "."
        else
            echo -n "E"
        fi
    done
    
    echo ""
    echo ""
    
    # Calculate statistics
    if [ ${#times[@]} -gt 0 ]; then
        local sum=0
        local min=${times[0]}
        local max=${times[0]}
        
        for time in "${times[@]}"; do
            sum=$((sum + time))
            [ $time -lt $min ] && min=$time
            [ $time -gt $max ] && max=$time
        done
        
        local avg=$((sum / ${#times[@]}))
        local successful=${#times[@]}
        local failed=$((BENCHMARK_REQUESTS - successful))
        
        echo "Order Creation Results (Proxy - Dual Write):"
        echo "• Successful: $successful/$BENCHMARK_REQUESTS"
        echo "• Failed: $failed"
        echo "• Average: ${avg}ms"
        echo "• Min: ${min}ms"
        echo "• Max: ${max}ms"
        
        # Calculate percentiles
        local sorted_times=($(printf '%s\n' "${times[@]}" | sort -n))
        local p50_idx=$(( ${#sorted_times[@]} * 50 / 100 ))
        local p95_idx=$(( ${#sorted_times[@]} * 95 / 100 ))
        local p99_idx=$(( ${#sorted_times[@]} * 99 / 100 ))
        
        echo "• P50: ${sorted_times[$p50_idx]}ms"
        echo "• P95: ${sorted_times[$p95_idx]}ms"
        echo "• P99: ${sorted_times[$p99_idx]}ms"
    else
        echo -e "${RED}No successful requests recorded${NC}"
    fi
    
    echo ""
}

# Function to benchmark order retrieval from Order Service
benchmark_order_service_retrieval() {
    echo -e "${BLUE}Benchmarking Order Service Retrieval${NC}"
    echo "===================================="
    
    if [ -z "$SAMPLE_ORDER_ID" ]; then
        echo -e "${RED}No sample order ID available${NC}"
        return
    fi
    
    local times=()
    
    echo "Measuring retrieval time for order: $SAMPLE_ORDER_ID"
    echo "Running $BENCHMARK_REQUESTS requests..."
    echo ""
    
    for ((i=1; i<=BENCHMARK_REQUESTS; i++)); do
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "%{http_code}" "http://localhost:8081/orders/$SAMPLE_ORDER_ID")
        local end_time=$(date +%s%N)
        
        local http_code=${response: -3}
        local duration=$(( (end_time - start_time) / 1000000 ))
        
        if [ "$http_code" -eq 200 ]; then
            times+=($duration)
            echo -n "."
        else
            echo -n "E"
        fi
        
        # Small delay to avoid overwhelming
        sleep 0.05
    done
    
    echo ""
    echo ""
    
    # Calculate statistics
    if [ ${#times[@]} -gt 0 ]; then
        local sum=0
        local min=${times[0]}
        local max=${times[0]}
        
        for time in "${times[@]}"; do
            sum=$((sum + time))
            [ $time -lt $min ] && min=$time
            [ $time -gt $max ] && max=$time
        done
        
        local avg=$((sum / ${#times[@]}))
        
        echo "Order Service Retrieval Results:"
        echo "• Average: ${avg}ms"
        echo "• Min: ${min}ms"
        echo "• Max: ${max}ms"
        
        # Store for comparison
        OS_AVG=$avg
        OS_MIN=$min
        OS_MAX=$max
    else
        echo -e "${RED}No successful requests recorded${NC}"
    fi
    
    echo ""
}

# Function to benchmark order retrieval from SAP Mock
benchmark_sap_retrieval() {
    echo -e "${BLUE}Benchmarking SAP Mock Retrieval${NC}"
    echo "==============================="
    
    if [ -z "$SAMPLE_ORDER_ID" ]; then
        echo -e "${RED}No sample order ID available${NC}"
        return
    fi
    
    local times=()
    
    echo "Measuring retrieval time for order: $SAMPLE_ORDER_ID"
    echo "Running $BENCHMARK_REQUESTS requests..."
    echo ""
    
    for ((i=1; i<=BENCHMARK_REQUESTS; i++)); do
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "%{http_code}" "http://localhost:8082/orders/$SAMPLE_ORDER_ID")
        local end_time=$(date +%s%N)
        
        local http_code=${response: -3}
        local duration=$(( (end_time - start_time) / 1000000 ))
        
        if [ "$http_code" -eq 200 ]; then
            times+=($duration)
            echo -n "."
        else
            echo -n "E"
        fi
        
        # Small delay to avoid overwhelming
        sleep 0.05
    done
    
    echo ""
    echo ""
    
    # Calculate statistics
    if [ ${#times[@]} -gt 0 ]; then
        local sum=0
        local min=${times[0]}
        local max=${times[0]}
        
        for time in "${times[@]}"; do
            sum=$((sum + time))
            [ $time -lt $min ] && min=$time
            [ $time -gt $max ] && max=$time
        done
        
        local avg=$((sum / ${#times[@]}))
        
        echo "SAP Mock Retrieval Results:"
        echo "• Average: ${avg}ms"
        echo "• Min: ${min}ms"
        echo "• Max: ${max}ms"
        
        # Store for comparison
        SAP_AVG=$avg
        SAP_MIN=$min
        SAP_MAX=$max
    else
        echo -e "${RED}No successful requests recorded${NC}"
    fi
    
    echo ""
}

# Function to benchmark comparison endpoint
benchmark_comparison_endpoint() {
    echo -e "${BLUE}Benchmarking Comparison Endpoint${NC}"
    echo "================================"
    
    local times=()
    
    echo "Measuring comparison endpoint performance..."
    echo "Running $BENCHMARK_REQUESTS requests..."
    echo ""
    
    for ((i=1; i<=BENCHMARK_REQUESTS; i++)); do
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "%{http_code}" "http://localhost:8080/compare/orders")
        local end_time=$(date +%s%N)
        
        local http_code=${response: -3}
        local duration=$(( (end_time - start_time) / 1000000 ))
        
        if [ "$http_code" -eq 200 ]; then
            times+=($duration)
            echo -n "."
        else
            echo -n "E"
        fi
        
        # Small delay
        sleep 0.1
    done
    
    echo ""
    echo ""
    
    # Calculate statistics
    if [ ${#times[@]} -gt 0 ]; then
        local sum=0
        local min=${times[0]}
        local max=${times[0]}
        
        for time in "${times[@]}"; do
            sum=$((sum + time))
            [ $time -lt $min ] && min=$time
            [ $time -gt $max ] && max=$time
        done
        
        local avg=$((sum / ${#times[@]}))
        
        echo "Comparison Endpoint Results:"
        echo "• Average: ${avg}ms"
        echo "• Min: ${min}ms"
        echo "• Max: ${max}ms"
    else
        echo -e "${RED}No successful requests recorded${NC}"
    fi
    
    echo ""
}

# Function to generate performance comparison
generate_performance_comparison() {
    echo -e "${CYAN}Performance Comparison Summary${NC}"
    echo "=============================="
    
    if [ ! -z "$OS_AVG" ] && [ ! -z "$SAP_AVG" ]; then
        echo "Retrieval Performance Comparison:"
        echo "• Order Service (PostgreSQL): ${OS_AVG}ms avg (${OS_MIN}-${OS_MAX}ms)"
        echo "• SAP Mock (In-Memory): ${SAP_AVG}ms avg (${SAP_MIN}-${SAP_MAX}ms)"
        echo ""
        
        if [ $OS_AVG -lt $SAP_AVG ]; then
            local improvement=$(( (SAP_AVG - OS_AVG) * 100 / SAP_AVG ))
            echo -e "${GREEN}✓ Order Service is ${improvement}% faster than SAP Mock${NC}"
            echo -e "  Speedup: ${SAP_AVG}ms → ${OS_AVG}ms"
        elif [ $OS_AVG -gt $SAP_AVG ]; then
            local degradation=$(( (OS_AVG - SAP_AVG) * 100 / SAP_AVG ))
            echo -e "${YELLOW}⚠ Order Service is ${degradation}% slower than SAP Mock${NC}"
            echo -e "  Change: ${SAP_AVG}ms → ${OS_AVG}ms"
        else
            echo -e "${BLUE}→ Order Service and SAP Mock have similar performance${NC}"
        fi
    else
        echo "Insufficient data for comparison"
    fi
    
    echo ""
    echo "Key Insights:"
    echo "• Dual-write pattern adds latency but ensures data consistency"
    echo "• Order Service (PostgreSQL) provides persistent, queryable storage"
    echo "• SAP Mock (In-Memory) is faster but data is volatile"
    echo "• Comparison endpoint provides real-time consistency verification"
    echo ""
    
    # Save results to file
    local timestamp=$(date +%Y%m%d-%H%M%S)
    local report_file="performance-benchmark-$timestamp.txt"
    
    {
        echo "Strangler Pattern Performance Benchmark Report"
        echo "=============================================="
        echo "Timestamp: $(date)"
        echo "Configuration: $BENCHMARK_REQUESTS requests per test"
        echo ""
        echo "Order Service Average: ${OS_AVG:-N/A}ms"
        echo "SAP Mock Average: ${SAP_AVG:-N/A}ms"
        echo ""
        echo "Test Details:"
        echo "- Warmup requests: $WARMUP_REQUESTS"
        echo "- Benchmark requests: $BENCHMARK_REQUESTS"
        echo "- Sample Order ID: $SAMPLE_ORDER_ID"
    } > "$report_file"
    
    echo -e "${GREEN}Detailed report saved to: $report_file${NC}"
}

# Main execution
main() {
    echo -e "${CYAN}This benchmark measures the performance characteristics of the strangler pattern implementation.${NC}"
    echo ""
    
    # Check dependencies
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: curl is required${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is required${NC}"
        exit 1
    fi
    
    # Run benchmarks
    check_services
    create_sample_order
    benchmark_order_creation
    benchmark_order_service_retrieval
    benchmark_sap_retrieval
    benchmark_comparison_endpoint
    generate_performance_comparison
    
    echo -e "${GREEN}Performance benchmark completed!${NC}"
    echo ""
    echo "For load testing, run: ./scripts/load-test.sh"
    echo "For data verification, run: ./scripts/compare-data.sh"
}

# Global variables for comparison
OS_AVG=""
OS_MIN=""
OS_MAX=""
SAP_AVG=""
SAP_MIN=""
SAP_MAX=""

# Run main function
main "$@"