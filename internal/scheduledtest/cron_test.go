package scheduledtest

import (
	"testing"
	"time"
)

func TestParseCron_MatchesStandardFiveFieldExpression(t *testing.T) {
	spec, err := ParseCron("*/15 9-17 * * 1-5")
	if err != nil {
		t.Fatalf("ParseCron() error = %v", err)
	}

	if !spec.Match(time.Date(2026, time.May, 15, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected weekday 09:30 to match")
	}
	if spec.Match(time.Date(2026, time.May, 15, 9, 31, 0, 0, time.UTC)) {
		t.Fatalf("expected minute 31 not to match")
	}
	if spec.Match(time.Date(2026, time.May, 16, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected Saturday not to match")
	}
}

func TestParseCron_DayOfMonthOrDayOfWeek(t *testing.T) {
	spec, err := ParseCron("0 8 1 * 5")
	if err != nil {
		t.Fatalf("ParseCron() error = %v", err)
	}

	if !spec.Match(time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected first day that is also Friday to match")
	}
	if !spec.Match(time.Date(2026, time.May, 8, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected Friday to match")
	}
	if !spec.Match(time.Date(2026, time.June, 1, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected first day of month to match")
	}
	if spec.Match(time.Date(2026, time.June, 2, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected non-first non-Friday not to match")
	}
}

func TestParseCron_RejectsInvalidExpression(t *testing.T) {
	tests := []string{
		"* * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * * 13 *",
		"* * * * 8",
		"*/0 * * * *",
		"10-5 * * * *",
	}

	for _, expr := range tests {
		if _, err := ParseCron(expr); err == nil {
			t.Fatalf("ParseCron(%q) error = nil, want error", expr)
		}
	}
}
