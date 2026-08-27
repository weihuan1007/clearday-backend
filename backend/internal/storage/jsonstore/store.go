package jsonstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"clearday/backend/internal/reminders"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("json store path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte("[]\n"), 0o600); err != nil {
			return nil, err
		}
	}

	return &Store{path: path}, nil
}

func (store *Store) List(context.Context) ([]reminders.Reminder, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.read()
}

func (store *Store) Get(_ context.Context, id string) (reminders.Reminder, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	items, err := store.read()
	if err != nil {
		return reminders.Reminder{}, err
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return reminders.Reminder{}, reminders.ErrNotFound
}

func (store *Store) Create(_ context.Context, reminder reminders.Reminder) (reminders.Reminder, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	items, err := store.read()
	if err != nil {
		return reminders.Reminder{}, err
	}

	items = append(items, reminder)
	if err := store.write(items); err != nil {
		return reminders.Reminder{}, err
	}

	return reminder, nil
}

func (store *Store) Update(_ context.Context, reminder reminders.Reminder) (reminders.Reminder, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	items, err := store.read()
	if err != nil {
		return reminders.Reminder{}, err
	}

	for index, item := range items {
		if item.ID == reminder.ID {
			items[index] = reminder
			return reminder, store.write(items)
		}
	}

	return reminders.Reminder{}, reminders.ErrNotFound
}

func (store *Store) Delete(_ context.Context, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	items, err := store.read()
	if err != nil {
		return err
	}

	next := items[:0]
	deleted := false
	for _, item := range items {
		if item.ID == id {
			deleted = true
			continue
		}
		next = append(next, item)
	}

	if !deleted {
		return reminders.ErrNotFound
	}

	return store.write(next)
}

func (store *Store) read() ([]reminders.Reminder, error) {
	data, err := os.ReadFile(store.path)
	if err != nil {
		return nil, err
	}

	var items []reminders.Reminder
	if len(data) == 0 {
		return items, nil
	}

	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	return items, nil
}

func (store *Store) write(items []reminders.Reminder) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	temp := store.path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, store.path)
}
