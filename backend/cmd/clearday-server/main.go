package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clearday/backend/internal/config"
	"clearday/backend/internal/reminders"
	"clearday/backend/internal/server"
	"clearday/backend/internal/storage/dynamodbstore"
	"clearday/backend/internal/storage/jsonstore"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	store, closeStore, err := openStore(context.Background(), cfg)
	if err != nil {
		logger.Error("open store", "error", err)
		os.Exit(1)
	}
	defer closeStore()

	service := reminders.NewService(store)
	router := server.NewRouter(server.Options{
		APIHandler: reminders.NewHandler(service, logger),
		APIToken:   cfg.APIToken,
		StaticDir:  cfg.StaticDir,
		Logger:     logger,
	})

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("clearday server listening", "addr", cfg.Addr, "store", cfg.Store)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
}

func openStore(ctx context.Context, cfg config.Config) (reminders.Repository, func(), error) {
	switch cfg.Store {
	case "json":
		store, err := jsonstore.Open(cfg.JSONPath)
		return store, func() {}, err
	case "dynamodb":
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
		if err != nil {
			return nil, func() {}, err
		}
		store, err := dynamodbstore.Open(cfg.DynamoDBTable, awsCfg)
		return store, func() {}, err
	default:
		return nil, func() {}, config.InvalidStoreError{Store: cfg.Store}
	}
}
