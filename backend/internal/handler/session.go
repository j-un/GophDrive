package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/session"
)

// SessionHandler handles session locking requests.
type SessionHandler struct {
	lockManager session.Locker
	jwtSecret   string
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(lockManager session.Locker, jwtSecret string) *SessionHandler {
	return &SessionHandler{lockManager: lockManager, jwtSecret: jwtSecret}
}

// AcquireLock
func (h *SessionHandler) AcquireLock(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	fileID := req.PathParameters["fileId"]
	if fileID == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing file ID"}, nil
	}

	session, err := h.lockManager.AcquireLock(ctx, fileID, userID)
	if err != nil {
		if err.Error() == "file is locked by another user" {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusConflict, Body: "File is locked by another user"}, nil
		}
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to acquire lock"}, nil
	}

	body, _ := json.Marshal(session)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(body)}, nil
}

// Heartbeat
func (h *SessionHandler) Heartbeat(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	fileID := req.PathParameters["fileId"]
	if fileID == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing file ID"}, nil
	}

	session, err := h.lockManager.Heartbeat(ctx, fileID, userID)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Lock not found or expired"}, nil
	}

	body, _ := json.Marshal(session)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(body)}, nil
}

// ReleaseLock
func (h *SessionHandler) ReleaseLock(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	fileID := req.PathParameters["fileId"]
	if fileID == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing file ID"}, nil
	}

	err = h.lockManager.ReleaseLock(ctx, fileID, userID)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to release lock"}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent}, nil
}
