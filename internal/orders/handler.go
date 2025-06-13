package orders

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jogardn/strangler-demo/internal/sap"
	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	sapClient *sap.Client
	logger    *logrus.Logger
}

func NewHandler(sapClient *sap.Client, logger *logrus.Logger) *Handler {
	return &Handler{
		sapClient: sapClient,
		logger:    logger,
	}
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
	}).Info("Processing order request")

	sapResp, err := h.sapClient.CreateOrder(&order)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create order in SAP")
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process order")
		return
	}

	if !sapResp.Success {
		h.respondWithError(w, http.StatusBadRequest, sapResp.Message)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, sapResp)
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