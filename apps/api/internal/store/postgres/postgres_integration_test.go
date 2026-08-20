package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/portfolio/pf-workspace/api/internal/domain"
)

func TestPostgresStorePersistsWorkspaceBoardAndMessages(t *testing.T) {
	url := os.Getenv("WORKSPACE_DATABASE_URL")
	if url == "" {
		t.Skip("WORKSPACE_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skip(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skip(err)
	}
	pool.Close()

	store, err := Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	ws, err := store.CreateWorkspace("pg-demo", "owner-pg", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.ID) != 26 {
		t.Fatalf("ulid length: %d", len(ws.ID))
	}
	board, cols, err := store.CreateBoard(ws.ID, "Board", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("columns: %d", len(cols))
	}
	card, err := store.CreateCard(cols[0].ID, "Task", "desc", now)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := store.MoveCard(card.ID, cols[1].ID, 0, card.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.MoveCard(card.ID, cols[2].ID, 0, card.Version, now)
	if err != domain.ErrConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
	ch, err := store.CreateChannel(ws.ID, "general", now)
	if err != nil {
		t.Fatal(err)
	}
	m1, err := store.AppendMessage(ch.ID, "owner-pg", "hi", nil, "", now)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := store.AppendMessage(ch.ID, "owner-pg", "there", []string{"owner-pg"}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if m1.Seq != 1 || m2.Seq != 2 {
		t.Fatalf("seq %d %d", m1.Seq, m2.Seq)
	}
	found, err := store.GetCard(moved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ColumnID != cols[1].ID || found.Version != 2 {
		t.Fatalf("card %#v", found)
	}
	_ = board
}
