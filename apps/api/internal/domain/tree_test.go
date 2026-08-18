package domain

import (
	"testing"
	"time"
)

func TestBuildPageTreeOrderAndNesting(t *testing.T) {
	now := time.Unix(0, 0).UTC()
	pages := []Page{
		{ID: "c", ParentID: "a", Title: "Child", Position: 0, CreatedAt: now},
		{ID: "b", ParentID: "", Title: "Second", Position: 1, CreatedAt: now},
		{ID: "a", ParentID: "", Title: "First", Position: 0, CreatedAt: now},
	}
	tree := BuildPageTree(pages)
	if len(tree) != 2 || tree[0].ID != "a" || tree[1].ID != "b" {
		t.Fatalf("roots: %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != "c" {
		t.Fatalf("child: %+v", tree[0].Children)
	}
}

func TestFilterGuestPagesHidesDraftAncestors(t *testing.T) {
	pages := []Page{
		{ID: "root", Status: PageStatusDraft, Title: "secret"},
		{ID: "pub", ParentID: "root", Status: PageStatusPublished, Title: "leaked?"},
		{ID: "ok", Status: PageStatusPublished, Title: "visible"},
	}
	got := FilterGuestPages(pages)
	if len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("got %+v", got)
	}
}
