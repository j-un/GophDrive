package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

// SearchHandler handles search requests.
type SearchHandler struct {
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(storageProvider adapter.StorageProvider, jwtSecret string) *SearchHandler {
	return &SearchHandler{
		storageProvider: storageProvider,
		jwtSecret:       jwtSecret,
	}
}

// getStorageAdapter extracts UserID and gets the storage adapter.
// (Duplicated helper or could be shared if extracted)
func (h *SearchHandler) getStorageAdapter(ctx context.Context, req events.APIGatewayProxyRequest) (adapter.StorageAdapter, error) {
	// Reusing GetUserID from auth.go (assuming it's in this package)
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	storage, err := h.storageProvider.GetAdapter(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage adapter: %w", err)
	}

	return storage, nil
}

// Search handles GET /search
func (h *SearchHandler) Search(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	query := req.QueryStringParameters["q"]
	if query == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Query parameter 'q' is required"}, nil
	}

	files, err := storage.SearchFiles(ctx, query)
	if err != nil {
		fmt.Printf("SearchFiles error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to search files"}, nil
	}

	if files == nil {
		files = []adapter.FileMetadata{}
	}

	body, _ := json.Marshal(files)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
