package dynamodbstore

import "testing"

func TestRecordToReminderAcceptsTimeAliases(t *testing.T) {
	item := record{
		ID:        "test-id",
		Title:     "Manual database item",
		Category:  "event",
		Date:      "2026-09-15",
		StartTime: "08:00",
		End:       "10:00",
	}

	reminder := item.toReminder()
	if reminder.Time != "08:00" {
		t.Fatalf("expected startTime alias to become Time, got %q", reminder.Time)
	}

	if reminder.StartTime != "08:00" {
		t.Fatalf("expected startTime alias to become StartTime, got %q", reminder.StartTime)
	}

	if reminder.EndTime != "10:00" {
		t.Fatalf("expected end alias to become EndTime, got %q", reminder.EndTime)
	}
}

func TestRecordToReminderAcceptsManualCasingAliases(t *testing.T) {
	item := record{
		ID:             "test-id",
		Title:          "Manual casing item",
		Category:       "event",
		Date:           "2026-09-15",
		StartTimeTitle: "13:00",
		EndTimeSnake:   "14:00",
	}

	reminder := item.toReminder()
	if reminder.Time != "13:00" {
		t.Fatalf("expected StartTime alias to become Time, got %q", reminder.Time)
	}

	if reminder.StartTime != "13:00" {
		t.Fatalf("expected StartTime alias to become StartTime, got %q", reminder.StartTime)
	}

	if reminder.EndTime != "14:00" {
		t.Fatalf("expected end_time alias to become EndTime, got %q", reminder.EndTime)
	}
}
