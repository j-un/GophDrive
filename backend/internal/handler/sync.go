package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// SyncHandler handles synchronization and conflict detection.
type SyncHandler struct {
	jwtSecret string
}

// NewSyncHandler creates a new SyncHandler.
func NewSyncHandler(jwtSecret string) *SyncHandler {
	return &SyncHandler{jwtSecret: jwtSecret}
}

// CheckConflictRequest represents the request body for conflict checking.
type CheckConflictRequest struct {
	LocalETag  string `json:"local_etag"`
	RemoteETag string `json:"remote_etag"`
}

// CheckConflictResponse represents the response body.
type CheckConflictResponse struct {
	HasConflict bool `json:"has_conflict"`
}

// CheckConflict determines if there is a conflict between local and remote versions.
func (h *SyncHandler) CheckConflict(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	_, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	var input CheckConflictRequest
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	// Simple conflict detection logic:
	// If localETag != remoteETag, it's a conflict?
	// Usually:
	// If base (last known) != remote, then remote has changed.
	// If local has changes (dirty), and remote has changed, then conflict.
	// Here the client sends what it has.
	// Actually, client should check if its 'base' etag matches current remote etag.
	// This endpoint might be redundant if we use conditional updates (If-Match) in UpdateNote.
	// But the plan included it.

	// Implementation:
	// Conflict if LocalETag != RemoteETag
	// (Assuming LocalETag is the version the client has, and RemoteETag is what server has?
	// No, client passes both? That seems like client-side logic.
	// Maybe endpoint accepts specific note ID and client's ETag, and server checks against DB.)

	// Re-reading plan: `POST /sync/check` -> `syncHandler` -> `CheckConflict(localEtag, remoteEtag) bool`
	// It seems to be a stateless check logic exposed as API?
	// Or maybe checking against server state?
	// If input is CheckConflictRequest with 2 tags, it's stateless.

	hasConflict := input.LocalETag != input.RemoteETag

	resp := CheckConflictResponse{
		HasConflict: hasConflict,
	}

	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
