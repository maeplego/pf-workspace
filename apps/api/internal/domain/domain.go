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
	CreatedAt time.Time `json:"createdAt"`
}

type Member struct {
	WorkspaceID string    `json:"workspaceId"`
	Sub         string    `json:"sub"`
	Role        Role      `json:"role"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type Board struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
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
)

type Channel struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channelId"`
	Sub       string    `json:"sub"`
	Body      string    `json:"body"`
	Seq       int       `json:"seq"`
	CreatedAt time.Time `json:"createdAt"`
}

type ChatTicket struct {
	ID        string    `json:"id"`
	Sub       string    `json:"sub"`
	ChannelID string    `json:"channelId"`
	ReadOnly  bool      `json:"readOnly"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Page struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspaceId"`
	ParentID         string    `json:"parentId,omitempty"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	Status           string    `json:"status"`
	Position         int       `json:"position"`
	Version          int       `json:"version"`
	CollabDocumentID string    `json:"collabDocumentId"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Document struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspaceId"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	CollabDocumentID string    `json:"collabDocumentId"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
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
