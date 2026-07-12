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

func TestClient_GetNote_NotFound(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer close()

	_, err := client.GetNote("missing-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
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

	results, err := client.Search("design decision", SearchOpts{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// ---- new Tier 1 tests ----

func TestRunSearch_RendersSnippet(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, []FileMetadata{
			{
				ID:           "note-s1",
				Name:         "decision: auth.md",
				Tags:         []string{"decision", "auth"},
				ModifiedTime: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
				Snippet:      "…JWT を採用した理由は X…",
			},
		})
	}))
	defer close()

	var buf strings.Builder
	err := runSearch(client, []string{"JWT"}, &buf)
	out := buf.String()

	if err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if !strings.Contains(out, "decision: auth") {
		t.Errorf("expected title in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[decision,auth]") {
		t.Errorf("expected tags in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-05-28") {
		t.Errorf("expected date in output, got:\n%s", out)
	}
	if !strings.Contains(out, "        > …JWT を採用した理由は X…") {
		t.Errorf("expected snippet line in output, got:\n%s", out)
	}
}

func TestRunSearch_PassesLimitFlag(t *testing.T) {
	var gotLimit string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--limit", "5"}, &strings.Builder{})
	if gotLimit != "5" {
		t.Errorf("expected limit=5 in query, got %q", gotLimit)
	}
}

func TestRunSearch_NoSnippet(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []FileMetadata{
			{ID: "n1", Name: "note.md", ModifiedTime: time.Now(), Snippet: "some snippet"},
		})
	}))
	defer close()

	var buf strings.Builder
	_ = runSearch(client, []string{"query", "--no-snippet"}, &buf)
	out := buf.String()

	if strings.Contains(out, "        > ") {
		t.Errorf("expected no snippet line with --no-snippet, got:\n%s", out)
	}
}

func TestRunSearch_PassesInFlag(t *testing.T) {
	var gotIn string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIn = r.URL.Query().Get("in")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--in", "headings"}, &strings.Builder{})
	if gotIn != "headings" {
		t.Errorf("expected in=headings in query, got %q", gotIn)
	}
}

func TestRunSearch_DefaultInFlagOmitted(t *testing.T) {
	var gotIn string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIn = r.URL.Query().Get("in")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query"}, &strings.Builder{})
	if gotIn != "" {
		t.Errorf("expected in to be absent when not specified, got %q", gotIn)
	}
}

func TestRunSearch_InFlagUppercaseLowercased(t *testing.T) {
	var gotIn string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIn = r.URL.Query().Get("in")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--in", "HEADINGS"}, &strings.Builder{})
	if gotIn != "headings" {
		t.Errorf("expected in=headings (lowercase), got %q", gotIn)
	}
}

func TestRunSearch_RepeatedTagFlag(t *testing.T) {
	var gotTags []string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTags = r.URL.Query()["tag"]
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--tag", "a", "--tag", "b"}, &strings.Builder{})
	if len(gotTags) != 2 || gotTags[0] != "a" || gotTags[1] != "b" {
		t.Errorf("expected tag=a and tag=b (repeatable), got %v", gotTags)
	}
}

func TestRunSearch_PassesTypeFlag(t *testing.T) {
	var gotType string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("type")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--type", "decision"}, &strings.Builder{})
	if gotType != "decision" {
		t.Errorf("expected type=decision in query, got %q", gotType)
	}
}

func TestRunSearch_TypeOnly_NoUsageError(t *testing.T) {
	var gotQuery, gotType string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotType = r.URL.Query().Get("type")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	err := runSearch(client, []string{"--type", "decision"}, &strings.Builder{})
	if err != nil {
		t.Fatalf("expected no usage error for type-only search, got: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("expected empty q, got %q", gotQuery)
	}
	if gotType != "decision" {
		t.Errorf("expected type=decision in query, got %q", gotType)
	}
}

func TestRunSearch_SinceFlag(t *testing.T) {
	var gotModifiedAfter string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotModifiedAfter = r.URL.Query().Get("modifiedAfter")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--since", "2026-07-01"}, &strings.Builder{})

	want, err := time.ParseInLocation("2006-01-02", "2026-07-01", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	wantStr := want.UTC().Format(time.RFC3339)
	if gotModifiedAfter != wantStr {
		t.Errorf("expected modifiedAfter=%s, got %q", wantStr, gotModifiedAfter)
	}
}

func TestRunSearch_UntilFlag_EndOfDay(t *testing.T) {
	var gotModifiedBefore string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotModifiedBefore = r.URL.Query().Get("modifiedBefore")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	_ = runSearch(client, []string{"query", "--until", "2026-07-01"}, &strings.Builder{})

	base, err := time.ParseInLocation("2006-01-02", "2026-07-01", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	wantStr := base.AddDate(0, 0, 1).UTC().Format(time.RFC3339)
	if gotModifiedBefore != wantStr {
		t.Errorf("expected modifiedBefore=%s (start of next local day), got %q", wantStr, gotModifiedBefore)
	}
}

func TestRunSearch_UntilFlag_RFC3339Passthrough(t *testing.T) {
	var gotModifiedBefore string
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotModifiedBefore = r.URL.Query().Get("modifiedBefore")
		writeJSON(w, 200, []FileMetadata{})
	}))
	defer close()

	const rfc = "2026-07-01T15:04:05Z"
	_ = runSearch(client, []string{"query", "--until", rfc}, &strings.Builder{})

	want, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		t.Fatal(err)
	}
	wantStr := want.UTC().Format(time.RFC3339)
	if gotModifiedBefore != wantStr {
		t.Errorf("expected RFC3339 --until to pass through as exclusive bound: want %s, got %q", wantStr, gotModifiedBefore)
	}
}

