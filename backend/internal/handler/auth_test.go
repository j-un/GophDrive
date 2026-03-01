package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/auth"
	"github.com/jun/gophdrive/backend/internal/crypto"
)

func TestDemoLogin_CreatesWelcomeNotes(t *testing.T) {
	// Setup dependencies with nil Dynamo but Mock KMS to avoid panics
	authService := auth.NewAuthService(nil, nil, "", crypto.NewMockEncryptor())
	storageProvider := memory.NewProvider(nil, authService)
	handler := NewAuthHandler(authService, storageProvider, "test-secret")

	// Execute DemoLogin
	ctx := context.Background()
	resp, err := handler.DemoLogin(ctx, events.APIGatewayProxyRequest{})
	if err != nil {
		t.Fatalf("DemoLogin failed: %v", err)
	}

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("Expected status 302, got %d. Body: %s", resp.StatusCode, resp.Body)
	}

	// Find the created user ID (should be one in the memory provider)
	// We can't directly access memory.Provider's internal map, but we know the userID prefix.
	// Let's assume the authService has one token saved.
	tokens := authService.GetTestTokens()
	if len(tokens) != 1 {
		t.Fatalf("Expected 1 user token, got %d", len(tokens))
	}

	var userID string
	for k := range tokens {
		userID = k
		break
	}

	// Get the adapter for this user
	adapter, err := storageProvider.GetAdapter(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get adapter for user %s: %v", userID, err)
	}

	// List files in root folder
	rootFolderID, err := authService.GetBaseFolderID(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get root folder ID: %v", err)
	}

	files, err := adapter.ListFiles(ctx, rootFolderID)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Check if both welcome notes exist (stripping .md because memory adapter does so in ListFiles)
	foundJP := false
	foundEN := false
	for _, f := range files {
		if f.Name == "ようこそ!" {
			foundJP = true
		}
		if f.Name == "Welcome!" {
			foundEN = true
		}
	}

	if !foundJP {
		t.Errorf("Japanese welcome note not found")
	}
	if !foundEN {
		t.Errorf("English welcome note not found")
	}
}
