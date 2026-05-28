package service

import (
	"context"
	"testing"
	"time"

	"gophermart/internal/auth"
	"gophermart/internal/model"
)

type stubRepo struct {
	createUserFn       func(ctx context.Context, login, passwordHash string) (int64, error)
	getUserByLoginFn   func(ctx context.Context, login string) (model.User, error)
	createSessionFn    func(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	getUserIDByTokenFn func(ctx context.Context, token string) (int64, error)
	addOrderFn         func(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error)
	listOrdersFn       func(ctx context.Context, userID int64) ([]model.Order, error)
	getBalanceFn       func(ctx context.Context, userID int64) (model.Balance, error)
	withdrawFn         func(ctx context.Context, userID int64, order string, sum float64) error
	listWithdrawalsFn  func(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	pingFn             func(ctx context.Context) error
}

func (s *stubRepo) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	return s.createUserFn(ctx, login, passwordHash)
}
func (s *stubRepo) GetUserByLogin(ctx context.Context, login string) (model.User, error) {
	return s.getUserByLoginFn(ctx, login)
}
func (s *stubRepo) CreateSession(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	return s.createSessionFn(ctx, userID, token, expiresAt)
}
func (s *stubRepo) GetUserIDByToken(ctx context.Context, token string) (int64, error) {
	return s.getUserIDByTokenFn(ctx, token)
}
func (s *stubRepo) AddOrder(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
	return s.addOrderFn(ctx, userID, number)
}
func (s *stubRepo) ListOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	return s.listOrdersFn(ctx, userID)
}
func (s *stubRepo) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	return s.getBalanceFn(ctx, userID)
}
func (s *stubRepo) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return s.withdrawFn(ctx, userID, order, sum)
}
func (s *stubRepo) ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return s.listWithdrawalsFn(ctx, userID)
}
func (s *stubRepo) Ping(ctx context.Context) error {
	return s.pingFn(ctx)
}

func TestIsValidOrderNumber(t *testing.T) {
	cases := []struct {
		number string
		valid  bool
	}{
		{number: "79927398713", valid: true},
		{number: "12345678903", valid: true},
		{number: "12345678904", valid: false},
		{number: "12ab", valid: false},
	}

	for _, tc := range cases {
		if got := isValidOrderNumber(tc.number); got != tc.valid {
			t.Fatalf("order %s valid=%v got=%v", tc.number, tc.valid, got)
		}
	}
}

func TestRegister(t *testing.T) {
	repo := &stubRepo{
		createUserFn: func(ctx context.Context, login, passwordHash string) (int64, error) {
			if login == "" || passwordHash == "" {
				t.Fatal("expected login and password hash")
			}
			return 1, nil
		},
		getUserByLoginFn: func(ctx context.Context, login string) (model.User, error) { return model.User{}, nil },
		createSessionFn:  func(ctx context.Context, userID int64, token string, expiresAt time.Time) error { return nil },
		getUserIDByTokenFn: func(ctx context.Context, token string) (int64, error) {
			return 0, nil
		},
		addOrderFn: func(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
			return 0, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	}

	svc := New(repo, auth.NewManager(24*time.Hour))
	token, err := svc.Register(context.Background(), model.Credentials{Login: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	manager := auth.NewManager(24 * time.Hour)
	hash, err := manager.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &stubRepo{
		createUserFn: func(ctx context.Context, login, passwordHash string) (int64, error) { return 0, nil },
		getUserByLoginFn: func(ctx context.Context, login string) (model.User, error) {
			return model.User{ID: 1, Login: login, PasswordHash: hash}, nil
		},
		createSessionFn:    func(ctx context.Context, userID int64, token string, expiresAt time.Time) error { return nil },
		getUserIDByTokenFn: func(ctx context.Context, token string) (int64, error) { return 0, nil },
		addOrderFn: func(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
			return 0, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	}

	svc := New(repo, manager)
	if _, err := svc.Login(context.Background(), model.Credentials{Login: "user", Password: "wrong"}); err != model.ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestWithdrawValidation(t *testing.T) {
	repo := &stubRepo{
		createUserFn:       func(ctx context.Context, login, passwordHash string) (int64, error) { return 0, nil },
		getUserByLoginFn:   func(ctx context.Context, login string) (model.User, error) { return model.User{}, nil },
		createSessionFn:    func(ctx context.Context, userID int64, token string, expiresAt time.Time) error { return nil },
		getUserIDByTokenFn: func(ctx context.Context, token string) (int64, error) { return 0, nil },
		addOrderFn: func(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
			return 0, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	}

	svc := New(repo, auth.NewManager(24*time.Hour))
	if err := svc.Withdraw(context.Background(), 1, model.WithdrawalRequest{Order: "bad", Sum: 10}); err != ErrInvalidOrderNumber {
		t.Fatalf("expected invalid order error, got %v", err)
	}
}

func TestUploadOrder(t *testing.T) {
	repo := &stubRepo{
		createUserFn:       func(ctx context.Context, login, passwordHash string) (int64, error) { return 0, nil },
		getUserByLoginFn:   func(ctx context.Context, login string) (model.User, error) { return model.User{}, nil },
		createSessionFn:    func(ctx context.Context, userID int64, token string, expiresAt time.Time) error { return nil },
		getUserIDByTokenFn: func(ctx context.Context, token string) (int64, error) { return 0, nil },
		addOrderFn: func(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
			return model.OrderUploadAccepted, nil
		},
		listOrdersFn:      func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
		getBalanceFn:      func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
		withdrawFn:        func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
		listWithdrawalsFn: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
		pingFn:            func(ctx context.Context) error { return nil },
	}

	svc := New(repo, auth.NewManager(24*time.Hour))
	result, err := svc.UploadOrder(context.Background(), 1, "79927398713")
	if err != nil {
		t.Fatalf("upload order: %v", err)
	}
	if result != model.OrderUploadAccepted {
		t.Fatalf("expected accepted result, got %v", result)
	}
}
