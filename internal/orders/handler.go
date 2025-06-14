package orders

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jogardn/strangler-demo/internal/sap"
	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

type WebSocketHub interface {
	Broadcast(messageType string, data interface{}, source string)
}

type Handler struct {
	sapClient          *sap.Client
	orderServiceClient *OrderServiceClient
	logger             *logrus.Logger
	wsHub              WebSocketHub
}

func NewHandler(sapClient *sap.Client, orderServiceClient *OrderServiceClient, logger *logrus.Logger) *Handler {
	return &Handler{
		sapClient:          sapClient,
		orderServiceClient: orderServiceClient,
		logger:             logger,
	}
}

func (h *Handler) SetWebSocketHub(hub WebSocketHub) {
	h.wsHub = hub
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order models.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		h.logger.WithError(err).Error("Failed to decode order request")
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if order.ID == "" {
		order.ID = uuid.New().String()
	}
	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}
	if order.Status == "" {
		order.Status = "pending"
	}

	h.logger.WithFields(logrus.Fields{
		"order_id":     order.ID,
		"customer_id":  order.CustomerID,
		"total_amount": order.TotalAmount,
		"items_count":  len(order.Items),
	}).Info("Processing order request - Phase 3: Order Service only")

	// Phase 3: Only write to the new order service
	// SAP will receive orders via Kafka events
	if h.orderServiceClient == nil {
		h.logger.Error("Order service client not configured")
		h.respondWithError(w, http.StatusInternalServerError, "Order service not available")
		return
	}

	orderServiceResp, err := h.orderServiceClient.CreateOrder(&order)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create order in order service")
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process order")
		return
	}

	if !orderServiceResp.Success {
		h.logger.WithField("message", orderServiceResp.Message).Error("Order service returned error")
		h.respondWithError(w, http.StatusBadRequest, orderServiceResp.Message)
		return
	}

	h.logger.WithField("order_id", order.ID).Info("Order successfully processed by order service - event published to Kafka")

	// Broadcast order creation via WebSocket
	if h.wsHub != nil {
		orderEvent := map[string]interface{}{
			"type":            "order_created",
			"order":           order,
			"source":          "proxy",
			"processing_time": time.Since(time.Now()).Milliseconds(), // This would be calculated properly in real implementation
		}
		h.wsHub.Broadcast("order_created", orderEvent, "proxy")
	}

	// Return order service response
	h.respondWithJSON(w, http.StatusCreated, orderServiceResp)
}

func (h *Handler) CompareOrders(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Comparing orders between systems")

	// Get orders from both systems
	var orderServiceOrders []models.Order
	var sapOrders []models.Order
	var orderServiceErr, sapErr error

	// Fetch from order service
	if h.orderServiceClient != nil {
		orderServiceOrders, orderServiceErr = h.orderServiceClient.GetOrders()
	}

	// Fetch from SAP
	sapOrders, sapErr = h.sapClient.GetOrders()

	// Create comparison result
	comparison := map[string]interface{}{
		"timestamp": time.Now(),
		"order_service": map[string]interface{}{
			"count":  len(orderServiceOrders),
			"orders": orderServiceOrders,
			"error":  getErrorString(orderServiceErr),
		},
		"sap": map[string]interface{}{
			"count":  len(sapOrders),
			"orders": sapOrders,
			"error":  getErrorString(sapErr),
		},
	}

	// Add comparison analysis
	comparison["analysis"] = h.analyzeOrderSets(orderServiceOrders, sapOrders)

	h.logger.WithFields(logrus.Fields{
		"order_service_count": len(orderServiceOrders),
		"sap_count":          len(sapOrders),
		"order_service_error": orderServiceErr != nil,
		"sap_error":          sapErr != nil,
	}).Info("Order comparison completed")

	h.respondWithJSON(w, http.StatusOK, comparison)
}

