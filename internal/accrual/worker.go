package accrual

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gophermart/internal/model"
)

type orderStore interface {
	GetOrdersForAccrual(ctx context.Context, limit int) ([]model.Order, error)
	UpdateOrderAccrual(ctx context.Context, number, status string, accrual *float64) error
}

type orderClient interface {
	FetchOrder(ctx context.Context, number string) (model.AccrualOrder, time.Duration, error)
}

type Worker struct {
	store    orderStore
	client   orderClient
	interval time.Duration
	limit    int
}

func NewWorker(store orderStore, client orderClient, interval time.Duration, limit int) *Worker {
	return &Worker{
		store:    store,
		client:   client,
		interval: interval,
		limit:    limit,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	orders, err := w.store.GetOrdersForAccrual(ctx, w.limit)
	if err != nil {
		return
	}

	for _, order := range orders {
		payload, retryAfter, err := w.client.FetchOrder(ctx, order.Number)
		if err != nil {
			if errors.Is(err, ErrOrderNotRegistered) {
				continue
			}
			if errors.Is(err, ErrRateLimited) {
				if retryAfter > 0 {
					timer := time.NewTimer(retryAfter)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
				return
			}
			continue
		}

		status := mapExternalStatus(payload.Status)
		_ = w.store.UpdateOrderAccrual(ctx, payload.Order, status, payload.Accrual)
	}
}

func mapExternalStatus(status string) string {
	switch status {
	case "INVALID":
		return model.OrderStatusInvalid
	case "PROCESSED":
		return model.OrderStatusProcessed
	case "REGISTERED", "PROCESSING":
		return model.OrderStatusProcessing
	default:
		return model.OrderStatusProcessing
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
