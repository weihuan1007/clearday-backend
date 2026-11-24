package telegram

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"clearday/backend/internal/money"
)

const usage = "Try RM12.50 lunch today tng, RM12.50 dinner, or RM12.50 yesterday. Meals use Food; other descriptions can use Other. Add cash, tng, or another account and today, yesterday, or YYYY-MM-DD. Use income RM1200 Salary or transfer RM100 Savings for other types. Edit any field from the saved card."

var (
	tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
	ownerPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
)

type ledger interface {
	LinkedTelegramChat(context.Context, string) (int64, error)
	LinkTelegramChat(context.Context, string, int64) error
	RecordTelegramTransaction(context.Context, string, money.Transaction) (bool, error)
	TelegramTransaction(context.Context, string, string) (money.Transaction, bool, error)
	ChangeTelegramTransaction(context.Context, string, money.Transaction, *money.Transaction) (bool, error)
	PendingTelegramEdit(context.Context, string, int64) (money.TelegramEdit, error)
	SaveTelegramEdit(context.Context, string, int64, money.TelegramEdit) error
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
			if err := bot.handleUpdate(ctx, botID, update); err != nil {
				bot.retry(ctx, err)
				break
			}
			// Telegram confirms only updates below the next requested offset.
			offset = update.ID + 1
			lastUpdate = time.Now()
		}
	}
}

func (bot *Bot) handleUpdate(ctx context.Context, botID int64, event update) error {
	if event.Callback != nil {
		// Callbacks are short-lived. A storage failure leaves the buttons usable
		// so the owner can retry without blocking later transaction messages.
		if err := bot.handleCallback(ctx, botID, *event.Callback); err != nil {
			bot.loggerError("Telegram card action failed", err)
			if msg := event.Callback.Message; msg != nil && event.Callback.From.ID == msg.Chat.ID && msg.Chat.Type == "private" {
				bot.reply(ctx, msg.Chat.ID, "Could not complete that action. Tap Refresh on the card to check the saved entry, then try again.")
			}
		}
		return nil
	}
	return bot.handle(ctx, botID, event.Message)
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
	if text == "/cancel" {
		if err := bot.ledger.SaveTelegramEdit(ctx, bot.owner, chat, money.TelegramEdit{}); err != nil {
			return err
		}
		bot.reply(ctx, chat, "Edit canceled. You can send a transaction or use the buttons on a card.")
		return nil
	}
	if message.ReplyTo != nil && message.ReplyTo.From.ID == botID {
		handled, err := bot.handleEditReply(ctx, botID, message)
		if handled || err != nil {
			return err
		}
	}
	if strings.HasPrefix(text, "/") {
		bot.reply(ctx, chat, usage)
		return nil
	}
	if message.ID <= 0 || message.Date <= 0 {
		return nil
	}

	id := transactionID(botID, chat, message.ID)
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
		// A failed receipt must never retry the write or create another entry.
		if err := bot.client.sendCard(ctx, chat, cardText(transaction, "Saved"), cardKeyboard(message.ID)); err != nil {
			bot.loggerError("Telegram card delivery failed", err)
			bot.reply(ctx, chat, "Your transaction was saved, but its card could not be sent. Check Money before sending it again.")
		}
	}
	return nil
}

func (bot *Bot) loggerError(text string, err error) {
	if bot.logger != nil {
		bot.logger.Warn(text, "error", err)
	}
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
