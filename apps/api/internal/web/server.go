package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/portfolio/pf-workspace/api/internal/auth"
	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/realtime"
	"github.com/portfolio/pf-workspace/api/internal/service"
)

type Server struct {
	svc           *service.Service
	corsOrigin    string
	internalToken string
	hub           *realtime.Hub
}

func New(svc *service.Service, corsOrigin, internalToken string, hub *realtime.Hub) *Server {
	if hub == nil {
		hub = realtime.NewHub()
	}
	svc.SetBroadcaster(hub)
	return &Server{svc: svc, corsOrigin: corsOrigin, internalToken: internalToken, hub: hub}
}

func (s *Server) Routes(mw *auth.Middleware) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	api := mw.Handler(http.HandlerFunc(s.handleAPI))
	mux.Handle("/v1/", api)
	mux.HandleFunc("GET /chat/ws", s.chatWS)
	mux.HandleFunc("POST /internal/v1/collab/authorize", s.internalAuthorize)
	mux.HandleFunc("POST /internal/v1/collab/plaintext", s.internalPlaintext)
	mux.HandleFunc("POST /internal/v1/collab/snapshot", s.internalSnapshot)
	return s.withCORS(mux)
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Dev-User-Sub")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
		s.createWorkspace(w, r, u.Sub)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces":
		s.listWorkspaces(w, u.Sub)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chat-tickets":
		s.issueChatTicket(w, r, u.Sub)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/channels"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/channels")
		s.createChannel(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/channels"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/channels")
		s.listChannels(w, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/channels/") && strings.HasSuffix(r.URL.Path, "/messages"):
		channelID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/channels/"), "/messages")
		s.postMessage(w, r, u.Sub, channelID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/channels/") && strings.HasSuffix(r.URL.Path, "/messages"):
		channelID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/channels/"), "/messages")
		s.listMessages(w, r, u.Sub, channelID)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/collab-tickets":
		s.issueCollabTicket(w, r, u.Sub)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/pages/tree"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/pages/tree")
		s.pageTree(w, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/pages"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/pages")
		s.createPage(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/documents"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/documents")
		s.createDocument(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/documents"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/documents")
		s.listDocuments(w, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/members"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members")
		s.listMembers(w, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/members"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members")
		s.addMember(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/boards"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/boards")
		s.createBoard(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/boards"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/boards")
		s.listBoards(w, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			s.getWorkspace(w, u.Sub, parts[0])
			return
		}
		http.NotFound(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/"):
		pageID := strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		s.getPage(w, u.Sub, pageID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/pages/"):
		pageID := strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		s.updatePage(w, r, u.Sub, pageID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/documents/"):
		docID := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
		s.updateDocument(w, r, u.Sub, docID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/documents/"):
		docID := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
		s.getDocument(w, u.Sub, docID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/boards/"):
		boardID := strings.TrimPrefix(r.URL.Path, "/v1/boards/")
		s.getBoard(w, u.Sub, boardID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/columns/") && strings.HasSuffix(r.URL.Path, "/cards"):
		columnID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/columns/"), "/cards")
		s.createCard(w, r, u.Sub, columnID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/cards/"):
		cardID := strings.TrimPrefix(r.URL.Path, "/v1/cards/")
		s.getCard(w, u.Sub, cardID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/cards/") && strings.HasSuffix(r.URL.Path, "/move"):
		cardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/cards/"), "/move")
		s.moveCard(w, r, u.Sub, cardID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/cards/"):
		cardID := strings.TrimPrefix(r.URL.Path, "/v1/cards/")
		s.updateCard(w, r, u.Sub, cardID)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid"}})
	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "unauthorized"}})
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "not found"}})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "forbidden"}})
	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "conflict", "message": "version conflict"}})
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request, sub string) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	ws, err := s.svc.CreateWorkspace(sub, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, sub string) {
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": s.svc.ListWorkspaces(sub)})
}

