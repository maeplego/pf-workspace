package domain

import "testing"

func TestLineDiffReplaceMiddle(t *testing.T) {
	got := LineDiff("a\nb\nc", "a\nx\nc")
	want := []DiffLine{
		{Op: "equal", Text: "a"},
		{Op: "delete", Text: "b"},
		{Op: "insert", Text: "x"},
		{Op: "equal", Text: "c"},
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestLineDiffEmpty(t *testing.T) {
	if len(LineDiff("", "")) != 0 {
		t.Fatal("empty")
	}
	got := LineDiff("", "hi")
	if len(got) != 1 || got[0].Op != "insert" || got[0].Text != "hi" {
		t.Fatalf("%+v", got)
	}
}
