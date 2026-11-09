package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"clearday/backend/internal/money"
)

type memoryLedger struct {
	owner string
	chat  int64
	saved []money.Transaction
	seen  map[string]bool
}

func (store *memoryLedger) LinkedTelegramChat(_ context.Context, owner string) (int64, error) {
	if owner != store.owner {
		return 0, errors.New("unexpected owner")
	}
	return store.chat, nil
}

func (store *memoryLedger) LinkTelegramChat(_ context.Context, owner string, chat int64) error {
	if owner != store.owner {
		return errors.New("unexpected owner")
	}
	store.chat = chat
	return nil
}

func (store *memoryLedger) RecordTelegramTransaction(_ context.Context, owner string, entry money.Transaction) (bool, error) {
	if owner != store.owner {
		return false, errors.New("unexpected owner")
	}
	if store.seen[entry.ID] {
		return false, nil
	}
	store.seen[entry.ID] = true
	store.saved = append(store.saved, entry)
	return true, nil
}

func TestTelegramPollingPairsAndRecordsOnlyTheOwnersChat(t *testing.T) {
	const owner = "aabbccdd-1234-1234-1234-aabbccddeeff"
	ledger := &memoryLedger{owner: owner, seen: make(map[string]bool)}
	acknowledged := make(chan struct{}, 1)
	replies := make(chan string, 10)
	eventsReady := make(chan struct{})
	var events []update
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/getMe":
			fmt.Fprint(response, `{"ok":true,"result":{"id":11,"is_bot":true,"username":"clearday_test_bot"}}`)
		case "/getUpdates":
			<-eventsReady
			var query struct {
				Offset int64 `json:"offset"`
			}
			if err := json.NewDecoder(request.Body).Decode(&query); err != nil {
				t.Error(err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			if query.Offset == 0 {
				_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "result": events})
				return
			}
			if query.Offset != int64(len(events)+1) {
				t.Errorf("offset %d did not acknowledge the complete batch", query.Offset)
			}
			select {
			case acknowledged <- struct{}{}:
			default:
			}
			<-request.Context().Done()
		case "/sendMessage":
			var reply struct {
				ChatID int64  `json:"chat_id"`
				Text   string `json:"text"`
			}
			if err := json.NewDecoder(request.Body).Decode(&reply); err != nil || reply.ChatID != 123 {
				t.Error("bot replied outside the linked private chat")
			}
			replies <- reply.Text
			fmt.Fprint(response, `{"ok":true,"result":{}}`)
		default:
			t.Errorf("unexpected Bot API method %s", request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	bot, err := NewBot("11:fake_token", owner, ledger, nil)
	if err != nil {
		t.Fatal(err)
	}
	bot.client.baseURL = server.URL + "/"
	handler := bot.PairHandler(func(_ context.Context, token string) (string, error) {
		switch token {
		case "owner":
			return owner, nil
		case "another-account":
			return "different-subject", nil
		default:
			return "", errors.New("invalid token")
		}
	})
	requestPair := func(method, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "/api/integrations/telegram/pair", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if requestPair(method, "").Code != http.StatusUnauthorized || requestPair(method, "another-account").Code != http.StatusForbidden {
			t.Fatalf("unauthorized account accessed Telegram pairing via %s", method)
		}
	}
	status := func(connected bool, wantURL string) {
		t.Helper()
		response := requestPair(http.MethodGet, "owner")
		var result struct {
			Connected *bool  `json:"connected"`
			URL       string `json:"url"`
		}
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" ||
			json.Unmarshal(response.Body.Bytes(), &result) != nil ||
			result.Connected == nil || *result.Connected != connected || result.URL != wantURL {
			t.Fatalf("unexpected Telegram status: %d %s", response.Code, response.Body)
		}
	}
	status(false, "")
	response := requestPair(http.MethodPost, "owner")
	var link struct {
		URL string `json:"url"`
	}
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" ||
		json.Unmarshal(response.Body.Bytes(), &link) != nil {
		t.Fatalf("pairing failed: %d %s", response.Code, response.Body)
	}
	telegramURL, err := url.Parse(link.URL)
	if err != nil || telegramURL.Host != "t.me" || telegramURL.Path != "/clearday_test_bot" {
		t.Fatalf("bad link %q", link.URL)
	}
	code := telegramURL.Query().Get("start")
	if code == "" {
		t.Fatal("link had no pairing code")
	}
	status(false, "")

	sentAt := time.Date(2026, 11, 9, 16, 5, 0, 0, time.UTC).Unix()
	add := func(id, chat int64, chatType, text string) {
		msg := message{ID: id, Date: sentAt, From: telegramUser{ID: chat}, Text: text}
		msg.Chat.ID, msg.Chat.Type = chat, chatType
		events = append(events, update{ID: int64(len(events) + 1), Message: msg})
	}
	add(1, 555, "group", "/start "+code)
	add(2, 555, "private", "RM20 Food ignored")
	add(3, 123, "private", "/start "+code)
	add(4, 555, "private", "/start "+code) // Consumed code cannot change the link.
	add(5, 123, "private", "RM20 food dinner")
	add(5, 123, "private", "RM20 food dinner") // Provider retries keep the message ID.
	add(6, 123, "private", "RM0 Food")
	add(7, 123, "private", "income RM1200 Salary @yesterday")
	close(eventsReady)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { bot.Run(ctx); close(done) }()
	select {
	case <-acknowledged:
	case <-time.After(5 * time.Second):
		t.Error("Telegram polling did not acknowledge the batch")
	}
	cancel()
	<-done
	if ledger.chat != 123 || len(ledger.saved) != 2 {
		t.Fatalf("got chat %d and %d saved transactions", ledger.chat, len(ledger.saved))
	}
	status(true, "https://t.me/clearday_test_bot")
	first, second := ledger.saved[0], ledger.saved[1]
	if first.AmountSen != 2000 || first.Category != "Food" || first.Note != "dinner" || first.Date != "2026-11-10" ||
		second.Type != "income" || second.AmountSen != 120000 || second.Date != "2026-11-09" {
		t.Fatalf("unexpected transactions %#v", ledger.saved)
	}
	if len(replies) != 4 {
		t.Fatalf("got %d replies, want pairing, two saved entries, and invalid input", len(replies))
	}
	if reply := <-replies; !strings.Contains(reply, "connected") {
		t.Fatalf("unexpected pairing reply %q", reply)
	}
	if reply := <-replies; !strings.Contains(reply, "RM20.00 · Food · 2026-11-10") {
		t.Fatalf("unexpected receipt %q", reply)
	}
}
