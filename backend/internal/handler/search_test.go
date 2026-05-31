package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

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

func TestSearch_LimitTruncates(t *testing.T) {
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
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 2 {
		t.Errorf("Expected 2 results with limit=2, got %d", len(results))
	}
}

func TestSearch_SortedByModifiedTimeDesc(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	searchH := handler.NewSearchHandler(provider, "test-secret")
	ctx := context.Background()

	// Create 3 notes; all match "sorttest"
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		noteH.CreateNote(ctx, makeRequest("POST", "/notes",
			`{"name":"`+name+`","content":"sorttest"}`))
	}

	req := makeRequest("GET", "/search", "")
	req.QueryStringParameters = map[string]string{"q": "sorttest"}
	resp, _ := searchH.Search(ctx, req)
	var results []adapter.FileMetadata
	json.Unmarshal([]byte(resp.Body), &results)

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
	json.Unmarshal([]byte(resp.Body), &results)
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
	json.Unmarshal([]byte(resp.Body), &results)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Snippet != "" {
		t.Errorf("Expected no snippet for title-only match, got %q", results[0].Snippet)
	}
}
