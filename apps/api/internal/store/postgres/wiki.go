package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/id"
)

func scanPage(scan func(dest ...any) error) (domain.Page, error) {
	var p domain.Page
	err := scan(&p.ID, &p.WorkspaceID, &p.ParentID, &p.Title, &p.Body, &p.Status, &p.Position, &p.Version, &p.CollabDocumentID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

const pageCols = `id, workspace_id, parent_id, title, body, status, position, version, collab_document_id, created_at, updated_at`

func (s *Store) CreatePage(wsID, parentID, title, body, status string, now time.Time) (domain.Page, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Page{}, err
	}
	if err := s.validateParent(ctx, "", wsID, parentID); err != nil {
		return domain.Page{}, err
	}
	var pos int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(position)+1, 0) FROM pages WHERE workspace_id = $1 AND parent_id = $2`, wsID, parentID).Scan(&pos)
	if err != nil {
		return domain.Page{}, err
	}
	p := domain.Page{
		ID: id.New(), WorkspaceID: wsID, ParentID: parentID, Title: title, Body: body, Status: status,
		Position: pos, Version: 1, CollabDocumentID: id.New(), CreatedAt: now, UpdatedAt: now,
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO pages (`+pageCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.WorkspaceID, p.ParentID, p.Title, p.Body, p.Status, p.Position, p.Version, p.CollabDocumentID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return domain.Page{}, err
	}
	return p, nil
}

func (s *Store) GetPage(pageID string) (domain.Page, error) {
	row := s.pool.QueryRow(context.Background(), "SELECT "+pageCols+" FROM pages WHERE id = $1", pageID)
	p, err := scanPage(row.Scan)
	if err != nil {
		return domain.Page{}, mapErr(err)
	}
	return p, nil
}

