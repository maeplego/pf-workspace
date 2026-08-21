package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/id"
	"github.com/portfolio/pf-workspace/api/internal/store"
)

var _ store.Store = (*Store)(nil)

type Store struct {
	mu           sync.RWMutex
	orgFilter    string
	skipTenant   bool
	workspaces   map[string]domain.Workspace
	members      map[string]map[string]domain.Member // workspaceID -> sub -> member
	invitations  map[string]domain.Invitation
	audits       map[string][]domain.AuditEvent
	boards       map[string]domain.Board
	columns      map[string]domain.Column
	cards        map[string]domain.Card
	boardCols    map[string][]string // boardID -> column IDs ordered
	columnCards  map[string][]string // columnID -> card IDs ordered
	pages        map[string]domain.Page
	documents    map[string]domain.Document
	tickets      map[string]domain.CollabTicket
	collabIndex  map[string]collabRef
	channels     map[string]domain.Channel
	messages     map[string][]domain.ChatMessage // channelID -> ordered
	chatReads    map[string]map[string]int       // channelID -> sub -> lastReadSeq
	chatTickets  map[string]domain.ChatTicket
	files        map[string]domain.StoredFile
	pageFiles    map[string][]string // pageID -> fileIDs
	sprints      map[string]domain.Sprint
	pageVersions map[string][]domain.PageVersion // pageID -> ordered
}

type collabRef struct {
	kind string // "page" | "document"
	id   string
}

func (s *Store) Ping() error { return nil }

// WithTenant scopes reads/writes to workspaces whose org_id matches (mirrors Postgres RLS).
func (s *Store) WithTenant(tenantID string) store.Store {
	cp := *s
	cp.orgFilter = tenantID
	cp.skipTenant = false
	return &cp
}

// Unscoped disables org filtering (invite token lookup, internal collab).
func (s *Store) Unscoped() store.Store {
	cp := *s
	cp.skipTenant = true
	cp.orgFilter = ""
	return &cp
}

func (s *Store) allowOrg(orgID string) bool {
	if s.skipTenant || s.orgFilter == "" {
		return true
	}
	return orgID == s.orgFilter
}

func New() *Store {
	return &Store{
		workspaces:   make(map[string]domain.Workspace),
		members:      make(map[string]map[string]domain.Member),
		invitations:  make(map[string]domain.Invitation),
		audits:       make(map[string][]domain.AuditEvent),
		boards:       make(map[string]domain.Board),
		columns:      make(map[string]domain.Column),
		cards:        make(map[string]domain.Card),
		boardCols:    make(map[string][]string),
		columnCards:  make(map[string][]string),
		pages:        make(map[string]domain.Page),
		documents:    make(map[string]domain.Document),
		tickets:      make(map[string]domain.CollabTicket),
		collabIndex:  make(map[string]collabRef),
		channels:     make(map[string]domain.Channel),
		messages:     make(map[string][]domain.ChatMessage),
		chatReads:    make(map[string]map[string]int),
		chatTickets:  make(map[string]domain.ChatTicket),
		files:        make(map[string]domain.StoredFile),
		pageFiles:    make(map[string][]string),
		sprints:      make(map[string]domain.Sprint),
		pageVersions: make(map[string][]domain.PageVersion),
	}
}

func (s *Store) CreateWorkspace(name, ownerSub, orgID string, now time.Time) (domain.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := domain.Workspace{
		ID:        id.New(),
		Name:      name,
		OrgID:     orgID,
		CreatedAt: now,
	}
	s.workspaces[ws.ID] = ws
	if s.members[ws.ID] == nil {
		s.members[ws.ID] = make(map[string]domain.Member)
	}
	s.members[ws.ID][ownerSub] = domain.Member{
		WorkspaceID: ws.ID,
		Sub:         ownerSub,
		DisplayName: domain.DevDisplayName(ownerSub),
		Role:        domain.RoleOwner,
		JoinedAt:    now,
	}
	return ws, nil
}

