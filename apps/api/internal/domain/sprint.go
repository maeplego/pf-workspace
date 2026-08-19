package domain

import (
	"strings"
	"time"
)

func ColumnIsDone(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), DoneColumnName)
}

func UTCDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func EachUTCDate(start, end time.Time) []time.Time {
	d := UTCDate(start)
	last := UTCDate(end)
	if last.Before(d) {
		return nil
	}
	var out []time.Time
	for !d.After(last) {
		out = append(out, d)
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func SprintDayCount(start, end time.Time) int {
	return len(EachUTCDate(start, end))
}

func ValidSprintWindow(start, end time.Time) bool {
	if !end.After(start) {
		return false
	}
	n := SprintDayCount(start, end)
	return n >= 1 && n <= MaxSprintDays
}

// CardOpenOn reports whether the card still counts as remaining at the end of day D (UTC).
// Membership is the current sprint assignment; there is no assignment event log.
func CardOpenOn(c Card, day time.Time) bool {
	day = UTCDate(day)
	if UTCDate(c.CreatedAt).After(day) {
		return false
	}
	if c.CompletedAt != nil && !UTCDate(*c.CompletedAt).After(day) {
		return false
	}
	return true
}

func BurndownFor(sprint Sprint, cards []Card) Burndown {
	out := Burndown{SprintID: sprint.ID, Unit: "cards", Points: []BurndownPoint{}}
	for _, day := range EachUTCDate(sprint.StartAt, sprint.EndAt) {
		remaining := 0
		for _, c := range cards {
			if c.SprintID != sprint.ID {
				continue
			}
			if CardOpenOn(c, day) {
				remaining++
			}
		}
		out.Points = append(out.Points, BurndownPoint{
			Date:      day.Format("2006-01-02"),
			Remaining: remaining,
		})
	}
	return out
}

func (v PageVersion) Info() PageVersionInfo {
	return PageVersionInfo{
		PageID:    v.PageID,
		Number:    v.Number,
		Title:     v.Title,
		Sub:       v.Sub,
		CreatedAt: v.CreatedAt,
	}
}
