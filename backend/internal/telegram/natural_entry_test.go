package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNaturalEntriesProduceSavedEditableCards(t *testing.T) {
	const owner = "aabbccdd-1234-1234-1234-aabbccddeeff"
	ledger := &memoryLedger{owner: owner, chat: 123, seen: make(map[string]bool)}
	var cards int
	var lastText string
	var receipts sync.Mutex
	endpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/sendMessage" {
			t.Errorf("unexpected bot call %s", request.URL.Path)
		}
		var input struct {
			ChatID   int64          `json:"chat_id"`
			Text     string         `json:"text"`
			Keyboard inlineKeyboard `json:"reply_markup"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.ChatID != 123 {
			t.Errorf("invalid private-chat receipt")
		}
		receipts.Lock()
		defer receipts.Unlock()
		lastText = input.Text
		if len(input.Keyboard.Rows) > 0 {
			cards++
			if len(input.Keyboard.Rows) != 4 || input.Keyboard.Rows[0][0].Text != "Amount" ||
				input.Keyboard.Rows[0][1].Text != "Category" || input.Keyboard.Rows[3][1].Text != "Delete" {
				t.Error("natural entry did not receive the full editable card")
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer endpoint.Close()
	bot, err := NewBot("11:fake_token", owner, ledger, nil)
	if err != nil {
		t.Fatal(err)
	}
	bot.client.baseURL = endpoint.URL + "/"
	sent := time.Date(2026, 11, 24, 16, 1, 0, 0, time.UTC).Unix()
	for index, test := range []struct {
		text, category, date, account, note string
	}{
		{"RM12.50 lunch today tng", "Food", "2026-11-25", "Touch 'n Go", "lunch"},
		{"RM12.50 lunch today", "Food", "2026-11-25", "Unspecified", "lunch"},
		{"RM12.50 dinner", "Food", "2026-11-25", "Unspecified", "dinner"},
		{"RM12.50 yesterday", "Other", "2026-11-24", "Unspecified", ""},
	} {
		message := privateMessage(int64(index+1), 123, test.text)
		message.Date = sent
		if index > 0 {
			card := privateMessage(100, 11, "Saved expense of RM12.50.")
			card.Chat.ID, card.From.IsBot = 123, true
			message.ReplyTo = &card
		}
		for retry := 0; retry < 2; retry++ {
			if err := bot.handleUpdate(context.Background(), 11, update{Message: message}); err != nil {
				t.Fatal(err)
			}
		}
		receipts.Lock()
		cardCount, receiptText := cards, lastText
		receipts.Unlock()
		if len(ledger.saved) != index+1 || cardCount != index+1 {
			t.Fatalf("entry or receipt was lost or duplicated for %q", test.text)
		}
		entry := ledger.saved[index]
		if entry.AmountSen != 1250 || entry.Type != "expense" || entry.Category != test.category ||
			entry.Date != test.date || entry.Account != test.account || entry.Note != test.note || entry.Source != "telegram" {
			t.Fatalf("wrong ledger entry for %q: %#v", test.text, entry)
		}
		if !strings.Contains(receiptText, "RM12.50") || !strings.Contains(receiptText, test.account) {
			t.Fatalf("receipt does not describe the saved entry: %s", receiptText)
		}
	}
	message := privateMessage(5, 123, "RM12.50 lunch cash tng")
	if err := bot.handleUpdate(context.Background(), 11, update{Message: message}); err != nil {
		t.Fatal(err)
	}
	receipts.Lock()
	defer receipts.Unlock()
	if len(ledger.saved) != 4 || cards != 4 || !strings.Contains(lastText, "Could not save") {
		t.Fatal("ambiguous input was saved instead of requesting a correction")
	}
}
