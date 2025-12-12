package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"payments-service/internal/db"
)

type createAccountRequest struct {
	UserID string `json:"user_id"`
}

type createAccountResponse struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}

type topupRequest struct {
	Amount        float64 `json:"amount"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type topupResponse struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}

type balanceResponse struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}

func CreateAccountHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
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

		err := db.CreateAccount(r.Context(), req.UserID)
		if err != nil {
			http.Error(w, "failed to create account: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := createAccountResponse{
			UserID:  req.UserID,
			Balance: 0,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func TopupHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 || pathParts[0] != "accounts" || pathParts[2] != "topup" {
			http.Error(w, "invalid URL path: expected /accounts/{user_id}/topup", http.StatusBadRequest)
			return
		}
		userIDStr := pathParts[1]

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		var req topupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Amount <= 0 {
			http.Error(w, "amount must be positive", http.StatusBadRequest)
			return
		}

		err = db.Deposit(r.Context(), userID, req.Amount)
		if err != nil {
			http.Error(w, "topup failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		balance, err := db.GetBalance(r.Context(), userID)
		if err != nil {
			http.Error(w, "failed to get balance: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := topupResponse{
			UserID:  userIDStr,
			Balance: balance,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func GetBalanceHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 || pathParts[0] != "accounts" || pathParts[2] != "balance" {
			http.Error(w, "invalid URL path: expected /accounts/{user_id}/balance", http.StatusBadRequest)
			return
		}
		userIDStr := pathParts[1]

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		balance, err := db.GetBalance(r.Context(), userID)
		if err != nil {
			http.Error(w, "get balance failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := balanceResponse{
			UserID:  userIDStr,
			Balance: balance,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
