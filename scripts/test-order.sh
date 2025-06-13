#!/bin/bash

echo "Testing Order Creation through Proxy..."
echo "======================================"

curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
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
  }' | jq .