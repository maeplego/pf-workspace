package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/id"
	"github.com/portfolio/pf-workspace/api/internal/store"
)

type Broadcaster interface {
	Broadcast(channelID string, payload []byte)
}

type FileOpts struct {
	PublicURL   string
	MediaAPIURL string
	UploadDir   string
}

type Service struct {
	store     store.Store
	now       func() time.Time
	bus       Broadcaster
	publicURL string
	mediaURL  string
	uploadDir string
}

func New(st store.Store) *Service {
	s := &Service{store: st, now: time.Now}
	s.SetFileOpts(FileOpts{})
	return s
}

func (s *Service) Ping() error {
	return s.store.Ping()
}

func (s *Service) SetFileOpts(o FileOpts) {
	if strings.TrimSpace(o.PublicURL) == "" {
		o.PublicURL = "http://localhost:8096"
	}
	if strings.TrimSpace(o.UploadDir) == "" {
		o.UploadDir = filepath.Join(os.TempDir(), "pf-workspace-uploads")
	}
	s.publicURL = strings.TrimRight(o.PublicURL, "/")
	s.mediaURL = strings.TrimRight(strings.TrimSpace(o.MediaAPIURL), "/")
	s.uploadDir = o.UploadDir
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

func (s *Service) CreateWorkspace(sub, name, orgID string) (domain.Workspace, error) {
	if name == "" {
		return domain.Workspace{}, domain.ErrForbidden
	}
	ws, err := s.store.CreateWorkspace(name, sub, strings.TrimSpace(orgID), s.now().UTC())
	if err != nil {
		return domain.Workspace{}, err
	}
	_, _ = s.store.CreateChannel(ws.ID, domain.DefaultChannelName, s.now().UTC())
	return ws, nil
}

func (s *Service) ListWorkspaces(sub string) []domain.Workspace {
	list := s.store.ListWorkspacesForSub(sub)
	if list == nil {
		return []domain.Workspace{}
	}
	return list
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

func (s *Service) GetMember(sub, wsID, memberSub string) (domain.Member, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return domain.Member{}, err
	}
	return s.store.GetMember(wsID, memberSub)
}

func (s *Service) SyncMemberDisplayName(sub, wsID, displayName string) (domain.Member, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return domain.Member{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return domain.Member{}, domain.ErrInvalid
	}
	return s.store.UpdateMemberDisplayName(wsID, sub, displayName)
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

func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) CreateInvitation(actorSub, wsID string, role domain.Role, invitedEmail string, maxUses int, ttlHours int) (domain.Invitation, string, error) {
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return domain.Invitation{}, "", err
	}
	if role == "" {
		role = domain.RoleMember
	}
	if role != domain.RoleMember && role != domain.RoleGuest {
		return domain.Invitation{}, "", domain.ErrInvalid
	}
	if maxUses <= 0 {
		maxUses = 1
	}
	if maxUses > domain.MaxInviteUses {
		return domain.Invitation{}, "", domain.ErrInvalid
	}
	if ttlHours <= 0 {
		ttlHours = 72
	}
	if ttlHours > 24*14 {
		return domain.Invitation{}, "", domain.ErrInvalid
	}
	rawToken, err := newInviteToken()
	if err != nil {
		return domain.Invitation{}, "", err
	}
	now := s.now().UTC()
	expiresAt := now.Add(time.Duration(ttlHours) * time.Hour)
	invitedEmail = normalizeEmail(invitedEmail)
	inv, err := s.store.CreateInvitation(wsID, hashInviteToken(rawToken), role, invitedEmail, maxUses, expiresAt, actorSub, now)
	if err != nil {
		return domain.Invitation{}, "", err
	}
	_ = s.store.AddAuditEvent(domain.AuditEvent{
		ID: id.New(), WorkspaceID: wsID, ActorSub: actorSub, Type: "workspace.invitation.created", InviteID: inv.ID, CreatedAt: now,
	})
	return inv, rawToken, nil
}

