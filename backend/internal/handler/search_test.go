package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/handler"
)

func TestSearch_Success(t *testing.T) {
	provider := memory.NewProvider(nil, nil)
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
	searchH := handler.NewSearchHandler(memory.NewProvider(nil, nil), "test-secret")
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
	provider := memory.NewProvider(nil, nil)
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

func TestSearch_Unauthorized(t *testing.T) {
	searchH := handler.NewSearchHandler(memory.NewProvider(nil, nil), "test-secret")
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
