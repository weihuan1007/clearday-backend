package money

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

// Match the Money form's categories so chat entries appear alongside manual ones.
var chatCategories = map[string][]string{
	"expense":  {"Food", "Transport", "Bills", "Shopping", "Health", "Family", "Entertainment", "Other"},
	"income":   {"Salary", "Freelance", "Refund", "Gift", "Investment", "Other"},
	"transfer": {"Top Up", "Bank Move", "Card Payment", "Savings", "Other"},
}

// ChatCategories returns the same choices used to parse new chat entries.
func ChatCategories(kind string) []string {
	return append([]string(nil), chatCategories[kind]...)
}

var editAmountPattern = regexp.MustCompile(`(?i)^(?:RM *)?((?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]{1,9})(?:\.[0-9]{1,2})?)$`)

// EditChatTransaction validates a single field using the ledger's normal rules.
// The caller saves it conditionally against the original transaction.
func EditChatTransaction(entry Transaction, field, value string, sentAt time.Time) (Transaction, error) {
	value = strings.TrimSpace(value)
	switch field {
	case "amount":
		match := editAmountPattern.FindStringSubmatch(value)
		if match == nil {
			return entry, errors.New("enter an amount like 20 or RM20.50")
		}
		amount, err := chatAmountSen(match[1])
		if err != nil {
			return entry, err
		}
		entry.AmountSen = amount
	case "category":
		entry.Category = value
	case "account":
		entry.Account = value
	case "note":
		if value == "-" {
			value = ""
		}
		entry.Note = value
	case "date":
		date, remaining, err := chatDate(strings.Fields(value), sentAt)
		if err != nil || value == "" || len(remaining) > 0 {
			return entry, errors.New("enter today, yesterday, or a date like 2026-11-24")
		}
		entry.Date = date
	case "type":
		entry.Type = strings.ToLower(value)
		// A category from the previous type may not be meaningful for this one.
		validCategory := false
		for _, category := range chatCategories[entry.Type] {
			validCategory = validCategory || entry.Category == category
		}
		if !validCategory {
			entry.Category = "Other"
		}
	default:
		return entry, errors.New("choose a field from the transaction card")
	}
	return entry, entry.validate()
}

// ParseChatEntry uses the original send time, so delivery delays cannot shift
// a transaction into the next day. Category and account inference is kept
// separate from amount/date validation, and the saved card shows the result.
func ParseChatEntry(text string, sentAt time.Time, id, source string) (Transaction, error) {
	words := strings.Fields(text)
	kind, words := chatType(words)
	amountSen, words, err := chatAmount(words)
	if err != nil {
		return Transaction{}, err
	}
	date, words, err := chatDate(words, sentAt)
	if err != nil {
		return Transaction{}, err
	}
	afterAmountKind, words := chatType(words)
	if afterAmountKind != "" {
		if kind != "" && kind != afterAmountKind {
			return Transaction{}, errors.New("choose one transaction type per message")
		}
		kind = afterAmountKind
	}
	if kind == "" {
		kind = "expense"
	}
	// Protect multiword categories such as Bank Move and Card Payment from
	// being mistaken for account names.
	category, words := takeChatCategory(kind, words)
	account, words, err := chatAccount(words)
	if err != nil {
		return Transaction{}, err
	}
	if category == "" {
		category, words = takeChatCategory(kind, words)
	}
	if category == "" {
		category = inferChatCategory(kind, words)
	}
	entry := Transaction{
		ID: id, Type: kind, AmountSen: amountSen, Date: date,
		Category: category, Account: account, Note: strings.Join(words, " "), Source: source,
	}
	return entry, entry.validate()
}

func chatAmountSen(value string) (int64, error) {
	ringgit, fraction, _ := strings.Cut(strings.ReplaceAll(value, ",", ""), ".")
	whole, err := strconv.ParseInt(ringgit, 10, 32)
	if err != nil || whole > maxAmountSen/100 {
		return 0, errors.New("the amount exceeds RM999,999,999.99")
	}
	// The pattern has already limited the fraction to one or two digits.
	fraction = (fraction + "00")[:2]
	sen := int64(fraction[0]-'0')*10 + int64(fraction[1]-'0')
	return whole*100 + sen, nil
}