func (s *Service) ListInvitations(actorSub, wsID string) ([]domain.Invitation, error) {
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return nil, err
	}
	return s.store.ListInvitations(wsID)
}

func (s *Service) RevokeInvitation(actorSub, wsID, inviteID string) (domain.Invitation, error) {
	if strings.TrimSpace(inviteID) == "" {
		return domain.Invitation{}, domain.ErrInvalid
	}
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return domain.Invitation{}, err
	}
	now := s.now().UTC()
	inv, revoked, err := s.store.RevokeInvitation(wsID, inviteID, now)
	if err != nil {
		return domain.Invitation{}, err
	}
	if revoked {
		_ = s.store.AddAuditEvent(domain.AuditEvent{
			ID: id.New(), WorkspaceID: wsID, ActorSub: actorSub, Type: "workspace.invitation.revoked", InviteID: inv.ID, CreatedAt: now,
		})
	}
	return inv, nil
}

func (s *Service) ResendInvitation(actorSub, wsID, inviteID string) (domain.Invitation, string, error) {
	if strings.TrimSpace(inviteID) == "" {
		return domain.Invitation{}, "", domain.ErrInvalid
	}
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return domain.Invitation{}, "", err
	}
	invitations, err := s.store.ListInvitations(wsID)
	if err != nil {
		return domain.Invitation{}, "", err
	}
	var base domain.Invitation
	found := false
	for _, inv := range invitations {
		if inv.ID == inviteID {
			base = inv
			found = true
			break
		}
	}
	if !found {
		return domain.Invitation{}, "", domain.ErrNotFound
	}
	ttlHours := int(base.ExpiresAt.Sub(base.CreatedAt).Hours())
	if ttlHours <= 0 {
		ttlHours = 72
	}
	if ttlHours > 24*14 {
		ttlHours = 24 * 14
	}
	inv, token, err := s.CreateInvitation(actorSub, wsID, base.Role, base.InvitedEmail, base.MaxUses, ttlHours)
	if err != nil {
		return domain.Invitation{}, "", err
	}
	now := s.now().UTC()
	_ = s.store.AddAuditEvent(domain.AuditEvent{
		ID: id.New(), WorkspaceID: wsID, ActorSub: actorSub, Type: "workspace.invitation.resent", InviteID: inv.ID, CreatedAt: now,
	})
	return inv, token, nil
}

func (s *Service) PreviewInvitation(sub, rawToken string) (domain.Invitation, domain.Workspace, error) {
	if strings.TrimSpace(sub) == "" || strings.TrimSpace(rawToken) == "" {
		return domain.Invitation{}, domain.Workspace{}, domain.ErrInvalid
	}
	inv, err := s.store.GetInvitationByTokenHash(hashInviteToken(rawToken))
	if err != nil {
		return domain.Invitation{}, domain.Workspace{}, domain.ErrNotFound
	}
	now := s.now().UTC()
	if inv.RevokedAt != nil || !now.Before(inv.ExpiresAt) || inv.UseCount >= inv.MaxUses {
		return domain.Invitation{}, domain.Workspace{}, domain.ErrNotFound
	}
	ws, err := s.store.GetWorkspace(inv.WorkspaceID)
	if err != nil {
		return domain.Invitation{}, domain.Workspace{}, domain.ErrNotFound
	}
	return inv, ws, nil
}

