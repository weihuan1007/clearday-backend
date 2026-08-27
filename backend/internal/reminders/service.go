package reminders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"time"
)

var ErrNotFound = errors.New("reminder not found")

type ValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (err ValidationError) Error() string {
	return "validation failed"
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

func (service *Service) List(ctx context.Context) ([]Reminder, error) {
	items, err := service.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(left, right int) bool {
		if items[left].Date == items[right].Date {
			return items[left].Title < items[right].Title
		}
		return items[left].Date < items[right].Date
	})

	return items, nil
}

func (service *Service) Create(ctx context.Context, input Input) (Reminder, error) {
	input = normalizeInput(input)
	if issues := validateInput(input); len(issues) > 0 {
		return Reminder{}, ValidationError{Fields: issues}
	}

	now := service.now().UTC()
	reminder := Reminder{
		ID:        newID(),
		Title:     input.Title,
		Category:  input.Category,
		Date:      input.Date,
		Notes:     input.Notes,
		IsDone:    input.IsDone,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return service.repository.Create(ctx, reminder)
}

func (service *Service) Update(ctx context.Context, id string, input Input) (Reminder, error) {
	input = normalizeInput(input)
	if issues := validateInput(input); len(issues) > 0 {
		return Reminder{}, ValidationError{Fields: issues}
	}

	item, err := service.repository.Get(ctx, id)
	if err != nil {
		return Reminder{}, err
	}

	item.Title = input.Title
	item.Category = input.Category
	item.Date = input.Date
	item.Notes = input.Notes
	item.IsDone = input.IsDone
	item.UpdatedAt = service.now().UTC()
	return service.repository.Update(ctx, item)
}

func (service *Service) Delete(ctx context.Context, id string) error {
	return service.repository.Delete(ctx, id)
}

func newID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return hex.EncodeToString(buffer[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}
