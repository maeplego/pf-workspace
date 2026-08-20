package store

import (
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
)

// Store is the persistence contract. Memory is for unit tests; Postgres is Compose / overlay.
type Store interface {
	Ping() error

	CreateWorkspace(name, ownerSub, orgID string, now time.Time) (domain.Workspace, error)
	ListWorkspacesForSub(sub string) []domain.Workspace
	GetWorkspace(wsID string) (domain.Workspace, error)
	MemberRole(wsID, sub string) (domain.Role, error)
	ListMembers(wsID string) ([]domain.Member, error)
	GetMember(wsID, sub string) (domain.Member, error)
	AddMember(wsID, sub string, role domain.Role, now time.Time) (domain.Member, error)
	UpdateMemberDisplayName(wsID, sub, displayName string) (domain.Member, error)
	UpdateMemberRole(wsID, sub string, role domain.Role) (domain.Member, error)
	RemoveMember(wsID, sub string) error
	CreateInvitation(wsID, tokenHash string, role domain.Role, invitedEmail string, maxUses int, expiresAt time.Time, invitedBy string, now time.Time) (domain.Invitation, error)
	ListInvitations(wsID string) ([]domain.Invitation, error)
	GetInvitationByTokenHash(tokenHash string) (domain.Invitation, error)
	UpdateInvitationPolicy(wsID, inviteID string, role domain.Role, invitedEmail string, maxUses int, expiresAt time.Time) (domain.Invitation, error)
	RevokeInvitation(wsID, inviteID string, now time.Time) (domain.Invitation, bool, error)
	AcceptInvitation(inviteID, sub string, now time.Time) (domain.Invitation, domain.Member, bool, error)
	AddAuditEvent(event domain.AuditEvent) error
	ListAuditEvents(wsID string) ([]domain.AuditEvent, error)

	CreateBoard(wsID, name string, now time.Time) (domain.Board, []domain.Column, error)
	ArchiveBoard(boardID string, now time.Time) error
	UnarchiveBoard(boardID string) error
	ListBoards(wsID string) ([]domain.Board, error)
	GetBoardDetail(boardID string) (domain.BoardDetail, error)
	BoardWorkspaceID(boardID string) (string, error)
	ColumnBoardID(columnID string) (string, error)
	CreateColumn(boardID, name string, now time.Time) (domain.Column, error)
	RenameColumn(columnID, name string) (domain.Column, error)
	DeleteColumn(columnID string) error
	ReorderColumns(boardID string, columnIDs []string) error
	CreateCard(columnID, title, description string, now time.Time) (domain.Card, error)
	GetCard(cardID string) (domain.Card, error)
	CardWorkspaceID(cardID string) (string, error)
	UpdateCard(cardID, title, description string, sprintID *string, version int, now time.Time) (domain.Card, error)
	MoveCard(cardID, targetColumnID string, position int, version int, now time.Time) (domain.Card, error)
	ListCardsInWorkspace(wsID string) ([]domain.Card, error)
	ListCardsOnBoard(boardID string) ([]domain.Card, error)

	CreatePage(wsID, parentID, title, body, status string, now time.Time) (domain.Page, error)
	GetPage(pageID string) (domain.Page, error)
	ListPages(wsID string) ([]domain.Page, error)
	PageWorkspaceID(pageID string) (string, error)
	UpdatePage(pageID string, title, body, status, parentID *string, version int, now time.Time) (domain.Page, error)
	ArchivePage(pageID string, now time.Time) error
	UnarchivePage(pageID string) error

	CreateDocument(wsID, title, body string, now time.Time) (domain.Document, error)
	ListDocuments(wsID string) ([]domain.Document, error)
	GetDocument(docID string) (domain.Document, error)
	UpdateDocumentTitle(docID, title string, now time.Time) (domain.Document, error)
	TrashDocument(docID string, now time.Time) error
	RestoreDocument(docID string) error
	LookupCollab(collabDocumentID string) (kind, id string, err error)
	CreateTicket(sub, collabDocumentID string, readOnly bool, now time.Time) domain.CollabTicket
	GetTicket(ticketID string) (domain.CollabTicket, error)
	ApplyCollabSnapshot(collabDocumentID, plaintext, editorSub string, now time.Time) error

	CreateChannel(wsID, name string, now time.Time) (domain.Channel, error)
	ListChannels(wsID string) ([]domain.Channel, error)
	GetChannel(channelID string) (domain.Channel, error)
	AppendMessage(channelID, sub, body string, mentions []string, attachmentFileID string, now time.Time) (domain.ChatMessage, error)
	ListMessages(channelID string, afterSeq int) ([]domain.ChatMessage, error)
	CountMessagesAfter(channelID string, afterSeq int) (int, error)
	GetChannelRead(channelID, sub string) (int, error)
	UpsertChannelRead(channelID, sub string, lastReadSeq int, now time.Time) error
	CreateChatTicket(sub, channelID string, readOnly bool, now time.Time) domain.ChatTicket
	GetChatTicket(ticketID string) (domain.ChatTicket, error)

	SaveFile(f domain.StoredFile) error
	GetFile(fileID string) (domain.StoredFile, error)
	AttachPageFile(pageID, fileID string) error
	ListPageFiles(pageID string) ([]domain.StoredFile, error)

	CreateSprint(boardID, name string, startAt, endAt, now time.Time) (domain.Sprint, error)
	ListSprints(boardID string) ([]domain.Sprint, error)
	GetSprint(sprintID string) (domain.Sprint, error)
	UpdateSprint(sprintID, name string, startAt, endAt *time.Time, now time.Time) (domain.Sprint, error)
	DeleteSprint(sprintID string) error

	AppendPageVersionIfChanged(pageID, title, body, status, sub string, now time.Time) (domain.PageVersion, bool, error)
	ListPageVersions(pageID string) ([]domain.PageVersion, error)
	GetPageVersion(pageID string, number int) (domain.PageVersion, error)
}
