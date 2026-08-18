package domain

import "testing"

func TestValidCollabRoom(t *testing.T) {
	if ValidCollabRoom("../etc/passwd") {
		t.Fatal("path rejected")
	}
	if ValidCollabRoom("") {
		t.Fatal("empty")
	}
	if !ValidCollabRoom("01ARZ3NDEKTSV4RRFFQ69G5FAV") {
		t.Fatal("ulid ok")
	}
}
