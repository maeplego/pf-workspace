package web

import (
	"net/http"
	"strings"
)

func (s *Server) search(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	types := strings.TrimSpace(r.URL.Query().Get("types"))
	hits, err := s.svc.Search(sub, wsID, q, types)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits})
}
