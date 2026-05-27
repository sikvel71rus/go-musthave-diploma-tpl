package repository

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"gophermart/internal/model"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var (
	ErrLoginTaken              = errors.New("login already taken")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrOrderOwnedByAnotherUser = errors.New("order belongs to another user")
	ErrInsufficientFunds       = errors.New("insufficient funds")
)

type OrderUploadResult int

const (
	OrderUploadAccepted OrderUploadResult = iota
	OrderUploadDuplicate
)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO gm_users (login, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`, login, passwordHash).Scan(&userID)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrLoginTaken
		}
		return 0, err
	}
	return userID, nil
}

func (s *Store) GetUserByLogin(ctx context.Context, login string) (model.User, error) {
	var user model.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, login, password_hash
		FROM gm_users
		WHERE login = $1
	`, login).Scan(&user.ID, &user.Login, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrInvalidCredentials
		}
		return model.User{}, err
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gm_sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	return err
}

func (s *Store) GetUserIDByToken(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id
		FROM gm_sessions
		WHERE token = $1 AND expires_at > NOW()
	`, token).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUnauthorized
		}
		return 0, err
	}
	return userID, nil
}

func (s *Store) AddOrder(ctx context.Context, userID int64, number string) (OrderUploadResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO gm_orders (number, user_id, status)
		VALUES ($1, $2, $3)
	`, number, userID, model.OrderStatusNew)
	if err == nil {
		return OrderUploadAccepted, tx.Commit()
	}
	if !isUniqueViolation(err) {
		return 0, err
	}

	var ownerID int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM gm_orders WHERE number = $1`, number).Scan(&ownerID); err != nil {
		return 0, err
	}
	if ownerID != userID {
		return 0, ErrOrderOwnedByAnotherUser
	}

	return OrderUploadDuplicate, nil
}

func (s *Store) ListOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT number, status, accrual, uploaded_at, user_id, rewarded
		FROM gm_orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt, &order.UserID, &order.Rewarded); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *Store) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	var balance model.Balance
	err := s.db.QueryRowContext(ctx, `
		SELECT current_balance, withdrawn_balance
		FROM gm_users
		WHERE id = $1
	`, userID).Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		return model.Balance{}, err
	}
	return balance, nil
}

func (s *Store) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var current float64
	if err := tx.QueryRowContext(ctx, `
		SELECT current_balance
		FROM gm_users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&current); err != nil {
		return err
	}
	if current < sum {
		return ErrInsufficientFunds
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE gm_users
		SET current_balance = current_balance - $2,
		    withdrawn_balance = withdrawn_balance + $2
		WHERE id = $1
	`, userID, sum); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO gm_withdrawals (user_id, order_number, sum)
		VALUES ($1, $2, $3)
	`, userID, order, sum); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT order_number, sum, processed_at
		FROM gm_withdrawals
		WHERE user_id = $1
		ORDER BY processed_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var withdrawal model.Withdrawal
		if err := rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func (s *Store) GetOrdersForAccrual(ctx context.Context, limit int) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT number, status, accrual, uploaded_at, user_id, rewarded
		FROM gm_orders
		WHERE status IN ($1, $2)
		ORDER BY uploaded_at ASC
		LIMIT $3
	`, model.OrderStatusNew, model.OrderStatusProcessing, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt, &order.UserID, &order.Rewarded); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *Store) UpdateOrderAccrual(ctx context.Context, number, status string, accrual *float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	var rewarded bool
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, rewarded
		FROM gm_orders
		WHERE number = $1
		FOR UPDATE
	`, number).Scan(&userID, &rewarded); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE gm_orders
		SET status = $2,
		    accrual = $3
		WHERE number = $1
	`, number, status, accrual); err != nil {
		return err
	}

	if status == model.OrderStatusProcessed && accrual != nil && !rewarded && *accrual > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE gm_users
			SET current_balance = current_balance + $2
			WHERE id = $1
		`, userID, *accrual); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE gm_orders
			SET rewarded = TRUE
			WHERE number = $1
		`, number); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

func DebugDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	return fmt.Sprintf("postgres:%d-bytes", len(dsn))
}