func (s *Service) AcceptInvitation(sub, email, rawToken string) (domain.Member, domain.Workspace, error) {
	if strings.TrimSpace(sub) == "" || strings.TrimSpace(rawToken) == "" {
		return domain.Member{}, domain.Workspace{}, domain.ErrInvalid
	}
	inv, err := s.store.GetInvitationByTokenHash(hashInviteToken(rawToken))
	if err != nil {
		return domain.Member{}, domain.Workspace{}, domain.ErrNotFound
	}
	if inv.InvitedEmail != "" && normalizeEmail(email) != inv.InvitedEmail {
		return domain.Member{}, domain.Workspace{}, domain.ErrForbidden
	}
	now := s.now().UTC()
	updated, member, joined, err := s.store.AcceptInvitation(inv.ID, sub, now)
	if err != nil {
		if err == domain.ErrForbidden {
			return domain.Member{}, domain.Workspace{}, domain.ErrNotFound
		}
		return domain.Member{}, domain.Workspace{}, err
	}
	if joined {
		_ = s.store.AddAuditEvent(domain.AuditEvent{
			ID: id.New(), WorkspaceID: updated.WorkspaceID, ActorSub: sub, TargetSub: sub, Type: "workspace.invitation.accepted", InviteID: updated.ID, CreatedAt: now,
		})
	}
	ws, err := s.store.GetWorkspace(updated.WorkspaceID)
	if err != nil {
		return domain.Member{}, domain.Workspace{}, err
	}
	return member, ws, nil
}

func (s *Service) ListAuditEvents(actorSub, wsID string) ([]domain.AuditEvent, error) {
	if err := s.requireRole(wsID, actorSub, domain.RoleOwner); err != nil {
		return nil, err
	}
	return s.store.ListAuditEvents(wsID)
}

func (s *Service) CreateBoard(sub, wsID, name string) (domain.BoardDetail, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.BoardDetail{}, err
	}
	if name == "" {
		return domain.BoardDetail{}, domain.ErrInvalid
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

func (s *Service) ArchiveBoard(sub, boardID string) error {
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return err
	}
	return s.store.ArchiveBoard(boardID, s.now().UTC())
}

func (s *Service) UnarchiveBoard(sub, boardID string) error {
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return err
	}
	return s.store.UnarchiveBoard(boardID)
}

