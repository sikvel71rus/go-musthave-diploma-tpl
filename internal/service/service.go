package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gophermart/internal/auth"
	"gophermart/internal/model"
)

var (
	ErrInvalidRequest     = errors.New("invalid request")
	ErrInvalidOrderNumber = errors.New("invalid order number")
)

type Repository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (int64, error)
	GetUserByLogin(ctx context.Context, login string) (model.User, error)
	CreateSession(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	GetUserIDByToken(ctx context.Context, token string) (int64, error)
	AddOrder(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error)
	ListOrders(ctx context.Context, userID int64) ([]model.Order, error)
	GetBalance(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	Ping(ctx context.Context) error
}

type Service struct {
	repo Repository
	auth *auth.Manager
}

func New(repo Repository, authManager *auth.Manager) *Service {
	return &Service{repo: repo, auth: authManager}
}

func (s *Service) Register(ctx context.Context, credentials model.Credentials) (string, error) {
	login := strings.TrimSpace(credentials.Login)
	password := strings.TrimSpace(credentials.Password)
	if login == "" || password == "" {
		return "", ErrInvalidRequest
	}

	hash, err := s.auth.HashPassword(password)
	if err != nil {
		return "", err
	}

	userID, err := s.repo.CreateUser(ctx, login, hash)
	if err != nil {
		return "", err
	}

	return s.createSession(ctx, userID)
}

func (s *Service) Login(ctx context.Context, credentials model.Credentials) (string, error) {
	login := strings.TrimSpace(credentials.Login)
	password := strings.TrimSpace(credentials.Password)
	if login == "" || password == "" {
		return "", ErrInvalidRequest
	}

	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		return "", err
	}
	if !s.auth.CheckPassword(user.PasswordHash, password) {
		return "", model.ErrInvalidCredentials
	}

	return s.createSession(ctx, user.ID)
}

func (s *Service) UserIDByToken(ctx context.Context, token string) (int64, error) {
	if strings.TrimSpace(token) == "" {
		return 0, model.ErrUnauthorized
	}
	return s.repo.GetUserIDByToken(ctx, token)
}

func (s *Service) UploadOrder(ctx context.Context, userID int64, number string) (model.OrderUploadResult, error) {
	number = strings.TrimSpace(number)
	if !isValidOrderNumber(number) {
		return 0, ErrInvalidOrderNumber
	}
	return s.repo.AddOrder(ctx, userID, number)
}

func (s *Service) ListOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	return s.repo.ListOrders(ctx, userID)
}

func (s *Service) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	return s.repo.GetBalance(ctx, userID)
}

func (s *Service) Withdraw(ctx context.Context, userID int64, request model.WithdrawalRequest) error {
	order := strings.TrimSpace(request.Order)
	if !isValidOrderNumber(order) || request.Sum <= 0 {
		return ErrInvalidOrderNumber
	}
	return s.repo.Withdraw(ctx, userID, order, request.Sum)
}

func (s *Service) ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return s.repo.ListWithdrawals(ctx, userID)
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *Service) createSession(ctx context.Context, userID int64) (string, error) {
	token, err := s.auth.NewToken()
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateSession(ctx, userID, token, s.auth.Expiry(time.Now().UTC())); err != nil {
		return "", err
	}
	return token, nil
}

func isValidOrderNumber(number string) bool {
	if number == "" {
		return false
	}
	for _, symbol := range number {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}

	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(number[i]))
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	return sum%10 == 0
}
