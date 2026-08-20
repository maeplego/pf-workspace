package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/auth"
	"github.com/portfolio/pf-workspace/api/internal/service"
	"github.com/portfolio/pf-workspace/api/internal/store/memory"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := memory.New()
	svc := service.New(store)
	mw := auth.New(true, "", "", "")
	srv := New(svc, "", "test-internal", nil)
	return httptest.NewServer(srv.Routes(mw))
}

func authReq(t *testing.T, method, url string, body any, sub string) *http.Request {
	return authReqDev(t, method, url, body, sub, "")
}

func authReqDev(t *testing.T, method, url string, body any, sub, email string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-User-Sub", sub)
	if email != "" {
		req.Header.Set("X-Dev-User-Email", email)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func authReqDevOrg(t *testing.T, method, url string, body any, sub, email, org string) *http.Request {
	req := authReqDev(t, method, url, body, sub, email)
	if org != "" {
		req.Header.Set("X-Dev-User-Org", org)
	}
	return req
}

func TestWorkspaceAndKanbanFlow(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Demo Team"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create workspace: %d", res.StatusCode)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": "Sprint 1"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create board: %d", res.StatusCode)
	}
	var board struct {
		ID      string `json:"id"`
		Columns []struct {
			ID string `json:"id"`
		} `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(board.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(board.Columns))
	}

	colID := board.Columns[0].ID
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/columns/"+colID+"/cards", map[string]string{"title": "Task A"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create card: %d", res.StatusCode)
	}
	var card struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&card); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	targetCol := board.Columns[1].ID
	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/cards/"+card.ID+"/move", map[string]any{
		"columnId": targetCol,
		"position": 0,
		"version":  card.Version,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("move card: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/cards/"+card.ID+"/move", map[string]any{
		"columnId": targetCol,
		"position": 0,
		"version":  1,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/boards/"+board.ID, nil, "guest-unknown"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest unknown: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{
		"sub":  "guest-1",
		"role": "guest",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/cards/"+card.ID+"/move", map[string]any{
		"columnId": targetCol,
		"position": 0,
		"version":  2,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest move forbidden: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestColumnCRUD(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Cols"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": "Board"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var board struct {
		ID      string `json:"id"`
		Columns []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/boards/"+board.ID+"/columns", map[string]string{"name": "Review"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create column: %d", res.StatusCode)
	}
	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if created.Name != "Review" {
		t.Fatalf("name=%s", created.Name)
	}

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/columns/"+created.ID, map[string]string{"name": "QA"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rename: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	ids := []string{board.Columns[2].ID, board.Columns[0].ID, board.Columns[1].ID, created.ID}
	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/boards/"+board.ID+"/columns/reorder", map[string]any{"columnIds": ids}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("reorder: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/columns/"+board.Columns[0].ID+"/cards", map[string]string{"title": "Keep"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodDelete, ts.URL+"/v1/columns/"+board.Columns[0].ID, nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-empty delete want 400 got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodDelete, ts.URL+"/v1/columns/"+created.ID, nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete empty: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestWikiTreeAndGuestVisibility(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Docs"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title":  "Root",
		"body":   "# Hello",
		"status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create page: %d", res.StatusCode)
	}
	var root struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&root); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title":    "Child",
		"parentId": root.ID,
		"body":     "nested",
		"status":   "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create child: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title":  "Secret",
		"body":   "draft",
		"status": "draft",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var draft struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{
		"sub": "guest-1", "role": "guest",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/pages/tree", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("tree: %d", res.StatusCode)
	}
	var tree struct {
		Tree []struct {
			Title    string `json:"title"`
			Children []struct {
				Title string `json:"title"`
			} `json:"children"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(tree.Tree) != 1 || tree.Tree[0].Title != "Root" || len(tree.Tree[0].Children) != 1 {
		t.Fatalf("guest tree %+v", tree.Tree)
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/pages/"+draft.ID, nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("guest draft get: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/pages/"+root.ID, map[string]any{
		"body": "hack", "version": 1,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest patch: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/pages/"+root.ID, map[string]any{
		"body": "updated", "version": 1,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("owner patch: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/pages/"+root.ID, map[string]any{
		"body": "stale", "version": 1,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale version: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/pages/"+root.ID, map[string]any{
		"parentId": root.ID, "version": 2,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("cycle: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestHealth(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestCollabTicketsAndDocuments(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Demo Team"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Root", "body": "hello collab", "status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create page: %d", res.StatusCode)
	}
	var page struct {
		ID               string `json:"id"`
		CollabDocumentID string `json:"collabDocumentId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Secret", "body": "draft", "status": "draft",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var draft struct {
		CollabDocumentID string `json:"collabDocumentId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{
		"sub": "guest-1", "role": "guest",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/collab-tickets", map[string]string{
		"collabDocumentId": "../etc/passwd",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("path room: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/collab-tickets", map[string]string{
		"collabDocumentId": page.CollabDocumentID,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("owner ticket: %d", res.StatusCode)
	}
	var ownerTicket struct {
		Ticket           string `json:"ticket"`
		ReadOnly         bool   `json:"readOnly"`
		CollabDocumentID string `json:"collabDocumentId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ownerTicket); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if ownerTicket.ReadOnly || ownerTicket.Ticket == "" {
		t.Fatalf("owner ticket %+v", ownerTicket)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/collab-tickets", map[string]string{
		"collabDocumentId": page.CollabDocumentID,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("guest published ticket: %d", res.StatusCode)
	}
	var guestTicket struct {
		ReadOnly bool `json:"readOnly"`
	}
	if err := json.NewDecoder(res.Body).Decode(&guestTicket); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if !guestTicket.ReadOnly {
		t.Fatal("guest should be read-only")
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/collab-tickets", map[string]string{
		"collabDocumentId": draft.CollabDocumentID,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("guest draft ticket: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	authBody, _ := json.Marshal(map[string]string{"ticket": ownerTicket.Ticket, "documentName": page.CollabDocumentID})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/internal/v1/collab/authorize", bytes.NewReader(authBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("authorize no token: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	req, err = http.NewRequest(http.MethodPost, ts.URL+"/internal/v1/collab/authorize", bytes.NewReader(authBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-internal")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authorize ok: %d", res.StatusCode)
	}
	var authz struct {
		Sub      string `json:"sub"`
		ReadOnly bool   `json:"readOnly"`
	}
	if err := json.NewDecoder(res.Body).Decode(&authz); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if authz.Sub != "owner-1" || authz.ReadOnly {
		t.Fatalf("authz %+v", authz)
	}

	mismatch, _ := json.Marshal(map[string]string{"ticket": ownerTicket.Ticket, "documentName": draft.CollabDocumentID})
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/internal/v1/collab/authorize", bytes.NewReader(mismatch))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-internal")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("mismatch: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/documents", map[string]string{
		"title": "Notes", "body": "seed",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create document: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/documents", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("guest list docs: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/documents", map[string]string{
		"title": "Nope",
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest create doc: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	plain, _ := json.Marshal(map[string]string{"collabDocumentId": page.CollabDocumentID})
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/internal/v1/collab/plaintext", bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-internal")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("plaintext: %d", res.StatusCode)
	}
	var pt struct {
		Plaintext string `json:"plaintext"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pt); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if pt.Plaintext != "hello collab" {
		t.Fatalf("plaintext %q", pt.Plaintext)
	}

	snap, _ := json.Marshal(map[string]string{"collabDocumentId": page.CollabDocumentID, "plaintext": "from crdt"})
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/internal/v1/collab/snapshot", bytes.NewReader(snap))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-internal")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("snapshot: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/pages/"+page.ID, nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var gotPage struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(res.Body).Decode(&gotPage); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if gotPage.Body != "from crdt" {
		t.Fatalf("snapshot body %q", gotPage.Body)
	}
}

func TestChatHistoryAndSeq(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Demo Team"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list channels: %d", res.StatusCode)
	}
	var listed struct {
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(listed.Channels) != 1 || listed.Channels[0].Name != "general" {
		t.Fatalf("default channel %+v", listed.Channels)
	}
	chID := listed.Channels[0].ID

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/messages", map[string]string{"body": "hello"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("post 1: %d", res.StatusCode)
	}
	var m1 struct {
		Seq int `json:"seq"`
	}
	if err := json.NewDecoder(res.Body).Decode(&m1); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if m1.Seq != 1 {
		t.Fatalf("seq %d", m1.Seq)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/messages", map[string]string{"body": "second"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/channels/"+chID+"/messages?afterSeq=1", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var hist struct {
		Messages []struct {
			Seq  int    `json:"seq"`
			Body string `json:"body"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(res.Body).Decode(&hist); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(hist.Messages) != 1 || hist.Messages[0].Seq != 2 || hist.Messages[0].Body != "second" {
		t.Fatalf("afterSeq %+v", hist.Messages)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{
		"sub": "guest-1", "role": "guest",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/messages", map[string]string{"body": "nope"}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest post: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/channels/"+chID+"/messages", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("guest get: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/chat-tickets", map[string]string{"channelId": chID}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("guest ticket: %d", res.StatusCode)
	}
	var gt struct {
		ReadOnly bool `json:"readOnly"`
	}
	if err := json.NewDecoder(res.Body).Decode(&gt); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if !gt.ReadOnly {
		t.Fatal("guest chat ticket should be read-only")
	}
}

func TestChannelUnreadAndMarkRead(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Unread"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "member-1", "role": "member"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var chs struct {
		Channels []struct {
			ID string `json:"id"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&chs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(chs.Channels) == 0 {
		t.Fatal("expected general channel")
	}
	chID := chs.Channels[0].ID

	for _, body := range []string{"one", "two", "three"} {
		res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/messages", map[string]string{"body": body}, "owner-1"))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("post: %d", res.StatusCode)
		}
		_ = res.Body.Close()
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "member-1"))
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Channels []struct {
			ID          string `json:"id"`
			UnreadCount int    `json:"unreadCount"`
			LastReadSeq int    `json:"lastReadSeq"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if listed.Channels[0].UnreadCount != 3 || listed.Channels[0].LastReadSeq != 0 {
		t.Fatalf("want unread=3 lastRead=0 got %+v", listed.Channels[0])
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/read", map[string]int{"lastSeq": 2}, "member-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("mark read: %d", res.StatusCode)
	}
	var marked struct {
		UnreadCount int `json:"unreadCount"`
		LastReadSeq int `json:"lastReadSeq"`
	}
	if err := json.NewDecoder(res.Body).Decode(&marked); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if marked.LastReadSeq != 2 || marked.UnreadCount != 1 {
		t.Fatalf("after mark %+v", marked)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chID+"/read", map[string]int{"lastSeq": 1}, "member-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(res.Body).Decode(&marked); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if marked.LastReadSeq != 2 {
		t.Fatalf("read cursor must be monotonic, got %d", marked.LastReadSeq)
	}
}

func TestSearchACLAndTypes(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Search"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "guest-1", "role": "guest"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Secret Draft", "body": "hidden pineapple", "status": "draft",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var draft struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Public Root", "body": "visible banana", "status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var pub struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pub); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Child of draft", "body": "published under draft pineapple", "parentId": draft.ID, "status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": "Board"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var board struct {
		ID      string `json:"id"`
		Columns []struct {
			ID string `json:"id"`
		} `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/columns/"+board.Columns[0].ID+"/cards", map[string]string{
		"title": "Card pineapple", "description": "desc",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var chs struct {
		Channels []struct {
			ID string `json:"id"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&chs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chs.Channels[0].ID+"/messages", map[string]string{
		"body": "chat about pineapple",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	ownerHits := searchHits(t, client, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q=pineapple", "owner-1")
	if !hasType(ownerHits, "page") || !hasType(ownerHits, "card") || !hasType(ownerHits, "message") {
		t.Fatalf("owner hits %+v", ownerHits)
	}
	foundDraft := false
	for _, h := range ownerHits {
		if h.Type == "page" && h.ID == draft.ID {
			foundDraft = true
		}
	}
	if !foundDraft {
		t.Fatal("owner should hit draft page")
	}

	guestHits := searchHits(t, client, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q=pineapple", "guest-1")
	for _, h := range guestHits {
		if h.Type == "page" && (h.ID == draft.ID || strings.Contains(strings.ToLower(h.Title), "child")) {
			t.Fatalf("guest should not see draft or draft-descendant: %+v", h)
		}
	}
	if !hasType(guestHits, "card") || !hasType(guestHits, "message") {
		t.Fatalf("guest should still see card and message: %+v", guestHits)
	}

	pubHits := searchHits(t, client, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q=banana", "guest-1")
	if len(pubHits) != 1 || pubHits[0].Type != "page" || pubHits[0].ID != pub.ID {
		t.Fatalf("guest public %+v", pubHits)
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q=", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty q: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q=x", nil, "stranger"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestMentionsOnPost(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Mentions"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "demo-user-b", "role": "member"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var chs struct {
		Channels []struct {
			ID string `json:"id"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&chs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chs.Channels[0].ID+"/messages", map[string]string{
		"body": "hi @demo-user-b and @not-a-member",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d", res.StatusCode)
	}
	var msg struct {
		Mentions []string `json:"mentions"`
		Seq      int      `json:"seq"`
	}
	if err := json.NewDecoder(res.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if msg.Seq != 1 || len(msg.Mentions) != 1 || msg.Mentions[0] != "demo-user-b" {
		t.Fatalf("mentions %+v", msg)
	}
}

func TestAttachmentsLocalAndGuestForbidden(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Files"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "guest-1", "role": "guest"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title": "Img", "body": "see below", "status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	wikiFile := uploadFile(t, client, ts.URL, ws.ID, "wiki", "owner-1", "pic.png", "image/png", []byte("png-bytes"))
	guestUpload := uploadFileRaw(t, client, ts.URL, ws.ID, "wiki", "guest-1", "no.png", "image/png", []byte("x"))
	if guestUpload.StatusCode != http.StatusForbidden {
		t.Fatalf("guest upload: %d", guestUpload.StatusCode)
	}
	_ = guestUpload.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/pages/"+page.ID+"/attachments", map[string]string{"fileId": wikiFile.ID}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("attach: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/pages/"+page.ID+"/attachments", map[string]string{"fileId": wikiFile.ID}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest attach: %d", res.StatusCode)
	}
	_ = res.Body.Close()

	contentURL := fileContentURL(t, ts.URL, wikiFile.URL)
	content, err := client.Get(contentURL)
	if err != nil {
		t.Fatal(err)
	}
	if content.StatusCode != http.StatusOK {
		t.Fatalf("content: %d", content.StatusCode)
	}
	got, _ := io.ReadAll(content.Body)
	_ = content.Body.Close()
	if string(got) != "png-bytes" {
		t.Fatalf("bytes %q", got)
	}

	bad, err := client.Get(strings.Split(contentURL, "?")[0] + "?t=nope")
	if err != nil {
		t.Fatal(err)
	}
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", bad.StatusCode)
	}
	_ = bad.Body.Close()

	chatFile := uploadFile(t, client, ts.URL, ws.ID, "chat", "owner-1", "shot.png", "image/png", []byte("chat-img"))
	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var chs struct {
		Channels []struct {
			ID string `json:"id"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(res.Body).Decode(&chs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/channels/"+chs.Channels[0].ID+"/messages", map[string]string{
		"body": "with file", "attachmentFileId": chatFile.ID,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("chat attach: %d", res.StatusCode)
	}
	var posted struct {
		AttachmentFileID string `json:"attachmentFileId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&posted); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if posted.AttachmentFileID != chatFile.ID {
		t.Fatalf("posted %+v", posted)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/uploads/link", map[string]any{
		"workspaceId": ws.ID, "purpose": "wiki", "fileId": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("link without p03: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

type searchHit struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

func searchHits(t *testing.T, client *http.Client, url, sub string) []searchHit {
	t.Helper()
	res, err := client.Do(authReq(t, http.MethodGet, url, nil, sub))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search %s: %d", sub, res.StatusCode)
	}
	var out struct {
		Hits []searchHit `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Hits
}

func hasType(hits []searchHit, typ string) bool {
	for _, h := range hits {
		if h.Type == typ {
			return true
		}
	}
	return false
}

func fileContentURL(t *testing.T, base, advertised string) string {
	t.Helper()
	u, err := url.Parse(advertised)
	if err != nil {
		t.Fatal(err)
	}
	return base + u.RequestURI()
}

type uploadedFile struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func uploadFile(t *testing.T, client *http.Client, base, wsID, purpose, sub, name, ctype string, data []byte) uploadedFile {
	t.Helper()
	res := uploadFileRaw(t, client, base, wsID, purpose, sub, name, ctype, data)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload %s: %d", purpose, res.StatusCode)
	}
	var f uploadedFile
	if err := json.NewDecoder(res.Body).Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f
}

func uploadFileRaw(t *testing.T, client *http.Client, base, wsID, purpose, sub, name, ctype string, data []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("workspaceId", wsID)
	_ = w.WriteField("purpose", purpose)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/uploads", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-User-Sub", sub)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSprintBurndownAndWikiHistory(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Sprint WS"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("ws %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "guest-1", "role": "guest"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("guest %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": "Main"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var board struct {
		ID      string `json:"id"`
		Columns []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	todoID := board.Columns[0].ID
	doneID := board.Columns[2].ID

	now := time.Now().UTC()
	startAt := now.Add(-24 * time.Hour).Format(time.RFC3339)
	endAt := now.Add(24 * time.Hour).Format(time.RFC3339)
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/boards/"+board.ID+"/sprints", map[string]string{
		"name":    "Sprint 7",
		"startAt": startAt,
		"endAt":   endAt,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("sprint create %d", res.StatusCode)
	}
	var sprint struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&sprint); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/boards/"+board.ID+"/sprints", map[string]string{
		"name":    "bad",
		"startAt": startAt,
		"endAt":   endAt,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest sprint %d", res.StatusCode)
	}
	_ = res.Body.Close()

	var cards []struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	for i := 0; i < 2; i++ {
		res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/columns/"+todoID+"/cards", map[string]string{"title": "C" + string(rune('A'+i))}, "owner-1"))
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		}
		if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/cards/"+c.ID, map[string]any{
			"title":    "C" + string(rune('A'+i)),
			"version":  c.Version,
			"sprintId": sprint.ID,
		}, "owner-1"))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("assign %d", res.StatusCode)
		}
		if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		cards = append(cards, c)
	}

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/cards/"+cards[0].ID+"/move", map[string]any{
		"columnId": doneID,
		"position": 0,
		"version":  cards[0].Version,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("done move %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/sprints/"+sprint.ID+"/burndown", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("burndown guest %d", res.StatusCode)
	}
	var bd struct {
		Unit   string `json:"unit"`
		Points []struct {
			Date      string `json:"date"`
			Remaining int    `json:"remaining"`
		} `json:"points"`
	}
	if err := json.NewDecoder(res.Body).Decode(&bd); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if bd.Unit != "cards" || len(bd.Points) < 2 {
		t.Fatalf("burndown %+v", bd)
	}
	last := bd.Points[len(bd.Points)-2] // today is typically the middle of a 3-day window
	today := now.Format("2006-01-02")
	foundToday := false
	for _, p := range bd.Points {
		if p.Date == today {
			foundToday = true
			if p.Remaining != 1 {
				t.Fatalf("today remaining %d", p.Remaining)
			}
		}
	}
	if !foundToday {
		t.Fatalf("today %s not in points, last=%+v", today, last)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title":  "History",
		"body":   "alpha",
		"status": "published",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/pages/"+page.ID, map[string]any{
		"body":    "alpha\nbeta",
		"version": page.Version,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/pages/"+page.ID+"/versions", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("versions guest %d", res.StatusCode)
	}
	var vers struct {
		Versions []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(res.Body).Decode(&vers); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(vers.Versions) != 2 {
		t.Fatalf("versions %d", len(vers.Versions))
	}
	if vers.Versions[0].Body != "" {
		t.Fatal("list should omit body")
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/pages/"+page.ID+"/diff?from=1&to=2", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("diff %d", res.StatusCode)
	}
	var diff struct {
		Lines []struct {
			Op   string `json:"op"`
			Text string `json:"text"`
		} `json:"lines"`
	}
	if err := json.NewDecoder(res.Body).Decode(&diff); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	hasInsert := false
	for _, ln := range diff.Lines {
		if ln.Op == "insert" && ln.Text == "beta" {
			hasInsert = true
		}
	}
	if !hasInsert {
		t.Fatalf("diff %+v", diff.Lines)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/pages/"+page.ID+"/restore", map[string]any{
		"number":  1,
		"version": page.Version,
	}, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("guest restore %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/pages/"+page.ID+"/restore", map[string]any{
		"number":  1,
		"version": page.Version,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("restore %d", res.StatusCode)
	}
	var restored struct {
		Body    string `json:"body"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&restored); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if restored.Body != "alpha" {
		t.Fatalf("restored body %q", restored.Body)
	}

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/pages", map[string]any{
		"title":  "Secret",
		"body":   "hidden",
		"status": "draft",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var draft struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/pages/"+draft.ID+"/versions", nil, "guest-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("draft history %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestEmptyBoardNameRejected(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()
	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "EmptyBoard"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": ""}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty board name: %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestInvitationJoinFlow(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Invite WS"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations", map[string]any{
		"role": "guest", "maxUses": 1, "ttlHours": 24,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create invite %d", res.StatusCode)
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if created.Token == "" {
		t.Fatal("empty token")
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/invitations/"+created.Token, nil, "new-user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "new-user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("accept %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/members", nil, "new-user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("member list %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "other-user"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("second accept should fail %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/audit-events", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("audit list %d", res.StatusCode)
	}
	var audits struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&audits); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(audits.Events) < 2 {
		t.Fatalf("expected invitation audit events, got %+v", audits.Events)
	}
}

func TestInvitationEmailBinding(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Email WS"}, "owner-1", "owner@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations", map[string]any{
		"role": "member", "maxUses": 1, "ttlHours": 24, "invitedEmail": "guest@example.com",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create invite %d", res.StatusCode)
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "wrong-user", "other@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong email accept should be forbidden, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "guest-user", "guest@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("matching email accept %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestWorkspaceOrgIDFromAuth(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]string{"name": "Org WS"})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/workspaces", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-User-Sub", "owner-1")
	req.Header.Set("X-Dev-User-Org", "org-demo-1")
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create workspace %d", res.StatusCode)
	}
	var ws struct {
		OrgID string `json:"orgId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if ws.OrgID != "org-demo-1" {
		t.Fatalf("orgId = %q", ws.OrgID)
	}
}

func TestInvitationRevoke(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Revoke WS"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations", map[string]any{
		"role": "member", "maxUses": 1, "ttlHours": 24,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Token      string `json:"token"`
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations/"+created.Invitation.ID+"/revoke", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("revoke %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/invitations/"+created.Token, nil, "new-user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("preview after revoke should 404, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "new-user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("accept after revoke should 404, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/audit-events", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var audits struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&audits); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	found := false
	for _, ev := range audits.Events {
		if ev.Type == "workspace.invitation.revoked" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected workspace.invitation.revoked audit event")
	}
}

func TestInvitationResend(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Resend WS"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations", map[string]any{
		"role": "member", "maxUses": 1, "ttlHours": 24, "invitedEmail": "guest@example.com",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Token      string `json:"token"`
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations/"+created.Invitation.ID+"/resend", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("resend %d", res.StatusCode)
	}
	var resent struct {
		Token      string `json:"token"`
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resent); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if resent.Token == "" || resent.Token == created.Token {
		t.Fatal("resend should issue a new token")
	}
	if resent.Invitation.ID == "" || resent.Invitation.ID == created.Invitation.ID {
		t.Fatal("resend should create a new invitation row")
	}

	res, err = client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "guest-legacy", "guest@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("old token accept %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/invitations/"+resent.Token+"/accept", map[string]string{}, "guest-new", "guest@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("resent token accept %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/audit-events", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var audits struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&audits); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	found := false
	for _, ev := range audits.Events {
		if ev.Type == "workspace.invitation.resent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected workspace.invitation.resent audit event")
	}
}

func TestInvitationPolicyUpdate(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Policy WS"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations", map[string]any{
		"role": "member", "maxUses": 1, "ttlHours": 24,
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Token      string `json:"token"`
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/workspaces/"+ws.ID+"/invitations/"+created.Invitation.ID, map[string]any{
		"role": "guest", "maxUses": 3, "ttlHours": 48, "invitedEmail": "bound@example.com",
	}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("policy update %d", res.StatusCode)
	}
	var updated struct {
		ID           string `json:"id"`
		Role         string `json:"role"`
		MaxUses      int    `json:"maxUses"`
		InvitedEmail string `json:"invitedEmail"`
	}
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if updated.ID != created.Invitation.ID || updated.Role != "guest" || updated.MaxUses != 3 || updated.InvitedEmail != "bound@example.com" {
		t.Fatalf("unexpected update %#v", updated)
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/invitations/"+created.Token, nil, "new-user"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("same token preview after policy update %d", res.StatusCode)
	}
	var preview struct {
		Invitation struct {
			Role         string `json:"role"`
			MaxUses      int    `json:"maxUses"`
			InvitedEmail string `json:"invitedEmail"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if preview.Invitation.Role != "guest" || preview.Invitation.MaxUses != 3 || preview.Invitation.InvitedEmail != "bound@example.com" {
		t.Fatalf("preview %#v", preview.Invitation)
	}

	res, err = client.Do(authReqDev(t, http.MethodPost, ts.URL+"/v1/invitations/"+created.Token+"/accept", map[string]string{}, "guest-1", "bound@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("accept after policy update %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/members/guest-1", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var member struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(res.Body).Decode(&member); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if member.Role != "guest" {
		t.Fatalf("member role %s", member.Role)
	}

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/audit-events", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var audits struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&audits); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	found := false
	for _, ev := range audits.Events {
		if ev.Type == "workspace.invitation.policy_updated" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected workspace.invitation.policy_updated audit event")
	}
}

func TestMemberRoleUpdateRemoveAndLeave(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Members"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "member-1", "role": "member"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("add member status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/workspaces/"+ws.ID+"/members/member-1", map[string]string{"role": "guest"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("patch role status %d", res.StatusCode)
	}
	var patched struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(res.Body).Decode(&patched); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if patched.Role != "guest" {
		t.Fatalf("role=%s", patched.Role)
	}

	res, err = client.Do(authReq(t, http.MethodPatch, ts.URL+"/v1/workspaces/"+ws.ID+"/members/member-1", map[string]string{"role": "owner"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("promote to owner want 400 got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodDelete, ts.URL+"/v1/workspaces/"+ws.ID+"/members/owner-1", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("self-remove via DELETE want 400 got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/leave", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("sole owner leave want 403 got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodDelete, ts.URL+"/v1/workspaces/"+ws.ID+"/members/member-1", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("remove member status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "member-2", "role": "member"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("re-add member status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/leave", nil, "member-2"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("leave status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/audit-events", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("audit status %d", res.StatusCode)
	}
	var audit struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&audit); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	want := map[string]bool{
		"workspace.member.role_updated": false,
		"workspace.member.removed":      false,
		"workspace.member.left":         false,
	}
	for _, ev := range audit.Events {
		if _, ok := want[ev.Type]; ok {
			want[ev.Type] = true
		}
	}
	for typ, ok := range want {
		if !ok {
			t.Fatalf("missing audit %s in %+v", typ, audit.Events)
		}
	}
}

func TestOrgMembersDevFallback(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()

	res, err := client.Do(authReqDevOrg(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Org WS"}, "owner-org", "owner@example.com", "org-demo"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReqDevOrg(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/members", map[string]string{"sub": "guest-org", "role": "guest"}, "owner-org", "owner@example.com", "org-demo"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("add member status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReqDevOrg(t, http.MethodGet, ts.URL+"/v1/org-members?q=guest", nil, "owner-org", "owner@example.com", "org-demo"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("org-members status %d", res.StatusCode)
	}
	var payload struct {
		Members []struct {
			Sub string `json:"sub"`
		} `json:"members"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	found := false
	for _, m := range payload.Members {
		if m.Sub == "guest-org" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected guest-org in %+v", payload.Members)
	}
}

func TestArchiveTrashAndChannelKept(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	client := ts.Client()
	res, err := client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces", map[string]string{"name": "Keep"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", map[string]string{"name": "Alpha board"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var board struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/boards/"+board.ID+"/archive", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("archive board %d", res.StatusCode)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/boards", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Boards []struct {
			ID string `json:"id"`
		}
		ArchivedBoards []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(listed.Boards) != 0 || len(listed.ArchivedBoards) != 1 || listed.ArchivedBoards[0].ID != board.ID {
		t.Fatalf("archived list %+v", listed)
	}
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/boards/"+board.ID+"/unarchive", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("unarchive %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/workspaces/"+ws.ID+"/documents", map[string]string{"title": "Doc A"}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/documents/"+doc.ID+"/trash", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("trash %d", res.StatusCode)
	}
	_ = res.Body.Close()
	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/documents", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var docs struct {
		Documents []struct {
			ID string `json:"id"`
		}
		Trashed []struct {
			ID string `json:"id"`
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&docs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(docs.Documents) != 0 || len(docs.Trashed) != 1 {
		t.Fatalf("trash list %+v", docs)
	}
	res, err = client.Do(authReq(t, http.MethodPost, ts.URL+"/v1/documents/"+doc.ID+"/untrash", map[string]string{}, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("untrash %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/channels", nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var chs struct {
		Channels []struct {
			ID string `json:"id"`
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&chs); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if len(chs.Channels) == 0 {
		t.Fatal("expected default channel")
	}
	res, err = client.Do(authReq(t, http.MethodDelete, ts.URL+"/v1/channels/"+chs.Channels[0].ID, nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode == http.StatusNoContent {
		t.Fatal("chat channel must not be hard-deleted")
	}
	_ = res.Body.Close()

	res, err = client.Do(authReq(t, http.MethodGet, ts.URL+"/v1/workspaces/"+ws.ID+"/search?q="+url.QueryEscape("Alpha board"), nil, "owner-1"))
	if err != nil {
		t.Fatal(err)
	}
	var hits struct {
		Hits []struct {
			Type    string `json:"type"`
			Title   string `json:"title"`
			Context string `json:"context"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&hits); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	found := false
	for _, h := range hits.Hits {
		if h.Type == "board" && strings.Contains(h.Title, "Alpha") {
			found = true
		}
	}
	if !found {
		t.Fatalf("board name search %+v", hits.Hits)
	}
}