func TestRunSearch_InvalidDate(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request; invalid date should fail before any HTTP call: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer close()

	err := runSearch(client, []string{"query", "--since", "not-a-date"}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
	if !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---- extractSection tests ----

func TestExtractSection_Basic(t *testing.T) {
	body := "## Background\nsome background\n## Decision\nJWT chosen\n## Rejected alternatives\nnope\n"
	got, found := extractSection(body, "Decision")
	if !found {
		t.Fatal("section not found")
	}
	if !strings.Contains(got, "JWT chosen") {
		t.Errorf("expected Decision body, got: %q", got)
	}
	if strings.Contains(got, "nope") {
		t.Errorf("should not include next section, got: %q", got)
	}
}

func TestExtractSection_CaseInsensitive(t *testing.T) {
	body := "## Rejected Alternatives\nalt1\nalt2\n"
	_, found := extractSection(body, "rejected")
	if !found {
		t.Error("case-insensitive match failed")
	}
}

func TestExtractSection_IncludesSubheadings(t *testing.T) {
	body := "## Decision\ntop\n### Sub\nsub content\n## Next\nother\n"
	got, found := extractSection(body, "Decision")
	if !found {
		t.Fatal("section not found")
	}
	if !strings.Contains(got, "sub content") {
		t.Errorf("should include H3 sub-section, got: %q", got)
	}
	if strings.Contains(got, "other") {
		t.Errorf("should not include next H2, got: %q", got)
	}
}

func TestExtractSection_SkipsCodeFence(t *testing.T) {
	body := "## Decision\ntext\n```\n## Not a heading\n```\nafter fence\n"
	got, found := extractSection(body, "Decision")
	if !found {
		t.Fatal("section not found")
	}
	if !strings.Contains(got, "after fence") {
		t.Errorf("should include content after fence, got: %q", got)
	}
}

func TestExtractSection_NotFound(t *testing.T) {
	body := "## Background\ntext\n"
	_, found := extractSection(body, "Decision")
	if found {
		t.Error("expected not found")
	}
}

func TestExtractSection_TildeFence(t *testing.T) {
	// Heading inside a ~~~ fence must not terminate the section;
	// the fence content (including the raw heading line) is still part of the output.
	body := "## Decision\ntext\n~~~\n## Not a heading\n~~~\nafter fence\n## Next\nother\n"
	got, found := extractSection(body, "Decision")
	if !found {
		t.Fatal("section not found")
	}
	// "after fence" must be present — shows the fenced heading did not stop extraction.
	if !strings.Contains(got, "after fence") {
		t.Errorf("should include content after tilde fence, got: %q", got)
	}
	// "other" is under ## Next (same level as ## Decision) and must NOT be present.
	if strings.Contains(got, "other") {
		t.Errorf("should stop at same-level heading after fence, got: %q", got)
	}
}

func TestRunRead_SectionNotFound(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/notes/") {
			writeJSON(w, 200, NoteResponse{
				ID:      "note-1",
				Name:    "test.md",
				Content: "## Background\ncontent\n",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer close()

	err := runRead(client, []string{"note-1", "--section", "Decision"}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error for missing section, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
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

	// Server handles the Fix-B existence check (GET /notes/{id}) and must not
	// receive /search or /folders — any such call means the cache was bypassed.
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/notes/"):
			writeJSON(w, 200, NoteResponse{ID: "cached-folder-id"})
		default:
			t.Errorf("unexpected request (cache bypassed): %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	// Prime the cache after creating the client so we have the correct CacheKey.
	_ = saveFolderCache(folderCache{client.CacheKey(): "cached-folder-id"})

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
	if got := loadFolderCache()[client.CacheKey()]; got != "relocated-id" {
		t.Errorf("cache not warmed after relocation resolve: got %q", got)
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

func TestResolveNoteID_ByAlias(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, []FileMetadata{
			{ID: "found-id", Name: "canonical title.md", Aliases: []string{"nickname", "other alias"}},
		})
	}))
	defer close()

	id, err := resolveNoteID(client, "nickname")
	if err != nil {
		t.Fatalf("resolveNoteID: %v", err)
	}
	if id != "found-id" {
		t.Errorf("expected found-id via alias match, got %s", id)
	}
}

// ---- folderCache persistence tests ----

func TestFolderCache_RoundTrip(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	original := folderCache{"https://example.com#ab12cd34": "test-id-123"}
	if err := saveFolderCache(original); err != nil {
		t.Fatalf("saveFolderCache: %v", err)
	}
	loaded := loadFolderCache()
	if loaded["https://example.com#ab12cd34"] != original["https://example.com#ab12cd34"] {
		t.Errorf("cache roundtrip failed: want %s, got %s",
			original["https://example.com#ab12cd34"], loaded["https://example.com#ab12cd34"])
	}
}

func TestFolderCache_MissingFile(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())
	// Remove cache file if present.
	_ = os.Remove(cachePath())

	c := loadFolderCache()
	if len(c) != 0 {
		t.Errorf("expected empty cache, got %v", c)
	}
}

// ---- CacheKey tests ----

func TestCacheKey_IsolatedByURLAndKey(t *testing.T) {
	c1 := NewClient("https://example.com", "key-a")
	c2 := NewClient("https://example.com", "key-b")
	c3 := NewClient("https://other.com", "key-a")

	if c1.CacheKey() == c2.CacheKey() {
		t.Error("different API keys must yield different CacheKeys")
	}
	if c1.CacheKey() == c3.CacheKey() {
		t.Error("different base URLs must yield different CacheKeys")
	}
}

func TestCacheKey_AnonymousForEmptyKey(t *testing.T) {
	c := NewClient("https://example.com", "")
	if !strings.HasSuffix(c.CacheKey(), "#anonymous") {
		t.Errorf("empty API key must use anonymous digest, got %s", c.CacheKey())
	}
}

func TestResolveAIMemoryFolder_StaleCacheAutoRepair(t *testing.T) {
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	searchCalled := false
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/notes/"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/search" && r.Method == "GET":
			searchCalled = true
			writeJSON(w, 200, []FileMetadata{
				{ID: "real-folder-id", Name: "AI Memory", MIMEType: folderMIMEType},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	_ = saveFolderCache(folderCache{client.CacheKey(): "stale-uuid"})

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("ResolveAIMemoryFolder: %v", err)
	}
	if id != "real-folder-id" {
		t.Errorf("expected real-folder-id after stale repair, got %s", id)
	}
	if !searchCalled {
		t.Error("expected Search to be called after 404 on stale cached ID")
	}
	if got := loadFolderCache()[client.CacheKey()]; got != "real-folder-id" {
		t.Errorf("cache should be updated to real-folder-id after repair, got %q", got)
	}
}

func TestResolveAIMemoryFolder_CachedGetServerError(t *testing.T) {
	// If the existence-check GET returns a non-404 error (e.g. 500), the function
	// must propagate the error and NOT fall through to Search/Create.
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/notes/"):
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request after 5xx (should abort): %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	_ = saveFolderCache(folderCache{client.CacheKey(): "cached-id"})

	if _, err := ResolveAIMemoryFolder(client); err == nil {
		t.Fatal("expected error on 5xx during cache verification, got nil")
	}
}

func TestResolveAIMemoryFolder_LegacyJSONFallback(t *testing.T) {
	// Old cache format was {"ai_memory_id": "..."} (a Go struct). Decoding it into
	// the new map[string]string succeeds — but the key "ai_memory_id" never matches
	// any CacheKey() (which is "baseURL#digest"), so the result is a cache miss.
	// Search/Create runs normally; no stale ID is used and no error is raised.
	t.Setenv("GOPHMEM_CACHE_DIR", t.TempDir())

	if err := os.WriteFile(cachePath(), []byte(`{"ai_memory_id":"legacy-folder-id"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search" && r.Method == "GET":
			writeJSON(w, 200, []FileMetadata{
				{ID: "fresh-folder-id", Name: "AI Memory", MIMEType: folderMIMEType},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer close()

	// The old JSON decodes into a non-empty map (key "ai_memory_id"), proving
	// no decode error is raised — the miss is purely due to key mismatch.
	rawCache := loadFolderCache()
	if _, hasLegacyKey := rawCache["ai_memory_id"]; !hasLegacyKey {
		t.Error("expected legacy key 'ai_memory_id' to survive decode (no decode error)")
	}

	id, err := ResolveAIMemoryFolder(client)
	if err != nil {
		t.Fatalf("unexpected error with legacy cache: %v", err)
	}
	if id != "fresh-folder-id" {
		t.Errorf("expected fresh-folder-id (legacy cache miss), got %s", id)
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
