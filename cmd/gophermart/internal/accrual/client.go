package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gophermart/cmd/gophermart/internal/model"
)

var (
	ErrOrderNotRegistered = errors.New("order not registered")
	ErrRateLimited        = errors.New("rate limited")
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) FetchOrder(ctx context.Context, number string) (model.AccrualOrder, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+number, nil)
	if err != nil {
		return model.AccrualOrder{}, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.AccrualOrder{}, 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var payload model.AccrualOrder
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return model.AccrualOrder{}, 0, err
		}
		return payload, 0, nil
	case http.StatusNoContent:
		return model.AccrualOrder{}, 0, ErrOrderNotRegistered
	case http.StatusTooManyRequests:
		retryAfter, _ := time.ParseDuration(resp.Header.Get("Retry-After") + "s")
		return model.AccrualOrder{}, retryAfter, ErrRateLimited
	default:
		return model.AccrualOrder{}, 0, fmt.Errorf("unexpected accrual status: %d", resp.StatusCode)
	}
}
