package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
)

// APIKeyHandler handles API key lifecycle for programmatic GophDrive access.
type APIKeyHandler struct {
	store      apikey.Store
	jwtSecret  string
	RandReader io.Reader // defaults to crypto/rand.Reader; override in tests
}

// NewAPIKeyHandler creates an APIKeyHandler.
func NewAPIKeyHandler(store apikey.Store, jwtSecret string) *APIKeyHandler {
	return &APIKeyHandler{store: store, jwtSecret: jwtSecret, RandReader: rand.Reader}
}

type issueResponse struct {
	Key       string `json:"key"`
	KeyPrefix string `json:"key_prefix"`
}

type statusResponse struct {
	HasKey        bool   `json:"has_key"`
	KeyPrefix     string `json:"key_prefix,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	FirstIssuedAt int64  `json:"first_issued_at,omitempty"`
}

// Issue generates and persists a new API key for the authenticated user.
// Demo sessions are rejected with 403. Re-issuing atomically replaces the
// previous key. The plaintext is returned once and never stored.
func (h *APIKeyHandler) Issue(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return jsonError(http.StatusUnauthorized, "unauthorized"), nil
	}
	if IsDemoUserID(claims.UserID) {
		return jsonError(http.StatusForbidden, "API keys are not available for demo sessions"), nil
	}

	raw := make([]byte, 32)
	if _, err := h.RandReader.Read(raw); err != nil {
		log.Printf("api_key Issue: rand.Read: %v", err)
		return jsonError(http.StatusInternalServerError, "failed to generate key"), nil
	}
	plaintext := hex.EncodeToString(raw) // 64 hex chars
	prefix := plaintext[:8]
	hash := apikey.HashKey(plaintext)

	if err := h.store.Issue(ctx, claims.UserID, claims.BaseFolderID, hash, prefix); err != nil {
		log.Printf("api_key Issue: store: %v", err)
		return jsonError(http.StatusInternalServerError, "failed to store key"), nil
	}

	body, _ := json.Marshal(issueResponse{Key: plaintext, KeyPrefix: prefix})
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

// Get returns whether the user has an active API key and its prefix/created_at.
// The plaintext key is never returned.
func (h *APIKeyHandler) Get(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return jsonError(http.StatusUnauthorized, "unauthorized"), nil
	}

	hasKey, prefix, createdAt, firstIssuedAt, err := h.store.StatusFor(ctx, claims.UserID)
	if err != nil {
		log.Printf("api_key Get: store: %v", err)
		return jsonError(http.StatusInternalServerError, "failed to read key status"), nil
	}

	resp := statusResponse{HasKey: hasKey}
	if hasKey {
		resp.KeyPrefix = prefix
		resp.CreatedAt = createdAt
		resp.FirstIssuedAt = firstIssuedAt
	}
	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

// Delete revokes the user's API key. No-op if no key exists.
func (h *APIKeyHandler) Delete(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return jsonError(http.StatusUnauthorized, "unauthorized"), nil
	}

	if err := h.store.Revoke(ctx, claims.UserID); err != nil {
		log.Printf("api_key Delete: store: %v", err)
		return jsonError(http.StatusInternalServerError, "failed to revoke key"), nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent}, nil
}

func jsonError(code int, msg string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return events.APIGatewayProxyResponse{StatusCode: code, Body: string(body)}
}
