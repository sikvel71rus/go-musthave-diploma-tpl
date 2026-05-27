package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gophermart/internal/model"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

type mockService struct {
	registerFn        func(ctx context.Context, credentials model.Credentials) (string, error)
	loginFn           func(ctx context.Context, credentials model.Credentials) (string, error)
	userIDByTokenFn   func(ctx context.Context, token string) (int64, error)
	uploadOrderFn     func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error)
	listOrdersFn      func(ctx context.Context, userID int64) ([]model.Order, error)
	getBalanceFn      func(ctx context.Context, userID int64) (model.Balance, error)
	withdrawFn        func(ctx context.Context, userID int64, request model.WithdrawalRequest) error
	listWithdrawalsFn func(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	pingFn            func(ctx context.Context) error
}

func (m *mockService) Register(ctx context.Context, credentials model.Credentials) (string, error) {
	return m.registerFn(ctx, credentials)
}
func (m *mockService) Login(ctx context.Context, credentials model.Credentials) (string, error) {
	return m.loginFn(ctx, credentials)
}
func (m *mockService) UserIDByToken(ctx context.Context, token string) (int64, error) {
	return m.userIDByTokenFn(ctx, token)
}
func (m *mockService) UploadOrder(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
	return m.uploadOrderFn(ctx, userID, number)
}
func (m *mockService) ListOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	return m.listOrdersFn(ctx, userID)
}
func (m *mockService) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	return m.getBalanceFn(ctx, userID)
}
func (m *mockService) Withdraw(ctx context.Context, userID int64, request model.WithdrawalRequest) error {
	return m.withdrawFn(ctx, userID, request)
}
func (m *mockService) ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return m.listWithdrawalsFn(ctx, userID)
}
func (m *mockService) Ping(ctx context.Context) error {
	return m.pingFn(ctx)
}

func TestRegister(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) {
			return "token", nil
		},
		loginFn:         func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) { return 1, nil },
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return repository.OrderUploadAccepted, nil
		},
		listOrdersFn: func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn: func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:   func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
			return nil, nil
		},
		pingFn: func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"login":"user","password":"pass"}`))
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRegisterValidationError(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) {
			return "", service.ErrInvalidRequest
		},
		loginFn:         func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) { return 1, nil },
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return repository.OrderUploadAccepted, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"login":"","password":""}`))
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUnauthorizedOrders(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		loginFn:    func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 0, repository.ErrUnauthorized
		},
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return 0, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListOrders(t *testing.T) {
	accrual := 500.0
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		loginFn:    func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 1, nil
		},
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return repository.OrderUploadAccepted, nil
		},
		listOrdersFn: func(ctx context.Context, userID int64) ([]model.Order, error) {
			return []model.Order{{Number: "9278923470", Status: model.OrderStatusProcessed, Accrual: &accrual, UploadedAt: time.Now()}}, nil
		},
		getBalanceFn: func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:   func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
			return nil, nil
		},
		pingFn: func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "token"})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var orders []model.Order
	if err := json.NewDecoder(rec.Body).Decode(&orders); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected one order, got %d", len(orders))
	}
}

func TestWithdrawInsufficientFunds(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		loginFn:    func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 1, nil
		},
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return repository.OrderUploadAccepted, nil
		},
		listOrdersFn: func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn: func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn: func(ctx context.Context, userID int64, request model.WithdrawalRequest) error {
			return repository.ErrInsufficientFunds
		},
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString(`{"order":"2377225624","sum":100}`))
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "token"})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}

func TestUploadOrderConflict(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		loginFn:    func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 1, nil
		},
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return 0, repository.ErrOrderOwnedByAnotherUser
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "token"})
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestPingFailure(t *testing.T) {
	h := New(&mockService{
		registerFn: func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		loginFn:    func(ctx context.Context, credentials model.Credentials) (string, error) { return "", nil },
		userIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 1, nil
		},
		uploadOrderFn: func(ctx context.Context, userID int64, number string) (repository.OrderUploadResult, error) {
			return 0, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, request model.WithdrawalRequest) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return errors.New("db down") },
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
