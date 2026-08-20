package domain

import "testing"

func TestParseSearchTypesDefaultAndInvalid(t *testing.T) {
	got, err := ParseSearchTypes("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("default types %v", got)
	}
	if _, err := ParseSearchTypes("page,bogus"); err != ErrInvalid {
		t.Fatalf("bogus: %v", err)
	}
	got, err = ParseSearchTypes(" card ,PAGE ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "card" || got[1] != "page" {
		t.Fatalf("got %v", got)
	}
}

func TestContainsFoldAndSnippet(t *testing.T) {
	if !ContainsFold("Hello Wiki", "wiki") {
		t.Fatal("expected match")
	}
	if ContainsFold("Hello", "xyz") {
		t.Fatal("unexpected match")
	}
	sn := Snippet("the secret draft lives here", "DRAFT", 80)
	if !ContainsFold(sn, "draft") {
		t.Fatalf("snippet %q", sn)
	}
}
