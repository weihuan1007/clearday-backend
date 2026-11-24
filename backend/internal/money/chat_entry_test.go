package money

import (
	"strings"
	"testing"
	"time"
)

func TestParseChatEntry(t *testing.T) {
	sent := time.Date(2026, 11, 9, 16, 5, 0, 0, time.UTC) // November 10 in Kuala Lumpur.
	tests := []struct {
		text, kind, date, category, note string
		sen                              int64
		account                          string
	}{
		{"RM20 food dinner", "expense", "2026-11-10", "Food", "dinner", 2000, "Unspecified"},
		{"rm 1,234.56 bills @yesterday", "expense", "2026-11-09", "Bills", "", 123456, "Unspecified"},
		{"income RM1200 Salary @2026-11-01", "income", "2026-11-01", "Salary", "", 120000, "Unspecified"},
		{"transfer RM5.2 bank move savings @today", "transfer", "2026-11-10", "Bank Move", "savings", 520, "Unspecified"},
		{"RM0.01 Other", "expense", "2026-11-10", "Other", "", 1, "Unspecified"},
		{"RM12.50 lunch today tng", "expense", "2026-11-10", "Food", "lunch", 1250, "Touch 'n Go"},
		{"RM12.50 lunch today", "expense", "2026-11-10", "Food", "lunch", 1250, "Unspecified"},
		{"RM12.50 dinner", "expense", "2026-11-10", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 yesterday", "expense", "2026-11-09", "Other", "", 1250, "Unspecified"},
		{"RM12.50", "expense", "2026-11-10", "Other", "", 1250, "Unspecified"},
		{"RM12.50 TNG dinner yesterday", "expense", "2026-11-09", "Food", "dinner", 1250, "Touch 'n Go"},
		{"TODAY rm 12.50 tng Lunch", "expense", "2026-11-10", "Food", "Lunch", 1250, "Touch 'n Go"},
		{"lunch RM12.50 using tng", "expense", "2026-11-10", "Food", "lunch", 1250, "Touch 'n Go"},
		{"RM12.50 lunch via Touch 'n Go", "expense", "2026-11-10", "Food", "lunch", 1250, "Touch 'n Go"},
		{"RM12.50 lunch paid with apple pay", "expense", "2026-11-10", "Food", "lunch", 1250, "Apple Pay"},
		{"RM12.50 lunch tng ewallet", "expense", "2026-11-10", "Food", "lunch", 1250, "Touch 'n Go"},
		{"RM12.50 lunch touch and go", "expense", "2026-11-10", "Food", "lunch", 1250, "Touch 'n Go"},
		{"RM12.50 lunch grabpay", "expense", "2026-11-10", "Food", "lunch", 1250, "GrabPay"},
		{"RM12.50 lunch credit card", "expense", "2026-11-10", "Food", "lunch", 1250, "Credit Card"},
		{"RM12.50 lunch debitcard", "expense", "2026-11-10", "Food", "lunch", 1250, "Debit Card"},
		{"spent RM12.50 breakfast ytd cash", "expense", "2026-11-09", "Food", "breakfast", 1250, "Cash"},
		{"paid MYR12.5 dinner tdy bank", "expense", "2026-11-10", "Food", "dinner", 1250, "Bank"},
		{"12.50 dinner yesterday cash", "expense", "2026-11-09", "Food", "dinner", 1250, "Cash"},
		{"12.50RM dinner", "expense", "2026-11-10", "Food", "dinner", 1250, "Unspecified"},
		{"12.50 RM dinner", "expense", "2026-11-10", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 dinner on 24/11/2026 tng", "expense", "2026-11-24", "Food", "dinner", 1250, "Touch 'n Go"},
		{"RM12.50 dinner 2026-11-24 tng", "expense", "2026-11-24", "Food", "dinner", 1250, "Touch 'n Go"},
		{"RM12.50 dinner @yesterday tng", "expense", "2026-11-09", "Food", "dinner", 1250, "Touch 'n Go"},
		{"RM12.50 dinner today @yesterday", "expense", "2026-11-09", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 dinner 2 days ago tng", "expense", "2026-11-08", "Food", "dinner", 1250, "Touch 'n Go"},
		{"RM12.50 dinner tomorrow", "expense", "2026-11-11", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 dinner tmr", "expense", "2026-11-11", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 dinner last night", "expense", "2026-11-09", "Food", "dinner", 1250, "Unspecified"},
		{"RM12.50 lunch for 2 cash", "expense", "2026-11-10", "Food", "lunch for 2", 1250, "Cash"},
		{"  RM 12.50\nLunch  today, TNG!  ", "expense", "2026-11-10", "Food", "Lunch", 1250, "Touch 'n Go"},
		{"RM20 Transport Grab dinner today", "expense", "2026-11-10", "Transport", "Grab dinner", 2000, "Unspecified"},
		{"RM12.50 parking tng", "expense", "2026-11-10", "Transport", "parking", 1250, "Touch 'n Go"},
		{"RM50 petrol", "expense", "2026-11-10", "Transport", "petrol", 5000, "Unspecified"},
		{"RM90 groceries cash", "expense", "2026-11-10", "Food", "groceries", 9000, "Cash"},
		{"RM90 Shopping groceries cash", "expense", "2026-11-10", "Shopping", "groceries", 9000, "Cash"},
		{"MYR1,234.56 internet bank today", "expense", "2026-11-10", "Bills", "internet", 123456, "Bank"},
		{"RM20 clinic", "expense", "2026-11-10", "Health", "clinic", 2000, "Unspecified"},
		{"RM20 shopee", "expense", "2026-11-10", "Shopping", "shopee", 2000, "Unspecified"},
		{"RM20 tuition", "expense", "2026-11-10", "Family", "tuition", 2000, "Unspecified"},
		{"RM20 netflix", "expense", "2026-11-10", "Entertainment", "netflix", 2000, "Unspecified"},
		{"RM12.50 something unusual", "expense", "2026-11-10", "Other", "something unusual", 1250, "Unspecified"},
		{"RM12.50 farm lunch", "expense", "2026-11-10", "Food", "farm lunch", 1250, "Unspecified"},
		{"RM12.50 uniform", "expense", "2026-11-10", "Other", "uniform", 1250, "Unspecified"},
		{"RM12.50 午餐", "expense", "2026-11-10", "Other", "午餐", 1250, "Unspecified"},
		{"received RM3000 salary today bank", "income", "2026-11-10", "Salary", "", 300000, "Bank"},
		{"I received RM3000 salary today bank", "income", "2026-11-10", "Salary", "", 300000, "Bank"},
		{"today income RM3000 salary bank", "income", "2026-11-10", "Salary", "", 300000, "Bank"},
		{"RM50 income refund cash", "income", "2026-11-10", "Refund", "", 5000, "Cash"},
		{"transfer RM100 topup tng", "transfer", "2026-11-10", "Top Up", "topup", 10000, "Touch 'n Go"},
		{"transfer RM100 card payment credit card", "transfer", "2026-11-10", "Card Payment", "", 10000, "Credit Card"},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			entry, err := ParseChatEntry(test.text, sent, "tg_test", "telegram")
			if err != nil {
				t.Fatal(err)
			}
			if entry.Type != test.kind || entry.Date != test.date || entry.Category != test.category ||
				entry.AmountSen != test.sen || entry.Note != test.note || entry.Source != "telegram" || entry.Account != test.account {
				t.Fatalf("unexpected transaction %#v", entry)
			}
		})
	}
	for _, text := range []string{"RM0 Food", "RM-20 Food", "RM20.123 Food", "RM1,20 Food", "RM1000000000 Food",
		"RM20 Food @2026-02-30", "USD20 Food", "RM12.50 lunch RM4 coffee", "RM20 lunch today yesterday",
		"RM20 lunch @today @yesterday", "RM20 lunch @nextweek", "RM20 lunch 30/02/2026",
		"RM20 lunch 24/11/26", "income RM20 expense lunch", "RM20 lunch cash tng", "12.50 lunch RM20",
		"RM20 lunch last week", "RM20 lunch next month"} {
		if _, err := ParseChatEntry(text, sent, "tg_test", "telegram"); err == nil {
			t.Errorf("accepted invalid message %q", text)
		}
	}
}

func TestChatMetadataCanAppearInDifferentOrders(t *testing.T) {
	sent := time.Date(2026, 11, 24, 16, 1, 0, 0, time.UTC) // Nov 25 in Malaysia.
	orders := [][]string{
		{"lunch", "yesterday", "tng"}, {"lunch", "tng", "yesterday"},
		{"yesterday", "lunch", "tng"}, {"yesterday", "tng", "lunch"},
		{"tng", "lunch", "yesterday"}, {"tng", "yesterday", "lunch"},
	}
	for _, order := range orders {
		for position := 0; position <= len(order); position++ {
			words := append([]string(nil), order[:position]...)
			words = append(words, "RM12.50")
			words = append(words, order[position:]...)
			text := strings.Join(words, " ")
			t.Run(text, func(t *testing.T) {
				entry, err := ParseChatEntry(text, sent, "tg_order", "telegram")
				if err != nil || entry.Category != "Food" || entry.Account != "Touch 'n Go" ||
					entry.AmountSen != 1250 || entry.Date != "2026-11-24" || entry.Note != "lunch" {
					t.Fatalf("incorrect parsed entry %#v, error %v", entry, err)
				}
			})
		}
	}
}

func TestRelativeChatDatesCrossCalendarBoundariesInMalaysia(t *testing.T) {
	for _, test := range []struct {
		sent string
		text string
		date string
	}{
		{"2026-12-31T15:59:00Z", "RM1 dinner today", "2026-12-31"},
		{"2026-12-31T16:01:00Z", "RM1 dinner today", "2027-01-01"},
		{"2026-12-31T16:01:00Z", "RM1 dinner yesterday", "2026-12-31"},
		{"2024-02-29T16:01:00Z", "RM1 dinner yesterday", "2024-02-29"},
		{"2026-02-28T16:01:00Z", "RM1 dinner yesterday", "2026-02-28"},
	} {
		sent, err := time.Parse(time.RFC3339, test.sent)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := ParseChatEntry(test.text, sent, "tg_boundary", "telegram")
		if err != nil || entry.Date != test.date {
			t.Fatalf("%s: date %s, error %v", test.sent, entry.Date, err)
		}
	}
}
