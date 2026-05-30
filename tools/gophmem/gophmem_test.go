package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeServer builds a minimal httptest server that mimics the GophDrive REST API.
// The returned cleanup function must be called by the test.
func fakeServer(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := NewClient(srv.URL, "test-key", srv.Client())
	return client, srv.Close
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- Client unit tests ----

func TestClient_GetUser(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/user" || r.Method != "GET" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, UserProfile{ID: "sub-abc", BaseFolderID: "folder-xyz"})
	}))
	defer close()

	p, err := client.GetUser()
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if p.ID != "sub-abc" || p.BaseFolderID != "folder-xyz" {
		t.Errorf("unexpected profile: %+v", p)
	}
}

func TestClient_CreateNote(t *testing.T) {
	var gotReq createNoteReq
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notes" || r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		writeJSON(w, 201, FileMetadata{
			ID:   "note-id-1",
			Name: gotReq.Name,
			ETag: "etag-1",
		})
	}))
	defer close()

	f, err := client.CreateNote("test.md", "hello world", "folder-id")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if f.ID != "note-id-1" {
		t.Errorf("unexpected ID: %s", f.ID)
	}
	if gotReq.ParentID != "folder-id" {
		t.Errorf("parentId not sent: %q", gotReq.ParentID)
	}
	if gotReq.Content != "hello world" {
		t.Errorf("content not sent: %q", gotReq.Content)
	}
}

func TestClient_UpdateNote_Success(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || !strings.HasPrefix(r.URL.Path, "/notes/") {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("If-Match") != "etag-original" {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		writeJSON(w, 200, FileMetadata{ID: "note-1", ETag: "etag-new"})
	}))
	defer close()

	f, err := client.UpdateNote("note-1", "new content", "etag-original")
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	if f.ETag != "etag-new" {
		t.Errorf("unexpected ETag: %s", f.ETag)
	}
}

func TestClient_UpdateNote_Conflict(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer close()

	_, err := client.UpdateNote("note-1", "content", "wrong-etag")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestClient_Search(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != "GET" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("q") != "design decision" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, 200, []FileMetadata{
			{ID: "n1", Name: "design decision.md"},
			{ID: "n2", Name: "design decision 2.md"},
		})
	}))
	defer close()

	results, err := client.Search("design decision", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// ---- Folder resolution tests ----

func TestResolveAIMemoryFolder_Existing(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search" && r.Method == "GET":
			writeJSON(w, 200, []FileMetadata{
				{ID: "folder-ai", Name: "AI Memory", MIMEType: folderMIMEType},
				{ID: "note-1", Name: "other.md", MIMEType: "text/markdown"},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("ResolveAIMemoryFolder: %v", err)
	}
	if id != "folder-ai" {
		t.Errorf("expected folder-ai, got %s", id)
	}
}

func TestResolveAIMemoryFolder_CreateNew(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search" && r.Method == "GET":
			// No AI Memory folder in the Vault yet.
			writeJSON(w, 200, []FileMetadata{})
		case r.URL.Path == "/folders" && r.Method == "POST":
			var req createFolderReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Name != aiMemoryFolderName {
				t.Errorf("unexpected folder name: %s", req.Name)
			}
			writeJSON(w, 200, FileMetadata{ID: "new-folder-id", Name: aiMemoryFolderName, MIMEType: folderMIMEType})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("ResolveAIMemoryFolder: %v", err)
	}
	if id != "new-folder-id" {
		t.Errorf("expected new-folder-id, got %s", id)
	}
}

func TestResolveAIMemoryFolder_Cache(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	// Prime the cache with a sentinel ID.
	_ = saveFolderCache(folderCache{AIMemoryID: "cached-folder-id"})

	// Server returns a different ID — if the cache is bypassed the assertion fails.
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []FileMetadata{
			{ID: "server-folder-id", Name: "AI Memory", MIMEType: folderMIMEType},
		})
	}))
	defer close()

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cached-folder-id" {
		t.Errorf("expected cached-folder-id (cache hit), got %s", id)
	}
}

