package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"clearday/backend/internal/money"
)

func transactionID(botID, chatID, sourceID int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d", botID, chatID, sourceID)))
	return fmt.Sprintf("tg_%x", digest)
}

// Buttons carry the original message number instead of the much longer ledger
// ID to fit Telegram's 64-byte callback limit. The bot reconstructs the same ID.
func cardButton(sourceID int64, text, action string, arguments ...string) inlineButton {
	parts := append([]string{"cd", strconv.FormatInt(sourceID, 10), action}, arguments...)
	return inlineButton{Text: text, Data: strings.Join(parts, ":")}
}

func cardKeyboard(sourceID int64) inlineKeyboard {
	return inlineKeyboard{Rows: [][]inlineButton{
		{cardButton(sourceID, "Amount", "amount"), cardButton(sourceID, "Category", "category")},
		{cardButton(sourceID, "Type", "type"), cardButton(sourceID, "Date", "date")},
		{cardButton(sourceID, "Account", "account"), cardButton(sourceID, "Note", "note")},
		{cardButton(sourceID, "Refresh", "view"), cardButton(sourceID, "Delete", "delete")},
	}}
}

func cardText(entry money.Transaction, action string) string {
	note := entry.Note
	if note == "" {
		note = "—"
	}
	return fmt.Sprintf("%s %s of RM%d.%02d · %s · %s.\n\nAccount · %s\nNote · %s",
		action, entry.Type, entry.AmountSen/100, entry.AmountSen%100, entry.Category, entry.Date, entry.Account, note)
}

func cardVersion(entry money.Transaction) string {
	encoded, _ := json.Marshal(entry)
	digest := sha256.Sum256(encoded)
	return base64.RawURLEncoding.EncodeToString(digest[:8])
}

func (bot *Bot) handleCallback(ctx context.Context, botID int64, query callbackQuery) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	msg := query.Message
	if msg == nil || msg.ID <= 0 || msg.Date <= 0 || msg.Chat.Type != "private" ||
		query.From.IsBot || query.From.ID != msg.Chat.ID || msg.From.ID != botID {
		_ = bot.client.answerCallback(ctx, query.ID, "This card is unavailable.")
		return nil
	}
	// Stop Telegram's button spinner before database or network work.
	_ = bot.client.answerCallback(ctx, query.ID, "")
	chat, err := bot.ledger.LinkedTelegramChat(ctx, bot.owner)
	if err != nil {
		return err
	}
	if chat != msg.Chat.ID {
		return nil
	}
	parts := strings.Split(query.Data, ":")
	if len(parts) < 3 || parts[0] != "cd" {
		return nil
	}
	sourceID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || sourceID <= 0 {
		return nil
	}
	entry, exists, err := bot.ledger.TelegramTransaction(ctx, bot.owner, transactionID(botID, chat, sourceID))
	if err != nil {
		return err
	}
	if !exists {
		return bot.client.editCard(ctx, chat, msg.ID, "This transaction has been deleted.", inlineKeyboard{Rows: [][]inlineButton{}})
	}
	action := parts[2]
	switch action {
	case "view":
		return bot.updateCard(ctx, chat, msg.ID, sourceID, entry)
	case "amount", "date", "note", "editcategory", "editaccount":
		field := strings.TrimPrefix(action, "edit")
		edit := money.TelegramEdit{
			SourceID: sourceID, CardID: msg.ID, Field: field, Before: entry,
		}
		return bot.sendEditPrompt(ctx, chat, edit, "")
	case "category", "type", "account":
		return bot.showChoices(ctx, chat, msg.ID, sourceID, action, entry)
	case "delete":
		keyboard := inlineKeyboard{Rows: [][]inlineButton{{
			cardButton(sourceID, "Keep transaction", "view"),
			cardButton(sourceID, "Confirm delete", "confirm", cardVersion(entry)),
		}}}
		return bot.client.editCard(ctx, chat, msg.ID, cardText(entry, "Delete")+"\n\nDelete this transaction from your ledger?", keyboard)
	case "confirm":
		if len(parts) != 4 {
			return nil
		}
		if parts[3] != cardVersion(entry) {
			return bot.staleCard(ctx, chat, msg.ID, sourceID, entry)
		}
		changed, err := bot.ledger.ChangeTelegramTransaction(ctx, bot.owner, entry, nil)
		if err != nil {
			return err
		}
		if !changed {
			bot.reply(ctx, chat, "This transaction changed. Tap Refresh and review it before deleting.")
			return nil
		}
		return bot.client.editCard(ctx, chat, msg.ID, "Transaction deleted from your ledger.", inlineKeyboard{Rows: [][]inlineButton{}})
	case "setcategory", "settype", "setaccount":
		if len(parts) != 5 {
			return nil
		}
		if parts[4] != cardVersion(entry) {
			return bot.staleCard(ctx, chat, msg.ID, sourceID, entry)
		}
		field := strings.TrimPrefix(action, "set")
		choices := fieldChoices(field, entry)
		index, err := strconv.Atoi(parts[3])
		if err != nil || index < 0 || index >= len(choices) {
			return nil
		}
		after, err := money.EditChatTransaction(entry, field, choices[index], time.Now())
		if err != nil {
			return err
		}
		changed, err := bot.ledger.ChangeTelegramTransaction(ctx, bot.owner, entry, &after)
		if err != nil {
			return err
		}
		if !changed {
			bot.reply(ctx, chat, "This transaction changed. Tap Refresh and choose again.")
			return nil
		}
		return bot.updateCard(ctx, chat, msg.ID, sourceID, after)
	}
	return nil
}

