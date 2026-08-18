package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	req.Header.Set("Content-Type", "application/json")
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
