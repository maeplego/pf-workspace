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
		if err := s.svc.Ping(); err != nil {
			http.Error(w, `{"ok":false}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	api := mw.Handler(http.HandlerFunc(s.handleAPI))
	mux.Handle("/v1/", api)
	mux.HandleFunc("GET /v1/files/{id}/content", s.serveFileContent)
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
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
	r = r.WithContext(withTenantSvc(r.Context(), s.svc.ForOrg(u.OrgID)))
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/search"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/search")
		s.search(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/boards/") && strings.HasSuffix(r.URL.Path, "/sprints"):
		boardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/boards/"), "/sprints")
		s.listSprints(w, r, u.Sub, boardID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/boards/") && strings.HasSuffix(r.URL.Path, "/sprints"):
		boardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/boards/"), "/sprints")
		s.createSprint(w, r, u.Sub, boardID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sprints/") && strings.HasSuffix(r.URL.Path, "/burndown"):
		sprintID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sprints/"), "/burndown")
		s.sprintBurndown(w, r, u.Sub, sprintID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sprints/"):
		sprintID := strings.TrimPrefix(r.URL.Path, "/v1/sprints/")
		if sprintID == "" || strings.Contains(sprintID, "/") {
			http.NotFound(w, r)
			return
		}
		s.getSprint(w, r, u.Sub, sprintID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/sprints/"):
		sprintID := strings.TrimPrefix(r.URL.Path, "/v1/sprints/")
		s.updateSprint(w, r, u.Sub, sprintID)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/sprints/"):
		sprintID := strings.TrimPrefix(r.URL.Path, "/v1/sprints/")
		s.deleteSprint(w, r, u.Sub, sprintID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/boards/") && strings.HasSuffix(r.URL.Path, "/archive"):
		boardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/boards/"), "/archive")
		s.archiveBoard(w, r, u.Sub, boardID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/boards/") && strings.HasSuffix(r.URL.Path, "/unarchive"):
		boardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/boards/"), "/unarchive")
		s.unarchiveBoard(w, r, u.Sub, boardID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/archive"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/archive")
		s.archivePage(w, r, u.Sub, pageID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/unarchive"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/unarchive")
		s.unarchivePage(w, r, u.Sub, pageID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/documents/") && strings.HasSuffix(r.URL.Path, "/trash"):
		docID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/documents/"), "/trash")
		s.trashDocument(w, r, u.Sub, docID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/documents/") && strings.HasSuffix(r.URL.Path, "/untrash"):
		docID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/documents/"), "/untrash")
		s.untrashDocument(w, r, u.Sub, docID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.Contains(r.URL.Path, "/versions/"):
		rest := strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		pageID, num, ok := strings.Cut(rest, "/versions/")
		if !ok || pageID == "" || num == "" {
			http.NotFound(w, r)
			return
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid version"}})
			return
		}
		s.getPageVersion(w, r, u.Sub, pageID, n)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/versions"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/versions")
		s.listPageVersions(w, r, u.Sub, pageID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/diff"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/diff")
		s.diffPageVersions(w, r, u.Sub, pageID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/restore"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/restore")
		s.restorePageVersion(w, r, u.Sub, pageID)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/uploads/config":
		s.uploadConfig(w)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads":
		s.uploadLocal(w, r, u.Sub)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads/link":
		s.linkRemote(w, r, u.Sub)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/attachments"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/attachments")
		s.attachPage(w, r, u.Sub, pageID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/") && strings.HasSuffix(r.URL.Path, "/attachments"):
		pageID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/pages/"), "/attachments")
		s.listPageAttachments(w, r, u.Sub, pageID)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
		s.createWorkspace(w, r, u)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces":
		s.listWorkspaces(w, r, u.Sub)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chat-tickets":
		s.issueChatTicket(w, r, u.Sub)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/channels"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/channels")
		s.createChannel(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/channels"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/channels")
		s.listChannels(w, r, u.Sub, wsID)
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
		s.pageTree(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/pages"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/pages")
		s.createPage(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/documents"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/documents")
		s.createDocument(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/documents"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/documents")
		s.listDocuments(w, r, u.Sub, wsID)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/members/me"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members/me")
		s.syncMemberDisplayName(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.Contains(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members/"):
		rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
		parts := strings.Split(rest, "/")
		if len(parts) == 3 && parts[1] == "members" && parts[2] != "" {
			s.getMember(w, r, u.Sub, parts[0], parts[2])
			return
		}
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/members"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members")
		s.listMembers(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/audit-events"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/audit-events")
		s.listAuditEvents(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.Contains(r.URL.Path, "/invitations/") && strings.HasSuffix(r.URL.Path, "/revoke"):
		rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
		rest = strings.TrimSuffix(rest, "/revoke")
		wsID, inviteID, ok := strings.Cut(rest, "/invitations/")
		if !ok || wsID == "" || inviteID == "" {
			http.NotFound(w, r)
			return
		}
		s.revokeInvitation(w, r, u.Sub, wsID, inviteID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.Contains(r.URL.Path, "/invitations/") && strings.HasSuffix(r.URL.Path, "/resend"):
		rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
		rest = strings.TrimSuffix(rest, "/resend")
		wsID, inviteID, ok := strings.Cut(rest, "/invitations/")
		if !ok || wsID == "" || inviteID == "" {
			http.NotFound(w, r)
			return
		}
		s.resendInvitation(w, r, u.Sub, wsID, inviteID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/invitations"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/invitations")
		s.listInvitations(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/invitations"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/invitations")
		s.createInvitation(w, r, u, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/invitations/"):
		token := strings.TrimPrefix(r.URL.Path, "/v1/invitations/")
		s.previewInvitation(w, r, u.Sub, token)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/invitations/") && strings.HasSuffix(r.URL.Path, "/accept"):
		token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/invitations/"), "/accept")
		s.acceptInvitation(w, r, u, token)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/members"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/members")
		s.addMember(w, r, u.Sub, wsID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/boards"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/boards")
		s.createBoard(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/") && strings.HasSuffix(r.URL.Path, "/boards"):
		wsID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/boards")
		s.listBoards(w, r, u.Sub, wsID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/workspaces/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			s.getWorkspace(w, r, u.Sub, parts[0])
			return
		}
		http.NotFound(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/files/"):
		fileID := strings.TrimPrefix(r.URL.Path, "/v1/files/")
		if fileID == "" || strings.Contains(fileID, "/") {
			http.NotFound(w, r)
			return
		}
		s.getFileMeta(w, r, u.Sub, fileID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/pages/"):
		pageID := strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		s.getPage(w, r, u.Sub, pageID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/pages/"):
		pageID := strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		s.updatePage(w, r, u.Sub, pageID)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/documents/"):
		docID := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
		s.updateDocument(w, r, u.Sub, docID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/documents/"):
		docID := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
		s.getDocument(w, r, u.Sub, docID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/boards/"):
		boardID := strings.TrimPrefix(r.URL.Path, "/v1/boards/")
		s.getBoard(w, r, u.Sub, boardID)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/columns/") && strings.HasSuffix(r.URL.Path, "/cards"):
		columnID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/columns/"), "/cards")
		s.createCard(w, r, u.Sub, columnID)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/cards/"):
		cardID := strings.TrimPrefix(r.URL.Path, "/v1/cards/")
		s.getCard(w, r, u.Sub, cardID)
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
	case errors.Is(err, domain.ErrTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]string{"code": "too_large", "message": "file too large"}})
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

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request, u auth.User) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	ws, err := s.ts(r.Context()).CreateWorkspace(u.Sub, strings.TrimSpace(body.Name), u.OrgID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request, sub string) {
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": s.ts(r.Context()).ListWorkspaces(sub)})
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	ws, err := s.ts(r.Context()).GetWorkspace(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	members, err := s.ts(r.Context()).ListMembers(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) getMember(w http.ResponseWriter, r *http.Request, sub, wsID, memberSub string) {
	m, err := s.ts(r.Context()).GetMember(sub, wsID, memberSub)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) syncMemberDisplayName(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	m, err := s.ts(r.Context()).SyncMemberDisplayName(sub, wsID, body.DisplayName)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
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
	m, err := s.ts(r.Context()).AddMember(actorSub, wsID, strings.TrimSpace(body.Sub), body.Role)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request, u auth.User, wsID string) {
	var body struct {
		Role         domain.Role `json:"role"`
		MaxUses      int         `json:"maxUses"`
		TTLHours     int         `json:"ttlHours"`
		InvitedEmail string      `json:"invitedEmail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	inv, token, err := s.ts(r.Context()).CreateInvitation(u.Sub, wsID, body.Role, body.InvitedEmail, body.MaxUses, body.TTLHours)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": inv,
		"token":      token,
	})
}

