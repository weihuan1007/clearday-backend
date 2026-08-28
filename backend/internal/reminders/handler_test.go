package reminders

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerCreatePreservesTimeRange(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	}
	handler := NewHandler(service, nil)

	body := bytes.NewBufferString(`{
		"title": "Morning appointment",
		"category": "event",
		"date": "2026-09-15",
		"time": "08:00",
		"startTime": "08:00",
		"endTime": "10:00",
		"notes": "Saved through the API"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/reminders", body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var reminder Reminder
	if err := json.NewDecoder(response.Body).Decode(&reminder); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if reminder.Time != "08:00" {
		t.Fatalf("expected response time to be 08:00, got %q", reminder.Time)
	}

	if reminder.StartTime != "08:00" {
		t.Fatalf("expected response startTime to be 08:00, got %q", reminder.StartTime)
	}

	if reminder.EndTime != "10:00" {
		t.Fatalf("expected response endTime to be 10:00, got %q", reminder.EndTime)
	}
}

func TestHandlerUpdateWithPostPreservesTimeRange(t *testing.T) {
	repository := &memoryRepository{
		items: []Reminder{{
			ID:       "reminder-1",
			Title:    "Original title",
			Category: "subscription",
			Date:     "2026-09-15",
		}},
	}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	}
	handler := NewHandler(service, nil)

	body := bytes.NewBufferString(`{
		"title": "Updated appointment",
		"category": "event",
		"date": "2026-09-16",
		"time": "09:45",
		"startTime": "09:45",
		"endTime": "10:45",
		"notes": "Updated through the API"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/reminders/reminder-1", body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, response.Code, response.Body.String())
	}

	var reminder Reminder
	if err := json.NewDecoder(response.Body).Decode(&reminder); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if reminder.Title != "Updated appointment" {
		t.Fatalf("expected updated title, got %q", reminder.Title)
	}

	if reminder.Time != "09:45" || reminder.StartTime != "09:45" || reminder.EndTime != "10:45" {
		t.Fatalf("expected updated range 09:45-10:45, got time=%q startTime=%q endTime=%q", reminder.Time, reminder.StartTime, reminder.EndTime)
	}
}

type memoryRepository struct {
	items []Reminder
}

func (repository *memoryRepository) List(context.Context) ([]Reminder, error) {
	return repository.items, nil
}

func (repository *memoryRepository) Get(_ context.Context, id string) (Reminder, error) {
	for _, item := range repository.items {
		if item.ID == id {
			return item, nil
		}
	}
	return Reminder{}, ErrNotFound
}

func (repository *memoryRepository) Create(_ context.Context, reminder Reminder) (Reminder, error) {
	repository.items = append(repository.items, reminder)
	return reminder, nil
}

func (repository *memoryRepository) Update(_ context.Context, reminder Reminder) (Reminder, error) {
	for index, item := range repository.items {
		if item.ID == reminder.ID {
			repository.items[index] = reminder
			return reminder, nil
		}
	}
	return Reminder{}, ErrNotFound
}

func (repository *memoryRepository) Delete(_ context.Context, id string) error {
	for index, item := range repository.items {
		if item.ID == id {
			repository.items = append(repository.items[:index], repository.items[index+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
