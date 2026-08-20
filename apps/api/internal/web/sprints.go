package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
)

func parseTimeRFC3339(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, domain.ErrInvalid
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, raw)
	}
	if err != nil {
		return time.Time{}, domain.ErrInvalid
	}
	return t.UTC(), nil
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request, sub, boardID string) {
	var body struct {
		Name    string `json:"name"`
		StartAt string `json:"startAt"`
		EndAt   string `json:"endAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	startAt, err := parseTimeRFC3339(body.StartAt)
	if err != nil {
		writeErr(w, domain.ErrInvalid)
		return
	}
	endAt, err := parseTimeRFC3339(body.EndAt)
	if err != nil {
		writeErr(w, domain.ErrInvalid)
		return
	}
	sp, err := s.ts(r.Context()).CreateSprint(sub, boardID, body.Name, startAt, endAt)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sp)
}

func (s *Server) listSprints(w http.ResponseWriter, r *http.Request, sub, boardID string) {
	list, err := s.ts(r.Context()).ListSprints(sub, boardID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []domain.Sprint{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sprints": list})
}

func (s *Server) getSprint(w http.ResponseWriter, r *http.Request, sub, sprintID string) {
	sp, err := s.ts(r.Context()).GetSprint(sub, sprintID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

func (s *Server) updateSprint(w http.ResponseWriter, r *http.Request, sub, sprintID string) {
	var body struct {
		Name    string  `json:"name"`
		StartAt *string `json:"startAt"`
		EndAt   *string `json:"endAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	var startAt, endAt *time.Time
	if body.StartAt != nil {
		t, err := parseTimeRFC3339(*body.StartAt)
		if err != nil {
			writeErr(w, domain.ErrInvalid)
			return
		}
		startAt = &t
	}
	if body.EndAt != nil {
		t, err := parseTimeRFC3339(*body.EndAt)
		if err != nil {
			writeErr(w, domain.ErrInvalid)
			return
		}
		endAt = &t
	}
	sp, err := s.ts(r.Context()).UpdateSprint(sub, sprintID, body.Name, startAt, endAt)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

func (s *Server) deleteSprint(w http.ResponseWriter, r *http.Request, sub, sprintID string) {
	if err := s.ts(r.Context()).DeleteSprint(sub, sprintID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sprintBurndown(w http.ResponseWriter, r *http.Request, sub, sprintID string) {
	bd, err := s.ts(r.Context()).SprintBurndown(sub, sprintID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bd)
}

func (s *Server) listPageVersions(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	list, err := s.ts(r.Context()).ListPageVersions(sub, pageID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []domain.PageVersionInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": list})
}

func (s *Server) getPageVersion(w http.ResponseWriter, r *http.Request, sub, pageID string, number int) {
	v, err := s.ts(r.Context()).GetPageVersion(sub, pageID, number)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) diffPageVersions(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	from, err1 := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("from")))
	to, err2 := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("to")))
	if err1 != nil || err2 != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "from and to required"}})
		return
	}
	diff, err := s.ts(r.Context()).DiffPageVersions(sub, pageID, from, to)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (s *Server) restorePageVersion(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	var body struct {
		Number  int `json:"number"`
		Version int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	page, err := s.ts(r.Context()).RestorePageVersion(sub, pageID, body.Number, body.Version)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
