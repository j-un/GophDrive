package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

// TagHandler handles tag-related requests.
type TagHandler struct {
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

// NewTagHandler creates a new TagHandler.
func NewTagHandler(storageProvider adapter.StorageProvider, jwtSecret string) *TagHandler {
	return &TagHandler{storageProvider: storageProvider, jwtSecret: jwtSecret}
}

func (h *TagHandler) getStorageAdapter(ctx context.Context, req events.APIGatewayProxyRequest) (adapter.StorageAdapter, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	storage, err := h.storageProvider.GetAdapter(ctx, claims.UserID, claims.BaseFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage adapter: %w", err)
	}
	return storage, nil
}

// ListTags handles GET /tags — returns [{name, count}] sorted by count desc.
func (h *TagHandler) ListTags(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	tags, err := storage.ListAllTags(ctx)
	if err != nil {
		if errors.Is(err, adapter.ErrUnauthorized) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Storage authentication failed. Please login again."}, nil
		}
		fmt.Printf("ListAllTags error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to list tags"}, nil
	}

	if tags == nil {
		tags = []adapter.TagCount{}
	}

	body, _ := json.Marshal(tags)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}
