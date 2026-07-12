package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
	"github.com/jun/gophdrive/backend/internal/handler"
)

func TestSearch_Success(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	// Create files
	req1 := makeRequest("POST", "/notes", `{"name":"hello.md","content":"world"}`)
	noteH.CreateNote(ctx, req1)
	req2 := makeRequest("POST", "/notes", `{"name":"other.md","content":"nothing"}`)
	noteH.CreateNote(ctx, req2)

	// Search
	searchReq := makeRequest("GET", "/search", "")
	searchReq.QueryStringParameters = map[string]string{"q": "hello"}
	resp, err := searchH.Search(ctx, searchReq)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}

	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	searchH := handler.NewSearchHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	searchReq := makeRequest("GET", "/search", "")
	searchReq.QueryStringParameters = map[string]string{}
	resp, err := searchH.Search(ctx, searchReq)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty query, got %d", resp.StatusCode)
	}
}

func TestSearch_NoResults(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	searchReq := makeRequest("GET", "/search", "")
	searchReq.QueryStringParameters = map[string]string{"q": "nonexistent"}
	resp, _ := searchH.Search(ctx, searchReq)

	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestSearch_TagFilter_Single(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"a.md","content":"Hello #develop world"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"b.md","content":"No tags here"}`))

	req := makeRequest("GET", "/search", "")
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"develop"}}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for tag=develop, got %d", len(results))
	}
}

func TestSearch_TagFilter_AndSemantics(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"ab.md","content":"Tags #alpha and #beta"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"a.md","content":"Only #alpha tag"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"b.md","content":"Only #beta tag"}`))

	req := makeRequest("GET", "/search", "")
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"alpha", "beta"}}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for tag=alpha AND tag=beta, got %d", len(results))
	}
}

func TestSearch_TagFilter_CaseInsensitive(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"ci.md","content":"Tag #Develop here"}`))

	req := makeRequest("GET", "/search", "")
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"develop"}}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for case-insensitive tag match, got %d", len(results))
	}
}

func TestSearch_TagFilter_QueryAndTagCombined(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"match.md","content":"sprint planning #develop"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"tagonly.md","content":"no query match #develop"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"qonly.md","content":"sprint planning no tag"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "sprint"}
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"develop"}}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result matching both q and tag, got %d", len(results))
	}
}

func TestSearch_TagOnly_NoQuery(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"t.md","content":"#develop task"}`))

	req := makeRequest("GET", "/search", "")
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"develop"}}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for tag-only query, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestSearch_TagFilter_ReturnsEmptyArrayNotNull(t *testing.T) {
	searchH := handler.NewSearchHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	req := makeRequest("GET", "/search", "")
	req.MultiValueQueryStringParameters = map[string][]string{"tag": {"nonexistent"}}
	resp, _ := searchH.Search(ctx, req)
	if resp.Body != "[]" {
		t.Errorf("Expected '[]' for no tag matches, got %s", resp.Body)
	}
}

func TestSearch_Unauthorized(t *testing.T) {
	searchH := handler.NewSearchHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	req := events.APIGatewayProxyRequest{
		Headers:               map[string]string{},
		QueryStringParameters: map[string]string{"q": "test"},
	}
	resp, _ := searchH.Search(ctx, req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestSearch_LimitAndSort(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		noteH.CreateNote(ctx, makeRequest("POST", "/notes",
			`{"name":"note.md","content":"match"}`))
	}

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "match", "limit": "2"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	var results []adapter.FileMetadata
	if err := json.Unmarshal([]byte(resp.Body), &results); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results with limit=2, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i].ModifiedTime.After(results[i-1].ModifiedTime) {
			t.Errorf("results not sorted by ModifiedTime DESC at index %d", i)
		}
	}
}

func TestSearch_BodyMatchHasSnippet(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"other.md","content":"the quick brown fox jumped"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "fox"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	if err := json.Unmarshal([]byte(resp.Body), &results); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Snippet == "" {
		t.Error("Expected snippet for body match, got empty")
	}
}

func TestSearch_TitleMatchHasNoSnippet(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"foxnote.md","content":"unrelated body text"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "fox"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	if err := json.Unmarshal([]byte(resp.Body), &results); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Snippet != "" {
		t.Errorf("Expected no snippet for title-only match, got %q", results[0].Snippet)
	}
}

func TestSearch_ScopeHeadings(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"h.md","content":"## ScopeTarget\nbody"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"b.md","content":"body has scopetarget but no heading"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "ScopeTarget", "in": "headings"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("expected 1 heading-match result, got %d", len(results))
	}
}