func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request, actorSub, wsID string) {
	invitations, err := s.ts(r.Context()).ListInvitations(actorSub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": invitations})
}

func (s *Server) revokeInvitation(w http.ResponseWriter, r *http.Request, actorSub, wsID, inviteID string) {
	inv, err := s.ts(r.Context()).RevokeInvitation(actorSub, wsID, inviteID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func (s *Server) resendInvitation(w http.ResponseWriter, r *http.Request, actorSub, wsID, inviteID string) {
	inv, token, err := s.ts(r.Context()).ResendInvitation(actorSub, wsID, inviteID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"invitation": inv,
		"token":      token,
	})
}

func (s *Server) previewInvitation(w http.ResponseWriter, r *http.Request, sub, token string) {
	inv, ws, err := s.ts(r.Context()).PreviewInvitation(sub, strings.TrimSpace(token))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspace": ws,
		"invitation": map[string]any{
			"id":           inv.ID,
			"role":         inv.Role,
			"maxUses":      inv.MaxUses,
			"useCount":     inv.UseCount,
			"expiresAt":    inv.ExpiresAt,
			"invitedEmail": inv.InvitedEmail,
		},
	})
}

func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request, u auth.User, token string) {
	member, ws, err := s.ts(r.Context()).AcceptInvitation(u.Sub, u.Email, strings.TrimSpace(token))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": member, "workspace": ws})
}

