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

func TestTagHandler_ListTags_Empty(t *testing.T) {
	tagH := handler.NewTagHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	resp, err := tagH.ListTags(ctx, makeRequest("GET", "/tags", ""))
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	if resp.Body != "[]" {
		t.Errorf("Expected '[]' for empty store, got %s", resp.Body)
	}
}

func TestTagHandler_ListTags_SortedByCountDesc(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	tagH := handler.NewTagHandler(provider, "test-secret")
	ctx := context.Background()

	// #beta appears twice, #alpha once
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n1.md","content":"Tag #alpha"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n2.md","content":"Tag #beta"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n3.md","content":"Tag #beta again"}`))

	resp, _ := tagH.ListTags(ctx, makeRequest("GET", "/tags", ""))
	var tags []adapter.TagCount
	json.Unmarshal([]byte(resp.Body), &tags)

	if len(tags) != 2 {
		t.Fatalf("Expected 2 tags, got %d: %v", len(tags), tags)
	}
	if tags[0].Name != "beta" || tags[0].Count != 2 {
		t.Errorf("Expected first tag beta:2, got %s:%d", tags[0].Name, tags[0].Count)
	}
	if tags[1].Name != "alpha" || tags[1].Count != 1 {
		t.Errorf("Expected second tag alpha:1, got %s:%d", tags[1].Name, tags[1].Count)
	}
}

func TestTagHandler_ListTags_TieBreakByName(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	tagH := handler.NewTagHandler(provider, "test-secret")
	ctx := context.Background()

	// #zebra and #apple each appear once — alphabetical tie-break expected
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n1.md","content":"Tag #zebra"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n2.md","content":"Tag #apple"}`))

	resp, _ := tagH.ListTags(ctx, makeRequest("GET", "/tags", ""))
	var tags []adapter.TagCount
	json.Unmarshal([]byte(resp.Body), &tags)

	if len(tags) != 2 {
		t.Fatalf("Expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "apple" {
		t.Errorf("Expected tie-break to sort alphabetically: first=apple, got %s", tags[0].Name)
	}
}

func TestTagHandler_ListTags_Unauthorized(t *testing.T) {
	tagH := handler.NewTagHandler(dynamo.NewProvider(nil), "test-secret")
	ctx := context.Background()

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/tags",
		Headers:    map[string]string{},
	}
	resp, err := tagH.ListTags(ctx, req)
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for missing auth, got %d", resp.StatusCode)
	}
}

func TestTagHandler_ListTags_IgnoresFolders(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	tagH := handler.NewTagHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateFolder(ctx, makeRequest("POST", "/folders", `{"name":"MyFolder"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"n1.md","content":"Tag #develop"}`))

	resp, _ := tagH.ListTags(ctx, makeRequest("GET", "/tags", ""))
	var tags []adapter.TagCount
	json.Unmarshal([]byte(resp.Body), &tags)

	if len(tags) != 1 || tags[0].Name != "develop" {
		t.Errorf("Expected only note tags (not folders), got %v", tags)
	}
}

func TestTagHandler_ListTags_IsolatesUsers(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	ctx := context.Background()

	// userA creates a note with a tag
	userAToken := makeToken("user-a")
	createReq := makeRequest("POST", "/notes", `{"name":"n.md","content":"Tag #secret"}`)
	createReq.Headers["Authorization"] = "Bearer " + userAToken
	noteH := handler.NewNoteHandler(provider, "test-secret")
	noteH.CreateNote(ctx, createReq)

	// userB should not see userA's tags
	userBToken := makeToken("user-b")
	tagH := handler.NewTagHandler(provider, "test-secret")
	tagReq := makeRequest("GET", "/tags", "")
	tagReq.Headers["Authorization"] = "Bearer " + userBToken
	resp, _ := tagH.ListTags(ctx, tagReq)

	var tags []adapter.TagCount
	json.Unmarshal([]byte(resp.Body), &tags)
	if len(tags) != 0 {
		t.Errorf("Expected user B to see 0 tags, got %v", tags)
	}
}