func (s *Store) ListWorkspacesForSub(sub string) []domain.Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Workspace
	for wsID, members := range s.members {
		if _, ok := members[sub]; ok {
			if ws, ok := s.workspaces[wsID]; ok && s.allowOrg(ws.OrgID) {
				out = append(out, ws)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Store) GetWorkspace(wsID string) (domain.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.workspaces[wsID]
	if !ok || !s.allowOrg(ws.OrgID) {
		return domain.Workspace{}, domain.ErrNotFound
	}
	return ws, nil
}

func (s *Store) MemberRole(wsID, sub string) (domain.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.members[wsID]
	if !ok {
		return "", domain.ErrNotFound
	}
	m, ok := members[sub]
	if !ok {
		return "", domain.ErrForbidden
	}
	return m.Role, nil
}

func (s *Store) ListMembers(wsID string) ([]domain.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.members[wsID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := make([]domain.Member, 0, len(members))
	for _, m := range members {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

func (s *Store) GetMember(wsID, sub string) (domain.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.members[wsID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m, ok := members[sub]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	return m, nil
}

func (s *Store) AddMember(wsID, sub string, role domain.Role, now time.Time) (domain.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	members, ok := s.members[wsID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	if _, exists := members[sub]; exists {
		return domain.Member{}, domain.ErrConflict
	}
	m := domain.Member{
		WorkspaceID: wsID,
		Sub:         sub,
		DisplayName: domain.DevDisplayName(sub),
		Role:        role,
		JoinedAt:    now,
	}
	members[sub] = m
	return m, nil
}

func (s *Store) UpdateMemberRole(wsID, sub string, role domain.Role) (domain.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	members, ok := s.members[wsID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m, ok := members[sub]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m.Role = role
	members[sub] = m
	return m, nil
}

func (s *Store) RemoveMember(wsID, sub string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	members, ok := s.members[wsID]
	if !ok {
		return domain.ErrNotFound
	}
	if _, ok := members[sub]; !ok {
		return domain.ErrNotFound
	}
	delete(members, sub)
	return nil
}

func (s *Store) CreateInvitation(wsID, tokenHash string, role domain.Role, invitedEmail string, maxUses int, expiresAt time.Time, invitedBy string, now time.Time) (domain.Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return domain.Invitation{}, domain.ErrNotFound
	}
	inv := domain.Invitation{
		ID:           id.New(),
		WorkspaceID:  wsID,
		TokenHash:    tokenHash,
		Role:         role,
		InvitedEmail: invitedEmail,
		MaxUses:      maxUses,
		UseCount:     0,
		ExpiresAt:    expiresAt,
		InvitedBy:    invitedBy,
		CreatedAt:    now,
	}
	s.invitations[inv.ID] = inv
	return inv, nil
}

func (s *Store) ListInvitations(wsID string) ([]domain.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	out := make([]domain.Invitation, 0)
	for _, inv := range s.invitations {
		if inv.WorkspaceID == wsID {
			out = append(out, inv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) GetInvitationByTokenHash(tokenHash string) (domain.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inv := range s.invitations {
		if inv.TokenHash == tokenHash {
			return inv, nil
		}
	}
	return domain.Invitation{}, domain.ErrNotFound
}

func (s *Store) UpdateInvitationPolicy(wsID, inviteID string, role domain.Role, invitedEmail string, maxUses int, expiresAt time.Time) (domain.Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invitations[inviteID]
	if !ok || inv.WorkspaceID != wsID {
		return domain.Invitation{}, domain.ErrNotFound
	}
	if inv.RevokedAt != nil {
		return domain.Invitation{}, domain.ErrForbidden
	}
	inv.Role = role
	inv.InvitedEmail = invitedEmail
	inv.MaxUses = maxUses
	inv.ExpiresAt = expiresAt
	s.invitations[inviteID] = inv
	return inv, nil
}

func (s *Store) RevokeInvitation(wsID, inviteID string, now time.Time) (domain.Invitation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invitations[inviteID]
	if !ok || inv.WorkspaceID != wsID {
		return domain.Invitation{}, false, domain.ErrNotFound
	}
	if inv.RevokedAt != nil {
		return inv, false, nil
	}
	inv.RevokedAt = &now
	s.invitations[inviteID] = inv
	return inv, true, nil
}

func (s *Store) AcceptInvitation(inviteID, sub string, now time.Time) (domain.Invitation, domain.Member, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invitations[inviteID]
	if !ok {
		return domain.Invitation{}, domain.Member{}, false, domain.ErrNotFound
	}
	if inv.RevokedAt != nil || !now.Before(inv.ExpiresAt) || inv.UseCount >= inv.MaxUses {
		return domain.Invitation{}, domain.Member{}, false, domain.ErrForbidden
	}
	members, ok := s.members[inv.WorkspaceID]
	if !ok {
		return domain.Invitation{}, domain.Member{}, false, domain.ErrNotFound
	}
	if existing, exists := members[sub]; exists {
		return inv, existing, false, nil
	}
	member := domain.Member{
		WorkspaceID: inv.WorkspaceID,
		Sub:         sub,
		DisplayName: domain.DevDisplayName(sub),
		Role:        inv.Role,
		JoinedAt:    now,
	}
	members[sub] = member
	inv.UseCount++
	s.invitations[inv.ID] = inv
	return inv, member, true, nil
}

func (s *Store) AddAuditEvent(event domain.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits[event.WorkspaceID] = append(s.audits[event.WorkspaceID], event)
	return nil
}

func (s *Store) ListAuditEvents(wsID string) ([]domain.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]domain.AuditEvent(nil), s.audits[wsID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) UpdateMemberDisplayName(wsID, sub, displayName string) (domain.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return domain.Member{}, domain.ErrInvalid
	}
	members, ok := s.members[wsID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m, ok := members[sub]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m.DisplayName = displayName
	members[sub] = m
	return m, nil
}

func (s *Store) CreateBoard(wsID, name string, now time.Time) (domain.Board, []domain.Column, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return domain.Board{}, nil, domain.ErrNotFound
	}
	board := domain.Board{
		ID:          id.New(),
		WorkspaceID: wsID,
		Name:        name,
		CreatedAt:   now,
	}
	s.boards[board.ID] = board
	defaultNames := []string{"To Do", "In Progress", "Done"}
	cols := make([]domain.Column, 0, len(defaultNames))
	colIDs := make([]string, 0, len(defaultNames))
	for i, n := range defaultNames {
		col := domain.Column{
			ID:        id.New(),
			BoardID:   board.ID,
			Name:      n,
			Position:  i,
			CreatedAt: now,
		}
		s.columns[col.ID] = col
		colIDs = append(colIDs, col.ID)
		s.columnCards[col.ID] = []string{}
		cols = append(cols, col)
	}
	s.boardCols[board.ID] = colIDs
	return board, cols, nil
}

func (s *Store) ArchiveBoard(boardID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.boards[boardID]
	if !ok {
		return domain.ErrNotFound
	}
	t := now
	b.ArchivedAt = &t
	s.boards[boardID] = b
	return nil
}

func (s *Store) UnarchiveBoard(boardID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.boards[boardID]
	if !ok {
		return domain.ErrNotFound
	}
	b.ArchivedAt = nil
	s.boards[boardID] = b
	return nil
}

func (s *Store) ListBoards(wsID string) ([]domain.Board, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Board
	for _, b := range s.boards {
		if b.WorkspaceID == wsID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) GetBoardDetail(boardID string) (domain.BoardDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	board, ok := s.boards[boardID]
	if !ok {
		return domain.BoardDetail{}, domain.ErrNotFound
	}
	colIDs := s.boardCols[boardID]
	cols := make([]domain.ColumnWithCards, 0, len(colIDs))
	for _, colID := range colIDs {
		col := s.columns[colID]
		cardIDs := s.columnCards[colID]
		cards := make([]domain.Card, 0, len(cardIDs))
		for _, cardID := range cardIDs {
			cards = append(cards, s.cards[cardID])
		}
		cols = append(cols, domain.ColumnWithCards{Column: col, Cards: cards})
	}
	return domain.BoardDetail{Board: board, Columns: cols}, nil
}

func (s *Store) BoardWorkspaceID(boardID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.boards[boardID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return b.WorkspaceID, nil
}

func (s *Store) ColumnBoardID(columnID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	col, ok := s.columns[columnID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return col.BoardID, nil
}

func (s *Store) CreateColumn(boardID, name string, now time.Time) (domain.Column, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.boards[boardID]; !ok {
		return domain.Column{}, domain.ErrNotFound
	}
	pos := len(s.boardCols[boardID])
	col := domain.Column{
		ID:        id.New(),
		BoardID:   boardID,
		Name:      name,
		Position:  pos,
		CreatedAt: now,
	}
	s.columns[col.ID] = col
	s.boardCols[boardID] = append(s.boardCols[boardID], col.ID)
	s.columnCards[col.ID] = []string{}
	return col, nil
}

func (s *Store) RenameColumn(columnID, name string) (domain.Column, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.columns[columnID]
	if !ok {
		return domain.Column{}, domain.ErrNotFound
	}
	col.Name = name
	s.columns[columnID] = col
	return col, nil
}

func (s *Store) DeleteColumn(columnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.columns[columnID]
	if !ok {
		return domain.ErrNotFound
	}
	if len(s.columnCards[columnID]) > 0 {
		return domain.ErrInvalid
	}
	ids := s.boardCols[col.BoardID]
	if len(ids) <= 1 {
		return domain.ErrForbidden
	}
	next := make([]string, 0, len(ids)-1)
	for _, id := range ids {
		if id != columnID {
			next = append(next, id)
		}
	}
	s.boardCols[col.BoardID] = next
	for i, id := range next {
		c := s.columns[id]
		c.Position = i
		s.columns[id] = c
	}
	delete(s.columns, columnID)
	delete(s.columnCards, columnID)
	return nil
}

func (s *Store) ReorderColumns(boardID string, columnIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.boards[boardID]; !ok {
		return domain.ErrNotFound
	}
	current := s.boardCols[boardID]
	if len(current) != len(columnIDs) {
		return domain.ErrInvalid
	}
	seen := make(map[string]bool, len(current))
	for _, id := range current {
		seen[id] = true
	}
	for _, id := range columnIDs {
		if !seen[id] {
			return domain.ErrInvalid
		}
		delete(seen, id)
	}
	if len(seen) != 0 {
		return domain.ErrInvalid
	}
	s.boardCols[boardID] = append([]string{}, columnIDs...)
	for i, id := range columnIDs {
		c := s.columns[id]
		c.Position = i
		s.columns[id] = c
	}
	return nil
}

func (s *Store) CreateCard(columnID, title, description string, now time.Time) (domain.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.columns[columnID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	board, ok := s.boards[col.BoardID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	pos := len(s.columnCards[columnID])
	card := domain.Card{
		ID:          id.New(),
		ColumnID:    columnID,
		BoardID:     board.ID,
		Title:       title,
		Description: description,
		Position:    pos,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if domain.ColumnIsDone(col.Name) {
		t := now
		card.CompletedAt = &t
	}
	s.cards[card.ID] = card
	s.columnCards[columnID] = append(s.columnCards[columnID], card.ID)
	return card, nil
}

func (s *Store) GetCard(cardID string) (domain.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cards[cardID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	return c, nil
}

func (s *Store) CardWorkspaceID(cardID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cards[cardID]
	if !ok {
		return "", domain.ErrNotFound
	}
	b, ok := s.boards[c.BoardID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return b.WorkspaceID, nil
}

func (s *Store) UpdateCard(cardID, title, description string, sprintID *string, version int, now time.Time) (domain.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cards[cardID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	if c.Version != version {
		return c, domain.ErrConflict
	}
	if title != "" {
		c.Title = title
	}
	c.Description = description
	if sprintID != nil {
		sid := *sprintID
		if sid == "" {
			c.SprintID = ""
		} else {
			sp, ok := s.sprints[sid]
			if !ok {
				return domain.Card{}, domain.ErrNotFound
			}
			if sp.BoardID != c.BoardID {
				return domain.Card{}, domain.ErrInvalid
			}
			c.SprintID = sid
		}
	}
	c.Version++
	c.UpdatedAt = now
	s.cards[cardID] = c
	return c, nil
}

func (s *Store) MoveCard(cardID, targetColumnID string, position int, version int, now time.Time) (domain.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cards[cardID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	if c.Version != version {
		return c, domain.ErrConflict
	}
	targetCol, ok := s.columns[targetColumnID]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	if targetCol.BoardID != c.BoardID {
		return domain.Card{}, domain.ErrForbidden
	}

	oldColumnID := c.ColumnID
	oldIDs := s.columnCards[oldColumnID]
	newOld := make([]string, 0, len(oldIDs)-1)
	for _, id := range oldIDs {
		if id != cardID {
			newOld = append(newOld, id)
		}
	}
	s.columnCards[oldColumnID] = newOld
	s.reindexColumnLocked(oldColumnID)

	targetIDs := append([]string(nil), s.columnCards[targetColumnID]...)
	if position < 0 {
		position = 0
	}
	if position > len(targetIDs) {
		position = len(targetIDs)
	}
	targetIDs = append(targetIDs[:position], append([]string{cardID}, targetIDs[position:]...)...)
	s.columnCards[targetColumnID] = targetIDs
	s.reindexColumnLocked(targetColumnID)

	c.ColumnID = targetColumnID
	c.Position = position
	if domain.ColumnIsDone(targetCol.Name) {
		if c.CompletedAt == nil {
			t := now
			c.CompletedAt = &t
		}
	} else {
		c.CompletedAt = nil
	}
	c.Version++
	c.UpdatedAt = now
	s.cards[cardID] = c
	return c, nil
}

func (s *Store) reindexColumnLocked(columnID string) {
	for i, cardID := range s.columnCards[columnID] {
		c := s.cards[cardID]
		c.Position = i
		s.cards[cardID] = c
	}
}

func (s *Store) CreatePage(wsID, parentID, title, body, status string, now time.Time) (domain.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return domain.Page{}, domain.ErrNotFound
	}
	if err := s.validateParentLocked("", wsID, parentID); err != nil {
		return domain.Page{}, err
	}
	pos := 0
	for _, p := range s.pages {
		if p.WorkspaceID == wsID && p.ParentID == parentID && p.Position >= pos {
			pos = p.Position + 1
		}
	}
	page := domain.Page{
		ID:               id.New(),
		WorkspaceID:      wsID,
		ParentID:         parentID,
		Title:            title,
		Body:             body,
		Status:           status,
		Position:         pos,
		Version:          1,
		CollabDocumentID: id.New(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.pages[page.ID] = page
	s.collabIndex[page.CollabDocumentID] = collabRef{kind: "page", id: page.ID}
	return page, nil
}

func (s *Store) ArchivePage(pageID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pages[pageID]; !ok {
		return domain.ErrNotFound
	}
	ids := append(s.descendantPageIDsLocked(pageID), pageID)
	t := now
	for _, id := range ids {
		p := s.pages[id]
		p.ArchivedAt = &t
		s.pages[id] = p
	}
	return nil
}

func (s *Store) UnarchivePage(pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pages[pageID]; !ok {
		return domain.ErrNotFound
	}
	ids := append(s.descendantPageIDsLocked(pageID), pageID)
	for _, id := range ids {
		p := s.pages[id]
		p.ArchivedAt = nil
		s.pages[id] = p
	}
	return nil
}

func (s *Store) descendantPageIDsLocked(parentID string) []string {
	var out []string
	for id, p := range s.pages {
		if p.ParentID == parentID {
			out = append(out, id)
			out = append(out, s.descendantPageIDsLocked(id)...)
		}
	}
	return out
}

func (s *Store) GetPage(pageID string) (domain.Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pages[pageID]
	if !ok {
		return domain.Page{}, domain.ErrNotFound
	}
	return p, nil
}

func (s *Store) ListPages(wsID string) ([]domain.Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Page
	for _, p := range s.pages {
		if p.WorkspaceID == wsID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Store) PageWorkspaceID(pageID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pages[pageID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return p.WorkspaceID, nil
}

func (s *Store) UpdatePage(pageID string, title, body, status, parentID *string, version int, now time.Time) (domain.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pages[pageID]
	if !ok {
		return domain.Page{}, domain.ErrNotFound
	}
	if p.Version != version {
		return p, domain.ErrConflict
	}
	if parentID != nil {
		if err := s.validateParentLocked(pageID, p.WorkspaceID, *parentID); err != nil {
			return domain.Page{}, err
		}
		p.ParentID = *parentID
	}
	if title != nil {
		p.Title = *title
	}
	if body != nil {
		p.Body = *body
	}
	if status != nil {
		p.Status = *status
	}
	p.Version++
	p.UpdatedAt = now
	s.pages[pageID] = p
	return p, nil
}

func (s *Store) validateParentLocked(pageID, wsID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == pageID {
		return domain.ErrInvalid
	}
	par, ok := s.pages[parentID]
	if !ok {
		return domain.ErrNotFound
	}
	if par.WorkspaceID != wsID {
		return domain.ErrInvalid
	}
	cur := par.ParentID
	seen := map[string]bool{parentID: true}
	for cur != "" {
		if cur == pageID {
			return domain.ErrInvalid
		}
		if seen[cur] {
			return domain.ErrInvalid
		}
		seen[cur] = true
		next, ok := s.pages[cur]
		if !ok {
			break
		}
		cur = next.ParentID
	}
	return nil
}

func (s *Store) CreateDocument(wsID, title, body string, now time.Time) (domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return domain.Document{}, domain.ErrNotFound
	}
	doc := domain.Document{
		ID:               id.New(),
		WorkspaceID:      wsID,
		Title:            title,
		Body:             body,
		CollabDocumentID: id.New(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.documents[doc.ID] = doc
	s.collabIndex[doc.CollabDocumentID] = collabRef{kind: "document", id: doc.ID}
	return doc, nil
}

func (s *Store) ListDocuments(wsID string) ([]domain.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Document
	for _, d := range s.documents {
		if d.WorkspaceID == wsID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetDocument(docID string) (domain.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.documents[docID]
	if !ok {
		return domain.Document{}, domain.ErrNotFound
	}
	return d, nil
}

func (s *Store) UpdateDocumentTitle(docID, title string, now time.Time) (domain.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.documents[docID]
	if !ok {
		return domain.Document{}, domain.ErrNotFound
	}
	d.Title = title
	d.UpdatedAt = now
	s.documents[docID] = d
	return d, nil
}

func (s *Store) TrashDocument(docID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.documents[docID]
	if !ok {
		return domain.ErrNotFound
	}
	t := now
	d.DeletedAt = &t
	s.documents[docID] = d
	return nil
}

func (s *Store) RestoreDocument(docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.documents[docID]
	if !ok {
		return domain.ErrNotFound
	}
	d.DeletedAt = nil
	s.documents[docID] = d
	return nil
}

func (s *Store) LookupCollab(collabDocumentID string) (kind, id string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ref, ok := s.collabIndex[collabDocumentID]
	if !ok {
		return "", "", domain.ErrNotFound
	}
	return ref.kind, ref.id, nil
}

func (s *Store) CreateTicket(sub, collabDocumentID string, readOnly bool, now time.Time) domain.CollabTicket {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := domain.CollabTicket{
		ID:               id.New(),
		Sub:              sub,
		CollabDocumentID: collabDocumentID,
		ReadOnly:         readOnly,
		ExpiresAt:        now.Add(domain.CollabTicketTTL),
	}
	s.tickets[t.ID] = t
	return t
}

func (s *Store) GetTicket(ticketID string) (domain.CollabTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tickets[ticketID]
	if !ok {
		return domain.CollabTicket{}, domain.ErrNotFound
	}
	return t, nil
}

func (s *Store) ApplyCollabSnapshot(collabDocumentID, plaintext, editorSub string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.collabIndex[collabDocumentID]
	if !ok {
		return domain.ErrNotFound
	}
	switch ref.kind {
	case "page":
		p, ok := s.pages[ref.id]
		if !ok {
			return domain.ErrNotFound
		}
		p.Body = plaintext
		p.UpdatedAt = now
		s.pages[ref.id] = p
	case "document":
		d, ok := s.documents[ref.id]
		if !ok {
			return domain.ErrNotFound
		}
		d.Body = plaintext
		d.UpdatedAt = now
		d.LastEditorSub = editorSub
		s.documents[ref.id] = d
	default:
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) CreateChannel(wsID, name string, now time.Time) (domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return domain.Channel{}, domain.ErrNotFound
	}
	ch := domain.Channel{
		ID:          id.New(),
		WorkspaceID: wsID,
		Name:        name,
		CreatedAt:   now,
	}
	s.channels[ch.ID] = ch
	s.messages[ch.ID] = nil
	return ch, nil
}

func (s *Store) ListChannels(wsID string) ([]domain.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Channel
	for _, ch := range s.channels {
		if ch.WorkspaceID == wsID {
			out = append(out, ch)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetChannel(channelID string) (domain.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, ok := s.channels[channelID]
	if !ok {
		return domain.Channel{}, domain.ErrNotFound
	}
	return ch, nil
}

func (s *Store) ListCardsInWorkspace(wsID string) ([]domain.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Card
	for _, c := range s.cards {
		b, ok := s.boards[c.BoardID]
		if ok && b.WorkspaceID == wsID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Store) AppendMessage(channelID, sub, body string, mentions []string, attachmentFileID string, now time.Time) (domain.ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[channelID]; !ok {
		return domain.ChatMessage{}, domain.ErrNotFound
	}
	copied := make([]string, len(mentions))
	copy(copied, mentions)
	seq := len(s.messages[channelID]) + 1
	msg := domain.ChatMessage{
		ID:               id.New(),
		ChannelID:        channelID,
		Sub:              sub,
		Body:             body,
		Mentions:         copied,
		AttachmentFileID: attachmentFileID,
		Seq:              seq,
		CreatedAt:        now,
	}
	s.messages[channelID] = append(s.messages[channelID], msg)
	return msg, nil
}

func (s *Store) SaveFile(f domain.StoredFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[f.WorkspaceID]; !ok {
		return domain.ErrNotFound
	}
	s.files[f.ID] = f
	return nil
}

func (s *Store) GetFile(fileID string) (domain.StoredFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.files[fileID]
	if !ok {
		return domain.StoredFile{}, domain.ErrNotFound
	}
	return f, nil
}

func (s *Store) AttachPageFile(pageID, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pages[pageID]; !ok {
		return domain.ErrNotFound
	}
	if _, ok := s.files[fileID]; !ok {
		return domain.ErrNotFound
	}
	for _, existing := range s.pageFiles[pageID] {
		if existing == fileID {
			return nil
		}
	}
	s.pageFiles[pageID] = append(s.pageFiles[pageID], fileID)
	return nil
}

func (s *Store) ListPageFiles(pageID string) ([]domain.StoredFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.pages[pageID]; !ok {
		return nil, domain.ErrNotFound
	}
	ids := s.pageFiles[pageID]
	out := make([]domain.StoredFile, 0, len(ids))
	for _, fid := range ids {
		if f, ok := s.files[fid]; ok {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *Store) ListMessages(channelID string, afterSeq int) ([]domain.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.channels[channelID]; !ok {
		return nil, domain.ErrNotFound
	}
	all := s.messages[channelID]
	if afterSeq <= 0 {
		out := make([]domain.ChatMessage, len(all))
		copy(out, all)
		return out, nil
	}
	var out []domain.ChatMessage
	for _, m := range all {
		if m.Seq > afterSeq {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Store) CountMessagesAfter(channelID string, afterSeq int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.channels[channelID]; !ok {
		return 0, domain.ErrNotFound
	}
	n := 0
	for _, m := range s.messages[channelID] {
		if m.Seq > afterSeq {
			n++
		}
	}
	return n, nil
}

func (s *Store) GetChannelRead(channelID, sub string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.channels[channelID]; !ok {
		return 0, domain.ErrNotFound
	}
	bySub := s.chatReads[channelID]
	if bySub == nil {
		return 0, nil
	}
	return bySub[sub], nil
}

func (s *Store) UpsertChannelRead(channelID, sub string, lastReadSeq int, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[channelID]; !ok {
		return domain.ErrNotFound
	}
	if lastReadSeq < 0 {
		return domain.ErrInvalid
	}
	bySub := s.chatReads[channelID]
	if bySub == nil {
		bySub = make(map[string]int)
		s.chatReads[channelID] = bySub
	}
	if lastReadSeq > bySub[sub] {
		bySub[sub] = lastReadSeq
	}
	_ = now
	return nil
}

func (s *Store) CreateChatTicket(sub, channelID string, readOnly bool, now time.Time) domain.ChatTicket {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := domain.ChatTicket{
		ID:        id.New(),
		Sub:       sub,
		ChannelID: channelID,
		ReadOnly:  readOnly,
		ExpiresAt: now.Add(domain.CollabTicketTTL),
	}
	s.chatTickets[t.ID] = t
	return t
}

func (s *Store) GetChatTicket(ticketID string) (domain.ChatTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.chatTickets[ticketID]
	if !ok {
		return domain.ChatTicket{}, domain.ErrNotFound
	}
	return t, nil
}

func (s *Store) CreateSprint(boardID, name string, startAt, endAt, now time.Time) (domain.Sprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	board, ok := s.boards[boardID]
	if !ok {
		return domain.Sprint{}, domain.ErrNotFound
	}
	sp := domain.Sprint{
		ID:          id.New(),
		BoardID:     boardID,
		WorkspaceID: board.WorkspaceID,
		Name:        name,
		StartAt:     startAt.UTC(),
		EndAt:       endAt.UTC(),
		CreatedAt:   now,
	}
	s.sprints[sp.ID] = sp
	return sp, nil
}

func (s *Store) ListSprints(boardID string) ([]domain.Sprint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.boards[boardID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Sprint
	for _, sp := range s.sprints {
		if sp.BoardID == boardID {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartAt.Equal(out[j].StartAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].StartAt.Before(out[j].StartAt)
	})
	return out, nil
}

func (s *Store) GetSprint(sprintID string) (domain.Sprint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sp, ok := s.sprints[sprintID]
	if !ok {
		return domain.Sprint{}, domain.ErrNotFound
	}
	return sp, nil
}

func (s *Store) UpdateSprint(sprintID, name string, startAt, endAt *time.Time, now time.Time) (domain.Sprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp, ok := s.sprints[sprintID]
	if !ok {
		return domain.Sprint{}, domain.ErrNotFound
	}
	if name != "" {
		sp.Name = name
	}
	if startAt != nil {
		sp.StartAt = startAt.UTC()
	}
	if endAt != nil {
		sp.EndAt = endAt.UTC()
	}
	s.sprints[sprintID] = sp
	_ = now
	return sp, nil
}

func (s *Store) DeleteSprint(sprintID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sprints[sprintID]; !ok {
		return domain.ErrNotFound
	}
	delete(s.sprints, sprintID)
	for id, c := range s.cards {
		if c.SprintID == sprintID {
			c.SprintID = ""
			s.cards[id] = c
		}
	}
	return nil
}

func (s *Store) ListCardsOnBoard(boardID string) ([]domain.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.boards[boardID]; !ok {
		return nil, domain.ErrNotFound
	}
	var out []domain.Card
	for _, c := range s.cards {
		if c.BoardID == boardID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Store) AppendPageVersionIfChanged(pageID, title, body, status, sub string, now time.Time) (domain.PageVersion, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pages[pageID]; !ok {
		return domain.PageVersion{}, false, domain.ErrNotFound
	}
	vers := s.pageVersions[pageID]
	if len(vers) > 0 {
		last := vers[len(vers)-1]
		if last.Title == title && last.Body == body && last.Status == status {
			return last, false, nil
		}
	}
	v := domain.PageVersion{
		PageID:    pageID,
		Number:    len(vers) + 1,
		Title:     title,
		Status:    status,
		Body:      body,
		Sub:       sub,
		CreatedAt: now,
	}
	if len(vers) > 0 {
		v.Number = vers[len(vers)-1].Number + 1
	}
	vers = append(vers, v)
	if len(vers) > domain.MaxPageVersions {
		vers = vers[len(vers)-domain.MaxPageVersions:]
	}
	s.pageVersions[pageID] = vers
	return v, true, nil
}

func (s *Store) ListPageVersions(pageID string) ([]domain.PageVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.pages[pageID]; !ok {
		return nil, domain.ErrNotFound
	}
	src := s.pageVersions[pageID]
	out := make([]domain.PageVersion, len(src))
	copy(out, src)
	return out, nil
}

func (s *Store) GetPageVersion(pageID string, number int) (domain.PageVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.pages[pageID]; !ok {
		return domain.PageVersion{}, domain.ErrNotFound
	}
	for _, v := range s.pageVersions[pageID] {
		if v.Number == number {
			return v, nil
		}
	}
	return domain.PageVersion{}, domain.ErrNotFound
}