func (s *Server) getWorkspace(w http.ResponseWriter, sub, wsID string) {
	ws, err := s.svc.GetWorkspace(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) listMembers(w http.ResponseWriter, sub, wsID string) {
	members, err := s.svc.ListMembers(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request, actorSub, wsID string) {
	var body struct {
		Sub  string      `json:"sub"`
		Role domain.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	m, err := s.svc.AddMember(actorSub, wsID, strings.TrimSpace(body.Sub), body.Role)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) createBoard(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	board, err := s.svc.CreateBoard(sub, wsID, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, board)
}

func (s *Server) listBoards(w http.ResponseWriter, sub, wsID string) {
	boards, err := s.svc.ListBoards(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"boards": boards})
}

func (s *Server) getBoard(w http.ResponseWriter, sub, boardID string) {
	board, err := s.svc.GetBoard(sub, boardID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) createCard(w http.ResponseWriter, r *http.Request, sub, columnID string) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	card, err := s.svc.CreateCard(sub, columnID, strings.TrimSpace(body.Title), body.Description)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) getCard(w http.ResponseWriter, sub, cardID string) {
	card, err := s.svc.GetCard(sub, cardID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request, sub, cardID string) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Version     int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	card, err := s.svc.UpdateCard(sub, cardID, strings.TrimSpace(body.Title), body.Description, body.Version)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "conflict", "message": "version conflict"}, "current": card})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) createPage(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		ParentID string `json:"parentId"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	page, err := s.svc.CreatePage(sub, wsID, strings.TrimSpace(body.ParentID), strings.TrimSpace(body.Title), body.Body, body.Status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, page)
}

func (s *Server) pageTree(w http.ResponseWriter, sub, wsID string) {
	tree, err := s.svc.PageTree(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tree": tree})
}

func (s *Server) getPage(w http.ResponseWriter, sub, pageID string) {
	page, err := s.svc.GetPage(sub, pageID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) updatePage(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	var body struct {
		Title    *string `json:"title"`
		Body     *string `json:"body"`
		Status   *string `json:"status"`
		ParentID *string `json:"parentId"`
		Version  int     `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	page, err := s.svc.UpdatePage(sub, pageID, body.Title, body.Body, body.Status, body.ParentID, body.Version)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "conflict", "message": "version conflict"}, "current": page})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) moveCard(w http.ResponseWriter, r *http.Request, sub, cardID string) {
	var body struct {
		ColumnID string `json:"columnId"`
		Position int    `json:"position"`
		Version  int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	card, err := s.svc.MoveCard(sub, cardID, body.ColumnID, body.Position, body.Version)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "conflict", "message": "version conflict"}, "current": card})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) issueCollabTicket(w http.ResponseWriter, r *http.Request, sub string) {
	var body struct {
		CollabDocumentID string `json:"collabDocumentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	ticket, err := s.svc.IssueCollabTicket(sub, strings.TrimSpace(body.CollabDocumentID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func (s *Server) createDocument(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	doc, err := s.svc.CreateDocument(sub, wsID, strings.TrimSpace(body.Title), body.Body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (s *Server) listDocuments(w http.ResponseWriter, sub, wsID string) {
	docs, err := s.svc.ListDocuments(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

func (s *Server) getDocument(w http.ResponseWriter, sub, docID string) {
	doc, err := s.svc.GetDocument(sub, docID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) updateDocument(w http.ResponseWriter, r *http.Request, sub, docID string) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	doc, err := s.svc.UpdateDocumentTitle(sub, docID, body.Title)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) requireInternal(w http.ResponseWriter, r *http.Request) bool {
	if s.internalToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "unauthorized"}})
		return false
	}
	got := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		got = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if got == "" {
		got = strings.TrimSpace(r.Header.Get("X-Internal-Token"))
	}
	if got != s.internalToken {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "unauthorized"}})
		return false
	}
	return true
}

func (s *Server) internalAuthorize(w http.ResponseWriter, r *http.Request) {
	if !s.requireInternal(w, r) {
		return
	}
	var body struct {
		Ticket       string `json:"ticket"`
		DocumentName string `json:"documentName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	authz, err := s.svc.AuthorizeCollab(strings.TrimSpace(body.Ticket), strings.TrimSpace(body.DocumentName))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authz)
}

func (s *Server) internalPlaintext(w http.ResponseWriter, r *http.Request) {
	if !s.requireInternal(w, r) {
		return
	}
	var body struct {
		CollabDocumentID string `json:"collabDocumentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	text, err := s.svc.CollabPlaintext(strings.TrimSpace(body.CollabDocumentID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plaintext": text})
}

func (s *Server) internalSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.requireInternal(w, r) {
		return
	}
	var body struct {
		CollabDocumentID string `json:"collabDocumentId"`
		Plaintext        string `json:"plaintext"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	if err := s.svc.ApplyCollabSnapshot(strings.TrimSpace(body.CollabDocumentID), body.Plaintext); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) issueChatTicket(w http.ResponseWriter, r *http.Request, sub string) {
	var body struct {
		ChannelID string `json:"channelId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	ticket, err := s.svc.IssueChatTicket(sub, strings.TrimSpace(body.ChannelID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func (s *Server) createChannel(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	ch, err := s.svc.CreateChannel(sub, wsID, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) listChannels(w http.ResponseWriter, sub, wsID string) {
	chs, err := s.svc.ListChannels(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": chs})
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request, sub, channelID string) {
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	msg, err := s.svc.PostMessage(sub, channelID, body.Body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request, sub, channelID string) {
	after := 0
	if q := strings.TrimSpace(r.URL.Query().Get("afterSeq")); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid afterSeq"}})
			return
		}
		after = n
	}
	msgs, err := s.svc.ListMessages(sub, channelID, after)
	if err != nil {
		writeErr(w, err)
		return
	}
	if msgs == nil {
		msgs = []domain.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}
