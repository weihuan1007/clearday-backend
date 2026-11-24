package money

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

var chatEntryPattern = regexp.MustCompile(`(?i)^(?:(expense|income|transfer) )?RM *((?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]{1,9})(?:\.[0-9]{1,2})?) (.+)$`)

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
		parsed, err := ParseChatEntry("RM1 Other @"+strings.TrimPrefix(value, "@"), sentAt, entry.ID, entry.Source)
		if err != nil {
			return entry, err
		}
		entry.Date = parsed.Date
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
// a transaction into the next day. An explicit @date takes precedence.
func ParseChatEntry(text string, sentAt time.Time, id, source string) (Transaction, error) {
	location, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		return Transaction{}, errors.New("Malaysia time zone is unavailable")
	}
	text = strings.Join(strings.Fields(text), " ")
	date := sentAt.In(location)
	if split := strings.LastIndex(text, " @"); split >= 0 {
		switch value := strings.ToLower(text[split+2:]); value {
		case "today":
		case "yesterday":
			date = date.AddDate(0, 0, -1)
		default:
			date, err = time.Parse(time.DateOnly, value)
			if err != nil {
				return Transaction{}, errors.New("use @today, @yesterday, or a date like @2026-11-09")
			}
		}
		text = text[:split]
	}

	fields := chatEntryPattern.FindStringSubmatch(text)
	if fields == nil {
		return Transaction{}, errors.New("start with an RM amount followed by a category")
	}
	entryType := strings.ToLower(fields[1])
	if entryType == "" {
		entryType = "expense"
	}
	amountSen, err := chatAmountSen(fields[2])
	if err != nil {
		return Transaction{}, err
	}

	categoryAndNote := fields[3]
	lower := strings.ToLower(categoryAndNote)
	for _, category := range chatCategories[entryType] {
		name := strings.ToLower(category)
		if lower != name && !strings.HasPrefix(lower, name+" ") {
			continue
		}
		transaction := Transaction{
			ID: id, Type: entryType, AmountSen: amountSen, Date: date.Format(time.DateOnly),
			Category: category, Account: "Unspecified", Note: strings.TrimSpace(categoryAndNote[len(category):]), Source: source,
		}
		err := transaction.validate()
		return transaction, err
	}
	return Transaction{}, errors.New("choose a category from the Money tab for this transaction type")
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
