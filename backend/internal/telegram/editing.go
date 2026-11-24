package telegram

import (
	"context"
	"fmt"
	"time"

	"clearday/backend/internal/money"
)

var editInstructions = map[string]string{
	"amount":   "Enter the new amount, such as 25 or RM25.50.",
	"category": "Enter the new category (up to 120 characters).",
	"account":  "Enter the new account (up to 120 characters).",
	"note":     "Enter the new note (up to 80 characters), or - to clear it.",
	"date":     "Enter the new date as YYYY-MM-DD, today, or yesterday.",
}

func (bot *Bot) sendEditPrompt(ctx context.Context, chatID int64, edit money.TelegramEdit, problem string) error {
	instructions, ok := editInstructions[edit.Field]
	if !ok {
		return fmt.Errorf("unknown Telegram edit field")
	}
	text := problem + "Change " + edit.Field + " for " + edit.Before.Category + " (" + edit.Before.Date + ").\n" +
		instructions + "\nReply to this message within 15 minutes. Send /cancel to cancel."
	prompt, err := bot.client.prompt(ctx, chatID, text)
	if err != nil {
		return err
	}
	if prompt.ID <= 0 {
		return fmt.Errorf("Telegram did not return an edit prompt")
	}
	edit.PromptID = prompt.ID
	edit.Expires = time.Now().Add(15 * time.Minute).Unix()
	return bot.ledger.SaveTelegramEdit(ctx, bot.owner, chatID, edit)
}

func (bot *Bot) handleEditReply(ctx context.Context, botID int64, reply message) error {
	chatID := reply.Chat.ID
	edit, err := bot.ledger.PendingTelegramEdit(ctx, bot.owner, chatID)
	if err != nil {
		return err
	}
	if edit.PromptID != reply.ReplyTo.ID || edit.Expires <= time.Now().Unix() || reply.Date <= 0 {
		bot.reply(ctx, chatID, "This edit is no longer active. Tap a field on the transaction card to start again.")
		return nil
	}
	// Reconstruct the ID from the authenticated private chat, never from reply text.
	id := transactionID(botID, chatID, edit.SourceID)
	entry, exists, err := bot.ledger.TelegramTransaction(ctx, bot.owner, id)
	if err != nil {
		return err
	}
	if !exists {
		bot.reply(ctx, chatID, "This transaction has been deleted. Your reply did not create a new one.")
		return nil
	}
	after, err := money.EditChatTransaction(edit.Before, edit.Field, reply.Text, time.Unix(reply.Date, 0))
	if err != nil {
		return bot.sendEditPrompt(ctx, chatID, edit, "Could not save that value. "+err.Error()+".\n\n")
	}
	// A replay after a successful save is safe. A different intervening edit
	// requires a fresh card instead of silently replacing the newer data.
	if entry != after {
		changed, err := bot.ledger.ChangeTelegramTransaction(ctx, bot.owner, edit.Before, &after)
		if err != nil {
			return err
		}
		if !changed {
			return bot.staleCard(ctx, chatID, edit.CardID, edit.SourceID, entry)
		}
	}
	if err := bot.updateCard(ctx, chatID, edit.CardID, edit.SourceID, after); err != nil {
		bot.loggerError("Telegram edited card delivery failed", err)
	}
	bot.reply(ctx, chatID, "Updated "+edit.Field+". The transaction card and your Money ledger use the saved value.")
	return nil
}
