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

	"clearday/backend/internal/auth"
	"clearday/backend/internal/config"
	"clearday/backend/internal/money"
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
	tokenVerifier, err := openTokenVerifier(cfg)
	if err != nil {
		logger.Error("configure authentication", "error", err)
		os.Exit(1)
	}
	options := server.Options{
		APIHandler: reminders.NewHandler(service, logger),
		APIToken:   cfg.APIToken,
		RuntimeAuth: server.RuntimeAuthConfig{
			Domain:      cfg.CognitoDomain,
			ClientID:    cfg.CognitoClientID,
			RedirectURI: cfg.CognitoRedirectURI,
			LogoutURI:   cfg.CognitoLogoutURI,
		},
		ExposeAPIToken: cfg.ExposeAPIToken,
		StaticDir:      cfg.StaticDir,
		Logger:         logger,
	}
	if tokenVerifier != nil {
		options.TokenVerifier = tokenVerifier
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	moneyHandler, moneyStore := openMoneyHandler(ctx, cfg, tokenVerifier, logger)
	options.MoneyHandler = moneyHandler
	if bot := openTelegramBot(cfg, moneyStore, tokenVerifier, logger); bot != nil {
		options.TelegramPairHandler = bot.PairHandler(tokenVerifier.VerifySubject)
		go bot.Run(ctx)
	}
	router := server.NewRouter(options)

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

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
}

func openTokenVerifier(cfg config.Config) (*auth.CognitoVerifier, error) {
	if cfg.CognitoUserPoolID == "" {
		return nil, nil
	}

	return auth.NewCognitoVerifier(auth.CognitoConfig{
		Region:     cfg.AWSRegion,
		UserPoolID: cfg.CognitoUserPoolID,
		ClientID:   cfg.CognitoClientID,
	})
}

func openMoneyHandler(ctx context.Context, cfg config.Config, cognito *auth.CognitoVerifier, logger *slog.Logger) (http.Handler, *money.DynamoStore) {
	if cfg.MoneyTable == "" {
		return nil, nil
	}

	var verify func(context.Context, string) (string, error)
	if cognito != nil {
		verify = cognito.VerifySubject
	} else if cfg.MoneyToken != cfg.APIToken {
		verify = auth.NewPersonalMoneyVerifier(cfg.MoneyToken)
	}
	if verify == nil || cfg.AWSRegion == "" {
		// A missing Money setting should keep the ledger closed and let reminders run.
		logger.Warn("Money API disabled", "reason", "configure AWS_REGION and Cognito or a separate 64-character hexadecimal CLEAR_DAY_MONEY_TOKEN")
		return nil, nil
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		logger.Error("configure money storage", "error", err)
		return nil, nil
	}
	store := money.NewDynamoStore(cfg.MoneyTable, awsCfg)
	return money.NewHandler(store, verify, logger), store
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
