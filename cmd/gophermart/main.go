package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
		go worker.Start(ctx)
	}

	server := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: logger.RequestLogger(middleware.GzipMiddleware(httpHandler.Routes())),
	}

	log.Printf("gophermart service is running on %s", cfg.RunAddress)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server failed: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}
