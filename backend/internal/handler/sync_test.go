package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/handler"
)

func TestCheckConflict_Match(t *testing.T) {
	h := handler.NewSyncHandler("test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sync/check", `{"local_etag":"abc","remote_etag":"abc"}`)
	resp, err := h.CheckConflict(ctx, req)
	if err != nil {
		t.Fatalf("CheckConflict returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		HasConflict bool `json:"has_conflict"`
	}
	json.Unmarshal([]byte(resp.Body), &result)
	if result.HasConflict {
		t.Error("Expected no conflict when ETags match")
	}
}

func TestCheckConflict_Mismatch(t *testing.T) {
	h := handler.NewSyncHandler("test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sync/check", `{"local_etag":"abc","remote_etag":"xyz"}`)
	resp, _ := h.CheckConflict(ctx, req)

	var result struct {
		HasConflict bool `json:"has_conflict"`
	}
	json.Unmarshal([]byte(resp.Body), &result)
	if !result.HasConflict {
		t.Error("Expected conflict when ETags differ")
	}
}

func TestCheckConflict_Unauthorized(t *testing.T) {
	h := handler.NewSyncHandler("test-secret")
	ctx := context.Background()

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{},
		Body:    `{"local_etag":"a","remote_etag":"b"}`,
	}
	resp, _ := h.CheckConflict(ctx, req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestCheckConflict_InvalidBody(t *testing.T) {
	h := handler.NewSyncHandler("test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sync/check", "not-json")
	resp, _ := h.CheckConflict(ctx, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid body, got %d", resp.StatusCode)
	}
}
