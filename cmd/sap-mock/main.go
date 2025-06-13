package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
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

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create order store
	store := NewSAPOrderStore()

	router := mux.NewRouter()
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/orders", createOrder(logger, store)).Methods("POST")
	router.HandleFunc("/orders", listOrders(logger, store)).Methods("GET")
	router.HandleFunc("/orders/{id}", getOrder(logger, store)).Methods("GET")

	port := "8082"
	logger.WithField("port", port).Info("Starting SAP mock server")
	
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.WithError(err).Fatal("Failed to start server")
	}
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