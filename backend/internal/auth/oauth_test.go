package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jun/gophdrive/backend/internal/crypto"
	"github.com/jun/gophdrive/backend/internal/model"
	"golang.org/x/oauth2"
)

func testAuthService() *AuthService {
	return NewAuthService(
		&oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		nil, // No DynamoDB client — uses in-memory fallback
		"test-tokens-table",
		crypto.NewMockEncryptor(),
	)
}

func TestAuthService_SaveAndGetUserToken(t *testing.T) {
	s := testAuthService()
	ctx := context.Background()

	token := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	err := s.SaveToken(ctx, "user1", token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	saved, err := s.GetUserToken(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUserToken failed: %v", err)
	}
	if saved.UserID != "user1" {
		t.Errorf("Expected user ID 'user1', got '%s'", saved.UserID)
	}
	// MockEncryptor prefixes with "mock:"
	if saved.EncryptedRefreshToken != "mock:refresh-456" {
		t.Errorf("Expected encrypted token 'mock:refresh-456', got '%s'", saved.EncryptedRefreshToken)
	}
}

func TestAuthService_GetUserToken_NotFound(t *testing.T) {
	s := testAuthService()
	ctx := context.Background()

	_, err := s.GetUserToken(ctx, "nonexistent-user")
	if err == nil {
		t.Error("Expected error for non-existing user, got nil")
	}
}

func TestAuthService_UpdateBaseFolderID(t *testing.T) {
	s := testAuthService()
	ctx := context.Background()

	// First, save a token
	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	s.SaveToken(ctx, "user1", token)

	// Update base folder ID
	err := s.UpdateBaseFolderID(ctx, "user1", "folder-abc")
	if err != nil {
		t.Fatalf("UpdateBaseFolderID failed: %v", err)
	}

	// Verify
	saved, _ := s.GetUserToken(ctx, "user1")
	if saved.BaseFolderID != "folder-abc" {
		t.Errorf("Expected BaseFolderID 'folder-abc', got '%s'", saved.BaseFolderID)
	}
}

func TestAuthService_SaveToken_PreservesBaseFolderID(t *testing.T) {
	s := testAuthService()
	ctx := context.Background()

	// Save initial token
	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh-1",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	s.SaveToken(ctx, "user1", token)

	// Set base folder ID
	s.UpdateBaseFolderID(ctx, "user1", "my-folder")

	// Save new token (should preserve BaseFolderID)
	newToken := &oauth2.Token{
		AccessToken:  "new-access",
		RefreshToken: "refresh-2",
		Expiry:       time.Now().Add(2 * time.Hour),
	}
	s.SaveToken(ctx, "user1", newToken)

	// Verify BaseFolderID is preserved
	saved, _ := s.GetUserToken(ctx, "user1")
	if saved.BaseFolderID != "my-folder" {
		t.Errorf("Expected BaseFolderID 'my-folder' to be preserved, got '%s'", saved.BaseFolderID)
	}
	if saved.EncryptedRefreshToken != "mock:refresh-2" {
		t.Errorf("Expected updated token, got '%s'", saved.EncryptedRefreshToken)
	}
}

func TestAuthService_GetAuthURL(t *testing.T) {
	s := testAuthService()

	url := s.GenerateAuthURL("test-state")
	if url == "" {
		t.Error("Expected non-empty auth URL")
	}
	if !contains(url, "test-state") {
		t.Errorf("Expected URL to contain state, got '%s'", url)
	}
	if !contains(url, "test-client-id") {
		t.Errorf("Expected URL to contain client ID, got '%s'", url)
	}
}

func TestAuthService_SaveToken_EmptyRefreshToken(t *testing.T) {
	s := testAuthService()
	ctx := context.Background()

	// First save with a refresh token
	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "original-refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	s.SaveToken(ctx, "user1", token)

	// Save again with empty refresh token
	noRefreshToken := &oauth2.Token{
		AccessToken:  "new-access",
		RefreshToken: "", // empty
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	s.SaveToken(ctx, "user1", noRefreshToken)

	// The original refresh token should be preserved
	saved, _ := s.GetUserToken(ctx, "user1")
	if saved.EncryptedRefreshToken != "mock:original-refresh" {
		t.Errorf("Expected original refresh token to be preserved, got '%s'", saved.EncryptedRefreshToken)
	}
}

func TestAuthService_InMemoryTokenStore(t *testing.T) {
	// Verify that with nil dynamoClient, tokens are stored in-memory
	s := testAuthService()
	ctx := context.Background()

	// Save multiple users
	for i, uid := range []string{"u1", "u2", "u3"} {
		token := &oauth2.Token{
			AccessToken:  "access",
			RefreshToken: "refresh-" + uid,
			Expiry:       time.Now().Add(1 * time.Hour),
		}
		err := s.SaveToken(ctx, uid, token)
		if err != nil {
			t.Fatalf("SaveToken for user %d failed: %v", i, err)
		}
	}

	// Retrieve all
	for _, uid := range []string{"u1", "u2", "u3"} {
		saved, err := s.GetUserToken(ctx, uid)
		if err != nil {
			t.Fatalf("GetUserToken for %s failed: %v", uid, err)
		}
		expected := model.UserToken{
			UserID:                uid,
			EncryptedRefreshToken: "mock:refresh-" + uid,
		}
		if saved.UserID != expected.UserID {
			t.Errorf("Expected UserID '%s', got '%s'", expected.UserID, saved.UserID)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
