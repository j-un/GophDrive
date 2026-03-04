package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/auth"
	"github.com/jun/gophdrive/backend/internal/crypto"
	"golang.org/x/oauth2"
)

func TestAuthHandler_Refresh(t *testing.T) {
	secret := "test-secret"
	authService := auth.NewAuthService(nil, nil, "", crypto.NewMockEncryptor())
	storageProvider := memory.NewProvider(nil, authService)
	h := NewAuthHandler(authService, storageProvider, secret)

	userID := "test-user"
	// 1. Save a token for the user in DynamoDB (mocked)
	authService.SaveToken(context.Background(), userID, &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
	})

	// 2. Create an EXPIRED JWT for this user
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, _ := token.SignedString([]byte(secret))

	// 3. Call Refresh with the expired token in cookie
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Cookie": fmt.Sprintf("session_token=%s", expiredToken),
		},
	}
	resp, err := h.Refresh(context.Background(), req)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, resp.Body)
	}

	// 4. Verify new token is returned in body
	var data struct {
		Token string `json:"token"`
		ID    string `json:"id"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}

	if data.Token == "" {
		t.Errorf("Expected new token in body, got empty")
	}
	if data.ID != userID {
		t.Errorf("Expected userID %s, got %s", userID, data.ID)
	}

	// 5. Verify new token is valid and not expired
	newToken, err := jwt.Parse(data.Token, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Errorf("New token is invalid: %v", err)
	}
	if !newToken.Valid {
		t.Errorf("New token is not valid")
	}

	// 6. Verify Set-Cookie header is present
	foundSetCookie := false
	for k, v := range resp.MultiValueHeaders {
		if strings.EqualFold(k, "Set-Cookie") {
			for _, c := range v {
				if strings.Contains(c, "session_token=") {
					foundSetCookie = true
					if !strings.Contains(c, "Max-Age=2592000") {
						t.Errorf("Set-Cookie missing expected Max-Age, got %s", c)
					}
				}
			}
		}
	}
	if !foundSetCookie {
		t.Errorf("Set-Cookie header missing session_token")
	}
}

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
