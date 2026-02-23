package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const testUserID = "test-user-123"

func makeToken(userID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	signed, _ := token.SignedString([]byte("test-secret"))
	return signed
}

func makeRequest(method, path, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: method,
		Path:       path,
		Body:       body,
		Headers: map[string]string{
			"Authorization": "Bearer " + makeToken(testUserID),
			"Content-Type":  "application/json",
		},
		PathParameters: map[string]string{},
	}
}

func TestNoteHandler_CreateAndList(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create a note
	req := makeRequest("POST", "/notes", `{"name":"test.md","content":"# Hello"}`)
	resp, err := h.CreateNote(ctx, req)
	if err != nil {
		t.Fatalf("CreateNote returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, resp.Body)
	}

	var created adapter.FileMetadata
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatalf("Failed to unmarshal created note: %v", err)
	}
	if created.Name != "test.md" {
		t.Errorf("Expected name 'test.md', got '%s'", created.Name)
	}
	if created.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if created.ETag == "" {
		t.Error("Expected non-empty ETag")
	}

	// List notes
	listReq := makeRequest("GET", "/notes", "")
	listResp, err := h.ListNotes(ctx, listReq)
	if err != nil {
		t.Fatalf("ListNotes returned error: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", listResp.StatusCode)
	}

	var notes []adapter.FileMetadata
	if err := json.Unmarshal([]byte(listResp.Body), &notes); err != nil {
		t.Fatalf("Failed to unmarshal notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("Expected 1 note, got %d", len(notes))
	}
	if notes[0].ID != created.ID {
		t.Errorf("Note ID mismatch: got %s, want %s", notes[0].ID, created.ID)
	}
}

func TestNoteHandler_GetNote(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"get-test.md","content":"body content"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Get
	getReq := makeRequest("GET", "/notes/"+created.ID, "")
	getReq.PathParameters["id"] = created.ID
	getResp, err := h.GetNote(ctx, getReq)
	if err != nil {
		t.Fatalf("GetNote returned error: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", getResp.StatusCode, getResp.Body)
	}

	var note struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Content string `json:"content"`
		ETag    string `json:"etag"`
	}
	json.Unmarshal([]byte(getResp.Body), &note)
	if note.Content != "body content" {
		t.Errorf("Expected content 'body content', got '%s'", note.Content)
	}
	if note.ETag == "" {
		t.Error("Expected non-empty ETag in GetNote response")
	}
}

func TestNoteHandler_UpdateNote(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"update-test.md","content":"v1"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Update with matching ETag
	updateReq := makeRequest("PUT", "/notes/"+created.ID, `{"content":"v2"}`)
	updateReq.PathParameters["id"] = created.ID
	updateReq.Headers["If-Match"] = created.ETag
	updateResp, err := h.UpdateNote(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdateNote returned error: %v", err)
	}
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", updateResp.StatusCode, updateResp.Body)
	}

	var updated adapter.FileMetadata
	json.Unmarshal([]byte(updateResp.Body), &updated)
	if updated.ETag == created.ETag {
		t.Error("Expected ETag to change after update")
	}

	// Verify content changed
	getReq := makeRequest("GET", "/notes/"+created.ID, "")
	getReq.PathParameters["id"] = created.ID
	getResp, _ := h.GetNote(ctx, getReq)
	var note struct {
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(getResp.Body), &note)
	if note.Content != "v2" {
		t.Errorf("Expected content 'v2', got '%s'", note.Content)
	}
}

func TestNoteHandler_UpdateNote_Conflict(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"conflict.md","content":"original"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// First update (succeeds, changes ETag)
	updateReq1 := makeRequest("PUT", "/notes/"+created.ID, `{"content":"updated-by-user-a"}`)
	updateReq1.PathParameters["id"] = created.ID
	updateReq1.Headers["If-Match"] = created.ETag
	resp1, _ := h.UpdateNote(ctx, updateReq1)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("First update should succeed, got %d", resp1.StatusCode)
	}

	// Second update with STALE ETag (should get 412)
	updateReq2 := makeRequest("PUT", "/notes/"+created.ID, `{"content":"updated-by-user-b"}`)
	updateReq2.PathParameters["id"] = created.ID
	updateReq2.Headers["If-Match"] = created.ETag // stale!
	resp2, err := h.UpdateNote(ctx, updateReq2)
	if err != nil {
		t.Fatalf("UpdateNote returned error: %v", err)
	}
	if resp2.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("Expected 412 Precondition Failed, got %d: %s", resp2.StatusCode, resp2.Body)
	}
}

func TestNoteHandler_DeleteNote(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"delete-test.md","content":"bye"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Delete
	deleteReq := makeRequest("DELETE", "/notes/"+created.ID, "")
	deleteReq.PathParameters["id"] = created.ID
	deleteResp, err := h.DeleteNote(ctx, deleteReq)
	if err != nil {
		t.Fatalf("DeleteNote returned error: %v", err)
	}
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d: %s", deleteResp.StatusCode, deleteResp.Body)
	}

	// Verify it's gone
	listReq := makeRequest("GET", "/notes", "")
	listResp, _ := h.ListNotes(ctx, listReq)
	var notes []adapter.FileMetadata
	json.Unmarshal([]byte(listResp.Body), &notes)
	if len(notes) != 0 {
		t.Errorf("Expected 0 notes after delete, got %d", len(notes))
	}
}

