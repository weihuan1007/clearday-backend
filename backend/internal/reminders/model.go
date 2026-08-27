package reminders

import (
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

type Reminder struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Date      string    `json:"date"`
	Notes     string    `json:"notes"`
	IsDone    bool      `json:"isDone"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Date     string `json:"date"`
	Notes    string `json:"notes"`
	IsDone   bool   `json:"isDone"`
}

var allowedCategories = map[string]bool{
	"subscription": true,
	"event":        true,
	"payment":      true,
	"personal":     true,
}

func normalizeInput(input Input) Input {
	input.Title = strings.TrimSpace(input.Title)
	input.Category = strings.TrimSpace(strings.ToLower(input.Category))
	input.Date = strings.TrimSpace(input.Date)
	input.Notes = strings.TrimSpace(input.Notes)

	if input.Category == "" {
		input.Category = "personal"
	}

	return input
}

func validateInput(input Input) map[string]string {
	issues := make(map[string]string)

	if input.Title == "" {
		issues["title"] = "Title is required."
	} else if len(input.Title) > 60 {
		issues["title"] = "Title must be 60 characters or less."
	}

	if !allowedCategories[input.Category] {
		issues["category"] = "Choose a valid reminder type."
	}

	if _, err := time.Parse(DateLayout, input.Date); err != nil {
		issues["date"] = "Use a valid date in YYYY-MM-DD format."
	}

	if len(input.Notes) > 180 {
		issues["notes"] = "Notes must be 180 characters or less."
	}

	return issues
}
