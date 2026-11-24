package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type client struct {
	baseURL string
	http    *http.Client
}

type telegramUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

type message struct {
	ID      int64        `json:"message_id"`
	Date    int64        `json:"date"`
	From    telegramUser `json:"from"`
	Text    string       `json:"text"`
	ReplyTo *message     `json:"reply_to_message"`
	Chat    struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

type update struct {
	ID       int64          `json:"update_id"`
	Message  message        `json:"message"`
	Callback *callbackQuery `json:"callback_query"`
}

type callbackQuery struct {
	ID      string       `json:"id"`
	From    telegramUser `json:"from"`
	Message *message     `json:"message"`
	Data    string       `json:"data"`
}

type inlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

type inlineKeyboard struct {
	Rows [][]inlineButton `json:"inline_keyboard"`
}

func (client *client) getMe(ctx context.Context) (telegramUser, error) {
	var user telegramUser
	err := client.call(ctx, "getMe", struct{}{}, &user)
	return user, err
}

func (client *client) updates(ctx context.Context, offset int64) ([]update, error) {
	var updates []update
	err := client.call(ctx, "getUpdates", map[string]any{
		"offset": offset, "timeout": 25, "limit": 20, "allowed_updates": []string{"message", "callback_query"},
	}, &updates)
	return updates, err
}

func (client *client) sendCard(ctx context.Context, chatID int64, text string, keyboard inlineKeyboard) error {
	return client.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID, "text": text, "reply_markup": keyboard,
	}, nil)
}

func (client *client) editCard(ctx context.Context, chatID, messageID int64, text string, keyboard inlineKeyboard) error {
	err := client.call(ctx, "editMessageText", map[string]any{
		"chat_id": chatID, "message_id": messageID, "text": text, "reply_markup": keyboard,
	}, nil)
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.code == 400 && strings.Contains(apiErr.description, "message is not modified") {
		return nil
	}
	return err
}

func (client *client) prompt(ctx context.Context, chatID int64, text string) (message, error) {
	var sent message
	err := client.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID, "text": text,
		"reply_markup": map[string]any{"force_reply": true, "input_field_placeholder": "Enter the new value"},
	}, &sent)
	return sent, err
}

func (client *client) answerCallback(ctx context.Context, id, text string) error {
	return client.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}

// Keep the Telegram description internal. Errors written to logs never include
// response text or a request URL that could contain credentials.
type apiError struct {
	method      string
	code        int
	description string
}

func (err *apiError) Error() string {
	return fmt.Sprintf("Telegram %s returned HTTP %d", err.method, err.code)
}

func (client *client) sendText(ctx context.Context, chatID int64, text string) error {
	return client.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

func (client *client) call(ctx context.Context, method string, input any, result any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("Telegram %s could not encode request", method)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Telegram %s could not construct request", method)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		// Transport errors can contain the request URL, which includes the bot token.
		return fmt.Errorf("Telegram %s could not reach the Bot API", method)
	}
	defer response.Body.Close()
	reply := struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}{}
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&reply)
	if response.StatusCode != http.StatusOK {
		return &apiError{method: method, code: response.StatusCode, description: reply.Description}
	}
	if decodeErr != nil || !reply.OK {
		return fmt.Errorf("Telegram %s returned an unsuccessful response", method)
	}
	if result != nil {
		if err := json.Unmarshal(reply.Result, result); err != nil {
			return fmt.Errorf("Telegram %s returned an invalid result", method)
		}
	}
	return nil
}
