package actions

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronSchedule represents a parsed 5-field cron expression
// (minute hour day-of-month month day-of-week), as used by GitHub Actions.
type cronSchedule struct {
	minutes    map[int]bool
	hours      map[int]bool
	daysOfMon  map[int]bool
	months     map[int]bool
	daysOfWeek map[int]bool
}

// parseCron parses a standard 5-field cron expression. GitHub Actions uses UTC
// and POSIX cron semantics.
func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression %q must have 5 fields", expr)
	}
	min, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hr, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, err
	}
	dom, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, err
	}
	mon, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, err
	}
	dow, err := parseCronField(fields[4], 0, 6) // 0 and 7 both Sunday; normalize 7->0
	if err != nil {
		return nil, err
	}
	if dow[7] {
		dow[0] = true
		delete(dow, 7)
	}
	return &cronSchedule{minutes: min, hours: hr, daysOfMon: dom, months: mon, daysOfWeek: dow}, nil
}

// parseCronField parses a single cron field supporting "*", lists ("1,2"),
// ranges ("1-5"), and steps ("*/15", "0-30/10").
func parseCronField(field string, lo, hi int) (map[int]bool, error) {
	out := make(map[int]bool)
	for _, part := range strings.Split(field, ",") {
		step := 1
		rangePart := part
		if slash := strings.Index(part, "/"); slash >= 0 {
			rangePart = part[:slash]
			s, err := strconv.Atoi(part[slash+1:])
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("invalid step in cron field %q", field)
			}
			step = s
		}

		start, end := lo, hi
		switch {
		case rangePart == "*":
			// full range
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			a, err1 := strconv.Atoi(bounds[0])
			b, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range in cron field %q", field)
			}
			start, end = a, b
		default:
			v, err := strconv.Atoi(rangePart)
			if err != nil {
				return nil, fmt.Errorf("invalid value in cron field %q", field)
			}
			start, end = v, v
		}

		if start < lo || end > hi || start > end {
			return nil, fmt.Errorf("cron field %q out of range [%d,%d]", field, lo, hi)
		}
		for v := start; v <= end; v += step {
			out[v] = true
		}
	}
	return out, nil
}

// Next returns the next activation time strictly after the given time, in UTC.
// It searches forward minute-by-minute up to roughly a year and returns the
// zero time if no match is found.
func (c *cronSchedule) Next(after time.Time) time.Time {
	t := after.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(366 * 24 * time.Hour)
	for t.Before(limit) {
		if c.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

func (c *cronSchedule) matches(t time.Time) bool {
	if !c.minutes[t.Minute()] || !c.hours[t.Hour()] || !c.months[int(t.Month())] {
		return false
	}
	// POSIX cron: if both DOM and DOW are restricted, a match on either suffices.
	domRestricted := len(c.daysOfMon) != 31
	dowRestricted := len(c.daysOfWeek) != 7
	domMatch := c.daysOfMon[t.Day()]
	dowMatch := c.daysOfWeek[int(t.Weekday())]
	switch {
	case domRestricted && dowRestricted:
		return domMatch || dowMatch
	case domRestricted:
		return domMatch
	case dowRestricted:
		return dowMatch
	default:
		return true
	}
}
