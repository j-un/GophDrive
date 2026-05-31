package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

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

// getStorageAdapter extracts session claims and returns the storage adapter
// scoped to the user's base folder.
func (h *SearchHandler) getStorageAdapter(ctx context.Context, req events.APIGatewayProxyRequest) (adapter.StorageAdapter, error) {
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

const searchDefaultLimit = 20
const searchMaxLimit = 100

// Search handles GET /search?q=...&tag=...&tag=...&limit=N
// Both q and tag are optional; at least one must be provided.
// Results are sorted by ModifiedTime descending and capped at limit (default 20, max 100).
func (h *SearchHandler) Search(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	query := req.QueryStringParameters["q"]
	tags := req.MultiValueQueryStringParameters["tag"]

	if query == "" && len(tags) == 0 {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "At least one of 'q' or 'tag' is required"}, nil
	}

	limit := searchDefaultLimit
	if lStr := req.QueryStringParameters["limit"]; lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil {
			limit = n
		}
	}
	if limit <= 0 || limit > searchMaxLimit {
		limit = searchDefaultLimit
	}

	rawIn := strings.ToLower(req.QueryStringParameters["in"])
	var scope adapter.SearchScope
	switch rawIn {
	case string(adapter.ScopeTitles):
		scope = adapter.ScopeTitles
	case string(adapter.ScopeHeadings):
		scope = adapter.ScopeHeadings
	case "all", string(adapter.ScopeAll):
		scope = adapter.ScopeAll
	default:
		scope = adapter.ScopeAll
	}

	files, err := storage.SearchFilesWithTags(ctx, query, tags, scope)
	if err != nil {
		if errors.Is(err, adapter.ErrUnauthorized) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Storage authentication failed. Please login again."}, nil
		}
		fmt.Printf("SearchFiles error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to search files"}, nil
	}

	if files == nil {
		files = []adapter.FileMetadata{}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedTime.After(files[j].ModifiedTime)
	})
	if len(files) > limit {
		files = files[:limit]
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
