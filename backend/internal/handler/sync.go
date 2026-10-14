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
