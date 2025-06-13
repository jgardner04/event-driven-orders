# Load Testing Guide for Strangler Pattern Demo

## Overview

This document provides comprehensive guidance for load testing the strangler pattern implementation, measuring performance differences between the new Order Service and SAP Mock, and verifying data consistency under load.

## Available Load Testing Scripts

### 1. Basic Load Test (`load-test.sh`)

**Purpose**: Simple load testing with concurrent order creation and basic verification.

**Usage**:
```bash
./scripts/load-test.sh [ORDER_COUNT] [CONCURRENCY] [BATCH_SIZE]
```

**Examples**:
```bash
./scripts/load-test.sh                # Default: 50 orders, 10 concurrent
./scripts/load-test.sh 100           # 100 orders, default concurrency
./scripts/load-test.sh 100 20        # 100 orders, 20 concurrent
./scripts/load-test.sh 100 20 10     # 100 orders, 20 concurrent, 10 batch size
```

**Features**:
- Concurrent order creation through proxy
- Real-time progress tracking
- Performance measurement for all systems
- Data consistency verification
- Detailed reporting with throughput metrics

### 2. Advanced Load Test (`advanced-load-test.sh`)

**Purpose**: Comprehensive load testing with predefined scenarios, detailed logging, and advanced analytics.

**Usage**:
```bash
./scripts/advanced-load-test.sh [OPTIONS] [SCENARIO]
```

**Options**:
- `-c, --count NUM`: Number of orders to create
- `-p, --concurrency NUM`: Number of concurrent requests
- `-b, --batch NUM`: Batch size for concurrency control
- `-s, --scenario NAME`: Use predefined scenario
- `-d, --detailed`: Enable detailed logging
- `-r, --real-time`: Enable real-time statistics
- `-h, --help`: Show help message

**Scenarios**:
- `light`: 20 orders, 5 concurrent (development testing)
- `medium`: 100 orders, 15 concurrent (integration testing)
- `heavy`: 500 orders, 25 concurrent (performance testing)
- `stress`: 1000 orders, 50 concurrent (stress testing)

**Examples**:
```bash
./scripts/advanced-load-test.sh -s medium              # Medium scenario
./scripts/advanced-load-test.sh -c 200 -p 20          # Custom config
./scripts/advanced-load-test.sh -s heavy -d -r        # Heavy with detailed logging
./scripts/advanced-load-test.sh --help                # Show all options
```

### 3. Performance Benchmark (`performance-benchmark.sh`)

**Purpose**: Focused performance measurement and comparison between services.

**Usage**:
```bash
./scripts/performance-benchmark.sh
```

**Features**:
- Dedicated performance measurement for each service
- Statistical analysis (min, max, average, percentiles)
- Side-by-side comparison of Order Service vs SAP Mock
- Response time distribution analysis
- Comparison endpoint performance testing

## Test Scenarios

### Development Testing (Light Load)
```bash
./scripts/advanced-load-test.sh -s light
```
- **Purpose**: Quick verification during development
- **Load**: 20 orders, 5 concurrent
- **Expected Duration**: ~30 seconds
- **Use Case**: Feature testing, debugging

### Integration Testing (Medium Load)
```bash
./scripts/advanced-load-test.sh -s medium
```
- **Purpose**: Integration pipeline testing
- **Load**: 100 orders, 15 concurrent
- **Expected Duration**: ~2 minutes
- **Use Case**: CI/CD validation, regression testing

### Performance Testing (Heavy Load)
```bash
./scripts/advanced-load-test.sh -s heavy -d
```
- **Purpose**: Performance characteristics under load
- **Load**: 500 orders, 25 concurrent
- **Expected Duration**: ~5-10 minutes
- **Use Case**: Performance benchmarking, optimization

### Stress Testing (Maximum Load)
```bash
./scripts/advanced-load-test.sh -s stress -d -r
```
- **Purpose**: Find system limits and breaking points
- **Load**: 1000 orders, 50 concurrent
- **Expected Duration**: ~10-20 minutes
- **Use Case**: Capacity planning, system limits

## Performance Metrics Measured

### 1. Order Creation Performance (Proxy - Dual Write)
- **Average Response Time**: Mean time for proxy to handle order creation
- **Percentiles**: P50, P95, P99 response times
- **Throughput**: Orders processed per second
- **Success Rate**: Percentage of successful order creations
- **Error Rate**: Failed requests and error types

### 2. Order Service Performance (PostgreSQL)
- **Retrieval Time**: Time to fetch orders from PostgreSQL
- **Database Performance**: Query execution time
- **Concurrent Load Handling**: Performance under concurrent access
- **Data Integrity**: Verification of stored data

### 3. SAP Mock Performance (In-Memory)
- **Retrieval Time**: Time to fetch orders from in-memory storage
- **Simulated Delay**: Artificial 1-3 second processing delay
- **Memory Usage**: Performance with large datasets
- **Concurrency Handling**: Thread-safe access performance

### 4. Comparison Endpoint Performance
- **Analysis Time**: Time to compare data between systems
- **Scalability**: Performance with increasing order counts
- **Accuracy**: Correctness of consistency analysis

## Key Performance Indicators (KPIs)

### Response Time Thresholds
- **Order Service**: < 100ms (warning), < 500ms (critical)
- **SAP Mock**: < 2000ms (warning), < 4000ms (critical)
- **Proxy (Dual-Write)**: < 3000ms (warning), < 5000ms (critical)

