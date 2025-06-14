# API Documentation

## Base URLs

- **Proxy Service**: `http://localhost:8080`
- **Order Service**: `http://localhost:8081`
- **SAP Mock Service**: `http://localhost:8082` (internal only)

## Authentication

Currently no authentication is required. This is a demo application.

## Endpoints

### Order Management

#### Create Order

Creates a new order through the proxy service. In Phase 3, orders are sent to the Order Service only, and SAP receives them via Kafka events.

**Endpoint**: `POST /orders`

**Headers**:
- `Content-Type: application/json`

**Request Body**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `customer_id` | string | Yes | Unique customer identifier |
| `items` | array | Yes | List of order items |
| `total_amount` | number | Yes | Total order amount |
| `delivery_date` | string | Yes | ISO 8601 formatted delivery date |

**Order Item Structure**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `product_id` | string | Yes | Product identifier |
| `quantity` | integer | Yes | Quantity ordered |
| `unit_price` | number | Yes | Price per unit |
| `specifications` | object | No | Custom product specifications |

**Example Request**:

```json
{
  "customer_id": "CUST-12345",
  "items": [
    {
      "product_id": "WIDGET-001",
      "quantity": 10,
      "unit_price": 25.99,
      "specifications": {
        "color": "blue",
        "finish": "matte",
        "delivery": "standard"
      }
    },
    {
      "product_id": "COMPONENT-042",
      "quantity": 5,
      "unit_price": 149.99,
      "specifications": {
        "size": "large",
        "material": "aluminum"
      }
    }
  ],
  "total_amount": 1009.85,
  "delivery_date": "2025-06-20T00:00:00Z"
}
```

**Success Response** (201 Created):

```json
{
  "success": true,
  "message": "Order created in SAP with ID: SAP-550e8400",
  "order": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "customer_id": "CUST-12345",
    "items": [
      {
        "product_id": "WIDGET-001",
        "quantity": 10,
        "unit_price": 25.99,
        "specifications": {
          "color": "blue",
          "finish": "matte",
          "delivery": "standard"
        }
      },
      {
        "product_id": "COMPONENT-042",
        "quantity": 5,
        "unit_price": 149.99,
        "specifications": {
          "size": "large",
          "material": "aluminum"
        }
      }
    ],
    "total_amount": 1009.85,
    "delivery_date": "2025-06-20T00:00:00Z",
    "status": "confirmed",
    "created_at": "2025-06-13T10:30:00Z"
  }
}
```

**Error Responses**:

- **400 Bad Request**: Invalid request body or missing required fields
  ```json
  {
    "success": false,
    "message": "Invalid request body"
  }
  ```

- **500 Internal Server Error**: SAP connection failure or processing error
  ```json
  {
    "success": false,
    "message": "Failed to process order"
  }
  ```

### Data Comparison (Phase 2)

#### Compare All Orders

Compares all orders between the Order Service and SAP Mock to verify data consistency.

**Endpoint**: `GET /compare/orders`

**Response** (200 OK):
```json
{
  "timestamp": "2025-06-13T15:30:00Z",
  "order_service": {
    "count": 5,
    "orders": [...],
    "error": ""
  },
  "sap": {
    "count": 5,
    "orders": [...],
    "error": ""
  },
  "analysis": {
    "total_count_match": true,
    "order_service_count": 5,
    "sap_count": 5,
    "missing_in_sap": [],
    "missing_in_order_service": [],
    "common_orders": ["order-1", "order-2", "..."],
    "sync_status": true
  }
}
```

#### Compare Specific Order

Compares a specific order between both systems.

**Endpoint**: `GET /compare/orders/{id}`

**Response** (200 OK):
```json
{
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-06-13T15:30:00Z",
  "order_service": {
    "order": {...},
    "error": "",
    "found": true
  },
  "sap": {
    "order": {...},
    "error": "",
    "found": true
  },
  "analysis": {
    "perfect_match": true,
    "id_match": true,
    "customer_id_match": true,
    "total_amount_match": true,
    "status_match": true,
    "delivery_date_match": true,
    "created_at_match": true,
    "items_count_match": true
  }
}
```

### Health Checks

#### Proxy Health Check

Checks the health status of the proxy service.

**Endpoint**: `GET /health`

**Response** (200 OK):
```json
{
  "status": "healthy",
  "service": "proxy"
}
```

#### All Services Health Check

Checks the health status of all services through the proxy.

**Endpoint**: `GET /api/health/all`

**Response** (200 OK):
```json
{
  "proxy": {
    "status": "healthy",
    "service": "proxy",
    "response_time": 15,
    "last_check": "2025-06-14T10:30:00Z"
  },
  "order_service": {
    "status": "healthy", 
    "service": "order_service",
    "response_time": 25,
    "last_check": "2025-06-14T10:30:00Z"
  },
  "sap_mock": {
    "status": "healthy",
    "service": "sap_mock", 
    "response_time": 150,
    "last_check": "2025-06-14T10:30:00Z"
  }
}
```

#### Get Orders

Retrieves all orders from the order service through the proxy.

**Endpoint**: `GET /orders`

**Headers**:
- `Cache-Control: no-cache, no-store, must-revalidate` (response)

**Response** (200 OK):
```json
{
  "success": true,
  "orders": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "customer_id": "CUST-12345",
      "items": [...],
      "total_amount": 1009.85,
      "delivery_date": "2025-06-20T00:00:00Z",
      "status": "confirmed",
      "created_at": "2025-06-13T10:30:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2025-06-14T10:30:00Z"
}
```

#### SAP Mock Health Check

Checks the health status of the SAP mock service (internal use).

