package auth

import "testing"

func TestOrgIDFromClaimsMappedAndFallback(t *testing.T) {
	names := ParseClaimNames("https://schemas.example/org", "org_memberships")
	claims := map[string]any{
		"https://schemas.example/org": "tenant-a",
	}
	if got := orgIDFromClaims(claims, names); got != "tenant-a" {
		t.Fatalf("mapped claim: %q", got)
	}

	claims = map[string]any{
		"org_memberships": []any{
			map[string]any{"org_id": "o1", "org_name": "One", "role": "owner"},
			map[string]any{"orgId": "o2", "name": "Two"},
		},
	}
	if got := orgIDFromClaims(claims, names); got != "o1" {
		t.Fatalf("from list: %q", got)
	}

	claims = map[string]any{"tid": "entra-tenant"}
	if got := orgIDFromClaims(claims, DefaultClaimNames()); got != "entra-tenant" {
		t.Fatalf("tid fallback: %q", got)
	}
}

func TestOrganizationsFromStringList(t *testing.T) {
	claims := map[string]any{"organizations": []any{"a", "b"}}
	orgs := organizationsFromClaims(claims, DefaultClaimNames())
	if len(orgs) != 2 || orgs[0].OrgID != "a" {
		t.Fatalf("%+v", orgs)
	}
}
