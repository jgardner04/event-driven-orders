package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jogardn/strangler-demo/internal/events"
	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

// In-memory storage for SAP orders
type SAPOrderStore struct {
	orders map[string]*models.Order
	mutex  sync.RWMutex
}

func NewSAPOrderStore() *SAPOrderStore {
	return &SAPOrderStore{
		orders: make(map[string]*models.Order),
	}
}

// Configuration for failure simulation
type SAPConfig struct {
	FailureRate    float64 // Percentage of requests to fail (0.0 to 1.0)
	SimulateOutage bool    // Simulate complete outage
}

var sapConfig = &SAPConfig{
	FailureRate:    0.0, // Default: no failures
	SimulateOutage: false,
}

// Implement RetryableOrderEventHandler interface
func (s *SAPOrderStore) HandleOrderCreated(event events.OrderCreatedEvent) error {
	// Check if we should simulate an outage
	if sapConfig.SimulateOutage {
		return fmt.Errorf("SAP system unavailable - simulated outage")
	}

	// Simulate random failures based on failure rate
	if sapConfig.FailureRate > 0 && rand.Float64() < sapConfig.FailureRate {
		return fmt.Errorf("SAP processing failed - simulated random failure")
	}

	// Simulate SAP processing delay
	delay := time.Duration(rand.Intn(2000)+1000) * time.Millisecond
	
	logger := logrus.New()
	logger.WithFields(logrus.Fields{
		"order_id": event.OrderID,
		"delay_ms": delay.Milliseconds(),
	}).Info("SAP processing order from Kafka event")
	
	time.Sleep(delay)

	// Simulate occasional processing errors
	if rand.Float64() < 0.05 { // 5% chance of processing error
		return fmt.Errorf("SAP internal processing error for order %s", event.OrderID)
	}

	// Create order from event data
	order := &models.Order{
		ID:          event.OrderID,
		CustomerID:  event.CustomerID,
		TotalAmount: event.TotalAmount,
		Status:      "confirmed",
		CreatedAt:   event.CreatedAt,
		Items:       []models.OrderItem{}, // Event only has summary data
	}

	// Store the order
	s.mutex.Lock()
	s.orders[order.ID] = order
	s.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"customer_id":  order.CustomerID,
		"total_amount": order.TotalAmount,
		"total_stored": len(s.orders),
	}).Info("Order processed from Kafka event and stored in SAP")

	return nil
}

// IsRetryable determines if an error should trigger a retry
func (s *SAPOrderStore) IsRetryable(err error) bool {
	// System unavailable errors are retryable
	if strings.Contains(err.Error(), "unavailable") {
		return true
	}
	
	// Temporary processing errors are retryable
	if strings.Contains(err.Error(), "processing error") {
		return true
	}
	
	// Random failures are retryable
	if strings.Contains(err.Error(), "random failure") {
		return true
	}
	
	// Other errors (e.g., data validation) are not retryable
	return false
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create order store
	store := NewSAPOrderStore()

	// Start Kafka consumer with retry logic
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	logger.WithField("brokers", kafkaBrokers).Info("Initializing Kafka consumer with retry support...")
	
	var consumer *events.KafkaConsumerWithRetry
	var err error
	
	// Retry connecting to Kafka
	for i := 0; i < 10; i++ {
		consumer, err = events.NewKafkaConsumerWithRetry(kafkaBrokers, "sap-consumer-group", store, logger)
		if err == nil {
			logger.Info("Successfully connected to Kafka with retry support")
			break
		}
		
		logger.WithError(err).WithField("attempt", i+1).Warn("Failed to connect to Kafka, retrying...")
		time.Sleep(5 * time.Second)
	}
	
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Kafka consumer after retries")
	}

	// Start consumer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		logger.WithField("brokers", kafkaBrokers).Info("Starting Kafka consumer for order events")
		if err := consumer.Start(ctx); err != nil {
			logger.WithError(err).Error("Kafka consumer error")
		}
	}()

	// Setup HTTP routes (keeping for backward compatibility and debugging)
	router := mux.NewRouter()
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/orders", createOrder(logger, store)).Methods("POST") // Legacy endpoint
	router.HandleFunc("/orders", listOrders(logger, store)).Methods("GET")
	router.HandleFunc("/orders/{id}", getOrder(logger, store)).Methods("GET")
	
	// Failure simulation endpoints
	router.HandleFunc("/admin/failure-rate", setFailureRate(logger)).Methods("POST")
	router.HandleFunc("/admin/simulate-outage", simulateOutage(logger)).Methods("POST")
	router.HandleFunc("/admin/metrics", getMetrics(logger, consumer)).Methods("GET")

	// Start HTTP server
	port := getEnv("SAP_PORT", "8082")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Handle graceful shutdown
	go func() {
		logger.WithField("port", port).Info("Starting SAP mock server (Phase 3: Event-driven)")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down SAP mock server...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("HTTP server forced to shutdown")
	}

	// Close Kafka consumer
	if err := consumer.Close(); err != nil {
		logger.WithError(err).Error("Failed to close Kafka consumer")
	}

	// Cancel consumer context
	cancel()

	logger.Info("SAP mock server gracefully stopped")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"service": "sap-mock",
	})
}

