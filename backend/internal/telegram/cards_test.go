package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Exercise real Bot API payloads and shared ledger mutations without contacting
// the owner's bot or creating transactions in the production account.
func TestTransactionCardsEditAndDeleteTheSavedLedgerEntry(t *testing.T) {
	const owner = "aabbccdd-1234-1234-1234-aabbccddeeff"
	ctx := context.Background()
	ledger := &memoryLedger{owner: owner, chat: 123, seen: make(map[string]bool)}
	messages := map[int64]message{}
	keyboards := map[int64]inlineKeyboard{}
	var nextMessage int64 = 100
	var answered int
	failEdit := false
	failCard := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		var input struct {
			ChatID int64  `json:"chat_id"`
			ID     int64  `json:"message_id"`
			Text   string `json:"text"`
			Markup struct {
				Rows  [][]inlineButton `json:"inline_keyboard"`
				Force bool             `json:"force_reply"`
			} `json:"reply_markup"`
			Allowed []string `json:"allowed_updates"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		switch request.URL.Path {
		case "/sendMessage", "/editMessageText":
			if input.ChatID != ledger.chat {
				t.Errorf("sent a card outside the linked private chat")
			}
			if failCard && len(input.Markup.Rows) > 0 && request.URL.Path == "/sendMessage" {
				failCard = false
				response.WriteHeader(503)
				fmt.Fprint(response, `{"ok":false,"description":"Temporary outage"}`)
				return
			}
			if failEdit && request.URL.Path == "/editMessageText" {
				response.WriteHeader(400)
				fmt.Fprint(response, `{"ok":false,"description":"Bad Request: message is not modified"}`)
				return
			}
			id := input.ID
			if id == 0 {
				id = nextMessage
				nextMessage++
			}
			msg := privateMessage(id, 11, input.Text)
			msg.From.IsBot = true
			msg.Chat.ID = ledger.chat
			messages[id] = msg
			keyboards[id] = inlineKeyboard{Rows: input.Markup.Rows}
			for _, row := range input.Markup.Rows {
				for _, button := range row {
					if len([]byte(button.Data)) > 64 || !utf8.ValidString(button.Data) {
						t.Errorf("invalid Telegram callback data %q", button.Data)
					}
				}
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "result": msg})
		case "/answerCallbackQuery":
			answered++
			fmt.Fprint(response, `{"ok":true,"result":true}`)
		case "/getUpdates":
			if strings.Join(input.Allowed, ",") != "message,callback_query" {
				t.Errorf("polling did not subscribe to card actions")
			}
			fmt.Fprint(response, `{"ok":true,"result":[]}`)
		default:
			t.Errorf("unexpected Telegram call %s", request.URL.Path)
			response.WriteHeader(404)
		}
	}))
	defer server.Close()
	newBot := func() *Bot {
		bot, err := NewBot("11:fake_token", owner, ledger, nil)
		if err != nil {
			t.Fatal(err)
		}
		bot.client.baseURL = server.URL + "/"
		return bot
	}
	bot := newBot()
	if _, err := bot.client.updates(ctx, 0); err != nil {
		t.Fatal(err)
	}
	send := func(msg message) {
		t.Helper()
		if err := bot.handleUpdate(ctx, 11, update{Message: msg}); err != nil {
			t.Fatal(err)
		}
	}
	press := func(cardID int64, label string) string {
		t.Helper()
		for _, row := range keyboards[cardID].Rows {
			for _, button := range row {
				if button.Text == label {
					return button.Data
				}
			}
		}
		t.Fatalf("missing %q on card %d", label, cardID)
		return ""
	}
	click := func(cardID int64, data string) {
		t.Helper()
		msg := messages[cardID]
		callback := callbackQuery{ID: "callback", From: telegramUser{ID: 123}, Message: &msg, Data: data}
		if err := bot.handleUpdate(ctx, 11, update{Callback: &callback}); err != nil {
			t.Fatal(err)
		}
	}
	reply := func(id int64, text string) message {
		msg := privateMessage(id, 123, text)
		prompt := messages[ledger.edit.PromptID]
		msg.ReplyTo = &prompt
		return msg
	}
	assertEntry := func(amount int64, category string) {
		t.Helper()
		if len(ledger.saved) != 1 || ledger.saved[0].AmountSen != amount || ledger.saved[0].Category != category {
			t.Fatalf("unexpected ledger entries %#v", ledger.saved)
		}
	}

	send(privateMessage(1, 123, "RM20 Food Dinner"))
	send(privateMessage(1, 123, "RM20 Food Dinner")) // Retried provider delivery.
	assertEntry(2000, "Food")
	const cardID int64 = 100
	if !strings.Contains(messages[cardID].Text, "Dinner") || !strings.Contains(messages[cardID].Text, "RM20.00") {
		t.Fatal("receipt did not include the saved transaction")
	}
	click(cardID, press(cardID, "Amount"))
	if ledger.edit.PromptID == 0 || ledger.edit.Field != "amount" {
		t.Fatal("amount action did not create a durable reply prompt")
	}
	send(reply(2, "RM0"))
	assertEntry(2000, "Food")
	validReply := reply(3, "RM25.50")
	bot = newBot() // Pending prompts and existing cards survive a restart.
	send(validReply)
	send(validReply)
	assertEntry(2550, "Food")
	if !strings.Contains(messages[cardID].Text, "RM25.50") {
		t.Fatal("amount change did not update the original Telegram card")
	}

	click(cardID, press(cardID, "Category"))
	transportChoice := press(cardID, "Transport")
	click(cardID, transportChoice)
	assertEntry(2550, "Transport")
	click(cardID, press(cardID, "Account"))
	click(cardID, press(cardID, "Cash"))
	if ledger.saved[0].Account != "Cash" {
		t.Fatal("account not updated")
	}
	click(cardID, press(cardID, "Note"))
	send(reply(4, "Bus home"))
	if ledger.saved[0].Note != "Bus home" {
		t.Fatal("note not updated")
	}
	click(cardID, press(cardID, "Date"))
	send(reply(5, "2026-11-01"))
	if ledger.saved[0].Date != "2026-11-01" {
		t.Fatal("date not updated")
	}
	click(cardID, press(cardID, "Type"))
	click(cardID, press(cardID, "Income"))
	assertEntry(2550, "Other")
	click(cardID, press(cardID, "Category"))
	click(cardID, press(cardID, "Salary"))
	if ledger.saved[0].Type != "income" {
		t.Fatal("type not updated")
	}
	click(cardID, press(cardID, "Category"))
	click(cardID, press(cardID, "Enter custom category"))
	send(reply(6, "Part time salary"))
	assertEntry(2550, "Part time salary")

	// An old button cannot revert changes since that menu was shown.
	click(cardID, transportChoice)
	assertEntry(2550, "Part time salary")
	click(cardID, press(cardID, "Amount"))
	staleReply := reply(7, "99")
	ledger.saved[0].Note = "Changed from app"
	send(staleReply)
	assertEntry(2550, "Part time salary")
	if ledger.saved[0].Note != "Changed from app" {
		t.Fatal("stale prompt overwrote an app edit")
	}
	click(cardID, press(cardID, "Amount"))
	canceledReply := reply(8, "30")
	send(privateMessage(9, 123, "/cancel"))
	send(canceledReply)
	assertEntry(2550, "Part time salary")
	click(cardID, press(cardID, "Amount"))
	expiredReply := reply(11, "RM30")
	ledger.edit.Expires = time.Now().Add(-time.Minute).Unix()
	send(expiredReply)
	assertEntry(2550, "Part time salary")

	// Another user and a forwarded card cannot mutate the owner's ledger.
	foreign := messages[cardID]
	foreign.Chat.ID = 456
	_ = bot.handleUpdate(ctx, 11, update{Callback: &callbackQuery{
		ID: "foreign", From: telegramUser{ID: 456}, Message: &foreign, Data: press(cardID, "Delete"),
	}})
	assertEntry(2550, "Part time salary")

	click(cardID, press(cardID, "Delete"))
	click(cardID, press(cardID, "Keep transaction"))
	assertEntry(2550, "Part time salary")
	click(cardID, press(cardID, "Delete"))
	oldConfirm := press(cardID, "Confirm delete")
	ledger.saved[0].AmountSen = 2600 // App changed it after confirmation appeared.
	click(cardID, oldConfirm)
	assertEntry(2600, "Part time salary")
	click(cardID, press(cardID, "Amount"))
	replyAfterDelete := reply(10, "RM50")
	click(cardID, press(cardID, "Delete"))
	confirm := press(cardID, "Confirm delete")
	click(cardID, confirm)
	click(cardID, confirm)
	send(replyAfterDelete)
	send(privateMessage(1, 123, "RM20 Food Dinner"))
	if len(ledger.saved) != 0 || !strings.Contains(messages[cardID].Text, "deleted") {
		t.Fatal("deleted transaction was recreated by a retried message or stale reply")
	}
	if answered < 20 {
		t.Fatal("card actions were not acknowledged")
	}
	failCard = true
	send(privateMessage(12, 123, "RM1 Food Receipt test"))
	send(privateMessage(12, 123, "RM1 Food Receipt test"))
	assertEntry(100, "Food")
	if !strings.Contains(messages[nextMessage-1].Text, "was saved") {
		t.Fatal("failed card delivery did not distinguish saved data from its receipt")
	}
	failEdit = true
	if err := bot.client.editCard(ctx, 123, cardID, "same", inlineKeyboard{}); err != nil {
		t.Fatalf("idempotent card refresh failed: %v", err)
	}
}

func privateMessage(id, from int64, text string) message {
	msg := message{ID: id, Date: time.Now().Unix(), From: telegramUser{ID: from}, Text: text}
	msg.Chat.ID, msg.Chat.Type = from, "private"
	return msg
}
