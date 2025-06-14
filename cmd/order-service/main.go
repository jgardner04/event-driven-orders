package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jogardn/strangler-demo/internal/events"
	"github.com/jogardn/strangler-demo/pkg/models"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type OrderService struct {
	db       *sql.DB
	logger   *logrus.Logger
	producer *events.KafkaProducer
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Database configuration
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "orderservice")
	dbPassword := getEnv("DB_PASSWORD", "orderservice")
	dbName := getEnv("DB_NAME", "orders")
	
	// Kafka configuration
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	
	// Service configuration
	port := getEnv("ORDER_SERVICE_PORT", "8081")

	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			logger.Info("Database connection established")
			break
		}
		logger.Info("Waiting for database...")
		time.Sleep(2 * time.Second)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		logger.WithError(err).Fatal("Failed to create tables")
	}

	// Initialize Kafka producer
	producer, err := events.NewKafkaProducer(kafkaBrokers, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Create service
	service := &OrderService{
		db:       db,
		logger:   logger,
		producer: producer,
	}

	// Set up routes
	router := mux.NewRouter()
	router.HandleFunc("/health", service.HealthCheck).Methods("GET")
	router.HandleFunc("/orders", service.CreateOrder).Methods("POST")
	router.HandleFunc("/orders/historical", service.CreateOrderHistorical).Methods("POST")
	router.HandleFunc("/orders", service.ListOrders).Methods("GET")
	router.HandleFunc("/orders/{id}", service.GetOrder).Methods("GET")

	// Middleware
	router.Use(loggingMiddleware(logger))

	// Create server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		logger.WithField("port", port).Info("Starting order service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Graceful shutdown
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

func (s *OrderService) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order models.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		s.logger.WithError(err).Error("Failed to decode order request")
		s.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Save to database
	if err := s.saveOrder(&order); err != nil {
		s.logger.WithError(err).Error("Failed to save order")
		s.respondWithError(w, http.StatusInternalServerError, "Failed to save order")
		return
	}

	// Publish event
	event := events.OrderCreatedEvent{
		OrderID:     order.ID,
		CustomerID:  order.CustomerID,
		TotalAmount: order.TotalAmount,
		CreatedAt:   order.CreatedAt,
	}

	if err := s.producer.PublishOrderCreated(event); err != nil {
		s.logger.WithError(err).Error("Failed to publish order created event")
		// Don't fail the request, just log the error
	}

	s.logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"customer_id":  order.CustomerID,
		"total_amount": order.TotalAmount,
	}).Info("Order created successfully")

	// Return response
	response := models.OrderResponse{
		Success: true,
		Message: "Order created successfully",
		Order:   &order,
	}

	s.respondWithJSON(w, http.StatusCreated, response)
}

func (s *OrderService) CreateOrderHistorical(w http.ResponseWriter, r *http.Request) {
	var order models.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		s.logger.WithError(err).Error("Failed to decode historical order request")
		s.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Save to database (no event publishing for historical orders)
	if err := s.saveOrder(&order); err != nil {
		s.logger.WithError(err).Error("Failed to save historical order")
		s.respondWithError(w, http.StatusInternalServerError, "Failed to save historical order")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"customer_id":  order.CustomerID,
		"total_amount": order.TotalAmount,
	}).Info("Historical order created successfully (no events published)")

	// Return response
	response := models.OrderResponse{
		Success: true,
		Message: "Historical order created successfully",
		Order:   &order,
	}

	s.respondWithJSON(w, http.StatusCreated, response)
}

func (s *OrderService) ListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.getAllOrders()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get orders")
		s.respondWithError(w, http.StatusInternalServerError, "Failed to get orders")
		return
	}

	s.logger.WithField("count", len(orders)).Info("Retrieved orders from database")

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"orders":  orders,
		"count":   len(orders),
	})
}

