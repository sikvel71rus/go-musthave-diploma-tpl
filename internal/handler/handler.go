package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gophermart/internal/model"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

const authCookieName = "gophermart_token"

type contextKey string

const userIDKey contextKey = "userID"

type Service interface {
	Register(ctx context.Context, credentials model.Credentials) (string, error)
	Login(ctx context.Context, credentials model.Credentials) (string, error)
	UserIDByToken(ctx context.Context, token string) (int64, error)
	UploadOrder(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error)
	ListOrders(ctx context.Context, userID int64) ([]model.Order, error)
	GetBalance(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, request model.WithdrawalRequest) error
	ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	Ping(ctx context.Context) error
}

type Handler struct {
	service Service
}

func New(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/ping", h.ping)
	r.Post("/api/user/register", h.register)
	r.Post("/api/user/login", h.login)

	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Post("/api/user/orders", h.uploadOrder)
		r.Get("/api/user/orders", h.listOrders)
		r.Get("/api/user/balance", h.getBalance)
		r.Post("/api/user/balance/withdraw", h.withdraw)
		r.Get("/api/user/withdrawals", h.listWithdrawals)
	})

	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var credentials model.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.service.Register(r.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			http.Error(w, "invalid request", http.StatusBadRequest)
		case errors.Is(err, repository.ErrLoginTaken):
			http.Error(w, "login already taken", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.setAuthCookie(w, token)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var credentials model.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.service.Login(r.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			http.Error(w, "invalid request", http.StatusBadRequest)
		case errors.Is(err, repository.ErrInvalidCredentials):
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.setAuthCookie(w, token)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) uploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	result, err := h.service.UploadOrder(r.Context(), userID, string(body))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOrderNumber):
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		case errors.Is(err, repository.ErrOrderOwnedByAnotherUser):
			http.Error(w, "order belongs to another user", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if result == repository.OrderUploadDuplicate {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.service.ListOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(orders)
}

func (h *Handler) getBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	balance, err := h.service.GetBalance(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balance)
}

func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var request model.WithdrawalRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.Withdraw(r.Context(), userID, request); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOrderNumber):
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		case errors.Is(err, repository.ErrInsufficientFunds):
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) listWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.service.ListWithdrawals(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(withdrawals)
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Ping(r.Context()); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := h.service.UserIDByToken(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

func readToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDKey).(int64)
	return userID, ok
}