**Endpoint**: `GET /health`

**Response** (200 OK):
```json
{
  "status": "healthy",
  "service": "sap-mock"
}
```

### WebSocket Real-Time Updates

Real-time WebSocket connection for receiving live updates about orders, metrics, and health status.

**Endpoint**: `WS /ws`

**Protocol**: WebSocket (ws:// or wss://)

**Connection URL**: `ws://localhost:8080/ws`

#### Supported Message Types

**1. Order Created Events**:
```json
{
  "type": "order_created",
  "order": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "customer_id": "CUST-12345",
    "items": [...],
    "total_amount": 1009.85,
    "status": "pending",
    "created_at": "2025-06-14T10:30:00Z"
  },
  "source": "proxy",
  "processing_time": 45
}
```

**2. Metrics Updates**:
```json
{
  "type": "metrics_update",
  "timestamp": "2025-06-14T10:30:00Z",
  "proxy": {
    "requests_per_second": 75,
    "avg_response_time": 125,
    "error_rate": 1.2,
    "active_connections": 25
  },
  "order_service": {
    "orders_created": 1250,
    "avg_processing_time": 45,
    "database_connections": 8,
    "kafka_events_published": 1250
  }
}
```

**3. Health Status Updates**:
```json
{
  "type": "health_update",
  "proxy": {
    "status": "healthy",
    "response_time": 15
  },
  "order_service": {
    "status": "healthy", 
    "response_time": 25
  },
  "sap_mock": {
    "status": "healthy",
    "response_time": 150
  }
}
```

#### Client Connection Example (JavaScript)

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  console.log('Connected to real-time updates');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data.type, data);
  
  switch(data.type) {
    case 'order_created':
      updateOrdersList(data.order);
      break;
    case 'metrics_update':
      updateMetrics(data);
      break;
    case 'health_update':
      updateHealthStatus(data);
      break;
  }
};

ws.onclose = () => {
  console.log('WebSocket connection closed');
};
```

## Response Times

- **Proxy Processing**: < 50ms
- **SAP Mock Delay**: 1-3 seconds (simulated)
- **Total Response Time**: 1-3 seconds
- **WebSocket Messages**: < 10ms

## Rate Limiting

No rate limiting is currently implemented.

## Error Handling

All errors follow a consistent format:

```json
{
  "success": false,
  "message": "Human-readable error description"
}
```

## Logging

The proxy service logs all requests with the following information:
- Request method and path
- Order ID and customer ID
- Processing duration
- Response status

Example log entry:
```json
{
  "level": "info",
  "method": "POST",
  "path": "/orders",
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "CUST-12345",
  "total_amount": 1009.85,
  "items_count": 2,
  "duration": 2156,
  "msg": "Request completed",
  "time": "2025-06-13T10:30:02Z"
}
```

## Phase 3 Event-Driven Architecture

In Phase 3, the architecture has evolved to complete event-driven decoupling:

### Order Flow

1. **Client → Proxy**: Order request sent to proxy
2. **Proxy → Order Service**: Proxy forwards to Order Service only (no SAP call)
3. **Order Service → PostgreSQL**: Order saved to database
4. **Order Service → Kafka**: `order.created` event published
5. **Kafka → SAP Mock**: SAP consumes event and processes order asynchronously

### Key Changes from Phase 2

- **No direct SAP calls**: Proxy no longer communicates directly with SAP
- **Event consumption**: SAP Mock implements Kafka consumer
- **Asynchronous processing**: Orders reach SAP via events, not HTTP
- **Complete decoupling**: Systems are fully independent

### Kafka Event Structure

**Topic**: `order.created`

**Event Payload**:
```json
{
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "CUST-12345",
  "total_amount": 259.90,
  "created_at": "2025-06-13T10:30:00Z",
  "event_time": "2025-06-13T10:30:02Z"
}
```

## Testing

### Using cURL

Create an order:
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "CUST-12345",
    "items": [{
      "product_id": "WIDGET-001",
      "quantity": 10,
      "unit_price": 25.99
    }],
    "total_amount": 259.90,
    "delivery_date": "2025-06-20T00:00:00Z"
  }'
```

Compare all orders:
```bash
curl http://localhost:8080/compare/orders | jq .
```

Compare specific order:
```bash
curl http://localhost:8080/compare/orders/{order-id} | jq .
```

List orders from Order Service:
```bash
curl http://localhost:8081/orders | jq .
```

List orders from SAP Mock:
```bash
curl http://localhost:8082/orders | jq .
```

Check health:
```bash
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health
```

### Using the Test Scripts

**Basic functionality test:**
```bash
./scripts/test-order.sh
```

**Data comparison and verification:**
```bash
./scripts/compare-data.sh
./scripts/demo-comparison.sh
```

**Performance testing:**
```bash
./scripts/performance-benchmark.sh
```

**Load testing:**
```bash
# Basic load test (50 orders, 10 concurrent)
./scripts/load-test.sh

# Custom load test
./scripts/load-test.sh 100 20 5  # 100 orders, 20 concurrent, 5 batch

# Advanced load test with scenarios
./scripts/advanced-load-test.sh -s medium    # Predefined scenario
./scripts/advanced-load-test.sh -c 200 -p 20 # Custom configuration
./scripts/advanced-load-test.sh -s heavy -d  # Heavy load with detailed logging
```

**Available load test scenarios:**
- `light`: 20 orders, 5 concurrent (development)
- `medium`: 100 orders, 15 concurrent (integration)
- `heavy`: 500 orders, 25 concurrent (performance)
- `stress`: 1000 orders, 50 concurrent (stress testing)