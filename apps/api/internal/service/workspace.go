package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/store/memory"
)

type Broadcaster interface {
	Broadcast(channelID string, payload []byte)
}

type Service struct {
	store *memory.Store
	now   func() time.Time
	bus   Broadcaster
}

func New(store *memory.Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) SetBroadcaster(bus Broadcaster) {
	s.bus = bus
}

func (s *Service) requireRole(wsID, sub string, need domain.Role) error {
	role, err := s.store.MemberRole(wsID, sub)
	if err != nil {
		return err
	}
	if !domain.RoleAtLeast(role, need) {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) requireWrite(wsID, sub string) error {
	return s.requireRole(wsID, sub, domain.RoleMember)
}

func (s *Service) requireRead(wsID, sub string) error {
	return s.requireRole(wsID, sub, domain.RoleGuest)
}

func (s *Service) CreateWorkspace(sub, name string) (domain.Workspace, error) {
	if name == "" {
		return domain.Workspace{}, domain.ErrForbidden
	}
	ws, err := s.store.CreateWorkspace(name, sub, s.now().UTC())
	if err != nil {
		return domain.Workspace{}, err
	}
	_, _ = s.store.CreateChannel(ws.ID, domain.DefaultChannelName, s.now().UTC())
	return ws, nil
}

func (s *Service) ListWorkspaces(sub string) []domain.Workspace {
	return s.store.ListWorkspacesForSub(sub)
}

func (s *Service) GetWorkspace(sub, wsID string) (domain.Workspace, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return domain.Workspace{}, err
	}
	return s.store.GetWorkspace(wsID)
}

func (s *Service) ListMembers(sub, wsID string) ([]domain.Member, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	return s.store.ListMembers(wsID)
}

func (s *Service) AddMember(actorSub, wsID, memberSub string, role domain.Role) (domain.Member, error) {
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return domain.Member{}, err
	}
	if role == "" {
		role = domain.RoleMember
	}
	if role == domain.RoleOwner {
		return domain.Member{}, domain.ErrForbidden
	}
	return s.store.AddMember(wsID, memberSub, role, s.now().UTC())
}

func (s *Service) CreateBoard(sub, wsID, name string) (domain.BoardDetail, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.BoardDetail{}, err
	}
	if name == "" {
		name = "Main board"
	}
	board, cols, err := s.store.CreateBoard(wsID, name, s.now().UTC())
	if err != nil {
		return domain.BoardDetail{}, err
	}
	detail := domain.BoardDetail{Board: board}
	for _, col := range cols {
		detail.Columns = append(detail.Columns, domain.ColumnWithCards{Column: col, Cards: []domain.Card{}})
	}
	return detail, nil
}

func (s *Service) ListBoards(sub, wsID string) ([]domain.Board, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	return s.store.ListBoards(wsID)
}

func (s *Service) GetBoard(sub, boardID string) (domain.BoardDetail, error) {
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return domain.BoardDetail{}, err
	}
	if err := s.requireRead(wsID, sub); err != nil {
		return domain.BoardDetail{}, err
	}
	return s.store.GetBoardDetail(boardID)
}

func (s *Service) CreateCard(sub, columnID, title, description string) (domain.Card, error) {
	boardID, err := s.store.ColumnBoardID(columnID)
	if err != nil {
		return domain.Card{}, err
	}
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return domain.Card{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Card{}, err
	}
	if title == "" {
		return domain.Card{}, domain.ErrForbidden
	}
	return s.store.CreateCard(columnID, title, description, s.now().UTC())
}

func (s *Service) GetCard(sub, cardID string) (domain.Card, error) {
	wsID, err := s.store.CardWorkspaceID(cardID)
	if err != nil {
		return domain.Card{}, err
	}
	if err := s.requireRead(wsID, sub); err != nil {
		return domain.Card{}, err
	}
	return s.store.GetCard(cardID)
}

