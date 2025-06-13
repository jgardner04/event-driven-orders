package models

import (
	"time"
)

type Order struct {
	ID           string      `json:"id"`
	CustomerID   string      `json:"customer_id"`
	Items        []OrderItem `json:"items"`
	TotalAmount  float64     `json:"total_amount"`
	DeliveryDate time.Time   `json:"delivery_date"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
}

type OrderItem struct {
	ProductID      string            `json:"product_id"`
	Quantity       int               `json:"quantity"`
	UnitPrice      float64           `json:"unit_price"`
	Specifications map[string]string `json:"specifications"`
}

type OrderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Order   *Order `json:"order,omitempty"`
}