package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	router := mux.NewRouter()
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/orders", createOrder(logger)).Methods("POST")

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

func createOrder(logger *logrus.Logger) http.HandlerFunc {
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

		response := models.OrderResponse{
			Success: true,
			Message: fmt.Sprintf("Order created in SAP with ID: %s", sapOrderID),
			Order:   &order,
		}

		logger.WithField("order_id", order.ID).Info("Order processed successfully")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
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