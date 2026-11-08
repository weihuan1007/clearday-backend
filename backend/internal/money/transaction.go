package money

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const maxAmountSen = 99_999_999_999

var (
	errNotFound = errors.New("transaction not found")
	errConflict = errors.New("a transaction already uses this ID")
	validID     = regexp.MustCompile("^[A-Za-z0-9_-]{1,120}$")
)

// Transaction stores MYR as integer sen so sums and comparisons are exact.
type Transaction struct {
	ID        string `json:"id" dynamodbav:"id"`
	Type      string `json:"type" dynamodbav:"type"`
	AmountSen int64  `json:"amountSen" dynamodbav:"amountSen"`
	Date      string `json:"date" dynamodbav:"date"`
	Category  string `json:"category" dynamodbav:"category"`
	Account   string `json:"account" dynamodbav:"account"`
	Note      string `json:"note" dynamodbav:"note"`
	Source    string `json:"source" dynamodbav:"source"`
}

func (transaction *Transaction) validate() error {
	if !validID.MatchString(transaction.ID) {
		return errors.New("invalid transaction ID")
	}
	switch transaction.Type {
	case "expense", "income", "transfer":
	default:
		return errors.New("choose expense, income, or transfer")
	}
	if transaction.AmountSen < 1 || transaction.AmountSen > maxAmountSen {
		return errors.New("amount must be between RM0.01 and RM999,999,999.99")
	}
	if _, err := time.Parse("2006-01-02", transaction.Date); err != nil || strings.HasPrefix(transaction.Date, "0000-") {
		return errors.New("choose a valid transaction date")
	}
	transaction.Category = strings.TrimSpace(transaction.Category)
	transaction.Account = strings.TrimSpace(transaction.Account)
	transaction.Note = strings.TrimSpace(transaction.Note)
	transaction.Source = strings.TrimSpace(transaction.Source)
	if transaction.Source == "" {
		transaction.Source = "manual"
	}
	if transaction.Category == "" || utf8.RuneCountInString(transaction.Category) > 120 ||
		transaction.Account == "" || utf8.RuneCountInString(transaction.Account) > 120 {
		return errors.New("category and account must each contain 1 to 120 characters")
	}
	if utf8.RuneCountInString(transaction.Note) > 80 || utf8.RuneCountInString(transaction.Source) > 40 {
		return errors.New("note must be at most 80 characters and source at most 40")
	}
	return nil
}

// Every operation receives the verified owner separately from the transaction.
type Store interface {
	List(ctx context.Context, owner string) ([]Transaction, error)
	Create(ctx context.Context, owner string, transaction Transaction) (created bool, err error)
	Update(ctx context.Context, owner string, transaction Transaction) error
	Delete(ctx context.Context, owner, id string) error
}
