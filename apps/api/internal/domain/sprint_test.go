package domain

import (
	"testing"
	"time"
)

func TestColumnIsDone(t *testing.T) {
	if !ColumnIsDone("Done") || !ColumnIsDone("done") {
		t.Fatal("Done column should match")
	}
	if ColumnIsDone("In Progress") {
		t.Fatal("In Progress is not Done")
	}
}

func TestBurndownRemaining(t *testing.T) {
	start := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	sp := Sprint{ID: "s1", StartAt: start, EndAt: end}
	done := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	cards := []Card{
		{ID: "a", SprintID: "s1", CreatedAt: start, CompletedAt: &done},
		{ID: "b", SprintID: "s1", CreatedAt: start},
		{ID: "other", SprintID: "other", CreatedAt: start},
	}
	bd := BurndownFor(sp, cards)
	if len(bd.Points) != 3 {
		t.Fatalf("points %d", len(bd.Points))
	}
	if bd.Unit != "cards" {
		t.Fatalf("unit %s", bd.Unit)
	}
	if bd.Points[0].Date != "2026-08-01" || bd.Points[0].Remaining != 2 {
		t.Fatalf("day0 %+v", bd.Points[0])
	}
	if bd.Points[1].Remaining != 1 {
		t.Fatalf("day1 remaining %d", bd.Points[1].Remaining)
	}
	if bd.Points[2].Remaining != 1 {
		t.Fatalf("day2 remaining %d", bd.Points[2].Remaining)
	}
}

func TestValidSprintWindow(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if ValidSprintWindow(start, start) {
		t.Fatal("equal instants invalid")
	}
	if !ValidSprintWindow(start, start.Add(time.Hour)) {
		t.Fatal("same calendar day with later end is valid")
	}
	if ValidSprintWindow(start, start.AddDate(0, 0, MaxSprintDays)) {
		t.Fatal("91 calendar days should be over the cap")
	}
}
