package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/portfolio/pf-workspace/api/internal/auth"
)

type orgMemberView struct {
	Sub         string `json:"sub"`
	Role        string `json:"role"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

func (s *Server) orgMembers(w http.ResponseWriter, r *http.Request, u auth.User) {
	orgID := strings.TrimSpace(u.OrgID)
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "org required"}})
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if authz := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(authz, "Bearer ") {
		if base := strings.TrimSpace(os.Getenv("OIDC_INTERNAL_BASE")); base != "" {
			list, err := fetchOrgMembers(base, orgID, authz)
			if err == nil {
				writeJSON(w, http.StatusOK, map[string]any{"members": filterOrgMembers(list, q)})
				return
			}
		}
	}
	// Dev auth fallback: enumerate members from currently visible workspaces in the same tenant.
	seen := map[string]orgMemberView{}
	for _, ws := range s.ts(r.Context()).ListWorkspaces(u.Sub) {
		members, err := s.ts(r.Context()).ListMembers(u.Sub, ws.ID)
		if err != nil {
			continue
		}
		for _, m := range members {
			if _, ok := seen[m.Sub]; ok {
				continue
			}
			seen[m.Sub] = orgMemberView{
				Sub:         m.Sub,
				Role:        string(m.Role),
				DisplayName: strings.TrimSpace(m.DisplayName),
			}
		}
	}
	out := make([]orgMemberView, 0, len(seen))
	for _, m := range seen {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b orgMemberView) int { return strings.Compare(a.Sub, b.Sub) })
	writeJSON(w, http.StatusOK, map[string]any{"members": filterOrgMembers(out, q)})
}

func fetchOrgMembers(base, orgID, authz string) ([]orgMemberView, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/organizations/"+url.PathEscape(orgID)+"/members", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authz)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("org members upstream failed")
	}
	var payload struct {
		Members []struct {
			UserID string `json:"userId"`
			Role   string `json:"role"`
			Email  string `json:"email"`
			Name   string `json:"name"`
		} `json:"members"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]orgMemberView, 0, len(payload.Members))
	for _, m := range payload.Members {
		if strings.TrimSpace(m.UserID) == "" {
			continue
		}
		out = append(out, orgMemberView{
			Sub:         strings.TrimSpace(m.UserID),
			Role:        strings.TrimSpace(m.Role),
			Email:       strings.ToLower(strings.TrimSpace(m.Email)),
			DisplayName: strings.TrimSpace(m.Name),
		})
	}
	return out, nil
}

func filterOrgMembers(members []orgMemberView, q string) []orgMemberView {
	if q == "" {
		return members
	}
	out := make([]orgMemberView, 0, len(members))
	for _, m := range members {
		if strings.Contains(strings.ToLower(m.Sub), q) ||
			strings.Contains(strings.ToLower(m.Email), q) ||
			strings.Contains(strings.ToLower(m.DisplayName), q) {
			out = append(out, m)
		}
	}
	return out
}
