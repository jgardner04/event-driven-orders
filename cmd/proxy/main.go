package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jogardn/strangler-demo/internal/orders"
	"github.com/jogardn/strangler-demo/internal/sap"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	port := getEnv("PROXY_PORT", "8080")
	sapURL := getEnv("SAP_URL", "http://localhost:8082")
	orderServiceURL := getEnv("ORDER_SERVICE_URL", "")

	sapClient := sap.NewClient(sapURL, logger)
	
	var orderServiceClient *orders.OrderServiceClient
	if orderServiceURL != "" {
		orderServiceClient = orders.NewOrderServiceClient(orderServiceURL, logger)
		logger.WithField("url", orderServiceURL).Info("Order service client configured")
	} else {
		logger.Info("Order service URL not configured - running in Phase 1 mode")
	}
	
	orderHandler := orders.NewHandler(sapClient, orderServiceClient, logger)

	router := mux.NewRouter()
	router.HandleFunc("/health", orderHandler.HealthCheck).Methods("GET")
	router.HandleFunc("/orders", orderHandler.CreateOrder).Methods("POST")
	router.HandleFunc("/compare/orders", orderHandler.CompareOrders).Methods("GET")
	router.HandleFunc("/compare/orders/{id}", orderHandler.CompareOrder).Methods("GET")

	router.Use(loggingMiddleware(logger))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.WithField("port", port).Info("Starting proxy server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server gracefully stopped")
}

func loggingMiddleware(logger *logrus.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			logger.WithFields(logrus.Fields{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			}).Info("Request received")

			next.ServeHTTP(w, r)

			logger.WithFields(logrus.Fields{
				"method":   r.Method,
				"path":     r.URL.Path,
				"duration": time.Since(start).Milliseconds(),
			}).Info("Request completed")
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}