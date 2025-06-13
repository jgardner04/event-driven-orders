package sap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Logger
}

func NewClient(baseURL string, logger *logrus.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) CreateOrder(order *models.Order) (*models.OrderResponse, error) {
	c.logger.WithField("order_id", order.ID).Info("Sending order to SAP")
	
	jsonData, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/orders", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to SAP: %w", err)
	}
	defer resp.Body.Close()

	var orderResp models.OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode SAP response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("SAP returned error status: %d", resp.StatusCode)
	}

	c.logger.WithFields(logrus.Fields{
		"order_id": order.ID,
		"status":   resp.StatusCode,
		"success":  orderResp.Success,
	}).Info("Received response from SAP")

	return &orderResp, nil
}

func (c *Client) GetOrders() ([]models.Order, error) {
	c.logger.Info("Fetching orders from SAP")
	
	req, err := http.NewRequest("GET", c.baseURL+"/orders", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to SAP: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Success bool            `json:"success"`
		Orders  []models.Order  `json:"orders"`
		Count   int             `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode SAP response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SAP returned error status: %d", resp.StatusCode)
	}

	c.logger.WithField("count", response.Count).Info("Retrieved orders from SAP")
	return response.Orders, nil
}

func (c *Client) GetOrder(orderID string) (*models.Order, error) {
	c.logger.WithField("order_id", orderID).Info("Fetching order from SAP")
	
	req, err := http.NewRequest("GET", c.baseURL+"/orders/"+orderID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to SAP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("order not found in SAP")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SAP returned error status: %d", resp.StatusCode)
	}

	var order models.Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("failed to decode SAP response: %w", err)
	}

	c.logger.WithField("order_id", orderID).Info("Retrieved order from SAP")
	return &order, nil
}