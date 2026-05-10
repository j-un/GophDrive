package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/auth"
	"golang.org/x/oauth2"
)

// OAuthExchanger covers the methods of *oauth2.Config the handler uses,
// so tests can inject a fake exchanger without hitting the network.
type OAuthExchanger interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
}

// AuthHandlerDeps groups all dependencies of AuthHandler. Construct via
// NewAuthHandler — leaving fields zero will produce a handler that fails
// fast at runtime.
type AuthHandlerDeps struct {
	Exchanger       OAuthExchanger
	Verifier        auth.Verifier
	StorageProvider adapter.StorageProvider
	JWTSecret       string
	AllowedEmails   []string
	FrontendURL     string
	DevMode         bool
	// SessionTTL is the lifetime of an issued session token. Zero defaults to 24h.
	SessionTTL time.Duration
	// DemoSessionTTL is the lifetime of demo-login JWTs. Zero defaults to 1h.
	DemoSessionTTL time.Duration
	// RootFolderName is the display name used when auto-creating each user's
	// notes root folder on first login. Zero defaults to "GophDrive".
	RootFolderName string
}

// AuthHandler handles authentication requests.
type AuthHandler struct {
	deps AuthHandlerDeps
}

// NewAuthHandler creates a new AuthHandler with sensible defaults filled in.
func NewAuthHandler(deps AuthHandlerDeps) *AuthHandler {
	if deps.SessionTTL == 0 {
		deps.SessionTTL = 24 * time.Hour
	}
	if deps.DemoSessionTTL == 0 {
		deps.DemoSessionTTL = 1 * time.Hour
	}
	if deps.RootFolderName == "" {
		deps.RootFolderName = "GophDrive"
	}
	if deps.FrontendURL == "" {
		deps.FrontendURL = "http://localhost:3000"
	}
	return &AuthHandler{deps: deps}
}

// isEmailAllowed checks if the given email is allowed to login.
// If allowedEmails is empty, all emails are allowed.
func isEmailAllowed(email string, allowedEmails []string) bool {
	if len(allowedEmails) == 0 {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range allowedEmails {
		if strings.ToLower(strings.TrimSpace(allowed)) == email {
			return true
		}
	}
	return false
}

func (h *AuthHandler) sameSite() string {
	if h.deps.DevMode {
		return "Lax"
	}
	return "None"
}

func (h *AuthHandler) sessionCookie(token string, maxAge int) string {
	return fmt.Sprintf("session_token=%s; HttpOnly; Path=/; Max-Age=%d; SameSite=%s; Secure", token, maxAge, h.sameSite())
}

// signSession produces a signed JWT for the given session claims.
func (h *AuthHandler) signSession(c SessionClaims, ttl time.Duration) (string, error) {
	mc := jwt.MapClaims{
		"sub":            c.UserID,
		"email":          c.Email,
		"name":           c.Name,
		"base_folder_id": c.BaseFolderID,
		"exp":            time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString([]byte(h.deps.JWTSecret))
}

// Login redirects the browser into Google's OAuth consent screen.
func (h *AuthHandler) Login(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// TODO: Generate a secure random state and store it in a cookie to prevent CSRF
	state := "random-state"
	url := h.deps.Exchanger.AuthCodeURL(state)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": url,
		},
	}, nil
}

// Callback handles the OAuth2 redirect from Google. It validates the returned
// ID token, ensures the user has a root folder, and issues a session JWT.
func (h *AuthHandler) Callback(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	code := req.QueryStringParameters["code"]
	if code == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing code"}, nil
	}

	token, err := h.deps.Exchanger.Exchange(ctx, code)
	if err != nil {
		fmt.Printf("Exchange error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to exchange code"}, nil
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "ID token missing from token response"}, nil
	}

	claims, err := h.deps.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		fmt.Printf("Verify error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Invalid ID token"}, nil
	}

	if !isEmailAllowed(claims.Email, h.deps.AllowedEmails) {
		fmt.Printf("Access denied for email: %s\n", claims.Email)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusForbidden, Body: "Access denied: your email is not authorized"}, nil
	}

	storage, err := h.deps.StorageProvider.GetAdapter(ctx, claims.Subject, "")
	if err != nil {
		fmt.Printf("GetAdapter error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get storage adapter"}, nil
	}

	rootFolderID, err := storage.EnsureRootFolder(ctx, h.deps.RootFolderName)
	if err != nil {
		fmt.Printf("EnsureRootFolder error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to ensure root folder"}, nil
	}

	signed, err := h.signSession(SessionClaims{
		UserID:       claims.Subject,
		Email:        claims.Email,
		Name:         claims.Name,
		BaseFolderID: rootFolderID,
	}, h.deps.SessionTTL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to sign token"}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": fmt.Sprintf("%s/?success=true", h.deps.FrontendURL),
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {h.sessionCookie(signed, 30*24*60*60)},
		},
	}, nil
}