func TestNoteHandler_Unauthorized(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Request with no auth header
	req := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/notes",
		Headers:    map[string]string{},
	}
	resp, err := h.ListNotes(ctx, req)
	if err != nil {
		t.Fatalf("ListNotes returned error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestNoteHandler_DuplicateNote(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"orig.md","content":"content"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Duplicate
	dupReq := makeRequest("POST", "/notes/"+created.ID+"/copy", "")
	dupReq.PathParameters["id"] = created.ID
	dupResp, err := h.DuplicateNote(ctx, dupReq)
	if err != nil {
		t.Fatalf("DuplicateNote returned error: %v", err)
	}
	if dupResp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", dupResp.StatusCode, dupResp.Body)
	}

	var duplicated adapter.FileMetadata
	json.Unmarshal([]byte(dupResp.Body), &duplicated)

	if duplicated.ID == created.ID {
		t.Error("Duplicated file should have new ID")
	}
	if duplicated.Name != "Copy of orig" {
		t.Errorf("Expected name 'Copy of orig', got '%s'", duplicated.Name)
	}
	if duplicated.Size != created.Size {
		t.Errorf("Expected size %d, got %d", created.Size, duplicated.Size)
	}

	// Verify content of duplicate
	getReq := makeRequest("GET", "/notes/"+duplicated.ID, "")
	getReq.PathParameters["id"] = duplicated.ID
	getResp, _ := h.GetNote(ctx, getReq)
	var note struct {
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(getResp.Body), &note)
	if note.Content != "content" {
		t.Errorf("Expected duplicated content 'content', got '%s'", note.Content)
	}
}

func TestNoteHandler_CreateFolder(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/folders", `{"name":"MyFolder"}`)
	resp, err := h.CreateFolder(ctx, req)
	if err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, resp.Body)
	}

	var folder adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &folder)
	if folder.Name != "MyFolder" {
		t.Errorf("Expected folder name 'MyFolder', got '%s'", folder.Name)
	}
	if folder.MIMEType != "application/vnd.google-apps.folder" {
		t.Errorf("Expected folder mimeType, got '%s'", folder.MIMEType)
	}
}

func TestNoteHandler_RenameNote(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"old-name.md","content":"data"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Rename
	renameReq := makeRequest("PATCH", "/notes/"+created.ID, `{"name":"new-name.md"}`)
	renameReq.PathParameters["id"] = created.ID
	resp, err := h.RenameNote(ctx, renameReq)
	if err != nil {
		t.Fatalf("RenameNote returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}

	var renamed adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &renamed)
	if renamed.Name != "new-name.md" {
		t.Errorf("Expected name 'new-name.md', got '%s'", renamed.Name)
	}
}

func TestNoteHandler_PatchNote_Star(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create
	createReq := makeRequest("POST", "/notes", `{"name":"star.md","content":"data"}`)
	createResp, _ := h.CreateNote(ctx, createReq)
	var created adapter.FileMetadata
	json.Unmarshal([]byte(createResp.Body), &created)

	// Patch: set starred
	patchReq := makeRequest("PATCH", "/notes/"+created.ID, `{"starred":true}`)
	patchReq.PathParameters["id"] = created.ID
	resp, err := h.PatchNote(ctx, patchReq)
	if err != nil {
		t.Fatalf("PatchNote returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}

	var patched adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &patched)
	if !patched.Starred {
		t.Error("Expected file to be starred after PATCH")
	}
}

func TestNoteHandler_ListStarredNotes(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	// Create two notes
	req1 := makeRequest("POST", "/notes", `{"name":"s1.md","content":"a"}`)
	resp1, _ := h.CreateNote(ctx, req1)
	var n1 adapter.FileMetadata
	json.Unmarshal([]byte(resp1.Body), &n1)

	makeRequest("POST", "/notes", `{"name":"s2.md","content":"b"}`)

	// Star s1
	patchReq := makeRequest("PATCH", "/notes/"+n1.ID, `{"starred":true}`)
	patchReq.PathParameters["id"] = n1.ID
	h.PatchNote(ctx, patchReq)

	// List starred
	listReq := makeRequest("GET", "/starred", "")
	listResp, err := h.ListStarredNotes(ctx, listReq)
	if err != nil {
		t.Fatalf("ListStarredNotes returned error: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", listResp.StatusCode)
	}

	var starred []adapter.FileMetadata
	json.Unmarshal([]byte(listResp.Body), &starred)
	if len(starred) != 1 {
		t.Fatalf("Expected 1 starred note, got %d", len(starred))
	}
	if starred[0].ID != n1.ID {
		t.Errorf("Expected starred note ID '%s', got '%s'", n1.ID, starred[0].ID)
	}
}

func TestNoteHandler_GetNote_NotFound(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
	h := handler.NewNoteHandler(provider, "test-secret")
	ctx := context.Background()

	req := makeRequest("GET", "/notes/nonexistent-id", "")
	req.PathParameters["id"] = "nonexistent-id"
	resp, err := h.GetNote(ctx, req)
	if err != nil {
		t.Fatalf("GetNote returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", resp.StatusCode, resp.Body)
	}
}