func createOrder(logger *logrus.Logger, store *SAPOrderStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var order models.Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			logger.WithError(err).Error("Failed to decode order")
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		delay := time.Duration(rand.Intn(2000)+1000) * time.Millisecond
		logger.WithFields(logrus.Fields{
			"order_id": order.ID,
			"delay_ms": delay.Milliseconds(),
		}).Info("Simulating SAP processing delay")
		
		time.Sleep(delay)

		order.Status = "confirmed"
		sapOrderID := fmt.Sprintf("SAP-%s", order.ID[:8])

		// Store the order in memory
		store.mutex.Lock()
		store.orders[order.ID] = &order
		store.mutex.Unlock()

		response := models.OrderResponse{
			Success: true,
			Message: fmt.Sprintf("Order created in SAP with ID: %s", sapOrderID),
			Order:   &order,
		}

		logger.WithFields(logrus.Fields{
			"order_id": order.ID,
			"total_stored": len(store.orders),
		}).Info("Order processed and stored successfully")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func listOrders(logger *logrus.Logger, store *SAPOrderStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store.mutex.RLock()
		orders := make([]*models.Order, 0, len(store.orders))
		for _, order := range store.orders {
			orders = append(orders, order)
		}
		store.mutex.RUnlock()

		logger.WithField("count", len(orders)).Info("Retrieved orders from SAP")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"orders":  orders,
			"count":   len(orders),
		})
	}
}

func getOrder(logger *logrus.Logger, store *SAPOrderStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		orderID := vars["id"]

		store.mutex.RLock()
		order, exists := store.orders[orderID]
		store.mutex.RUnlock()

		if !exists {
			logger.WithField("order_id", orderID).Warn("Order not found in SAP")
			respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}

		logger.WithField("order_id", orderID).Info("Retrieved order from SAP")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.OrderResponse{
		Success: false,
		Message: message,
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Admin endpoints for failure simulation
func setFailureRate(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FailureRate float64 `json:"failure_rate"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		
		if req.FailureRate < 0 || req.FailureRate > 1 {
			respondWithError(w, http.StatusBadRequest, "Failure rate must be between 0.0 and 1.0")
			return
		}
		
		sapConfig.FailureRate = req.FailureRate
		
		logger.WithField("failure_rate", req.FailureRate).Info("Updated SAP failure rate")
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"failure_rate": sapConfig.FailureRate,
			"message":      fmt.Sprintf("Failure rate set to %.2f%%", sapConfig.FailureRate*100),
		})
	}
}

func simulateOutage(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Outage bool `json:"outage"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		
		sapConfig.SimulateOutage = req.Outage
		
		logger.WithField("simulate_outage", req.Outage).Info("Updated SAP outage simulation")
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":        true,
			"simulate_outage": sapConfig.SimulateOutage,
			"message":        fmt.Sprintf("Outage simulation %s", map[bool]string{true: "enabled", false: "disabled"}[req.Outage]),
		})
	}
}

func getMetrics(logger *logrus.Logger, consumer *events.KafkaConsumerWithRetry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := consumer.GetMetrics()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"consumer_metrics": map[string]interface{}{
				"processed_count": metrics.ProcessedCount,
				"success_count":   metrics.SuccessCount,
				"failure_count":   metrics.FailureCount,
				"retry_count":     metrics.RetryCount,
				"dlq_count":       metrics.DLQCount,
			},
			"failure_config": map[string]interface{}{
				"failure_rate":    sapConfig.FailureRate,
				"simulate_outage": sapConfig.SimulateOutage,
			},
			"timestamp": time.Now(),
		})
	}
}