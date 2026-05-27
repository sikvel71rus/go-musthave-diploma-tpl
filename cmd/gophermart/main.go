package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"gophermart/internal/accrual"
	"gophermart/internal/auth"
	"gophermart/internal/config"
	"gophermart/internal/handler"
	"gophermart/internal/logger"
	"gophermart/internal/middleware"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

func main() {
	cfg := config.Parse()
	if cfg.DatabaseURI == "" {
		log.Fatal("database URI is required")
	}

	if err := logger.Initialize("info"); err != nil {
		log.Fatalf("logger init failed: %v", err)
	}

	store, err := repository.New(cfg.DatabaseURI)
	if err != nil {
		log.Fatalf("repository init failed: %v", err)
	}
	defer store.Close()

	authManager := auth.NewManager(24 * time.Hour)
	svc := service.New(store, authManager)
	httpHandler := handler.New(svc)

	if cfg.AccrualSystemAddress != "" {
		worker := accrual.NewWorker(
			store,
			accrual.NewClient(cfg.AccrualSystemAddress, accrual.NewHTTPClient(5*time.Second)),
			2*time.Second,
			20,
		)
		go worker.Start(context.Background())
	}

	server := logger.RequestLogger(middleware.GzipMiddleware(httpHandler.Routes()))
	log.Printf("gophermart service is running on %s", cfg.RunAddress)
	log.Fatal(http.ListenAndServe(cfg.RunAddress, server))
}
