package accrual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchOrderSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"123","status":"PROCESSED","accrual":42.5}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, NewHTTPClient(time.Second))
	order, _, err := client.FetchOrder(context.Background(), "123")
	if err != nil {
		t.Fatalf("fetch order: %v", err)
	}
	if order.Order != "123" || order.Status != "PROCESSED" {
		t.Fatalf("unexpected payload: %+v", order)
	}
}

func TestFetchOrderRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "too many", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL, NewHTTPClient(time.Second))
	_, retryAfter, err := client.FetchOrder(context.Background(), "123")
	if err != ErrRateLimited {
		t.Fatalf("expected rate limit error, got %v", err)
	}
	if retryAfter != time.Minute {
		t.Fatalf("expected retry after 1m, got %v", retryAfter)
	}
}
