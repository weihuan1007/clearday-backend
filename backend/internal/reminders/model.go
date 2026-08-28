package reminders

import (
	"strings"
	"time"
)

const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04"
)

type Reminder struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Date      string    `json:"date"`
	Time      string    `json:"time,omitempty"`
	StartTime string    `json:"startTime,omitempty"`
	EndTime   string    `json:"endTime,omitempty"`
	Notes     string    `json:"notes"`
	IsDone    bool      `json:"isDone"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Title          string `json:"title"`
	Category       string `json:"category"`
	Date           string `json:"date"`
	Time           string `json:"time"`
	Start          string `json:"start"`
	StartTime      string `json:"startTime"`
	StartTimeTitle string `json:"StartTime"`
	StartTimeSnake string `json:"start_time"`
	End            string `json:"end"`
	EndTime        string `json:"endTime"`
	EndTimeTitle   string `json:"EndTime"`
	EndTimeSnake   string `json:"end_time"`
	Notes          string `json:"notes"`
	IsDone         bool   `json:"isDone"`
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
	input.Time = strings.TrimSpace(input.Time)
	input.Start = strings.TrimSpace(input.Start)
	input.StartTime = strings.TrimSpace(input.StartTime)
	input.StartTimeTitle = strings.TrimSpace(input.StartTimeTitle)
	input.StartTimeSnake = strings.TrimSpace(input.StartTimeSnake)
	input.End = strings.TrimSpace(input.End)
	input.EndTime = strings.TrimSpace(input.EndTime)
	input.EndTimeTitle = strings.TrimSpace(input.EndTimeTitle)
	input.EndTimeSnake = strings.TrimSpace(input.EndTimeSnake)
	input.Notes = strings.TrimSpace(input.Notes)

	if input.Time == "" {
		input.Time = firstNonEmpty(input.StartTime, input.StartTimeTitle, input.StartTimeSnake, input.Start)
	}
	if input.EndTime == "" {
		input.EndTime = firstNonEmpty(input.EndTimeTitle, input.EndTimeSnake, input.End)
	}
	input.StartTime = input.Time
	input.StartTimeTitle = input.Time
	input.StartTimeSnake = input.Time
	input.Start = input.Time
	input.End = input.EndTime
	input.EndTimeTitle = input.EndTime
	input.EndTimeSnake = input.EndTime

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

	start, hasStart := parseClockTime(input.Time, issues, "time", "Use a valid start time in HH:MM format.")
	end, hasEnd := parseClockTime(input.EndTime, issues, "endTime", "Use a valid end time in HH:MM format.")

	if input.Time == "" && input.EndTime != "" {
		issues["time"] = "Choose a start time before adding an end time."
	}

	if hasStart && hasEnd && !end.After(start) {
		issues["endTime"] = "End time must be after start time."
	}

	if len(input.Notes) > 180 {
		issues["notes"] = "Notes must be 180 characters or less."
	}

	return issues
}

func parseClockTime(value string, issues map[string]string, field string, message string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(TimeLayout, value)
	if err != nil {
		issues[field] = message
		return time.Time{}, false
	}

	return parsed, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