func (s *Store) ListPages(wsID string) ([]domain.Page, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, "SELECT "+pageCols+" FROM pages WHERE workspace_id = $1", wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Page
	for rows.Next() {
		p, err := scanPage(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PageWorkspaceID(pageID string) (string, error) {
	var wsID string
	err := s.pool.QueryRow(context.Background(), "SELECT workspace_id FROM pages WHERE id = $1", pageID).Scan(&wsID)
	if err != nil {
		return "", mapErr(err)
	}
	return wsID, nil
}

func (s *Store) UpdatePage(pageID string, title, body, status, parentID *string, version int, now time.Time) (domain.Page, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Page{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, "SELECT "+pageCols+" FROM pages WHERE id = $1 FOR UPDATE", pageID)
	p, err := scanPage(row.Scan)
	if err != nil {
		return domain.Page{}, mapErr(err)
	}
	if p.Version != version {
		return p, domain.ErrConflict
	}
	if parentID != nil {
		if err := s.validateParentTx(ctx, tx, pageID, p.WorkspaceID, *parentID); err != nil {
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
	cmd, err := tx.Exec(ctx, `UPDATE pages SET parent_id=$2, title=$3, body=$4, status=$5, version=$6, updated_at=$7
		WHERE id=$1 AND version=$8`, pageID, p.ParentID, p.Title, p.Body, p.Status, p.Version, p.UpdatedAt, version)
	if err != nil {
		return domain.Page{}, err
	}
	if cmd.RowsAffected() != 1 {
		return p, domain.ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Page{}, err
	}
	return p, nil
}

func (s *Store) validateParent(ctx context.Context, pageID, wsID, parentID string) error {
	return s.validateParentTx(ctx, s.pool, pageID, wsID, parentID)
}

type pageLookup interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Store) validateParentTx(ctx context.Context, q pageLookup, pageID, wsID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == pageID {
		return domain.ErrInvalid
	}
	var parWS, cur string
	err := q.QueryRow(ctx, "SELECT workspace_id, parent_id FROM pages WHERE id = $1", parentID).Scan(&parWS, &cur)
	if err != nil {
		return mapErr(err)
	}
	if parWS != wsID {
		return domain.ErrInvalid
	}
	seen := map[string]bool{parentID: true}
	for cur != "" {
		if cur == pageID {
			return domain.ErrInvalid
		}
		if seen[cur] {
			return domain.ErrInvalid
		}
		seen[cur] = true
		var next string
		err := q.QueryRow(ctx, "SELECT parent_id FROM pages WHERE id = $1", cur).Scan(&next)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		cur = next
	}
	return nil
}

func (s *Store) CreateDocument(wsID, title, body string, now time.Time) (domain.Document, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Document{}, err
	}
	d := domain.Document{ID: id.New(), WorkspaceID: wsID, Title: title, Body: body, CollabDocumentID: id.New(), CreatedAt: now, UpdatedAt: now}
	_, err := s.pool.Exec(ctx, `INSERT INTO documents (id, workspace_id, title, body, collab_document_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, d.ID, d.WorkspaceID, d.Title, d.Body, d.CollabDocumentID, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return domain.Document{}, err
	}
	return d, nil
}

func (s *Store) ListDocuments(wsID string) ([]domain.Document, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, workspace_id, title, body, collab_document_id, created_at, updated_at
		FROM documents WHERE workspace_id = $1 ORDER BY created_at, id`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Document
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Body, &d.CollabDocumentID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(docID string) (domain.Document, error) {
	var d domain.Document
	err := s.pool.QueryRow(context.Background(), `SELECT id, workspace_id, title, body, collab_document_id, created_at, updated_at FROM documents WHERE id = $1`, docID).
		Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Body, &d.CollabDocumentID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	return d, nil
}

func (s *Store) UpdateDocumentTitle(docID, title string, now time.Time) (domain.Document, error) {
	var d domain.Document
	err := s.pool.QueryRow(context.Background(), `UPDATE documents SET title=$2, updated_at=$3 WHERE id=$1
		RETURNING id, workspace_id, title, body, collab_document_id, created_at, updated_at`, docID, title, now).
		Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Body, &d.CollabDocumentID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	return d, nil
}

func (s *Store) LookupCollab(collabDocumentID string) (kind, id string, err error) {
	ctx := context.Background()
	var pageID string
	err = s.pool.QueryRow(ctx, "SELECT id FROM pages WHERE collab_document_id = $1", collabDocumentID).Scan(&pageID)
	if err == nil {
		return "page", pageID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}
	var docID string
	err = s.pool.QueryRow(ctx, "SELECT id FROM documents WHERE collab_document_id = $1", collabDocumentID).Scan(&docID)
	if err != nil {
		return "", "", mapErr(err)
	}
	return "document", docID, nil
}

func (s *Store) CreateTicket(sub, collabDocumentID string, readOnly bool, now time.Time) domain.CollabTicket {
	t := domain.CollabTicket{ID: id.New(), Sub: sub, CollabDocumentID: collabDocumentID, ReadOnly: readOnly, ExpiresAt: now.Add(domain.CollabTicketTTL)}
	_, _ = s.pool.Exec(context.Background(), `INSERT INTO collab_tickets (id, sub, collab_document_id, read_only, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		t.ID, t.Sub, t.CollabDocumentID, t.ReadOnly, t.ExpiresAt)
	return t
}

func (s *Store) GetTicket(ticketID string) (domain.CollabTicket, error) {
	var t domain.CollabTicket
	err := s.pool.QueryRow(context.Background(), `SELECT id, sub, collab_document_id, read_only, expires_at FROM collab_tickets WHERE id = $1`, ticketID).
		Scan(&t.ID, &t.Sub, &t.CollabDocumentID, &t.ReadOnly, &t.ExpiresAt)
	if err != nil {
		return domain.CollabTicket{}, mapErr(err)
	}
	return t, nil
}

func (s *Store) ApplyCollabSnapshot(collabDocumentID, plaintext string, now time.Time) error {
	ctx := context.Background()
	cmd, err := s.pool.Exec(ctx, "UPDATE pages SET body=$2, updated_at=$3 WHERE collab_document_id=$1", collabDocumentID, plaintext, now)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 1 {
		return nil
	}
	cmd, err = s.pool.Exec(ctx, "UPDATE documents SET body=$2, updated_at=$3 WHERE collab_document_id=$1", collabDocumentID, plaintext, now)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) CreateChannel(wsID, name string, now time.Time) (domain.Channel, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return domain.Channel{}, err
	}
	ch := domain.Channel{ID: id.New(), WorkspaceID: wsID, Name: name, CreatedAt: now}
	_, err := s.pool.Exec(ctx, "INSERT INTO channels (id, workspace_id, name, created_at) VALUES ($1,$2,$3,$4)", ch.ID, ch.WorkspaceID, ch.Name, ch.CreatedAt)
	if err != nil {
		return domain.Channel{}, err
	}
	return ch, nil
}

func (s *Store) ListChannels(wsID string) ([]domain.Channel, error) {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, wsID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, "SELECT id, workspace_id, name, created_at FROM channels WHERE workspace_id = $1 ORDER BY created_at, id", wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Channel
	for rows.Next() {
		var ch domain.Channel
		if err := rows.Scan(&ch.ID, &ch.WorkspaceID, &ch.Name, &ch.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *Store) GetChannel(channelID string) (domain.Channel, error) {
	var ch domain.Channel
	err := s.pool.QueryRow(context.Background(), "SELECT id, workspace_id, name, created_at FROM channels WHERE id = $1", channelID).
		Scan(&ch.ID, &ch.WorkspaceID, &ch.Name, &ch.CreatedAt)
	if err != nil {
		return domain.Channel{}, mapErr(err)
	}
	return ch, nil
}

func (s *Store) AppendMessage(channelID, sub, body string, mentions []string, attachmentFileID string, now time.Time) (domain.ChatMessage, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	defer tx.Rollback(ctx)
	var n int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM channels WHERE id = $1 FOR UPDATE", channelID).Scan(&n); err != nil {
		return domain.ChatMessage{}, mapErr(err)
	}
	var seq int
	if err := tx.QueryRow(ctx, "SELECT COALESCE(MAX(seq),0)+1 FROM chat_messages WHERE channel_id = $1", channelID).Scan(&seq); err != nil {
		return domain.ChatMessage{}, err
	}
	if mentions == nil {
		mentions = []string{}
	}
	msg := domain.ChatMessage{
		ID: id.New(), ChannelID: channelID, Sub: sub, Body: body, Mentions: mentions,
		AttachmentFileID: attachmentFileID, Seq: seq, CreatedAt: now,
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_messages (id, channel_id, sub, body, mentions, attachment_file_id, seq, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, msg.ID, msg.ChannelID, msg.Sub, msg.Body, msg.Mentions, msg.AttachmentFileID, msg.Seq, msg.CreatedAt)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) ListMessages(channelID string, afterSeq int) ([]domain.ChatMessage, error) {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM channels WHERE id = $1", channelID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id, channel_id, sub, body, mentions, attachment_file_id, seq, created_at
		FROM chat_messages WHERE channel_id = $1 AND seq > $2 ORDER BY seq`, channelID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.Sub, &m.Body, &m.Mentions, &m.AttachmentFileID, &m.Seq, &m.CreatedAt); err != nil {
			return nil, err
		}
		if m.Mentions == nil {
			m.Mentions = []string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateChatTicket(sub, channelID string, readOnly bool, now time.Time) domain.ChatTicket {
	t := domain.ChatTicket{ID: id.New(), Sub: sub, ChannelID: channelID, ReadOnly: readOnly, ExpiresAt: now.Add(domain.CollabTicketTTL)}
	_, _ = s.pool.Exec(context.Background(), `INSERT INTO chat_tickets (id, sub, channel_id, read_only, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		t.ID, t.Sub, t.ChannelID, t.ReadOnly, t.ExpiresAt)
	return t
}

func (s *Store) GetChatTicket(ticketID string) (domain.ChatTicket, error) {
	var t domain.ChatTicket
	err := s.pool.QueryRow(context.Background(), `SELECT id, sub, channel_id, read_only, expires_at FROM chat_tickets WHERE id = $1`, ticketID).
		Scan(&t.ID, &t.Sub, &t.ChannelID, &t.ReadOnly, &t.ExpiresAt)
	if err != nil {
		return domain.ChatTicket{}, mapErr(err)
	}
	return t, nil
}

func (s *Store) SaveFile(f domain.StoredFile) error {
	ctx := context.Background()
	if err := s.workspaceExists(ctx, f.WorkspaceID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO stored_files (id, workspace_id, uploader_sub, purpose, provider, content_type, size, name, view_token, path, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		f.ID, f.WorkspaceID, f.UploaderSub, f.Purpose, f.Provider, f.ContentType, f.Size, f.Name, f.ViewToken, f.Path, f.CreatedAt)
	return err
}

func (s *Store) GetFile(fileID string) (domain.StoredFile, error) {
	var f domain.StoredFile
	err := s.pool.QueryRow(context.Background(), `SELECT id, workspace_id, uploader_sub, purpose, provider, content_type, size, name, view_token, path, created_at
		FROM stored_files WHERE id = $1`, fileID).
		Scan(&f.ID, &f.WorkspaceID, &f.UploaderSub, &f.Purpose, &f.Provider, &f.ContentType, &f.Size, &f.Name, &f.ViewToken, &f.Path, &f.CreatedAt)
	if err != nil {
		return domain.StoredFile{}, mapErr(err)
	}
	return f, nil
}

func (s *Store) AttachPageFile(pageID, fileID string) error {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM pages WHERE id = $1", pageID).Scan(&n); err != nil {
		return mapErr(err)
	}
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM stored_files WHERE id = $1", fileID).Scan(&n); err != nil {
		return mapErr(err)
	}
	_, err := s.pool.Exec(ctx, "INSERT INTO page_files (page_id, file_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", pageID, fileID)
	return err
}

func (s *Store) ListPageFiles(pageID string) ([]domain.StoredFile, error) {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM pages WHERE id = $1", pageID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT f.id, f.workspace_id, f.uploader_sub, f.purpose, f.provider, f.content_type, f.size, f.name, f.view_token, f.path, f.created_at
		FROM page_files pf JOIN stored_files f ON f.id = pf.file_id WHERE pf.page_id = $1`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StoredFile
	for rows.Next() {
		var f domain.StoredFile
		if err := rows.Scan(&f.ID, &f.WorkspaceID, &f.UploaderSub, &f.Purpose, &f.Provider, &f.ContentType, &f.Size, &f.Name, &f.ViewToken, &f.Path, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CreateSprint(boardID, name string, startAt, endAt, now time.Time) (domain.Sprint, error) {
	ctx := context.Background()
	var wsID string
	if err := s.pool.QueryRow(ctx, "SELECT workspace_id FROM boards WHERE id = $1", boardID).Scan(&wsID); err != nil {
		return domain.Sprint{}, mapErr(err)
	}
	sp := domain.Sprint{ID: id.New(), BoardID: boardID, WorkspaceID: wsID, Name: name, StartAt: startAt.UTC(), EndAt: endAt.UTC(), CreatedAt: now}
	_, err := s.pool.Exec(ctx, `INSERT INTO sprints (id, board_id, workspace_id, name, start_at, end_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sp.ID, sp.BoardID, sp.WorkspaceID, sp.Name, sp.StartAt, sp.EndAt, sp.CreatedAt)
	if err != nil {
		return domain.Sprint{}, err
	}
	return sp, nil
}

func (s *Store) ListSprints(boardID string) ([]domain.Sprint, error) {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM boards WHERE id = $1", boardID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id, board_id, workspace_id, name, start_at, end_at, created_at FROM sprints WHERE board_id = $1 ORDER BY start_at, id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Sprint
	for rows.Next() {
		var sp domain.Sprint
		if err := rows.Scan(&sp.ID, &sp.BoardID, &sp.WorkspaceID, &sp.Name, &sp.StartAt, &sp.EndAt, &sp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Store) GetSprint(sprintID string) (domain.Sprint, error) {
	var sp domain.Sprint
	err := s.pool.QueryRow(context.Background(), `SELECT id, board_id, workspace_id, name, start_at, end_at, created_at FROM sprints WHERE id = $1`, sprintID).
		Scan(&sp.ID, &sp.BoardID, &sp.WorkspaceID, &sp.Name, &sp.StartAt, &sp.EndAt, &sp.CreatedAt)
	if err != nil {
		return domain.Sprint{}, mapErr(err)
	}
	return sp, nil
}

func (s *Store) UpdateSprint(sprintID, name string, startAt, endAt *time.Time, now time.Time) (domain.Sprint, error) {
	_ = now
	sp, err := s.GetSprint(sprintID)
	if err != nil {
		return domain.Sprint{}, err
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
	_, err = s.pool.Exec(context.Background(), `UPDATE sprints SET name=$2, start_at=$3, end_at=$4 WHERE id=$1`, sprintID, sp.Name, sp.StartAt, sp.EndAt)
	if err != nil {
		return domain.Sprint{}, err
	}
	return sp, nil
}

func (s *Store) DeleteSprint(sprintID string) error {
	ctx := context.Background()
	cmd, err := s.pool.Exec(ctx, "DELETE FROM sprints WHERE id = $1", sprintID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, "UPDATE cards SET sprint_id = '' WHERE sprint_id = $1", sprintID)
	return err
}

func (s *Store) AppendPageVersionIfChanged(pageID, title, body, sub string, now time.Time) (domain.PageVersion, bool, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PageVersion{}, false, err
	}
	defer tx.Rollback(ctx)
	var n int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM pages WHERE id = $1 FOR UPDATE", pageID).Scan(&n); err != nil {
		return domain.PageVersion{}, false, mapErr(err)
	}
	var last domain.PageVersion
	err = tx.QueryRow(ctx, `SELECT page_id, number, title, body, sub, created_at FROM page_versions WHERE page_id = $1 ORDER BY number DESC LIMIT 1`, pageID).
		Scan(&last.PageID, &last.Number, &last.Title, &last.Body, &last.Sub, &last.CreatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.PageVersion{}, false, err
	}
	if err == nil && last.Title == title && last.Body == body {
		if err := tx.Commit(ctx); err != nil {
			return domain.PageVersion{}, false, err
		}
		return last, false, nil
	}
	num := 1
	if err == nil {
		num = last.Number + 1
	}
	v := domain.PageVersion{PageID: pageID, Number: num, Title: title, Body: body, Sub: sub, CreatedAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO page_versions (page_id, number, title, body, sub, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		v.PageID, v.Number, v.Title, v.Body, v.Sub, v.CreatedAt); err != nil {
		return domain.PageVersion{}, false, err
	}
	if v.Number > domain.MaxPageVersions {
		cutoff := v.Number - domain.MaxPageVersions
		if _, err := tx.Exec(ctx, `DELETE FROM page_versions WHERE page_id = $1 AND number <= $2`, pageID, cutoff); err != nil {
			return domain.PageVersion{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PageVersion{}, false, err
	}
	return v, true, nil
}

func (s *Store) ListPageVersions(pageID string) ([]domain.PageVersion, error) {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM pages WHERE id = $1", pageID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT page_id, number, title, body, sub, created_at FROM page_versions WHERE page_id = $1 ORDER BY number`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PageVersion
	for rows.Next() {
		var v domain.PageVersion
		if err := rows.Scan(&v.PageID, &v.Number, &v.Title, &v.Body, &v.Sub, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetPageVersion(pageID string, number int) (domain.PageVersion, error) {
	ctx := context.Background()
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT 1 FROM pages WHERE id = $1", pageID).Scan(&n); err != nil {
		return domain.PageVersion{}, mapErr(err)
	}
	var v domain.PageVersion
	err := s.pool.QueryRow(ctx, `SELECT page_id, number, title, body, sub, created_at FROM page_versions WHERE page_id = $1 AND number = $2`, pageID, number).
		Scan(&v.PageID, &v.Number, &v.Title, &v.Body, &v.Sub, &v.CreatedAt)
	if err != nil {
		return domain.PageVersion{}, mapErr(err)
	}
	return v, nil
}