func (s *Service) UpdateCard(sub, cardID, title, description string, version int) (domain.Card, error) {
	wsID, err := s.store.CardWorkspaceID(cardID)
	if err != nil {
		return domain.Card{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Card{}, err
	}
	return s.store.UpdateCard(cardID, title, description, version, s.now().UTC())
}

func (s *Service) MoveCard(sub, cardID, columnID string, position, version int) (domain.Card, error) {
	wsID, err := s.store.CardWorkspaceID(cardID)
	if err != nil {
		return domain.Card{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Card{}, err
	}
	return s.store.MoveCard(cardID, columnID, position, version, s.now().UTC())
}

func (s *Service) CreatePage(sub, wsID, parentID, title, body, status string) (domain.Page, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Page{}, err
	}
	if title == "" || len(title) > domain.MaxPageTitle {
		return domain.Page{}, domain.ErrInvalid
	}
	if len(body) > domain.MaxPageBody {
		return domain.Page{}, domain.ErrInvalid
	}
	if status == "" {
		status = domain.PageStatusDraft
	}
	if status != domain.PageStatusDraft && status != domain.PageStatusPublished {
		return domain.Page{}, domain.ErrInvalid
	}
	return s.store.CreatePage(wsID, parentID, title, body, status, s.now().UTC())
}

func (s *Service) PageTree(sub, wsID string) ([]domain.PageNode, error) {
	role, err := s.store.MemberRole(wsID, sub)
	if err != nil {
		return nil, err
	}
	pages, err := s.store.ListPages(wsID)
	if err != nil {
		return nil, err
	}
	if role == domain.RoleGuest {
		pages = domain.FilterGuestPages(pages)
	}
	return domain.BuildPageTree(pages), nil
}

func (s *Service) GetPage(sub, pageID string) (domain.Page, error) {
	wsID, err := s.store.PageWorkspaceID(pageID)
	if err != nil {
		return domain.Page{}, err
	}
	role, err := s.store.MemberRole(wsID, sub)
	if err != nil {
		return domain.Page{}, err
	}
	page, err := s.store.GetPage(pageID)
	if err != nil {
		return domain.Page{}, err
	}
	if role == domain.RoleGuest && !domain.PageVisibleToGuest(page) {
		return domain.Page{}, domain.ErrNotFound
	}
	if role == domain.RoleGuest {
		ancestors, err := s.store.ListPages(wsID)
		if err != nil {
			return domain.Page{}, err
		}
		visible := domain.FilterGuestPages(ancestors)
		ok := false
		for _, p := range visible {
			if p.ID == pageID {
				ok = true
				break
			}
		}
		if !ok {
			return domain.Page{}, domain.ErrNotFound
		}
	}
	return page, nil
}

func (s *Service) UpdatePage(sub, pageID string, title, body, status *string, parentID *string, version int) (domain.Page, error) {
	wsID, err := s.store.PageWorkspaceID(pageID)
	if err != nil {
		return domain.Page{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Page{}, err
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" || len(t) > domain.MaxPageTitle {
			return domain.Page{}, domain.ErrInvalid
		}
		title = &t
	}
	if body != nil && len(*body) > domain.MaxPageBody {
		return domain.Page{}, domain.ErrInvalid
	}
	if status != nil && *status != domain.PageStatusDraft && *status != domain.PageStatusPublished {
		return domain.Page{}, domain.ErrInvalid
	}
	return s.store.UpdatePage(pageID, title, body, status, parentID, version, s.now().UTC())
}

func (s *Service) CreateDocument(sub, wsID, title, body string) (domain.Document, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Document{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > domain.MaxPageTitle {
		return domain.Document{}, domain.ErrInvalid
	}
	if len(body) > domain.MaxPageBody {
		return domain.Document{}, domain.ErrInvalid
	}
	return s.store.CreateDocument(wsID, title, body, s.now().UTC())
}

func (s *Service) ListDocuments(sub, wsID string) ([]domain.Document, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	return s.store.ListDocuments(wsID)
}

func (s *Service) GetDocument(sub, docID string) (domain.Document, error) {
	doc, err := s.store.GetDocument(docID)
	if err != nil {
		return domain.Document{}, err
	}
	if err := s.requireRead(doc.WorkspaceID, sub); err != nil {
		return domain.Document{}, err
	}
	return doc, nil
}

func (s *Service) UpdateDocumentTitle(sub, docID, title string) (domain.Document, error) {
	doc, err := s.store.GetDocument(docID)
	if err != nil {
		return domain.Document{}, err
	}
	if err := s.requireWrite(doc.WorkspaceID, sub); err != nil {
		return domain.Document{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > domain.MaxPageTitle {
		return domain.Document{}, domain.ErrInvalid
	}
	return s.store.UpdateDocumentTitle(docID, title, s.now().UTC())
}

type IssuedTicket struct {
	Ticket           string    `json:"ticket"`
	CollabDocumentID string    `json:"collabDocumentId"`
	ReadOnly         bool      `json:"readOnly"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

func (s *Service) IssueCollabTicket(sub, collabDocumentID string) (IssuedTicket, error) {
	if !domain.ValidCollabRoom(collabDocumentID) {
		return IssuedTicket{}, domain.ErrInvalid
	}
	kind, targetID, err := s.store.LookupCollab(collabDocumentID)
	if err != nil {
		return IssuedTicket{}, err
	}
	readOnly := false
	switch kind {
	case "page":
		page, err := s.GetPage(sub, targetID)
		if err != nil {
			return IssuedTicket{}, err
		}
		role, err := s.store.MemberRole(page.WorkspaceID, sub)
		if err != nil {
			return IssuedTicket{}, err
		}
		readOnly = role == domain.RoleGuest
	case "document":
		doc, err := s.GetDocument(sub, targetID)
		if err != nil {
			return IssuedTicket{}, err
		}
		role, err := s.store.MemberRole(doc.WorkspaceID, sub)
		if err != nil {
			return IssuedTicket{}, err
		}
		readOnly = !domain.RoleAtLeast(role, domain.RoleMember)
	default:
		return IssuedTicket{}, domain.ErrNotFound
	}
	t := s.store.CreateTicket(sub, collabDocumentID, readOnly, s.now().UTC())
	return IssuedTicket{
		Ticket:           t.ID,
		CollabDocumentID: t.CollabDocumentID,
		ReadOnly:         t.ReadOnly,
		ExpiresAt:        t.ExpiresAt,
	}, nil
}

func (s *Service) AuthorizeCollab(ticketID, documentName string) (domain.CollabAuth, error) {
	if !domain.ValidCollabRoom(documentName) {
		return domain.CollabAuth{}, domain.ErrInvalid
	}
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return domain.CollabAuth{}, domain.ErrUnauthorized
	}
	if !s.now().UTC().Before(t.ExpiresAt) {
		return domain.CollabAuth{}, domain.ErrUnauthorized
	}
	if t.CollabDocumentID != documentName {
		return domain.CollabAuth{}, domain.ErrForbidden
	}
	return domain.CollabAuth{
		Sub:              t.Sub,
		CollabDocumentID: t.CollabDocumentID,
		ReadOnly:         t.ReadOnly,
	}, nil
}

func (s *Service) CollabPlaintext(collabDocumentID string) (string, error) {
	if !domain.ValidCollabRoom(collabDocumentID) {
		return "", domain.ErrInvalid
	}
	kind, targetID, err := s.store.LookupCollab(collabDocumentID)
	if err != nil {
		return "", err
	}
	switch kind {
	case "page":
		p, err := s.store.GetPage(targetID)
		if err != nil {
			return "", err
		}
		return p.Body, nil
	case "document":
		d, err := s.store.GetDocument(targetID)
		if err != nil {
			return "", err
		}
		return d.Body, nil
	default:
		return "", domain.ErrNotFound
	}
}

func (s *Service) ApplyCollabSnapshot(collabDocumentID, plaintext string) error {
	if !domain.ValidCollabRoom(collabDocumentID) {
		return domain.ErrInvalid
	}
	if len(plaintext) > domain.MaxPageBody {
		return domain.ErrInvalid
	}
	return s.store.ApplyCollabSnapshot(collabDocumentID, plaintext, s.now().UTC())
}

func (s *Service) CreateChannel(sub, wsID, name string) (domain.Channel, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Channel{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > domain.MaxChannelName {
		return domain.Channel{}, domain.ErrInvalid
	}
	return s.store.CreateChannel(wsID, name, s.now().UTC())
}

func (s *Service) ListChannels(sub, wsID string) ([]domain.Channel, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	return s.store.ListChannels(wsID)
}

func (s *Service) GetChannel(sub, channelID string) (domain.Channel, error) {
	ch, err := s.store.GetChannel(channelID)
	if err != nil {
		return domain.Channel{}, err
	}
	if err := s.requireRead(ch.WorkspaceID, sub); err != nil {
		return domain.Channel{}, err
	}
	return ch, nil
}

func (s *Service) PostMessage(sub, channelID, body string) (domain.ChatMessage, error) {
	ch, err := s.store.GetChannel(channelID)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	if err := s.requireWrite(ch.WorkspaceID, sub); err != nil {
		return domain.ChatMessage{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > domain.MaxChatMessage {
		return domain.ChatMessage{}, domain.ErrInvalid
	}
	msg, err := s.store.AppendMessage(channelID, sub, body, s.now().UTC())
	if err != nil {
		return domain.ChatMessage{}, err
	}
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]any{"type": "message", "message": msg})
		s.bus.Broadcast(channelID, payload)
	}
	return msg, nil
}

func (s *Service) ListMessages(sub, channelID string, afterSeq int) ([]domain.ChatMessage, error) {
	ch, err := s.store.GetChannel(channelID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRead(ch.WorkspaceID, sub); err != nil {
		return nil, err
	}
	return s.store.ListMessages(channelID, afterSeq)
}

type IssuedChatTicket struct {
	Ticket    string    `json:"ticket"`
	ChannelID string    `json:"channelId"`
	ReadOnly  bool      `json:"readOnly"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Service) IssueChatTicket(sub, channelID string) (IssuedChatTicket, error) {
	ch, err := s.GetChannel(sub, channelID)
	if err != nil {
		return IssuedChatTicket{}, err
	}
	role, err := s.store.MemberRole(ch.WorkspaceID, sub)
	if err != nil {
		return IssuedChatTicket{}, err
	}
	readOnly := !domain.RoleAtLeast(role, domain.RoleMember)
	t := s.store.CreateChatTicket(sub, channelID, readOnly, s.now().UTC())
	return IssuedChatTicket{
		Ticket:    t.ID,
		ChannelID: t.ChannelID,
		ReadOnly:  t.ReadOnly,
		ExpiresAt: t.ExpiresAt,
	}, nil
}

func (s *Service) AuthorizeChat(ticketID, channelID string) (domain.ChatTicket, error) {
	t, err := s.store.GetChatTicket(ticketID)
	if err != nil {
		return domain.ChatTicket{}, domain.ErrUnauthorized
	}
	if !s.now().UTC().Before(t.ExpiresAt) {
		return domain.ChatTicket{}, domain.ErrUnauthorized
	}
	if channelID != "" && t.ChannelID != channelID {
		return domain.ChatTicket{}, domain.ErrForbidden
	}
	return t, nil
}

func (s *Service) BroadcastTyping(channelID, sub string) {
	if s.bus == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": "typing", "sub": sub, "channelId": channelID})
	s.bus.Broadcast(channelID, payload)
}
