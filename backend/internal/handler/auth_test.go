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
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
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

// testOAuthState is the canonical state value used by callbackReq helpers.
const testOAuthState = "test-state-AAAA1111"

// callbackReq builds an API Gateway request shaped for AuthHandler.Callback.
// Empty arguments are omitted so individual fields can be exercised in isolation.
func callbackReq(state, cookie, code string) events.APIGatewayProxyRequest {
	qs := map[string]string{}
	if state != "" {
		qs["state"] = state
	}
	if code != "" {
		qs["code"] = code
	}
	headers := map[string]string{}
	if cookie != "" {
		headers["Cookie"] = "oauth_state=" + cookie
	}
	return events.APIGatewayProxyRequest{QueryStringParameters: qs, Headers: headers}
}

func newAuthHandler(t *testing.T, deps handler.AuthHandlerDeps) *handler.AuthHandler {
	t.Helper()
	if deps.JWTSecret == "" {
		deps.JWTSecret = "test-secret"
	}
	if deps.StorageProvider == nil {
		deps.StorageProvider = dynamo.NewProvider(nil)
	}
	if deps.FrontendURL == "" {
		deps.FrontendURL = "http://test"
	}
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

			req := callbackReq(testOAuthState, testOAuthState, "abc")
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

	cookies := resp.MultiValueHeaders["Set-Cookie"]
	if len(cookies) != 1 || !strings.HasPrefix(cookies[0], "oauth_state=") {
		t.Fatalf("expected oauth_state Set-Cookie, got %v", cookies)
	}
	cookie := cookies[0]
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Secure", "Path=/", "Max-Age=300"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("oauth_state cookie missing %q: %s", want, cookie)
		}
	}

	cookieState := extractCookieValue(t, cookie, "oauth_state")
	if cookieState == "" {
		t.Fatal("oauth_state cookie value is empty")
	}
	// fakeExchanger.AuthCodeURL echoes state into ?state=<v>; the URL state must
	// match the cookie so Callback's double-submit check can succeed.
	parsedURL, err := url.Parse(resp.Headers["Location"])
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if got := parsedURL.Query().Get("state"); got != cookieState {
		t.Errorf("URL state = %q, cookie state = %q; want them to match", got, cookieState)
	}
}

// State must be unpredictable; two consecutive Login calls must not collide.
func TestAuthHandler_Login_StateIsRandom(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{url: "https://accounts.google.com/o/oauth2/auth"},
		Verifier:  &fakeVerifier{},
	})

	resp1, _ := h.Login(context.Background(), events.APIGatewayProxyRequest{})
	resp2, _ := h.Login(context.Background(), events.APIGatewayProxyRequest{})
	s1 := extractCookieValue(t, resp1.MultiValueHeaders["Set-Cookie"][0], "oauth_state")
	s2 := extractCookieValue(t, resp2.MultiValueHeaders["Set-Cookie"][0], "oauth_state")
	if s1 == s2 {
		t.Errorf("consecutive Login calls produced identical state %q — must be unpredictable", s1)
	}
	if len(s1) < 32 {
		t.Errorf("state %q is too short (%d chars); want >=32 for 256-bit entropy", s1, len(s1))
	}
}

func TestAuthHandler_Callback_Success(t *testing.T) {
	verified := &auth.Claims{Subject: "google-sub-42", Email: "user@example.com", Name: "Alice"}
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger:   &fakeExchanger{token: tokenWithIDToken("opaque")},
		Verifier:    &fakeVerifier{claims: verified},
		FrontendURL: "https://app.example",
	})

	resp, err := h.Callback(context.Background(), callbackReq(testOAuthState, testOAuthState, "abc"))
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
	if len(cookies) != 2 {
		t.Fatalf("expected session_token + oauth_state-clear cookies, got %v", cookies)
	}
	if !strings.Contains(cookies[0], "session_token=") {
		t.Fatalf("first Set-Cookie should be session_token, got %q", cookies[0])
	}
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Secure"} {
		if !strings.Contains(cookies[0], want) {
			t.Errorf("session_token cookie missing %q: %s", want, cookies[0])
		}
	}
	// Second cookie must clear oauth_state (Max-Age=0) so the state can't be
	// replayed on a stale browser tab.
	if !strings.HasPrefix(cookies[1], "oauth_state=;") || !strings.Contains(cookies[1], "Max-Age=0") {
		t.Errorf("expected oauth_state cleared with Max-Age=0, got %q", cookies[1])
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

	// Valid state but no code — exercise the missing-code branch specifically
	// rather than the state-mismatch branch (both return 400).
	resp, _ := h.Callback(context.Background(), callbackReq(testOAuthState, testOAuthState, ""))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "Missing code") {
		t.Errorf("body = %q, want Missing code branch", resp.Body)
	}
}

