package reminders

import "testing"

func TestValidateInputAcceptsTimeRange(t *testing.T) {
	input := normalizeInput(Input{
		Title:    "Morning appointment",
		Category: "event",
		Date:     "2026-09-15",
		Time:     "08:00",
		EndTime:  "10:00",
	})

	if issues := validateInput(input); len(issues) > 0 {
		t.Fatalf("expected valid time range, got issues: %v", issues)
	}
}

func TestNormalizeInputAcceptsStartTimeAlias(t *testing.T) {
	input := normalizeInput(Input{
		Title:     "Morning appointment",
		Category:  "event",
		Date:      "2026-09-15",
		StartTime: "08:00",
		EndTime:   "10:00",
	})

	if input.Time != "08:00" {
		t.Fatalf("expected startTime alias to set Time, got %q", input.Time)
	}

	if input.StartTime != "08:00" {
		t.Fatalf("expected canonical StartTime to stay in sync, got %q", input.StartTime)
	}

	if issues := validateInput(input); len(issues) > 0 {
		t.Fatalf("expected valid startTime alias, got issues: %v", issues)
	}
}

func TestValidateInputRejectsEndTimeWithoutStartTime(t *testing.T) {
	input := normalizeInput(Input{
		Title:    "Incomplete appointment",
		Category: "event",
		Date:     "2026-09-15",
		EndTime:  "10:00",
	})

	issues := validateInput(input)
	if issues["time"] == "" {
		t.Fatalf("expected time issue, got: %v", issues)
	}
}

func TestValidateInputRejectsEndTimeBeforeStartTime(t *testing.T) {
	input := normalizeInput(Input{
		Title:    "Backwards appointment",
		Category: "event",
		Date:     "2026-09-15",
		Time:     "10:00",
		EndTime:  "08:00",
	})

	issues := validateInput(input)
	if issues["endTime"] == "" {
		t.Fatalf("expected endTime issue, got: %v", issues)
	}
}