### Success Rate Targets
- **Order Creation**: > 95% (warning), > 90% (critical)
- **Data Consistency**: 100% (must be perfect)

### Throughput Targets
- **Development**: > 5 orders/second
- **Integration**: > 10 orders/second
- **Production**: > 20 orders/second

## Expected Performance Characteristics

### Order Service vs SAP Mock Comparison

**Order Service (PostgreSQL)**:
- ✅ **Pros**: Persistent storage, ACID compliance, real-world performance
- ⚠️ **Cons**: Database overhead, network latency
- 📊 **Expected**: 50-200ms average response time

**SAP Mock (In-Memory)**:
- ✅ **Pros**: Fast in-memory access, no database overhead
- ⚠️ **Cons**: Simulated 1-3s delay, volatile storage
- 📊 **Expected**: 1000-3000ms average response time (due to artificial delay)

**Typical Results**:
```
Order Service:     ~80ms average  (without artificial delay)
SAP Mock:         ~1500ms average (with 1-3s artificial delay)
Proxy (Combined): ~1600ms average (sum of both + overhead)
```

## Data Consistency Verification

### Automatic Verification
All load tests automatically verify:
- **Order Count Matching**: Both systems have same number of orders
- **Data Synchronization**: All orders exist in both systems
- **Field Accuracy**: Order data matches exactly between systems
- **No Data Loss**: Every successful proxy call results in data in both systems

### Verification Process
1. **During Test**: Each order is verified in both systems
2. **Post-Test**: Comprehensive comparison using `/compare/orders` endpoint
3. **Sampling**: Individual order verification for data accuracy
4. **Reporting**: Detailed consistency report with any discrepancies

## Interpreting Results

### Success Indicators
- ✅ **High Success Rate**: > 95% of orders created successfully
- ✅ **Data Consistency**: sync_status = true in comparison results
- ✅ **Acceptable Performance**: Response times within thresholds
- ✅ **No Errors**: Minimal errors in error logs

### Warning Signs
- ⚠️ **Degraded Performance**: Response times approaching thresholds
- ⚠️ **Increased Errors**: > 5% failure rate
- ⚠️ **Data Lag**: Temporary inconsistencies that resolve quickly

### Critical Issues
- 🚨 **High Failure Rate**: > 10% order creation failures
- 🚨 **Data Inconsistency**: Orders missing from one system
- 🚨 **Service Unavailability**: Health check failures
- 🚨 **Performance Collapse**: Response times > critical thresholds

## Report Files Generated

### Basic Load Test Output
- `load-test-report-YYYYMMDD-HHMMSS.txt`: Summary report
- Console output with real-time statistics

### Advanced Load Test Output
- `advanced-load-test-report-YYYYMMDD-HHMMSS.txt`: Comprehensive report
- `advanced-load-test-metrics-YYYYMMDD-HHMMSS.csv`: Detailed metrics
- Real-time logging files (when detailed logging enabled)

### Performance Benchmark Output
- `performance-benchmark-YYYYMMDD-HHMMSS.txt`: Benchmark results
- Console output with statistical analysis

## Troubleshooting Load Tests

### Common Issues

**1. Services Not Responding**
```bash
# Check service health
curl http://localhost:8080/health
curl http://localhost:8081/health  
curl http://localhost:8082/health

# Restart services if needed
docker-compose restart
```

**2. High Error Rates**
- Check service logs: `docker-compose logs [service]`
- Reduce concurrency: Use lower `-p` values
- Check resource limits: CPU, memory, disk space

**3. Slow Performance**
- Verify system resources: `docker stats`
- Check database connections: `docker-compose logs postgres`
- Monitor Kafka: Check Kafka UI at http://localhost:8090

**4. Data Inconsistency**
- Wait for async operations: Add delays between test and verification
- Check Kafka events: Verify events are being published
- Manual verification: Use comparison endpoints directly

### Resource Requirements

**Minimum System Requirements**:
- CPU: 2 cores
- RAM: 4GB
- Disk: 10GB free space
- Network: Stable local networking

**Recommended for Heavy Testing**:
- CPU: 4+ cores
- RAM: 8GB+
- Disk: 20GB+ free space
- Docker: Latest version with adequate resource allocation

## Best Practices

### Before Testing
1. **Clean Environment**: Restart services and clear data
2. **Resource Check**: Ensure adequate system resources
3. **Baseline**: Run light test first to verify functionality
4. **Monitoring**: Have monitoring tools ready (Kafka UI, docker stats)

### During Testing
1. **Monitor Resources**: Watch CPU, memory, disk usage
2. **Check Logs**: Monitor service logs for errors
3. **Progressive Load**: Start with light load, increase gradually
4. **Document Issues**: Note any anomalies or performance changes

### After Testing
1. **Analyze Reports**: Review generated report files
2. **Compare Results**: Compare with previous test runs
3. **Investigate Issues**: Follow up on any failures or inconsistencies
4. **Clean Up**: Remove test data if needed

## Integration with CI/CD

### Automated Testing Pipeline
```bash
# Example CI/CD integration
./scripts/advanced-load-test.sh -s light    # Quick smoke test
./scripts/advanced-load-test.sh -s medium   # Integration verification
./scripts/compare-data.sh                   # Data consistency check
```

### Exit Codes
- `0`: All tests passed successfully
- `1`: Service health check failures
- `2`: High error rate or data inconsistency
- `3`: Performance thresholds exceeded

This comprehensive load testing suite provides confidence that the strangler pattern implementation maintains data consistency and acceptable performance under various load conditions.