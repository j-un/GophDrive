package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/handler"
	"github.com/jun/gophdrive/backend/internal/model"
	"github.com/jun/gophdrive/backend/internal/session"
)

func TestSessionHandler_AcquireLock_Success(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sessions/file1/lock", "")
	req.PathParameters = map[string]string{"fileId": "file1"}
	resp, err := h.AcquireLock(ctx, req)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}

	var s model.EditingSession
	json.Unmarshal([]byte(resp.Body), &s)
	if s.FileID != "file1" {
		t.Errorf("Expected fileID 'file1', got '%s'", s.FileID)
	}
}

func TestSessionHandler_AcquireLock_Unauthorized(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	req := events.APIGatewayProxyRequest{
		Headers:        map[string]string{},
		PathParameters: map[string]string{"fileId": "file1"},
	}
	resp, _ := h.AcquireLock(ctx, req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestSessionHandler_AcquireLock_MissingFileID(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sessions//lock", "")
	req.PathParameters = map[string]string{}
	resp, _ := h.AcquireLock(ctx, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestSessionHandler_Heartbeat_Success(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	// First acquire
	req := makeRequest("POST", "/sessions/file1/lock", "")
	req.PathParameters = map[string]string{"fileId": "file1"}
	h.AcquireLock(ctx, req)

	// Heartbeat
	hbReq := makeRequest("POST", "/sessions/file1/heartbeat", "")
	hbReq.PathParameters = map[string]string{"fileId": "file1"}
	resp, err := h.Heartbeat(ctx, hbReq)
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestSessionHandler_Heartbeat_NotFound(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	req := makeRequest("POST", "/sessions/nonexistent/heartbeat", "")
	req.PathParameters = map[string]string{"fileId": "nonexistent"}
	resp, _ := h.Heartbeat(ctx, req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func TestSessionHandler_ReleaseLock_Success(t *testing.T) {
	locker := session.NewMockLocker()
	h := handler.NewSessionHandler(locker, "test-secret")
	ctx := context.Background()

	// Acquire
	acqReq := makeRequest("POST", "/sessions/file1/lock", "")
	acqReq.PathParameters = map[string]string{"fileId": "file1"}
	h.AcquireLock(ctx, acqReq)

	// Release
	relReq := makeRequest("DELETE", "/sessions/file1/lock", "")
	relReq.PathParameters = map[string]string{"fileId": "file1"}
	resp, err := h.ReleaseLock(ctx, relReq)
	if err != nil {
		t.Fatalf("ReleaseLock returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}
}