func TestResolveAIMemoryFolder_Relocated(t *testing.T) {
	// Folder was moved out of root — Search-based resolution must still find it.
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search" && r.Method == "GET":
			writeJSON(w, 200, []FileMetadata{
				{ID: "relocated-id", Name: "AI Memory", MIMEType: folderMIMEType},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("ResolveAIMemoryFolder: %v", err)
	}
	if id != "relocated-id" {
		t.Errorf("expected relocated-id, got %s", id)
	}
	if c := loadFolderCache(); c.AIMemoryID != "relocated-id" {
		t.Errorf("cache not warmed after relocation resolve: got %q", c.AIMemoryID)
	}
}

// ---- appendToNote tests ----

func TestAppendToNote_Success(t *testing.T) {
	stored := NoteResponse{
		ID:      "note-abc",
		Name:    "memo.md",
		Content: "original content",
		ETag:    "etag-1",
	}

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/note-abc"):
			writeJSON(w, 200, stored)
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/note-abc"):
			if r.Header.Get("If-Match") != stored.ETag {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			var req updateNoteReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			stored.Content = req.Content
			stored.ETag = "etag-2"
			writeJSON(w, 200, FileMetadata{ID: "note-abc", ETag: stored.ETag})
		default:
			http.NotFound(w, r)
		}
	}))
	defer close()

	name, err := appendToNote(client, "note-abc", "appended text")
	if err != nil {
		t.Fatalf("appendToNote: %v", err)
	}
	if name != "memo.md" {
		t.Errorf("unexpected name: %s", name)
	}
	if !strings.Contains(stored.Content, "appended text") {
		t.Errorf("content not appended: %q", stored.Content)
	}
}

func TestAppendToNote_412Retry(t *testing.T) {
	callCount := 0
	etags := []string{"etag-0", "etag-1", "etag-2"}

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/note-xyz"):
			writeJSON(w, 200, NoteResponse{
				ID:      "note-xyz",
				Name:    "log.md",
				Content: "content",
				ETag:    etags[callCount],
			})
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/note-xyz"):
			if callCount < 2 {
				// Simulate concurrent write: always return 412 until last try.
				callCount++
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			writeJSON(w, 200, FileMetadata{ID: "note-xyz", ETag: "etag-final"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer close()

	_, err := appendToNote(client, "note-xyz", "new line")
	if err != nil {
		t.Fatalf("appendToNote with retry: %v", err)
	}
}

func TestAppendToNote_412ExhaustsRetries(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET":
			writeJSON(w, 200, NoteResponse{ID: "note-1", ETag: "etag", Content: "c"})
		case r.Method == "PUT":
			w.WriteHeader(http.StatusPreconditionFailed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer close()

	_, err := appendToNote(client, "note-1", "text")
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if !strings.Contains(err.Error(), "retries") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- resolveNoteID tests ----

func TestResolveNoteID_UUID(t *testing.T) {
	// A UUID-shaped string is returned as-is without any HTTP call.
	// Using an unreachable address: any network call would fail, proving the
	// UUID short-circuit path is taken when no error is returned.
	client := NewClient("http://127.0.0.1:0", "key")

	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	id, err := resolveNoteID(client, uuid)
	if err != nil {
		t.Fatalf("resolveNoteID: %v", err)
	}
	if id != uuid {
		t.Errorf("unexpected id: %s", id)
	}
}

func TestResolveNoteID_ByTitle(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, []FileMetadata{
			{ID: "found-id", Name: "design decision.md", ModifiedTime: time.Now()},
		})
	}))
	defer close()

	id, err := resolveNoteID(client, "design decision")
	if err != nil {
		t.Fatalf("resolveNoteID: %v", err)
	}
	if id != "found-id" {
		t.Errorf("expected found-id, got %s", id)
	}
}

func TestResolveNoteID_NotFound(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_, err := resolveNoteID(client, "nonexistent note")
	if err == nil {
		t.Fatal("expected error for not found, got nil")
	}
}

// ---- folderCache persistence tests ----

func TestFolderCache_RoundTrip(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	original := folderCache{AIMemoryID: "test-id-123"}
	if err := saveFolderCache(original); err != nil {
		t.Fatalf("saveFolderCache: %v", err)
	}
	loaded := loadFolderCache()
	if loaded.AIMemoryID != original.AIMemoryID {
		t.Errorf("cache roundtrip failed: want %s, got %s", original.AIMemoryID, loaded.AIMemoryID)
	}
}

func TestFolderCache_MissingFile(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())
	// Remove cache file if present.
	_ = os.Remove(cachePath())

	c := loadFolderCache()
	if c.AIMemoryID != "" {
		t.Errorf("expected empty cache, got %q", c.AIMemoryID)
	}
}

// ---- looksLikeUUID tests ----

func TestLooksLikeUUID(t *testing.T) {
	valid := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"550E8400-E29B-41D4-A716-446655440000", // uppercase hex
		"00000000-0000-0000-0000-000000000000",
	}
	for _, s := range valid {
		if !looksLikeUUID(s) {
			t.Errorf("expected true for %q", s)
		}
	}

	invalid := []string{
		"",
		"not-a-uuid",
		// too short
		"550e8400-e29b-41d4-a716",
		// hyphens at wrong positions (would pass old len+Count check)
		"5-50e8400-e29b-41d4-a716446655440000",
		// right structure but non-hex chars ('g'–'l') past position 23
		"abcdefab-abcd-abcd-abcd-abcdefghijkl",
	}
	for _, s := range invalid {
		if looksLikeUUID(s) {
			t.Errorf("expected false for %q", s)
		}
	}
}
