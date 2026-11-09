package main

import (
	"log/slog"

	"clearday/backend/internal/auth"
	"clearday/backend/internal/config"
	"clearday/backend/internal/money"
	"clearday/backend/internal/telegram"
)

func openTelegramBot(cfg config.Config, store *money.DynamoStore, cognito *auth.CognitoVerifier, logger *slog.Logger) *telegram.Bot {
	if cfg.TelegramToken == "" {
		return nil
	}
	if store == nil || cognito == nil {
		logger.Warn("Telegram Money integration requires an enabled ledger and Cognito")
		return nil
	}
	bot, err := telegram.NewBot(cfg.TelegramToken, cfg.TelegramOwner, store, logger)
	if err != nil {
		logger.Warn("Telegram Money integration disabled", "reason", err)
		return nil
	}
	return bot
}