func (s *Service) ListBoards(sub, wsID string) ([]domain.Board, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	list, err := s.store.ListBoards(wsID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []domain.Board{}, nil
	}
	return list, nil
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

func (s *Service) UpdateCard(sub, cardID, title, description string, sprintID *string, version int) (domain.Card, error) {
	wsID, err := s.store.CardWorkspaceID(cardID)
	if err != nil {
		return domain.Card{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Card{}, err
	}
	return s.store.UpdateCard(cardID, title, description, sprintID, version, s.now().UTC())
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
	page, err := s.store.CreatePage(wsID, parentID, title, body, status, s.now().UTC())
	if err != nil {
		return domain.Page{}, err
	}
	_, _, _ = s.store.AppendPageVersionIfChanged(page.ID, page.Title, page.Body, page.Status, sub, s.now().UTC())
	return page, nil
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
	var live []domain.Page
	for _, p := range pages {
		if p.ArchivedAt == nil {
			live = append(live, p)
		}
	}
	return domain.BuildPageTree(live), nil
}

func (s *Service) ArchivedPages(sub, wsID string) ([]domain.Page, error) {
	role, err := s.store.MemberRole(wsID, sub)
	if err != nil {
		return nil, err
	}
	if role == domain.RoleGuest {
		return []domain.Page{}, nil
	}
	pages, err := s.store.ListPages(wsID)
	if err != nil {
		return nil, err
	}
	var out []domain.Page
	for _, p := range pages {
		if p.ArchivedAt != nil {
			out = append(out, p)
		}
	}
	if out == nil {
		out = []domain.Page{}
	}
	return out, nil
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
	if page.ArchivedAt != nil && role == domain.RoleGuest {
		return domain.Page{}, domain.ErrNotFound
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
	page, err := s.store.UpdatePage(pageID, title, body, status, parentID, version, s.now().UTC())
	if err != nil {
		return page, err
	}
	if title != nil || body != nil || status != nil {
		_, _, _ = s.store.AppendPageVersionIfChanged(page.ID, page.Title, page.Body, page.Status, sub, s.now().UTC())
	}
	return page, nil
}

func (s *Service) ArchivePage(sub, pageID string) error {
	wsID, err := s.store.PageWorkspaceID(pageID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return err
	}
	return s.store.ArchivePage(pageID, s.now().UTC())
}

func (s *Service) UnarchivePage(sub, pageID string) error {
	wsID, err := s.store.PageWorkspaceID(pageID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return err
	}
	return s.store.UnarchivePage(pageID)
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
	list, err := s.store.ListDocuments(wsID)
	if err != nil {
		return nil, err
	}
	var live []domain.Document
	for _, d := range list {
		if d.DeletedAt == nil {
			live = append(live, d)
		}
	}
	if live == nil {
		live = []domain.Document{}
	}
	return live, nil
}

func (s *Service) ListTrashedDocuments(sub, wsID string) ([]domain.Document, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		if err == domain.ErrForbidden {
			return []domain.Document{}, nil
		}
		return nil, err
	}
	list, err := s.store.ListDocuments(wsID)
	if err != nil {
		return nil, err
	}
	var trash []domain.Document
	for _, d := range list {
		if d.DeletedAt != nil {
			trash = append(trash, d)
		}
	}
	if trash == nil {
		trash = []domain.Document{}
	}
	return trash, nil
}

func (s *Service) GetDocument(sub, docID string) (domain.Document, error) {
	doc, err := s.store.GetDocument(docID)
	if err != nil {
		return domain.Document{}, err
	}
	if err := s.requireRead(doc.WorkspaceID, sub); err != nil {
		return domain.Document{}, err
	}
	if doc.DeletedAt != nil {
		role, err := s.store.MemberRole(doc.WorkspaceID, sub)
		if err != nil {
			return domain.Document{}, err
		}
		if role == domain.RoleGuest {
			return domain.Document{}, domain.ErrNotFound
		}
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

func (s *Service) TrashDocument(sub, docID string) error {
	doc, err := s.store.GetDocument(docID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(doc.WorkspaceID, sub); err != nil {
		return err
	}
	return s.store.TrashDocument(docID, s.now().UTC())
}

func (s *Service) RestoreDocument(sub, docID string) error {
	doc, err := s.store.GetDocument(docID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(doc.WorkspaceID, sub); err != nil {
		return err
	}
	return s.store.RestoreDocument(docID)
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

func (s *Service) ApplyCollabSnapshot(collabDocumentID, plaintext, editorSub string) error {
	if !domain.ValidCollabRoom(collabDocumentID) {
		return domain.ErrInvalid
	}
	if len(plaintext) > domain.MaxPageBody {
		return domain.ErrInvalid
	}
	kind, targetID, err := s.store.LookupCollab(collabDocumentID)
	if err != nil {
		return err
	}
	if err := s.store.ApplyCollabSnapshot(collabDocumentID, plaintext, editorSub, s.now().UTC()); err != nil {
		return err
	}
	if kind == "page" {
		p, err := s.store.GetPage(targetID)
		if err != nil {
			return err
		}
		_, _, _ = s.store.AppendPageVersionIfChanged(p.ID, p.Title, p.Body, p.Status, editorSub, s.now().UTC())
	}
	return nil
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

func (s *Service) PostMessage(sub, channelID, body, attachmentFileID string) (domain.ChatMessage, error) {
	ch, err := s.store.GetChannel(channelID)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	if err := s.requireWrite(ch.WorkspaceID, sub); err != nil {
		return domain.ChatMessage{}, err
	}
	body = strings.TrimSpace(body)
	attachmentFileID = strings.TrimSpace(attachmentFileID)
	if body == "" && attachmentFileID == "" {
		return domain.ChatMessage{}, domain.ErrInvalid
	}
	if len(body) > domain.MaxChatMessage {
		return domain.ChatMessage{}, domain.ErrInvalid
	}
	if attachmentFileID != "" {
		if err := s.requireChatFile(ch.WorkspaceID, attachmentFileID); err != nil {
			return domain.ChatMessage{}, err
		}
	}
	members, err := s.store.ListMembers(ch.WorkspaceID)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	subs := make([]string, 0, len(members))
	for _, m := range members {
		subs = append(subs, m.Sub)
	}
	mentions := domain.ExtractMentions(body, subs)
	msg, err := s.store.AppendMessage(channelID, sub, body, mentions, attachmentFileID, s.now().UTC())
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
