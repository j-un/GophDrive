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
	"time"

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

// parseSearchTime parses a modifiedAfter/modifiedBefore query param. An empty
// string yields the zero time (no filter), with a nil error. Accepts either
// an RFC3339 timestamp or a bare YYYY-MM-DD date (interpreted as UTC midnight).
func parseSearchTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q: want RFC3339 or YYYY-MM-DD", s)
}

// Search handles GET /search?q=...&tag=...&tag=...&type=...&modifiedAfter=...&modifiedBefore=...&limit=N
// At least one of q, tag or type must be provided; modifiedAfter/modifiedBefore
// are modifiers only and cannot satisfy that requirement alone.
// Results are ranked by relevance Score descending, then ModifiedTime descending,
// then ID ascending, and capped at limit (default 20, max 100).
func (h *SearchHandler) Search(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	query := req.QueryStringParameters["q"]
	tags := req.MultiValueQueryStringParameters["tag"]
	noteType := req.QueryStringParameters["type"]

	if query == "" && len(tags) == 0 && noteType == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "At least one of 'q', 'tag' or 'type' is required"}, nil
	}

	after, err := parseSearchTime(req.QueryStringParameters["modifiedAfter"])
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "invalid modifiedAfter/modifiedBefore: want RFC3339 or YYYY-MM-DD"}, nil
	}
	before, err := parseSearchTime(req.QueryStringParameters["modifiedBefore"])
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "invalid modifiedAfter/modifiedBefore: want RFC3339 or YYYY-MM-DD"}, nil
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

	files, err := storage.SearchFilesWithTags(ctx, query, tags, scope, noteType)
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

	// Date window is a post-filter, applied before truncation to limit so the
	// cap doesn't cut into results the window would otherwise exclude anyway.
	if !after.IsZero() || !before.IsZero() {
		filtered := make([]adapter.FileMetadata, 0, len(files))
		for _, f := range files {
			if (after.IsZero() || !f.ModifiedTime.Before(after)) && (before.IsZero() || f.ModifiedTime.Before(before)) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	// Rank by relevance first; tag-only/type-only queries leave every Score at
	// 0, so ModifiedTime desc (then ID asc for full determinism) preserves
	// today's recency ordering for those cases.
	sort.Slice(files, func(i, j int) bool {
		if files[i].Score != files[j].Score {
			return files[i].Score > files[j].Score
		}
		if !files[i].ModifiedTime.Equal(files[j].ModifiedTime) {
			return files[i].ModifiedTime.After(files[j].ModifiedTime)
		}
		return files[i].ID < files[j].ID
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