func TestSearch_ScopeTitles(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"titlekw.md","content":"unrelated body"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"other.md","content":"body has titlekw"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "titlekw", "in": "titles"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("expected 1 title-match result, got %d", len(results))
	}
}

func TestSearch_InvalidScopeFallsBackToAll(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"note.md","content":"body has fallbackkw"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "fallbackkw", "in": "invalid-scope"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("expected 1 result with invalid scope (fallback to all), got %d", len(results))
	}
}

func TestSearch_ScopeAllExplicit(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"note.md","content":"body has allkw"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "allkw", "in": "all"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("expected 1 result with ?in=all (explicit), got %d", len(results))
	}
}

func TestSearch_ScopeHeadingsCaseInsensitive(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"h.md","content":"## CITarget\nbody"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes",
		`{"name":"b.md","content":"body has citarget but no heading"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "CITarget", "in": "HEADINGS"} // uppercase
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Errorf("expected 1 result with ?in=HEADINGS (uppercase), got %d", len(results))
	}
}

// TestSearch_RankedOrder locks in the new relevance-first sort: a title match
// must outrank a body-only match even when the title match is the OLDER note
// (previously results were sorted purely by ModifiedTime desc).
func TestSearch_RankedOrder(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	// Older note: query hits the title.
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"apple.md","content":"unrelated body"}`))
	time.Sleep(2 * time.Millisecond)
	// Newer note: query hits only the body.
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"other.md","content":"apple mentioned here"}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "apple"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	var results []adapter.FileMetadata
	if err := json.Unmarshal([]byte(resp.Body), &results); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "apple" {
		t.Errorf("expected the older title match to rank first, got %q first", results[0].Name)
	}
}

// TestSearch_DateFilter covers both modifiedAfter and modifiedBefore as a
// half-open [after, before) window.
func TestSearch_DateFilter(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"old.md","content":"shared text"}`))
	time.Sleep(2 * time.Millisecond)
	cutover := time.Now()
	time.Sleep(2 * time.Millisecond)
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"new.md","content":"shared text"}`))

	afterReq := makeRequest("GET", "/search", "")
	afterReq.QueryStringParameters = map[string]string{"q": "shared", "modifiedAfter": cutover.Format(time.RFC3339Nano)}
	resp, err := searchH.Search(ctx, afterReq)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	var afterResults []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &afterResults)
	if len(afterResults) != 1 || afterResults[0].Name != "new" {
		t.Errorf("modifiedAfter: expected only 'new', got %+v", afterResults)
	}

	beforeReq := makeRequest("GET", "/search", "")
	beforeReq.QueryStringParameters = map[string]string{"q": "shared", "modifiedBefore": cutover.Format(time.RFC3339Nano)}
	resp2, err := searchH.Search(ctx, beforeReq)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	var beforeResults []adapter.FileMetadata
	json.Unmarshal([]byte(resp2.Body), &beforeResults)
	if len(beforeResults) != 1 || beforeResults[0].Name != "old" {
		t.Errorf("modifiedBefore: expected only 'old', got %+v", beforeResults)
	}
}

func TestSearch_InvalidDateParam(t *testing.T) {
	searchH := handler.NewSearchHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "x", "modifiedAfter": "not-a-date"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid modifiedAfter, got %d", resp.StatusCode)
	}
}

// TestSearch_DateOnly_NoQueryTagType covers the "date modifiers alone are
// insufficient" rule: modifiedAfter/modifiedBefore must not satisfy the
// "at least one of q/tag/type" requirement by themselves.
func TestSearch_DateOnly_NoQueryTagType(t *testing.T) {
	searchH := handler.NewSearchHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"modifiedAfter": "2020-01-01"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for date-only request, got %d", resp.StatusCode)
	}
}

// TestSearch_TypeFilter verifies the 'type' query param reaches the adapter
// and is observable in the filtered results (no q/tag needed).
func TestSearch_TypeFilter(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	decisionBody, _ := json.Marshal("---\ntype: decision\n---\nbody")
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"a.md","content":`+string(decisionBody)+`}`))
	howtoBody, _ := json.Marshal("---\ntype: howto\n---\nbody")
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"b.md","content":`+string(howtoBody)+`}`))

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"type": "decision"}
	resp, err := searchH.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 || results[0].Name != "a" {
		t.Errorf("expected only 'a' (type=decision), got %+v", results)
	}
}