func (s *Server) listAuditEvents(w http.ResponseWriter, r *http.Request, actorSub, wsID string) {
	events, err := s.ts(r.Context()).ListAuditEvents(actorSub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) createBoard(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	board, err := s.ts(r.Context()).CreateBoard(sub, wsID, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, board)
}

func (s *Server) listBoards(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	boards, err := s.ts(r.Context()).ListBoards(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	live := make([]domain.Board, 0)
	archived := make([]domain.Board, 0)
	for _, b := range boards {
		if b.ArchivedAt == nil {
			live = append(live, b)
		} else {
			archived = append(archived, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"boards": live, "archivedBoards": archived})
}

func (s *Server) getBoard(w http.ResponseWriter, r *http.Request, sub, boardID string) {
	board, err := s.ts(r.Context()).GetBoard(sub, boardID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) archiveBoard(w http.ResponseWriter, r *http.Request, sub, boardID string) {
	if err := s.ts(r.Context()).ArchiveBoard(sub, boardID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unarchiveBoard(w http.ResponseWriter, r *http.Request, sub, boardID string) {
	if err := s.ts(r.Context()).UnarchiveBoard(sub, boardID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	card, err := s.ts(r.Context()).CreateCard(sub, columnID, strings.TrimSpace(body.Title), body.Description)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request, sub, cardID string) {
	card, err := s.ts(r.Context()).GetCard(sub, cardID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request, sub, cardID string) {
	var body struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Version     int     `json:"version"`
		SprintID    *string `json:"sprintId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	var sprintID *string
	if body.SprintID != nil {
		sid := strings.TrimSpace(*body.SprintID)
		sprintID = &sid
	}
	card, err := s.ts(r.Context()).UpdateCard(sub, cardID, strings.TrimSpace(body.Title), body.Description, sprintID, body.Version)
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
	page, err := s.ts(r.Context()).CreatePage(sub, wsID, strings.TrimSpace(body.ParentID), strings.TrimSpace(body.Title), body.Body, body.Status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, page)
}

func (s *Server) pageTree(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	tree, err := s.ts(r.Context()).PageTree(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	archived, err := s.ts(r.Context()).ArchivedPages(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tree": tree, "archived": archived})
}

func (s *Server) getPage(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	page, err := s.ts(r.Context()).GetPage(sub, pageID)
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
	page, err := s.ts(r.Context()).UpdatePage(sub, pageID, body.Title, body.Body, body.Status, body.ParentID, body.Version)
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

func (s *Server) archivePage(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	if err := s.ts(r.Context()).ArchivePage(sub, pageID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unarchivePage(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	if err := s.ts(r.Context()).UnarchivePage(sub, pageID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	card, err := s.ts(r.Context()).MoveCard(sub, cardID, body.ColumnID, body.Position, body.Version)
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
	ticket, err := s.ts(r.Context()).IssueCollabTicket(sub, strings.TrimSpace(body.CollabDocumentID))
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
	doc, err := s.ts(r.Context()).CreateDocument(sub, wsID, strings.TrimSpace(body.Title), body.Body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	docs, err := s.ts(r.Context()).ListDocuments(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	trashed, err := s.ts(r.Context()).ListTrashedDocuments(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs, "trashed": trashed})
}

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request, sub, docID string) {
	doc, err := s.ts(r.Context()).GetDocument(sub, docID)
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
	doc, err := s.ts(r.Context()).UpdateDocumentTitle(sub, docID, body.Title)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) trashDocument(w http.ResponseWriter, r *http.Request, sub, docID string) {
	if err := s.ts(r.Context()).TrashDocument(sub, docID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) untrashDocument(w http.ResponseWriter, r *http.Request, sub, docID string) {
	if err := s.ts(r.Context()).RestoreDocument(sub, docID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	authz, err := s.svc.Unscoped().AuthorizeCollab(strings.TrimSpace(body.Ticket), strings.TrimSpace(body.DocumentName))
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
	text, err := s.svc.Unscoped().CollabPlaintext(strings.TrimSpace(body.CollabDocumentID))
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
		EditorSub        string `json:"editorSub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	if err := s.svc.Unscoped().ApplyCollabSnapshot(strings.TrimSpace(body.CollabDocumentID), body.Plaintext, strings.TrimSpace(body.EditorSub)); err != nil {
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
	ticket, err := s.ts(r.Context()).IssueChatTicket(sub, strings.TrimSpace(body.ChannelID))
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
	ch, err := s.ts(r.Context()).CreateChannel(sub, wsID, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request, sub, wsID string) {
	chs, err := s.ts(r.Context()).ListChannels(sub, wsID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": chs})
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request, sub, channelID string) {
	var body struct {
		Body             string `json:"body"`
		AttachmentFileID string `json:"attachmentFileId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	msg, err := s.ts(r.Context()).PostMessage(sub, channelID, body.Body, strings.TrimSpace(body.AttachmentFileID))
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
	msgs, err := s.ts(r.Context()).ListMessages(sub, channelID, after)
	if err != nil {
		writeErr(w, err)
		return
	}
	if msgs == nil {
		msgs = []domain.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}