func (h *Handler) CompareOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	h.logger.WithField("order_id", orderID).Info("Comparing specific order between systems")

	// Get order from both systems
	var orderServiceOrder, sapOrder *models.Order
	var orderServiceErr, sapErr error

	// Fetch from order service
	if h.orderServiceClient != nil {
		orderServiceOrder, orderServiceErr = h.orderServiceClient.GetOrder(orderID)
	}

	// Fetch from SAP
	sapOrder, sapErr = h.sapClient.GetOrder(orderID)

	// Create comparison result
	comparison := map[string]interface{}{
		"order_id":  orderID,
		"timestamp": time.Now(),
		"order_service": map[string]interface{}{
			"order": orderServiceOrder,
			"error": getErrorString(orderServiceErr),
			"found": orderServiceOrder != nil,
		},
		"sap": map[string]interface{}{
			"order": sapOrder,
			"error": getErrorString(sapErr),
			"found": sapOrder != nil,
		},
	}

	// Add comparison analysis
	if orderServiceOrder != nil && sapOrder != nil {
		comparison["analysis"] = h.compareOrders(orderServiceOrder, sapOrder)
	} else {
		comparison["analysis"] = map[string]interface{}{
			"status": "incomplete",
			"reason": "Order not found in one or both systems",
		}
	}

	h.logger.WithFields(logrus.Fields{
		"order_id":            orderID,
		"order_service_found": orderServiceOrder != nil,
		"sap_found":          sapOrder != nil,
	}).Info("Single order comparison completed")

	h.respondWithJSON(w, http.StatusOK, comparison)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"service": "proxy",
	})
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (h *Handler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]interface{}{
		"success": false,
		"message": message,
	})
}

func getErrorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func (h *Handler) analyzeOrderSets(orderServiceOrders, sapOrders []models.Order) map[string]interface{} {
	analysis := map[string]interface{}{
		"total_count_match": len(orderServiceOrders) == len(sapOrders),
		"order_service_count": len(orderServiceOrders),
		"sap_count": len(sapOrders),
	}

	// Create maps for easier lookup
	orderServiceMap := make(map[string]models.Order)
	sapMap := make(map[string]models.Order)

	for _, order := range orderServiceOrders {
		orderServiceMap[order.ID] = order
	}
	
	for _, order := range sapOrders {
		sapMap[order.ID] = order
	}

	// Find missing orders
	var missingInSAP []string
	var missingInOrderService []string
	var commonOrders []string

	for id := range orderServiceMap {
		if _, exists := sapMap[id]; exists {
			commonOrders = append(commonOrders, id)
		} else {
			missingInSAP = append(missingInSAP, id)
		}
	}

	for id := range sapMap {
		if _, exists := orderServiceMap[id]; !exists {
			missingInOrderService = append(missingInOrderService, id)
		}
	}

	analysis["missing_in_sap"] = missingInSAP
	analysis["missing_in_order_service"] = missingInOrderService
	analysis["common_orders"] = commonOrders
	analysis["sync_status"] = len(missingInSAP) == 0 && len(missingInOrderService) == 0

	return analysis
}

func (h *Handler) compareOrders(order1, order2 *models.Order) map[string]interface{} {
	analysis := map[string]interface{}{
		"id_match": order1.ID == order2.ID,
		"customer_id_match": order1.CustomerID == order2.CustomerID,
		"total_amount_match": order1.TotalAmount == order2.TotalAmount,
		"status_match": order1.Status == order2.Status,
		"items_count_match": len(order1.Items) == len(order2.Items),
	}

	// Check delivery date (allow small differences due to serialization)
	deliveryDiff := order1.DeliveryDate.Sub(order2.DeliveryDate)
	if deliveryDiff < 0 {
		deliveryDiff = -deliveryDiff
	}
	analysis["delivery_date_match"] = deliveryDiff < time.Second

	// Check created at (allow small differences)
	createdDiff := order1.CreatedAt.Sub(order2.CreatedAt)
	if createdDiff < 0 {
		createdDiff = -createdDiff
	}
	analysis["created_at_match"] = createdDiff < time.Second

	// Overall match
	allMatch := analysis["id_match"].(bool) &&
		analysis["customer_id_match"].(bool) &&
		analysis["total_amount_match"].(bool) &&
		analysis["delivery_date_match"].(bool) &&
		analysis["items_count_match"].(bool)

	analysis["perfect_match"] = allMatch

	// Detailed differences if not matching
	if !allMatch {
		differences := []string{}
		if !analysis["id_match"].(bool) {
			differences = append(differences, "id")
		}
		if !analysis["customer_id_match"].(bool) {
			differences = append(differences, "customer_id")
		}
		if !analysis["total_amount_match"].(bool) {
			differences = append(differences, "total_amount")
		}
		if !analysis["status_match"].(bool) {
			differences = append(differences, "status")
		}
		if !analysis["delivery_date_match"].(bool) {
			differences = append(differences, "delivery_date")
		}
		if !analysis["items_count_match"].(bool) {
			differences = append(differences, "items_count")
		}
		analysis["differences"] = strings.Join(differences, ", ")
	}

	return analysis
}