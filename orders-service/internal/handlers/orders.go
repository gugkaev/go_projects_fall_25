package handlers

import (
	"encoding/json"
	"net/http"
	"order-service/internal/db"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type createOrderRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
	Items  []interface{} `json:"items,omitempty"`
}

func CreateOrderHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var req createOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		if _, err := strconv.ParseInt(req.UserID, 10, 64); err != nil {
			http.Error(w, "user_id must be integer", http.StatusBadRequest)
			return
		}
		if req.Amount <= 0 {
			http.Error(w, "amount must be positive", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(ctx)
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(ctx)

		orderID, err := db.InsertOrderTx(tx, req.UserID, req.Amount)
		if err != nil {
			http.Error(w, "failed to create order: "+err.Error(), http.StatusInternalServerError)
			return
		}

		messageID := uuid.New().String()

		userIDInt, _ := strconv.ParseInt(req.UserID, 10, 64)
		payload := map[string]interface{}{
			"message_id": messageID,
			"order_id":   orderID,
			"user_id":    userIDInt,
			"amount":     req.Amount,
		}

		if err := db.InsertOutboxTx(tx, messageID, payload); err != nil {
			http.Error(w, "failed to write to outbox: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(ctx); err != nil {
			http.Error(w, "transaction commit failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"order_id": orderID,
			"status":   "PENDING",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
	}
}

func ListOrdersHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id query parameter", http.StatusBadRequest)
			return
		}

		orders, err := db.ListOrders(ctx, userID)
		if err != nil {
			http.Error(w, "failed to fetch orders: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(orders)
	}
}

func GetOrderHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 3 {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}
		orderIDStr := pathParts[2]

		orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
		if err != nil {
			http.Error(w, "order_id must be integer", http.StatusBadRequest)
			return
		}

		order, err := db.GetOrder(ctx, orderID)
		if err != nil {
			if err.Error() == "order not found" {
				http.Error(w, "order not found", http.StatusNotFound)
			} else {
				http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		response := map[string]interface{}{
			"order_id": order.ID,
			"status":   order.Status,
			"amount":   order.Amount,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