func fieldChoices(field string, entry money.Transaction) []string {
	switch field {
	case "category":
		return money.ChatCategories(entry.Type)
	case "type":
		return []string{"expense", "income", "transfer"}
	case "account":
		return []string{"Touch 'n Go", "Apple Pay", "Bank", "Debit Card", "Credit Card", "GrabPay", "Cash", "Unspecified"}
	}
	return nil
}

func (bot *Bot) showChoices(ctx context.Context, chatID, cardID, sourceID int64, field string, entry money.Transaction) error {
	keyboard := inlineKeyboard{Rows: [][]inlineButton{}}
	for index, choice := range fieldChoices(field, entry) {
		label := choice
		if field == "type" {
			label = strings.ToUpper(choice[:1]) + choice[1:]
		}
		button := cardButton(sourceID, label, "set"+field, strconv.Itoa(index), cardVersion(entry))
		if index%2 == 0 {
			keyboard.Rows = append(keyboard.Rows, []inlineButton{})
		}
		last := len(keyboard.Rows) - 1
		keyboard.Rows[last] = append(keyboard.Rows[last], button)
	}
	if field != "type" {
		keyboard.Rows = append(keyboard.Rows, []inlineButton{cardButton(sourceID, "Enter custom "+field, "edit"+field)})
	}
	keyboard.Rows = append(keyboard.Rows, []inlineButton{cardButton(sourceID, "Back to transaction", "view")})
	return bot.client.editCard(ctx, chatID, cardID, cardText(entry, "Saved")+"\n\nChoose "+field+".", keyboard)
}

func (bot *Bot) staleCard(ctx context.Context, chatID, cardID, sourceID int64, entry money.Transaction) error {
	bot.reply(ctx, chatID, "This transaction has changed since that button was shown. Review the latest card and choose again.")
	return bot.updateCard(ctx, chatID, cardID, sourceID, entry)
}

func (bot *Bot) updateCard(ctx context.Context, chatID, cardID, sourceID int64, entry money.Transaction) error {
	text, keyboard := cardText(entry, "Saved"), cardKeyboard(sourceID)
	if err := bot.client.editCard(ctx, chatID, cardID, text, keyboard); err != nil {
		bot.loggerError("Telegram card refresh failed", err)
		return bot.client.sendCard(ctx, chatID, text, keyboard)
	}
	return nil
}
