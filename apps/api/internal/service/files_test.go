package service

import (
	"strings"
	"testing"

	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/store/memory"
)

func TestSaveLocalFileRejectsOversize(t *testing.T) {
	svc := New(memory.New())
	ws, err := svc.CreateWorkspace("owner-1", "Files")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveLocalFile("owner-1", ws.ID, domain.PurposeWiki, "big.bin", "application/octet-stream", domain.MaxUploadBytes+1, strings.NewReader("x"))
	if err != domain.ErrTooLarge {
		t.Fatalf("got %v", err)
	}
}
