package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/id"
	"github.com/portfolio/pf-workspace/api/internal/store"
)

var _ store.Store = (*Store)(nil)

type Store struct {
	pool       *pgxpool.Pool
	tenant     string
	skipTenant bool
}

func Connect(databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping() error {
	return s.pool.Ping(context.Background())
}

func (s *Store) migrate() error {
	for _, stmt := range splitSQL(schemaSQL) {
		if _, err := s.db().exec(context.Background(), stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSQL(raw string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			b.Reset()
			if stmt != "" {
				out = append(out, strings.TrimSuffix(stmt, ";"))
			}
		}
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func (s *Store) workspaceExists(ctx context.Context, wsID string) error {
	var n int
	err := s.db().QueryRow(ctx, "SELECT 1 FROM workspaces WHERE id = $1", wsID).Scan(&n)
	if err != nil {
		return mapErr(err)
	}
	return nil
}

func (s *Store) CreateWorkspace(name, ownerSub, orgID string, now time.Time) (domain.Workspace, error) {
	ctx := context.Background()
	ws := domain.Workspace{ID: id.New(), Name: name, OrgID: orgID, CreatedAt: now}
	tx, err := s.db().begin(ctx)
	if err != nil {
		return domain.Workspace{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "INSERT INTO workspaces (id, name, org_id, created_at) VALUES ($1,$2,$3,$4)", ws.ID, ws.Name, ws.OrgID, ws.CreatedAt); err != nil {
		return domain.Workspace{}, err
	}
	ownerName := domain.DevDisplayName(ownerSub)
	if _, err := tx.Exec(ctx, "INSERT INTO members (workspace_id, sub, role, display_name, joined_at) VALUES ($1,$2,$3,$4,$5)", ws.ID, ownerSub, domain.RoleOwner, ownerName, now); err != nil {
		return domain.Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Workspace{}, err
	}
	return ws, nil
}

func (s *Store) ListWorkspacesForSub(sub string) []domain.Workspace {
	ctx := context.Background()
	rows, err := s.db().Query(ctx, `
		SELECT w.id, w.name, w.org_id, w.created_at
		FROM workspaces w
		JOIN members m ON m.workspace_id = w.id
		WHERE m.sub = $1
		ORDER BY w.created_at`, sub)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []domain.Workspace
	for rows.Next() {
		var w domain.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.OrgID, &w.CreatedAt); err != nil {
			return out
		}
		out = append(out, w)
	}
	return out
}

func (s *Store) GetWorkspace(wsID string) (domain.Workspace, error) {
	var w domain.Workspace
	err := s.db().QueryRow(context.Background(), "SELECT id, name, org_id, created_at FROM workspaces WHERE id = $1", wsID).
		Scan(&w.ID, &w.Name, &w.OrgID, &w.CreatedAt)
	if err != nil {
		return domain.Workspace{}, mapErr(err)
	}
	return w, nil
}

func (s *Store) MemberRole(wsID, sub string) (domain.Role, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return "", err
	}
	var role domain.Role
	err := s.db().QueryRow(ctx, "SELECT role FROM members WHERE workspace_id = $1 AND sub = $2", wsID, sub).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrForbidden
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *Store) ListMembers(wsID string) ([]domain.Member, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.db().Query(ctx, "SELECT workspace_id, sub, role, display_name, joined_at FROM members WHERE workspace_id = $1 ORDER BY joined_at", wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.WorkspaceID, &m.Sub, &m.Role, &m.DisplayName, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMember(wsID, sub string) (domain.Member, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Member{}, err
	}
	var m domain.Member
	err := s.db().QueryRow(ctx, "SELECT workspace_id, sub, role, display_name, joined_at FROM members WHERE workspace_id = $1 AND sub = $2", wsID, sub).
		Scan(&m.WorkspaceID, &m.Sub, &m.Role, &m.DisplayName, &m.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, err
	}
	return m, nil
}

func (s *Store) AddMember(wsID, sub string, role domain.Role, now time.Time) (domain.Member, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Member{}, err
	}
	displayName := domain.DevDisplayName(sub)
	m := domain.Member{WorkspaceID: wsID, Sub: sub, Role: role, DisplayName: displayName, JoinedAt: now}
	_, err := s.db().exec(ctx, "INSERT INTO members (workspace_id, sub, role, display_name, joined_at) VALUES ($1,$2,$3,$4,$5)", wsID, sub, role, displayName, now)
	if isUnique(err) {
		return domain.Member{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Member{}, err
	}
	return m, nil
}

func (s *Store) UpdateMemberDisplayName(wsID, sub, displayName string) (domain.Member, error) {
	ctx := context.Background()
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return domain.Member{}, domain.ErrInvalid
	}
	tag, err := s.db().exec(ctx, "UPDATE members SET display_name = $3 WHERE workspace_id = $1 AND sub = $2", wsID, sub, displayName)
	if err != nil {
		return domain.Member{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Member{}, domain.ErrNotFound
	}
	return s.GetMember(wsID, sub)
}

func (s *Store) CreateInvitation(wsID, tokenHash string, role domain.Role, invitedEmail string, maxUses int, expiresAt time.Time, invitedBy string, now time.Time) (domain.Invitation, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Invitation{}, err
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
	_, err := s.db().exec(ctx, `INSERT INTO invitations (id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		inv.ID, inv.WorkspaceID, inv.TokenHash, inv.Role, inv.InvitedEmail, inv.MaxUses, inv.UseCount, inv.ExpiresAt, inv.InvitedBy, inv.CreatedAt)
	if isUnique(err) {
		return domain.Invitation{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Invitation{}, err
	}
	return inv, nil
}

func (s *Store) ListInvitations(wsID string) ([]domain.Invitation, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.db().Query(ctx, `SELECT id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at, revoked_at
		FROM invitations WHERE workspace_id = $1 ORDER BY created_at DESC`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Invitation
	for rows.Next() {
		var inv domain.Invitation
		if err := rows.Scan(&inv.ID, &inv.WorkspaceID, &inv.TokenHash, &inv.Role, &inv.InvitedEmail, &inv.MaxUses, &inv.UseCount, &inv.ExpiresAt, &inv.InvitedBy, &inv.CreatedAt, &inv.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) GetInvitationByTokenHash(tokenHash string) (domain.Invitation, error) {
	var inv domain.Invitation
	err := s.db().QueryRow(context.Background(), `SELECT id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at, revoked_at
		FROM invitations WHERE token_hash = $1`, tokenHash).
		Scan(&inv.ID, &inv.WorkspaceID, &inv.TokenHash, &inv.Role, &inv.InvitedEmail, &inv.MaxUses, &inv.UseCount, &inv.ExpiresAt, &inv.InvitedBy, &inv.CreatedAt, &inv.RevokedAt)
	if err != nil {
		return domain.Invitation{}, mapErr(err)
	}
	return inv, nil
}

func (s *Store) RevokeInvitation(wsID, inviteID string, now time.Time) (domain.Invitation, bool, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Invitation{}, false, err
	}
	var inv domain.Invitation
	err := s.db().QueryRow(ctx, `UPDATE invitations SET revoked_at = $3
		WHERE id = $1 AND workspace_id = $2 AND revoked_at IS NULL
		RETURNING id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at, revoked_at`,
		inviteID, wsID, now).
		Scan(&inv.ID, &inv.WorkspaceID, &inv.TokenHash, &inv.Role, &inv.InvitedEmail, &inv.MaxUses, &inv.UseCount, &inv.ExpiresAt, &inv.InvitedBy, &inv.CreatedAt, &inv.RevokedAt)
	if err == nil {
		return inv, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Invitation{}, false, mapErr(err)
	}
	err = s.db().QueryRow(ctx, `SELECT id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at, revoked_at
		FROM invitations WHERE id = $1 AND workspace_id = $2`, inviteID, wsID).
		Scan(&inv.ID, &inv.WorkspaceID, &inv.TokenHash, &inv.Role, &inv.InvitedEmail, &inv.MaxUses, &inv.UseCount, &inv.ExpiresAt, &inv.InvitedBy, &inv.CreatedAt, &inv.RevokedAt)
	if err != nil {
		return domain.Invitation{}, false, mapErr(err)
	}
	if inv.RevokedAt != nil {
		return inv, false, nil
	}
	return domain.Invitation{}, false, domain.ErrNotFound
}

func (s *Store) AcceptInvitation(inviteID, sub string, now time.Time) (domain.Invitation, domain.Member, bool, error) {
	ctx := context.Background()
	tx, err := s.db().begin(ctx)
	if err != nil {
		return domain.Invitation{}, domain.Member{}, false, err
	}
	defer tx.Rollback(ctx)
	var inv domain.Invitation
	err = tx.QueryRow(ctx, `SELECT id, workspace_id, token_hash, role, invited_email, max_uses, use_count, expires_at, invited_by, created_at, revoked_at
		FROM invitations WHERE id = $1 FOR UPDATE`, inviteID).
		Scan(&inv.ID, &inv.WorkspaceID, &inv.TokenHash, &inv.Role, &inv.InvitedEmail, &inv.MaxUses, &inv.UseCount, &inv.ExpiresAt, &inv.InvitedBy, &inv.CreatedAt, &inv.RevokedAt)
	if err != nil {
		return domain.Invitation{}, domain.Member{}, false, mapErr(err)
	}
	if inv.RevokedAt != nil || !now.Before(inv.ExpiresAt) || inv.UseCount >= inv.MaxUses {
		return domain.Invitation{}, domain.Member{}, false, domain.ErrForbidden
	}
	displayName := domain.DevDisplayName(sub)
	member := domain.Member{WorkspaceID: inv.WorkspaceID, Sub: sub, Role: inv.Role, DisplayName: displayName, JoinedAt: now}
	cmd, err := tx.Exec(ctx, `INSERT INTO members (workspace_id, sub, role, display_name, joined_at) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (workspace_id, sub) DO NOTHING`, inv.WorkspaceID, sub, inv.Role, displayName, now)
	if err != nil {
		return domain.Invitation{}, domain.Member{}, false, err
	}
	joined := cmd.RowsAffected() == 1
	if joined {
		if _, err := tx.Exec(ctx, `UPDATE invitations SET use_count = use_count + 1 WHERE id = $1`, inv.ID); err != nil {
			return domain.Invitation{}, domain.Member{}, false, err
		}
		inv.UseCount++
	} else {
		err = tx.QueryRow(ctx, "SELECT workspace_id, sub, role, display_name, joined_at FROM members WHERE workspace_id = $1 AND sub = $2", inv.WorkspaceID, sub).
			Scan(&member.WorkspaceID, &member.Sub, &member.Role, &member.DisplayName, &member.JoinedAt)
		if err != nil {
			return domain.Invitation{}, domain.Member{}, false, mapErr(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Invitation{}, domain.Member{}, false, err
	}
	return inv, member, joined, nil
}

func (s *Store) AddAuditEvent(event domain.AuditEvent) error {
	_, err := s.db().exec(context.Background(), `INSERT INTO audit_events (id, workspace_id, actor_sub, type, target_sub, invite_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		event.ID, event.WorkspaceID, event.ActorSub, event.Type, event.TargetSub, event.InviteID, event.CreatedAt)
	return err
}

func (s *Store) ListAuditEvents(wsID string) ([]domain.AuditEvent, error) {
	rows, err := s.db().Query(context.Background(), `SELECT id, workspace_id, actor_sub, type, target_sub, invite_id, created_at
		FROM audit_events WHERE workspace_id = $1 ORDER BY created_at DESC`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditEvent
	for rows.Next() {
		var ev domain.AuditEvent
		if err := rows.Scan(&ev.ID, &ev.WorkspaceID, &ev.ActorSub, &ev.Type, &ev.TargetSub, &ev.InviteID, &ev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) CreateBoard(wsID, name string, now time.Time) (domain.Board, []domain.Column, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Board{}, nil, err
	}
	board := domain.Board{ID: id.New(), WorkspaceID: wsID, Name: name, CreatedAt: now}
	tx, err := s.db().begin(ctx)
	if err != nil {
		return domain.Board{}, nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "INSERT INTO boards (id, workspace_id, name, created_at) VALUES ($1,$2,$3,$4)", board.ID, wsID, name, now); err != nil {
		return domain.Board{}, nil, err
	}
	defaultNames := []string{"To Do", "In Progress", "Done"}
	cols := make([]domain.Column, 0, len(defaultNames))
	for i, n := range defaultNames {
		col := domain.Column{ID: id.New(), BoardID: board.ID, Name: n, Position: i, CreatedAt: now}
		if _, err := tx.Exec(ctx, "INSERT INTO columns (id, board_id, name, position, created_at) VALUES ($1,$2,$3,$4,$5)", col.ID, col.BoardID, col.Name, col.Position, col.CreatedAt); err != nil {
			return domain.Board{}, nil, err
		}
		cols = append(cols, col)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Board{}, nil, err
	}
	return board, cols, nil
}

func (s *Store) ArchiveBoard(boardID string, now time.Time) error {
	ctx := context.Background()
	cmd, err := s.db().exec(ctx, "UPDATE boards SET archived_at = $2 WHERE id = $1 AND archived_at IS NULL", boardID, now)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) UnarchiveBoard(boardID string) error {
	ctx := context.Background()
	cmd, err := s.db().exec(ctx, "UPDATE boards SET archived_at = NULL WHERE id = $1", boardID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListBoards(wsID string) ([]domain.Board, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.db().Query(ctx, "SELECT id, workspace_id, name, created_at, archived_at FROM boards WHERE workspace_id = $1 ORDER BY created_at", wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Board
	for rows.Next() {
		var b domain.Board
		if err := rows.Scan(&b.ID, &b.WorkspaceID, &b.Name, &b.CreatedAt, &b.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func scanCard(scan func(dest ...any) error) (domain.Card, error) {
	var c domain.Card
	var completed, due *time.Time
	err := scan(
		&c.ID, &c.ColumnID, &c.BoardID, &c.Title, &c.Description, &c.Position, &c.Version,
		&c.SprintID, &completed, &c.AssigneeSub, &c.Priority, &due, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return domain.Card{}, err
	}
	c.CompletedAt = completed
	c.DueDate = due
	return c, nil
}

const cardCols = `id, column_id, board_id, title, description, position, version, sprint_id, completed_at, assignee_sub, priority, due_date, created_at, updated_at`

func (s *Store) GetBoardDetail(boardID string) (domain.BoardDetail, error) {
	ctx := context.Background()
	var board domain.Board
	err := s.db().QueryRow(ctx, "SELECT id, workspace_id, name, created_at, archived_at FROM boards WHERE id = $1", boardID).
		Scan(&board.ID, &board.WorkspaceID, &board.Name, &board.CreatedAt, &board.ArchivedAt)
	if err != nil {
		return domain.BoardDetail{}, mapErr(err)
	}
	colRows, err := s.db().Query(ctx, "SELECT id, board_id, name, position, created_at FROM columns WHERE board_id = $1 ORDER BY position", boardID)
	if err != nil {
		return domain.BoardDetail{}, err
	}
	defer colRows.Close()
	var cols []domain.ColumnWithCards
	for colRows.Next() {
		var col domain.Column
		if err := colRows.Scan(&col.ID, &col.BoardID, &col.Name, &col.Position, &col.CreatedAt); err != nil {
			return domain.BoardDetail{}, err
		}
		cols = append(cols, domain.ColumnWithCards{Column: col})
	}
	if err := colRows.Err(); err != nil {
		return domain.BoardDetail{}, err
	}
	cardRows, err := s.db().Query(ctx, "SELECT "+cardCols+" FROM cards WHERE board_id = $1 ORDER BY column_id, position", boardID)
	if err != nil {
		return domain.BoardDetail{}, err
	}
	defer cardRows.Close()
	byCol := map[string][]domain.Card{}
	for cardRows.Next() {
		c, err := scanCard(cardRows.Scan)
		if err != nil {
			return domain.BoardDetail{}, err
		}
		byCol[c.ColumnID] = append(byCol[c.ColumnID], c)
	}
	for i := range cols {
		cols[i].Cards = byCol[cols[i].ID]
		if cols[i].Cards == nil {
			cols[i].Cards = []domain.Card{}
		}
	}
	return domain.BoardDetail{Board: board, Columns: cols}, nil
}

func (s *Store) BoardWorkspaceID(boardID string) (string, error) {
	var wsID string
	err := s.db().QueryRow(context.Background(), "SELECT workspace_id FROM boards WHERE id = $1", boardID).Scan(&wsID)
	if err != nil {
		return "", mapErr(err)
	}
	return wsID, nil
}

func (s *Store) ColumnBoardID(columnID string) (string, error) {
	var boardID string
	err := s.db().QueryRow(context.Background(), "SELECT board_id FROM columns WHERE id = $1", columnID).Scan(&boardID)
	if err != nil {
		return "", mapErr(err)
	}
	return boardID, nil
}

func (s *Store) CreateCard(columnID, title, description string, now time.Time) (domain.Card, error) {
	ctx := context.Background()
	var boardID, colName string
	err := s.db().QueryRow(ctx, "SELECT board_id, name FROM columns WHERE id = $1", columnID).Scan(&boardID, &colName)
	if err != nil {
		return domain.Card{}, mapErr(err)
	}
	var pos int
	_ = s.db().QueryRow(ctx, "SELECT COUNT(*) FROM cards WHERE column_id = $1", columnID).Scan(&pos)
	card := domain.Card{
		ID: id.New(), ColumnID: columnID, BoardID: boardID, Title: title, Description: description,
		Position: pos, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if domain.ColumnIsDone(colName) {
		t := now
		card.CompletedAt = &t
	}
	_, err = s.db().exec(ctx, `INSERT INTO cards (`+cardCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		card.ID, card.ColumnID, card.BoardID, card.Title, card.Description, card.Position, card.Version,
		card.SprintID, card.CompletedAt, card.AssigneeSub, card.Priority, card.DueDate, card.CreatedAt, card.UpdatedAt,
	)
	if err != nil {
		return domain.Card{}, err
	}
	return card, nil
}

func (s *Store) GetCard(cardID string) (domain.Card, error) {
	row := s.db().QueryRow(context.Background(), "SELECT "+cardCols+" FROM cards WHERE id = $1", cardID)
	c, err := scanCard(row.Scan)
	if err != nil {
		return domain.Card{}, mapErr(err)
	}
	return c, nil
}

func (s *Store) CardWorkspaceID(cardID string) (string, error) {
	var wsID string
	err := s.db().QueryRow(context.Background(), `
		SELECT b.workspace_id FROM cards c JOIN boards b ON b.id = c.board_id WHERE c.id = $1`, cardID).Scan(&wsID)
	if err != nil {
		return "", mapErr(err)
	}
	return wsID, nil
}

func (s *Store) UpdateCard(cardID, title, description string, sprintID *string, version int, now time.Time) (domain.Card, error) {
	ctx := context.Background()
	tx, err := s.db().begin(ctx)
	if err != nil {
		return domain.Card{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, "SELECT "+cardCols+" FROM cards WHERE id = $1 FOR UPDATE", cardID)
	c, err := scanCard(row.Scan)
	if err != nil {
		return domain.Card{}, mapErr(err)
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
			var boardID string
			err := tx.QueryRow(ctx, "SELECT board_id FROM sprints WHERE id = $1", sid).Scan(&boardID)
			if err != nil {
				return domain.Card{}, mapErr(err)
			}
			if boardID != c.BoardID {
				return domain.Card{}, domain.ErrInvalid
			}
			c.SprintID = sid
		}
	}
	c.Version++
	c.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE cards SET title=$2, description=$3, sprint_id=$4, version=$5, updated_at=$6
		WHERE id=$1 AND version=$7`, cardID, c.Title, c.Description, c.SprintID, c.Version, c.UpdatedAt, version)
	if err != nil {
		return domain.Card{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Card{}, err
	}
	return c, nil
}

func (s *Store) MoveCard(cardID, targetColumnID string, position int, version int, now time.Time) (domain.Card, error) {
	ctx := context.Background()
	tx, err := s.db().begin(ctx)
	if err != nil {
		return domain.Card{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, "SELECT "+cardCols+" FROM cards WHERE id = $1 FOR UPDATE", cardID)
	c, err := scanCard(row.Scan)
	if err != nil {
		return domain.Card{}, mapErr(err)
	}
	if c.Version != version {
		return c, domain.ErrConflict
	}
	var targetBoard, targetName string
	err = tx.QueryRow(ctx, "SELECT board_id, name FROM columns WHERE id = $1", targetColumnID).Scan(&targetBoard, &targetName)
	if err != nil {
		return domain.Card{}, mapErr(err)
	}
	if targetBoard != c.BoardID {
		return domain.Card{}, domain.ErrForbidden
	}
	oldColumnID := c.ColumnID
	if err := reindexWithout(ctx, tx, oldColumnID, cardID); err != nil {
		return domain.Card{}, err
	}
	ids, err := columnCardIDs(ctx, tx, targetColumnID)
	if err != nil {
		return domain.Card{}, err
	}
	if position < 0 {
		position = 0
	}
	if position > len(ids) {
		position = len(ids)
	}
	newIDs := append([]string{}, ids[:position]...)
	newIDs = append(newIDs, cardID)
	newIDs = append(newIDs, ids[position:]...)
	if err := applyPositions(ctx, tx, newIDs); err != nil {
		return domain.Card{}, err
	}
	c.ColumnID = targetColumnID
	c.Position = position
	if domain.ColumnIsDone(targetName) {
		if c.CompletedAt == nil {
			t := now
			c.CompletedAt = &t
		}
	} else {
		c.CompletedAt = nil
	}
	c.Version++
	c.UpdatedAt = now
	cmd, err := tx.Exec(ctx, `UPDATE cards SET column_id=$2, position=$3, completed_at=$4, version=$5, updated_at=$6
		WHERE id=$1 AND version=$7`, cardID, c.ColumnID, c.Position, c.CompletedAt, c.Version, c.UpdatedAt, version)
	if err != nil {
		return domain.Card{}, err
	}
	if cmd.RowsAffected() != 1 {
		return c, domain.ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Card{}, err
	}
	return c, nil
}

func columnCardIDs(ctx context.Context, tx pgx.Tx, columnID string) ([]string, error) {
	rows, err := tx.Query(ctx, "SELECT id FROM cards WHERE column_id = $1 ORDER BY position, id", columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func reindexWithout(ctx context.Context, tx pgx.Tx, columnID, skipID string) error {
	ids, err := columnCardIDs(ctx, tx, columnID)
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != skipID {
			kept = append(kept, id)
		}
	}
	return applyPositions(ctx, tx, kept)
}

func applyPositions(ctx context.Context, tx pgx.Tx, ids []string) error {
	for i, id := range ids {
		if _, err := tx.Exec(ctx, "UPDATE cards SET position = $2 WHERE id = $1", id, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListCardsInWorkspace(wsID string) ([]domain.Card, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.db().Query(ctx, "SELECT c.id, c.column_id, c.board_id, c.title, c.description, c.position, c.version, c.sprint_id, c.completed_at, c.assignee_sub, c.priority, c.due_date, c.created_at, c.updated_at FROM cards c JOIN boards b ON b.id = c.board_id WHERE b.workspace_id = $1", wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func (s *Store) ListCardsOnBoard(boardID string) ([]domain.Card, error) {
	ctx := context.Background()
	var n int
	if err := s.db().QueryRow(ctx, "SELECT 1 FROM boards WHERE id = $1", boardID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.db().Query(ctx, "SELECT "+cardCols+" FROM cards WHERE board_id = $1", boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func scanCards(rows pgx.Rows) ([]domain.Card, error) {
	var out []domain.Card
	for rows.Next() {
		c, err := scanCard(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
