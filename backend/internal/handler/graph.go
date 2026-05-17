package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

// GraphHandler returns a lightweight knowledge-graph view of all the caller's
// notes. Body content is intentionally excluded to keep the payload small;
// consumers fetch bodies individually via GET /notes/{id}.
type GraphHandler struct {
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

func NewGraphHandler(provider adapter.StorageProvider, jwtSecret string) *GraphHandler {
	return &GraphHandler{storageProvider: provider, jwtSecret: jwtSecret}
}

// Graph handles GET /graph.
func (h *GraphHandler) Graph(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "unauthorized"}, nil
	}

	storage, err := h.storageProvider.GetAdapter(ctx, claims.UserID, claims.BaseFolderID)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to get storage adapter"}, nil
	}

	nodes, err := storage.Graph(ctx)
	if err != nil {
		fmt.Printf("Graph error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to build graph"}, nil
	}

	if nodes == nil {
		nodes = []adapter.GraphNode{}
	}

	body, err := json.Marshal(nodes)
	if err != nil {
		fmt.Printf("Graph marshal error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to encode graph"}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
