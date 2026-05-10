package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/auth"
	"github.com/jun/gophdrive/backend/internal/handler"
	"golang.org/x/oauth2"
)

// fakeExchanger satisfies handler.OAuthExchanger without hitting Google.
type fakeExchanger struct {
	url   string
	token *oauth2.Token
	err   error
}

func (f *fakeExchanger) AuthCodeURL(state string, _ ...oauth2.AuthCodeOption) string {
	return f.url + "?state=" + url.QueryEscape(state)
}

func (f *fakeExchanger) Exchange(_ context.Context, _ string, _ ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return f.token, f.err
}

// fakeVerifier satisfies auth.Verifier so callback tests bypass Google's JWKS.
type fakeVerifier struct {
	claims *auth.Claims
	err    error
}

func (f *fakeVerifier) Verify(_ context.Context, _ string) (*auth.Claims, error) {
	return f.claims, f.err
}

// tokenWithIDToken wraps an oauth2.Token to attach an id_token extra value.
func tokenWithIDToken(idToken string) *oauth2.Token {
	t := &oauth2.Token{AccessToken: "access"}
	return t.WithExtra(map[string]interface{}{"id_token": idToken})
}

func newAuthHandler(t *testing.T, deps handler.AuthHandlerDeps) *handler.AuthHandler {
	t.Helper()
	if deps.JWTSecret == "" {
		deps.JWTSecret = "test-secret"
	}
	if deps.StorageProvider == nil {
		deps.StorageProvider = memory.NewProvider(nil)
	}
	if deps.FrontendURL == "" {
		deps.FrontendURL = "http://test"
	}
	deps.DevMode = true
	return handler.NewAuthHandler(deps)
}

func TestIsEmailAllowed(t *testing.T) {
	// isEmailAllowed is unexported; tested indirectly through Callback.
	// Each subtest configures the allow list and verifies the response code.
	verified := &auth.Claims{Subject: "google-1", Email: "user@example.com", Name: "User"}

	tests := []struct {
		name       string
		allowed    []string
		email      string
		wantStatus int
	}{
		{name: "empty list allows all", allowed: nil, email: "anyone@example.com", wantStatus: http.StatusFound},
		{name: "matching email is allowed", allowed: []string{"user@example.com"}, email: "user@example.com", wantStatus: http.StatusFound},
		{name: "non-matching email is denied", allowed: []string{"user@example.com"}, email: "intruder@example.com", wantStatus: http.StatusForbidden},
		{name: "case insensitive match", allowed: []string{"user@example.com"}, email: "USER@Example.COM", wantStatus: http.StatusFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := *verified
			c.Email = tt.email
			h := newAuthHandler(t, handler.AuthHandlerDeps{
				Exchanger:     &fakeExchanger{token: tokenWithIDToken("opaque")},
				Verifier:      &fakeVerifier{claims: &c},
				AllowedEmails: tt.allowed,
			})

			req := events.APIGatewayProxyRequest{QueryStringParameters: map[string]string{"code": "abc"}}
			resp, err := h.Callback(context.Background(), req)
			if err != nil {
				t.Fatalf("Callback error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d, body=%q", resp.StatusCode, tt.wantStatus, resp.Body)
			}
		})
	}
}

func TestAuthHandler_Login_RedirectsToOAuthURL(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{url: "https://accounts.google.com/o/oauth2/auth"},
		Verifier:  &fakeVerifier{},
	})

	resp, err := h.Login(context.Background(), events.APIGatewayProxyRequest{})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Headers["Location"], "https://accounts.google.com/") {
		t.Errorf("Location header = %q, want Google auth URL", resp.Headers["Location"])
	}
}

func TestAuthHandler_Callback_Success(t *testing.T) {
	verified := &auth.Claims{Subject: "google-sub-42", Email: "user@example.com", Name: "Alice"}
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger:   &fakeExchanger{token: tokenWithIDToken("opaque")},
		Verifier:    &fakeVerifier{claims: verified},
		FrontendURL: "https://app.example",
	})

	resp, err := h.Callback(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{"code": "abc"},
	})
	if err != nil {
		t.Fatalf("Callback error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302. body=%s", resp.StatusCode, resp.Body)
	}
	if got := resp.Headers["Location"]; got != "https://app.example/?success=true" {
		t.Errorf("Location = %q, want app redirect", got)
	}

	cookies := resp.MultiValueHeaders["Set-Cookie"]
	if len(cookies) != 1 || !strings.Contains(cookies[0], "session_token=") {
		t.Fatalf("expected session_token cookie, got %v", cookies)
	}

	// JWT must carry sub and a non-empty base_folder_id (auto-created at login).
	tokStr := extractCookieValue(t, cookies[0], "session_token")
	claims := parseClaims(t, tokStr, "test-secret")
	if claims["sub"] != "google-sub-42" {
		t.Errorf("sub = %v, want google-sub-42", claims["sub"])
	}
	if bf, _ := claims["base_folder_id"].(string); bf == "" {
		t.Errorf("base_folder_id is empty; want auto-minted folder id")
	}
}

