package domain

import "testing"

func TestExtractMentionsResolvesMembersOnly(t *testing.T) {
	members := []string{"demo-user-a", "owner-1"}
	got := ExtractMentions("hey @demo-user-a and @nobody and @owner-1 again @demo-user-a", members)
	if len(got) != 2 || got[0] != "demo-user-a" || got[1] != "owner-1" {
		t.Fatalf("got %v", got)
	}
	empty := ExtractMentions("no mentions", members)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty %v", empty)
	}
}