// Refresh re-issues a session JWT preserving claims. The previous JWT may be
// expired but must verify against the current signing key.
func (h *AuthHandler) Refresh(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	tokenString := GetTokenString(req)
	if tokenString == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "No token found"}, nil
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(h.deps.JWTSecret), nil
	})
	// Only "expired" is acceptable; any other failure means the token isn't ours.
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: fmt.Sprintf("Invalid token: %v", err)}, nil
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Invalid token claims"}, nil
	}

	sub, _ := mc["sub"].(string)
	if sub == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Missing subject in token"}, nil
	}

	sc := SessionClaims{UserID: sub}
	if v, ok := mc["email"].(string); ok {
		sc.Email = v
	}
	if v, ok := mc["name"].(string); ok {
		sc.Name = v
	}
	if v, ok := mc["base_folder_id"].(string); ok {
		sc.BaseFolderID = v
	}

	signed, err := h.signSession(sc, h.deps.SessionTTL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to sign token"}, nil
	}

	response := map[string]interface{}{
		"id":             sc.UserID,
		"base_folder_id": sc.BaseFolderID,
		"token":          signed,
	}
	body, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {h.sessionCookie(signed, 30*24*60*60)},
		},
	}, nil
}

// GetUser returns the authenticated user's profile, derived directly from JWT
// claims (no DB lookup needed).
func (h *AuthHandler) GetUser(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.deps.JWTSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	profile := map[string]string{
		"id":             claims.UserID,
		"base_folder_id": claims.BaseFolderID,
	}
	body, _ := json.Marshal(profile)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// DemoLogin issues a temporary JWT without Google OAuth, seeding a demo user
// with a fresh root folder and welcome notes.
func (h *AuthHandler) DemoLogin(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID := fmt.Sprintf("demo-user-%s", uuid.New().String())
	email := "demo@gophdrive.local"

	storage, err := h.deps.StorageProvider.GetAdapter(ctx, userID, "")
	if err != nil {
		fmt.Printf("DemoLogin GetAdapter error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get storage adapter"}, nil
	}

	rootFolderID, err := storage.EnsureRootFolder(ctx, "Demo Notes")
	if err != nil {
		fmt.Printf("DemoLogin EnsureRootFolder error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to create root folder"}, nil
	}

	for _, note := range demoWelcomeNotes() {
		if _, err := storage.CreateFile(ctx, note.Name, []byte(note.Content), rootFolderID); err != nil {
			fmt.Printf("DemoLogin CreateFile (%s) error: %v\n", note.Name, err)
		}
	}

	signed, err := h.signSession(SessionClaims{
		UserID:       userID,
		Email:        email,
		Name:         "Demo User",
		BaseFolderID: rootFolderID,
	}, h.deps.DemoSessionTTL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to sign token"}, nil
	}

	cookie := fmt.Sprintf("session_token=%s; HttpOnly; Path=/; Max-Age=2592000; SameSite=Lax; Secure", signed)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": fmt.Sprintf("%s/?token=%s", h.deps.FrontendURL, signed),
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {cookie},
		},
	}, nil
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	cookie := fmt.Sprintf("session_token=; HttpOnly; Path=/; Max-Age=0; SameSite=%s; Secure", h.sameSite())

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"success":true}`,
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {cookie},
		},
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
