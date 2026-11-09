package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	ID   int64        `json:"message_id"`
	Date int64        `json:"date"`
	From telegramUser `json:"from"`
	Text string       `json:"text"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

type update struct {
	ID      int64   `json:"update_id"`
	Message message `json:"message"`
}

func (client *client) getMe(ctx context.Context) (telegramUser, error) {
	var user telegramUser
	err := client.call(ctx, "getMe", struct{}{}, &user)
	return user, err
}

func (client *client) updates(ctx context.Context, offset int64) ([]update, error) {
	var updates []update
	err := client.call(ctx, "getUpdates", map[string]any{
		"offset": offset, "timeout": 25, "limit": 20, "allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
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
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram %s returned HTTP %d", method, response.StatusCode)
	}
	reply := struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}{}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&reply); err != nil || !reply.OK {
		return fmt.Errorf("Telegram %s returned an unsuccessful response", method)
	}
	if result != nil {
		if err := json.Unmarshal(reply.Result, result); err != nil {
			return fmt.Errorf("Telegram %s returned an invalid result", method)
		}
	}
	return nil
}
