package scheduledtest

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronField struct {
	values map[int]struct{}
	any    bool
}

// CronSpec is a parsed standard five-field cron expression.
type CronSpec struct {
	expr   string
	minute cronField
	hour   cronField
	dom    cronField
	month  cronField
	dow    cronField
}

// ParseCron parses a standard five-field cron expression:
// minute hour day-of-month month day-of-week.
func ParseCron(expr string) (CronSpec, error) {
	normalized := strings.Join(strings.Fields(expr), " ")
	parts := strings.Fields(normalized)
	if len(parts) != 5 {
		return CronSpec{}, fmt.Errorf("cron expression must have 5 fields")
	}

	minute, err := parseCronField(parts[0], 0, 59, 60, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("minute field: %w", err)
	}
	hour, err := parseCronField(parts[1], 0, 23, 24, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("hour field: %w", err)
	}
	dom, err := parseCronField(parts[2], 1, 31, 31, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("day-of-month field: %w", err)
	}
	month, err := parseCronField(parts[3], 1, 12, 12, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("month field: %w", err)
	}
	dow, err := parseCronField(parts[4], 0, 7, 7, true)
	if err != nil {
		return CronSpec{}, fmt.Errorf("day-of-week field: %w", err)
	}

	return CronSpec{
		expr:   normalized,
		minute: minute,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
	}, nil
}

// ValidateCronExpression validates a standard five-field cron expression.
func ValidateCronExpression(expr string) error {
	_, err := ParseCron(expr)
	return err
}

// Match reports whether t falls on this cron schedule.
func (s CronSpec) Match(t time.Time) bool {
	if !s.minute.match(t.Minute()) || !s.hour.match(t.Hour()) || !s.month.match(int(t.Month())) {
		return false
	}

	domMatch := s.dom.match(t.Day())
	dowMatch := s.dow.match(int(t.Weekday()))
	if !s.dom.any && !s.dow.any {
		return domMatch || dowMatch
	}
	return domMatch && dowMatch
}

func parseCronField(raw string, minValue, maxValue, fullCount int, normalizeWeekday bool) (cronField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cronField{}, fmt.Errorf("empty field")
	}

	field := cronField{values: make(map[int]struct{})}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return cronField{}, fmt.Errorf("empty list item")
		}
		base := part
		step := 1
		if before, after, ok := strings.Cut(part, "/"); ok {
			base = strings.TrimSpace(before)
			parsedStep, err := strconv.Atoi(strings.TrimSpace(after))
			if err != nil || parsedStep <= 0 {
				return cronField{}, fmt.Errorf("invalid step %q", after)
			}
			step = parsedStep
		}

		start, end, err := cronRange(base, minValue, maxValue, step)
		if err != nil {
			return cronField{}, err
		}
		for value := start; value <= end; value += step {
			normalized := value
			if normalizeWeekday && normalized == 7 {
				normalized = 0
			}
			field.values[normalized] = struct{}{}
		}
	}
	field.any = len(field.values) == fullCount
	return field, nil
}

func cronRange(base string, minValue, maxValue, step int) (int, int, error) {
	base = strings.TrimSpace(base)
	switch {
	case base == "*":
		return minValue, maxValue, nil
	case strings.Contains(base, "-"):
		left, right, _ := strings.Cut(base, "-")
		start, err := parseCronNumber(left, minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseCronNumber(right, minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		if start > end {
			return 0, 0, fmt.Errorf("range start %d is greater than end %d", start, end)
		}
		return start, end, nil
	default:
		start, err := parseCronNumber(base, minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		if step > 1 {
			return start, maxValue, nil
		}
		return start, start, nil
	}
}

func parseCronNumber(raw string, minValue, maxValue int) (int, error) {
	raw = strings.TrimSpace(raw)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", raw)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("value %d out of range %d-%d", value, minValue, maxValue)
	}
	return value, nil
}

func (f cronField) match(value int) bool {
	_, ok := f.values[value]
	return ok
}
