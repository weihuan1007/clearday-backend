package telegram

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"clearday/backend/internal/money"
)

const usage = "Try RM20 Food dinner, income RM1200 Salary, or RM20 Food dinner @yesterday."

var (
	tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
	ownerPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
)

type ledger interface {
	LinkedTelegramChat(context.Context, string) (int64, error)
	LinkTelegramChat(context.Context, string, int64) error
	RecordTelegramTransaction(context.Context, string, money.Transaction) (bool, error)
}

type Bot struct {
	client *client
	ledger ledger
	owner  string
	logger *slog.Logger

	pairMu      sync.Mutex
	pairCode    string
	pairExpires time.Time
}

func NewBot(token, owner string, ledger ledger, logger *slog.Logger) (*Bot, error) {
	if !tokenPattern.MatchString(token) || !ownerPattern.MatchString(owner) {
		return nil, errors.New("set a valid Telegram bot token and the ledger owner's Cognito user ID")
	}
	return &Bot{
		client: &client{
			baseURL: "https://api.telegram.org/bot" + token + "/",
			http: &http.Client{
				Timeout:       35 * time.Second,
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			},
		},
		ledger: ledger, owner: owner, logger: logger,
	}, nil
}

func (bot *Bot) Run(ctx context.Context) {
	for ctx.Err() == nil {
		user, err := bot.client.getMe(ctx)
		if err == nil && user.ID > 0 && user.IsBot {
			bot.poll(ctx, user.ID)
			return
		}
		if err == nil {
			err = errors.New("Telegram did not return a bot identity")
		}
		bot.retry(ctx, err)
	}
}

func (bot *Bot) poll(ctx context.Context, botID int64) {
	var offset int64
	var lastUpdate time.Time
	for ctx.Err() == nil {
		// Telegram can choose a new update-ID sequence after a week of inactivity.
		if offset != 0 && time.Since(lastUpdate) > 24*time.Hour {
			offset = 0
		}
		updates, err := bot.client.updates(ctx, offset)
		if err != nil {
			bot.retry(ctx, err)
			continue
		}
		for _, update := range updates {
			if update.ID < offset {
				continue
			}
			if err := bot.handle(ctx, botID, update.Message); err != nil {
				bot.retry(ctx, err)
				break
			}
			// Telegram confirms only updates below the next requested offset.
			offset = update.ID + 1
			lastUpdate = time.Now()
		}
	}
}

func (bot *Bot) handle(ctx context.Context, botID int64, message message) error {
	text := strings.TrimSpace(message.Text)
	if text == "" || message.Chat.Type != "private" || message.Chat.ID <= 0 ||
		message.From.IsBot || message.From.ID != message.Chat.ID {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if command, code, found := strings.Cut(text, " "); found && command == "/start" {
		linked, err := bot.redeemLink(ctx, message.Chat.ID, strings.TrimSpace(code))
		if err != nil {
			return errors.New("could not link Telegram to the Money ledger")
		}
		if linked {
			bot.reply(ctx, message.Chat.ID, "ClearDay Money is connected. "+usage)
			return nil
		}
	}
	chat, err := bot.ledger.LinkedTelegramChat(ctx, bot.owner)
	if err != nil {
		return errors.New("could not read the Telegram link")
	}
	if chat != message.Chat.ID {
		return nil
	}
	if strings.HasPrefix(text, "/") {
		bot.reply(ctx, chat, usage)
		return nil
	}
	if message.ID <= 0 || message.Date <= 0 {
		return nil
	}

	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d", botID, chat, message.ID)))
	id := fmt.Sprintf("tg_%x", digest)
	transaction, err := money.ParseChatEntry(text, time.Unix(message.Date, 0), id, "telegram")
	if err != nil {
		bot.reply(ctx, chat, "Could not save the entry. "+err.Error()+". "+usage)
		return nil
	}
	created, err := bot.ledger.RecordTelegramTransaction(ctx, bot.owner, transaction)
	if err != nil {
		return errors.New("could not save the Telegram transaction")
	}
	if created {
		bot.reply(ctx, chat, fmt.Sprintf("Saved %s of RM%d.%02d · %s · %s.", transaction.Type,
			transaction.AmountSen/100, transaction.AmountSen%100, transaction.Category, transaction.Date))
	}
	return nil
}

func (bot *Bot) reply(ctx context.Context, chat int64, text string) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	// A failed confirmation must not undo a transaction already saved.
	if err := bot.client.sendText(ctx, chat, text); err != nil && bot.logger != nil {
		bot.logger.Warn("Telegram reply failed", "error", err)
	}
}

func (bot *Bot) retry(ctx context.Context, err error) {
	if ctx.Err() != nil {
		return
	}
	if bot.logger != nil {
		bot.logger.Warn("Telegram will retry", "error", err)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