// Callback must reject requests where the state query param is missing.
func TestAuthHandler_Callback_MissingState(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), callbackReq("", testOAuthState, "abc"))
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(resp.Body, "Invalid state") {
		t.Errorf("status=%d body=%q, want 400 Invalid state", resp.StatusCode, resp.Body)
	}
}

// Callback must reject requests where the oauth_state cookie is missing,
// even when the URL carries a state parameter.
func TestAuthHandler_Callback_MissingStateCookie(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), callbackReq(testOAuthState, "", "abc"))
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(resp.Body, "Invalid state") {
		t.Errorf("status=%d body=%q, want 400 Invalid state", resp.StatusCode, resp.Body)
	}
}

// Callback must reject when the URL state and cookie state diverge.
func TestAuthHandler_Callback_StateMismatch(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{token: tokenWithIDToken("opaque")},
		Verifier:  &fakeVerifier{claims: &auth.Claims{Subject: "x", Email: "y@z"}},
	})

	resp, _ := h.Callback(context.Background(), callbackReq("attacker-state", testOAuthState, "abc"))
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(resp.Body, "Invalid state") {
		t.Errorf("status=%d body=%q, want 400 Invalid state", resp.StatusCode, resp.Body)
	}
}

func TestAuthHandler_Callback_ExchangeError(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{err: errors.New("network down")},
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), callbackReq(testOAuthState, testOAuthState, "abc"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAuthHandler_Callback_MissingIDToken(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{token: &oauth2.Token{AccessToken: "x"}}, // no id_token extra
		Verifier:  &fakeVerifier{},
	})

	resp, _ := h.Callback(context.Background(), callbackReq(testOAuthState, testOAuthState, "abc"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAuthHandler_Callback_InvalidIDToken(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{token: tokenWithIDToken("opaque")},
		Verifier:  &fakeVerifier{err: errors.New("bad signature")},
	})

	resp, _ := h.Callback(context.Background(), callbackReq(testOAuthState, testOAuthState, "abc"))
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

// Demo sessions are deliberately ephemeral (1h JWT, 60min data TTL on DDB).
// Refresh must refuse them so users can't accidentally extend a demo
// session into an authenticated-but-empty state.
func TestAuthHandler_Refresh_RejectsDemoUser(t *testing.T) {
	h := newAuthHandler(t, handler.AuthHandlerDeps{
		Exchanger: &fakeExchanger{},
		Verifier:  &fakeVerifier{},
	})

	demoToken := signClaims(t, "test-secret", jwt.MapClaims{
		"sub":            "demo-user-abc123",
		"email":          "demo@gophdrive.local",
		"base_folder_id": "demo-folder",
		"exp":            time.Now().Add(-1 * time.Minute).Unix(),
	})

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{"Authorization": "Bearer " + demoToken},
	}
	resp, err := h.Refresh(context.Background(), req)
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for demo refresh", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(resp.Body), "demo") {
		t.Errorf("body = %q, want a message that mentions demo so frontend logs/UX is unambiguous", resp.Body)
	}
	// No Set-Cookie header on refusal — otherwise the browser's stored cookie
	// (Max-Age=30d) would persist and make the failure look like a flake.
	if cookies := resp.MultiValueHeaders["Set-Cookie"]; len(cookies) != 0 {
		t.Errorf("rejection should not Set-Cookie, got %v", cookies)
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
	provider := dynamo.NewProvider(nil)
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