func (s *OrderService) GetOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	order, err := s.getOrderByID(orderID)
	if err != nil {
		if err == sql.ErrNoRows {
			s.respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}
		s.logger.WithError(err).Error("Failed to get order")
		s.respondWithError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	s.respondWithJSON(w, http.StatusOK, order)
}

func (s *OrderService) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := s.db.Ping(); err != nil {
		s.respondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"service": "order-service",
			"error": "database connection failed",
		})
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"service": "order-service",
	})
}

func (s *OrderService) saveOrder(order *models.Order) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert order
	query := `
		INSERT INTO orders (id, customer_id, total_amount, delivery_date, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(query, order.ID, order.CustomerID, order.TotalAmount, 
		order.DeliveryDate, order.Status, order.CreatedAt)
	if err != nil {
		return err
	}

	// Insert order items
	for _, item := range order.Items {
		itemQuery := `
			INSERT INTO order_items (order_id, product_id, quantity, unit_price, specifications)
			VALUES ($1, $2, $3, $4, $5)
		`
		specJSON, _ := json.Marshal(item.Specifications)
		_, err = tx.Exec(itemQuery, order.ID, item.ProductID, item.Quantity, 
			item.UnitPrice, string(specJSON))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *OrderService) getAllOrders() ([]*models.Order, error) {
	// Get all orders
	query := `
		SELECT id, customer_id, total_amount, delivery_date, status, created_at
		FROM orders ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.Order
	orderMap := make(map[string]*models.Order)

	for rows.Next() {
		order := &models.Order{}
		err := rows.Scan(
			&order.ID, &order.CustomerID, &order.TotalAmount,
			&order.DeliveryDate, &order.Status, &order.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
		orderMap[order.ID] = order
	}

	// Get all order items for each order
	for _, order := range orders {
		itemsQuery := `
			SELECT product_id, quantity, unit_price, specifications
			FROM order_items WHERE order_id = $1
		`
		itemRows, err := s.db.Query(itemsQuery, order.ID)
		if err != nil {
			return nil, err
		}

		for itemRows.Next() {
			var item models.OrderItem
			var specJSON string
			err := itemRows.Scan(&item.ProductID, &item.Quantity, &item.UnitPrice, &specJSON)
			if err != nil {
				itemRows.Close()
				return nil, err
			}
			json.Unmarshal([]byte(specJSON), &item.Specifications)
			order.Items = append(order.Items, item)
		}
		itemRows.Close()
	}

	return orders, nil
}

func (s *OrderService) getOrderByID(orderID string) (*models.Order, error) {
	order := &models.Order{}
	
	// Get order
	query := `
		SELECT id, customer_id, total_amount, delivery_date, status, created_at
		FROM orders WHERE id = $1
	`
	err := s.db.QueryRow(query, orderID).Scan(
		&order.ID, &order.CustomerID, &order.TotalAmount,
		&order.DeliveryDate, &order.Status, &order.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Get order items
	itemsQuery := `
		SELECT product_id, quantity, unit_price, specifications
		FROM order_items WHERE order_id = $1
	`
	rows, err := s.db.Query(itemsQuery, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.OrderItem
		var specJSON string
		err := rows.Scan(&item.ProductID, &item.Quantity, &item.UnitPrice, &specJSON)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(specJSON), &item.Specifications)
		order.Items = append(order.Items, item)
	}

	return order, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(255) PRIMARY KEY,
			customer_id VARCHAR(255) NOT NULL,
			total_amount DECIMAL(10,2) NOT NULL,
			delivery_date TIMESTAMP NOT NULL,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id VARCHAR(255) NOT NULL REFERENCES orders(id),
			product_id VARCHAR(255) NOT NULL,
			quantity INTEGER NOT NULL,
			unit_price DECIMAL(10,2) NOT NULL,
			specifications JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (s *OrderService) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (s *OrderService) respondWithError(w http.ResponseWriter, code int, message string) {
	s.respondWithJSON(w, code, map[string]interface{}{
		"success": false,
		"message": message,
	})
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