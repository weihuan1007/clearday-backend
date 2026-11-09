package money

import (
	"testing"
	"time"
)

func TestParseChatEntry(t *testing.T) {
	sent := time.Date(2026, 11, 9, 16, 5, 0, 0, time.UTC) // November 10 in Kuala Lumpur.
	tests := []struct {
		text, kind, date, category, note string
		sen                              int64
	}{
		{"RM20 food dinner", "expense", "2026-11-10", "Food", "dinner", 2000},
		{"rm 1,234.56 bills @yesterday", "expense", "2026-11-09", "Bills", "", 123456},
		{"income RM1200 Salary @2026-11-01", "income", "2026-11-01", "Salary", "", 120000},
		{"transfer RM5.2 bank move savings @today", "transfer", "2026-11-10", "Bank Move", "savings", 520},
		{"RM0.01 Other", "expense", "2026-11-10", "Other", "", 1},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			entry, err := ParseChatEntry(test.text, sent, "tg_test", "telegram")
			if err != nil {
				t.Fatal(err)
			}
			if entry.Type != test.kind || entry.Date != test.date || entry.Category != test.category ||
				entry.AmountSen != test.sen || entry.Note != test.note || entry.Source != "telegram" || entry.Account != "Unspecified" {
				t.Fatalf("unexpected transaction %#v", entry)
			}
		})
	}
	for _, text := range []string{"RM0 Food", "RM-20 Food", "RM20.123 Food", "RM1,20 Food", "RM1000000000 Food",
		"RM20", "RM20 Foodstuff dinner", "RM20 Food @2026-02-30", "income RM20 Food", "USD20 Food"} {
		if _, err := ParseChatEntry(text, sent, "tg_test", "telegram"); err == nil {
			t.Errorf("accepted invalid message %q", text)
		}
	}
}