func TestAuthHandler_Callback_MissingCode(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), events.APIGatewayProxyRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAuthHandler_Callback_ExchangeError(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{err: errors.New("network down")},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{"code": "abc"},
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAuthHandler_Callback_MissingIDToken(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{token: &oauth2.Token{AccessToken: "x"}}, // no id_token extra
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{"code": "abc"},
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAuthHandler_Callback_InvalidIDToken(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{token: tokenWithIDToken("opaque")},
		Verifier:  &fakeVerifier{err: errors.New("bad signature")},
	})

	resp, _ := h.Callback(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{"code": "abc"},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthHandler_Refresh_PreservesClaims(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	expired := signClaims(t, "test-secret", jwt.MapClaims{
		"sub":            "user-7",
		"email":          "user@example.com",
		"base_folder_id": "folder-xyz",
		"exp":            time.Now().Add(-1 * time.Hour).Unix(),
	})

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{"Authorization": "Bearer " + expired},
	}
	resp, err := h.Refresh(context.Background(), req)
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", resp.StatusCode, resp.Body)
	}

	var body struct {
		ID           string `json:"id"`
		BaseFolderID string `json:"base_folder_id"`
		Token        string `json:"token"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.ID != "user-7" {
		t.Errorf("id = %q, want user-7", body.ID)
	}
	if body.BaseFolderID != "folder-xyz" {
		t.Errorf("base_folder_id = %q, want folder-xyz", body.BaseFolderID)
	}

	// New token is valid, unexpired, and re-carries the original base_folder_id.
	claims := parseClaims(t, body.Token, "test-secret")
	if claims["base_folder_id"] != "folder-xyz" {
		t.Errorf("new token base_folder_id = %v, want folder-xyz", claims["base_folder_id"])
	}
}

func TestAuthHandler_Refresh_Unauthorized(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	t.Run("no token", func(t *testing.T) {
		resp, _ := h.Refresh(context.Background(), events.APIGatewayProxyRequest{})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("bad signature", func(t *testing.T) {
		bad := signClaims(t, "other-secret", jwt.MapClaims{"sub": "user-1"})
		req := events.APIGatewayProxyRequest{Headers: map[string]string{"Authorization": "Bearer " + bad}}
		resp, _ := h.Refresh(context.Background(), req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})
}

func TestAuthHandler_GetUser_FromJWTClaims(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	tokStr := signClaims(t, "test-secret", jwt.MapClaims{
		"sub":            "user-9",
		"base_folder_id": "fld",
		"exp":            time.Now().Add(1 * time.Hour).Unix(),
	})
	req := events.APIGatewayProxyRequest{Headers: map[string]string{"Authorization": "Bearer " + tokStr}}

	resp, err := h.GetUser(context.Background(), req)
	if err != nil {
		t.Fatalf("GetUser error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", resp.StatusCode, resp.Body)
	}

	var body struct {
		ID           string `json:"id"`
		BaseFolderID string `json:"base_folder_id"`
	}
	json.Unmarshal([]byte(resp.Body), &body)
	if body.ID != "user-9" || body.BaseFolderID != "fld" {
		t.Errorf("body = %+v, want id=user-9 base_folder_id=fld", body)
	}
}

func TestAuthHandler_GetUser_Unauthorized(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.GetUser(context.Background(), events.APIGatewayProxyRequest{})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthHandler_DemoLogin_SeedsRootFolderAndWelcomeNotes(t *testing.T) {
	provider := memory.NewProvider(nil)
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger:       &fakeExchanger{},
		Verifier:        &fakeVerifier{},
		StorageProvider: provider,
		FrontendURL:     "http://demo",
	})

	resp, err := h.DemoLogin(context.Background(), events.APIGatewayProxyRequest{})
	if err != nil {
		t.Fatalf("DemoLogin error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302. body=%s", resp.StatusCode, resp.Body)
	}

	loc := resp.Headers["Location"]
	if !strings.HasPrefix(loc, "http://demo/?token=") {
		t.Fatalf("Location = %q, want demo redirect carrying token", loc)
	}
	tokStr := strings.TrimPrefix(loc, "http://demo/?token=")
	claims := parseClaims(t, tokStr, "test-secret")
	userID, _ := claims["sub"].(string)
	baseFolderID, _ := claims["base_folder_id"].(string)
	if !strings.HasPrefix(userID, "demo-user-") {
		t.Errorf("sub = %q, want demo-user- prefix", userID)
	}
	if baseFolderID == "" {
		t.Errorf("base_folder_id is empty")
	}

	// Confirm welcome notes were seeded under the minted folder.
	storage, _ := provider.GetAdapter(context.Background(), userID, baseFolderID)
	files, err := storage.ListFiles(context.Background(), baseFolderID)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 welcome notes, got %d (%v)", len(files), files)
	}
}

func TestAuthHandler_Logout_ClearsCookie(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	resp, err := h.Logout(context.Background(), events.APIGatewayProxyRequest{})
	if err != nil {
		t.Fatalf("Logout error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cookies := resp.MultiValueHeaders["Set-Cookie"]
	if len(cookies) != 1 || !strings.Contains(cookies[0], "Max-Age=0") {
		t.Errorf("Set-Cookie missing Max-Age=0: %v", cookies)
	}
}

func signClaims(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func parseClaims(t *testing.T, tokStr, secret string) jwt.MapClaims {
	t.Helper()
	tok, err := jwt.Parse(tokStr, func(*jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("parse jwt: %v", err)
	}
	c, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims not MapClaims")
	}
	return c
}

func extractCookieValue(t *testing.T, cookie, name string) string {
	t.Helper()
	prefix := name + "="
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	t.Fatalf("cookie %q not found in %q", name, cookie)
	return ""
}

// Suppress unused warning for fmt — kept for potential debug prints.
var _ = fmt.Sprintf
