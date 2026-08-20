package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
	RoleGuest  Role = "guest"
)

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OrgID     string    `json:"orgId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Member struct {
	WorkspaceID string    `json:"workspaceId"`
	Sub         string    `json:"sub"`
	DisplayName string    `json:"displayName,omitempty"`
	Role        Role      `json:"role"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type Invitation struct {
	ID           string     `json:"id"`
	WorkspaceID  string     `json:"workspaceId"`
	TokenHash    string     `json:"-"`
	Role         Role       `json:"role"`
	InvitedEmail string     `json:"invitedEmail,omitempty"`
	MaxUses      int        `json:"maxUses"`
	UseCount     int        `json:"useCount"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	InvitedBy    string     `json:"invitedBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
}

type AuditEvent struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	ActorSub    string    `json:"actorSub"`
	Type        string    `json:"type"`
	TargetSub   string    `json:"targetSub,omitempty"`
	InviteID    string    `json:"inviteId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Board struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"createdAt"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
}

type Column struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"boardId"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
}

type Card struct {
	ID          string     `json:"id"`
	ColumnID    string     `json:"columnId"`
	BoardID     string     `json:"boardId"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Position    int        `json:"position"`
	Version     int        `json:"version"`
	SprintID    string     `json:"sprintId,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	AssigneeSub string     `json:"assigneeSub,omitempty"`
	Priority    string     `json:"priority,omitempty"`
	DueDate     *time.Time `json:"dueDate,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type ColumnWithCards struct {
	Column
	Cards []Card `json:"cards"`
}

type BoardDetail struct {
	Board
	Columns []ColumnWithCards `json:"columns"`
}

const (
	PageStatusDraft     = "draft"
	PageStatusPublished = "published"
	MaxPageTitle        = 160
	MaxPageBody         = 100000
	MaxChannelName      = 80
	MaxChatMessage      = 4000
	DefaultChannelName  = "general"
	MaxUploadBytes      = 20 * 1024 * 1024
	PurposeWiki         = "wiki"
	PurposeChat         = "chat"
	FileProviderLocal   = "local"
	FileProviderP03     = "p03"
	DoneColumnName      = "Done"
	MaxSprintName       = 80
	MaxSprintDays       = 90
	MaxPageVersions     = 100
	MaxInviteUses       = 100
)

type Sprint struct {
	ID          string    `json:"id"`
	BoardID     string    `json:"boardId"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type BurndownPoint struct {
	Date      string `json:"date"`
	Remaining int    `json:"remaining"`
}

type Burndown struct {
	SprintID string          `json:"sprintId"`
	Unit     string          `json:"unit"`
	Points   []BurndownPoint `json:"points"`
}

type PageVersion struct {
	PageID    string    `json:"pageId"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Status    string    `json:"status,omitempty"`
	Body      string    `json:"body,omitempty"`
	Sub       string    `json:"sub,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type PageVersionInfo struct {
	PageID    string    `json:"pageId"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Status    string    `json:"status,omitempty"`
	Sub       string    `json:"sub,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type DiffLine struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

type PageDiff struct {
	PageID       string     `json:"pageId"`
	From         int        `json:"from"`
	To           int        `json:"to"`
	TitleChanged bool       `json:"titleChanged"`
	FromTitle    string     `json:"fromTitle"`
	ToTitle      string     `json:"toTitle"`
	Lines        []DiffLine `json:"lines"`
}

var DefaultSearchTypes = []string{"page", "document", "board", "card", "channel", "message"}

type Channel struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ChatMessage struct {
	ID               string    `json:"id"`
	ChannelID        string    `json:"channelId"`
	Sub              string    `json:"sub"`
	Body             string    `json:"body"`
	Mentions         []string  `json:"mentions"`
	AttachmentFileID string    `json:"attachmentFileId,omitempty"`
	Seq              int       `json:"seq"`
	CreatedAt        time.Time `json:"createdAt"`
}

type SearchHit struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Context    string            `json:"context,omitempty"`
	MatchLabel string            `json:"matchLabel,omitempty"`
	Snippet    string            `json:"snippet"`
	HrefHints  map[string]string `json:"hrefHints"`
}

type StoredFile struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	UploaderSub string    `json:"uploaderSub"`
	Purpose     string    `json:"purpose"`
	Provider    string    `json:"provider"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	Name        string    `json:"name"`
	ViewToken   string    `json:"-"`
	Path        string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ChatTicket struct {
	ID        string    `json:"id"`
	Sub       string    `json:"sub"`
	ChannelID string    `json:"channelId"`
	ReadOnly  bool      `json:"readOnly"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Page struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspaceId"`
	ParentID         string     `json:"parentId,omitempty"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	Status           string     `json:"status"`
	Position         int        `json:"position"`
	Version          int        `json:"version"`
	CollabDocumentID string     `json:"collabDocumentId"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
}

type Document struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspaceId"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	CollabDocumentID string     `json:"collabDocumentId"`
	LastEditorSub    string     `json:"lastEditorSub,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type CollabTicket struct {
	ID               string    `json:"id"`
	Sub              string    `json:"sub"`
	CollabDocumentID string    `json:"collabDocumentId"`
	ReadOnly         bool      `json:"readOnly"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type CollabAuth struct {
	Sub              string `json:"sub"`
	CollabDocumentID string `json:"collabDocumentId"`
	ReadOnly         bool   `json:"readOnly"`
}

// PageNode is a tree row without body (tree payloads stay small).
type PageNode struct {
	ID       string     `json:"id"`
	ParentID string     `json:"parentId,omitempty"`
	Title    string     `json:"title"`
	Status   string     `json:"status"`
	Position int        `json:"position"`
	Children []PageNode `json:"children"`
}

func PageVisibleToGuest(p Page) bool {
	return p.Status == PageStatusPublished
}

func RoleAtLeast(have, need Role) bool {
	order := map[Role]int{RoleGuest: 0, RoleMember: 1, RoleOwner: 2}
	return order[have] >= order[need]
}
