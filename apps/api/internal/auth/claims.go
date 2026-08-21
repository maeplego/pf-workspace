package auth

import (
	"encoding/json"
	"strings"
)

// ClaimNames maps portfolio tenant fields onto IdP-specific claim keys (Auth0 / Entra / P01).
type ClaimNames struct {
	OrgID         string // default org_id
	Organizations string // default organizations (array of objects or strings)
}

func DefaultClaimNames() ClaimNames {
	return ClaimNames{OrgID: "org_id", Organizations: "organizations"}
}

func ParseClaimNames(orgClaim, orgsClaim string) ClaimNames {
	c := DefaultClaimNames()
	if v := strings.TrimSpace(orgClaim); v != "" {
		c.OrgID = v
	}
	if v := strings.TrimSpace(orgsClaim); v != "" {
		c.Organizations = v
	}
	return c
}

// claimString reads a string claim; supports dotted paths (e.g. extension_OrgId is a single key).
func claimString(claims map[string]any, key string) string {
	if key == "" || claims == nil {
		return ""
	}
	// Prefer exact key; also try first segment of comma-separated alternates.
	for _, k := range strings.Split(key, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if v, ok := claims[k]; ok {
			switch t := v.(type) {
			case string:
				return strings.TrimSpace(t)
			case json.Number:
				return strings.TrimSpace(t.String())
			case float64:
				return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(jsonNumber(t)), ".0"))
			}
		}
	}
	return ""
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func orgIDFromClaims(claims map[string]any, names ClaimNames) string {
	if id := claimString(claims, names.OrgID); id != "" {
		return id
	}
	// Fallbacks used by common IdPs when custom mapping is not set.
	for _, k := range []string{"org_id", "orgId", "organization_id", "tid"} {
		if id := claimString(claims, k); id != "" {
			return id
		}
	}
	orgs := organizationsFromClaims(claims, names)
	if len(orgs) > 0 {
		return orgs[0].OrgID
	}
	return ""
}

type orgMembership struct {
	OrgID   string
	OrgName string
	Role    string
}

func organizationsFromClaims(claims map[string]any, names ClaimNames) []orgMembership {
	if claims == nil {
		return nil
	}
	keys := []string{names.Organizations, "organizations", "orgs"}
	var raw any
	for _, k := range keys {
		if k == "" {
			continue
		}
		if v, ok := claims[k]; ok {
			raw = v
			break
		}
	}
	if raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]orgMembership, 0, len(arr))
	for _, item := range arr {
		switch t := item.(type) {
		case string:
			id := strings.TrimSpace(t)
			if id != "" {
				out = append(out, orgMembership{OrgID: id, OrgName: id, Role: "member"})
			}
		case map[string]any:
			id := firstString(t, "org_id", "orgId", "id")
			if id == "" {
				continue
			}
			out = append(out, orgMembership{
				OrgID:   id,
				OrgName: firstString(t, "org_name", "orgName", "name", "org_id", "orgId"),
				Role:    firstString(t, "role", "roles"),
			})
		}
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := claimString(m, k); s != "" {
			return s
		}
	}
	return ""
}
