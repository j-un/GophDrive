package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const csrfFrontendURL = "http://localhost:5173"

// newCSRFTestApp builds a minimal App for CSRF tests.
// apiGatewaySecret is intentionally empty so the X-Origin-Verify gate passes
// (both sides are "" → condition false → not blocked).
func newCSRFTestApp() *App {
	p := dynamo.NewProvider(nil)
	store := apikey.NewInMemoryStore()
	return &App{
		noteHandler:   handler.NewNoteHandler(p, "csrf-secret"),
		searchHandler: handler.NewSearchHandler(p, "csrf-secret"),
		exportHandler: handler.NewExportHandler(p, "csrf-secret"),
		tagHandler:    handler.NewTagHandler(p, "csrf-secret"),
		graphHandler:  handler.NewGraphHandler(p, "csrf-secret"),
		apiKeyHandler: handler.NewAPIKeyHandler(store, "csrf-secret"),
		apiKeys:       store,
		jwtSecret:     "csrf-secret",
		frontendURL:   csrfFrontendURL,
	}
}

func TestCSRF_GETPassesWithoutOrigin(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{},
	})
	// 401 because no auth — but NOT 403 (CSRF gate does not apply to GET)
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("GET without Origin: want non-403, got 403 (CSRF gate should not apply to GET)")
	}
}

func TestCSRF_POSTBlockedOnBadOrigin(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"Origin":           "http://evil.example",
			"X-Requested-With": "XMLHttpRequest",
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("bad Origin: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestCSRF_POSTBlockedOnEmptyOrigin(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"X-Requested-With": "XMLHttpRequest",
			// Origin omitted — most common CSRF attack vector (form POST)
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("empty Origin: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestCSRF_POSTBlockedOnMissingXRW(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"Origin": csrfFrontendURL,
			// X-Requested-With omitted
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("missing X-Requested-With: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestCSRF_POSTPassesWithCorrectHeaders(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"Origin":           csrfFrontendURL,
			"X-Requested-With": "XMLHttpRequest",
		},
	})
	// 401 because no auth — but NOT 403 (CSRF gate passed)
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("correct CSRF headers: want non-403, got 403")
	}
}

func TestCSRF_RefreshPassesWithCorrectHeaders(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/auth/refresh",
		Headers: map[string]string{
			"Origin":           csrfFrontendURL,
			"X-Requested-With": "XMLHttpRequest",
		},
	})
	// 401 because no valid session — but NOT 403 (CSRF gate passed)
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("/auth/refresh with correct CSRF headers: want non-403, got 403")
	}
}

func TestCSRF_RefreshBlockedOnMissingXRW(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/auth/refresh",
		Headers: map[string]string{
			"Origin": csrfFrontendURL,
			// X-Requested-With omitted
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("/auth/refresh missing X-Requested-With: want 403, got %d", resp.StatusCode)
	}
}

func TestCSRF_POSTWithAuthHeaderSkipsCsrf(t *testing.T) {
	ta := newAPIKeyTestApp("csrf-api-user", "", "csrf-api-secret")
	ta.app.frontendURL = csrfFrontendURL
	resp, _ := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"Authorization": "Bearer " + ta.keyPlain,
			// Origin and X-Requested-With deliberately absent
		},
		Body: `{"name":"via-api-key","content":"hello"}`,
	})
	// API key is valid → 200 or 201, not 403
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("Bearer Authorization present: CSRF gate must not block, got 403")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Bearer Authorization present: want 200/201, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestCSRF_POSTWithGarbageAuthHeaderTriggersCsrf(t *testing.T) {
	a := newCSRFTestApp()
	resp, _ := a.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers: map[string]string{
			"Authorization": "hello", // not "bearer ..." → CSRF gate applies
			"Origin":        "http://evil.example",
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("garbage auth + bad Origin: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

// apiKeyTestApp bundles a minimal App and the plaintext API key issued to it.
type apiKeyTestApp struct {
	app      *App
	keyPlain string // 64-char hex plaintext for the issued key
}

// newAPIKeyTestApp constructs a minimal App wired for API key tests.
// dynamo.NewProvider(nil) uses the in-memory map backend (no real DynamoDB).
// apiGatewaySecret is intentionally left empty so the X-Origin-Verify gate
// passes (both sides are "" → condition is false → not blocked).
func newAPIKeyTestApp(userID, baseFolderID, jwtSecret string) apiKeyTestApp {
	store := apikey.NewInMemoryStore()
	// Produce a deterministic plaintext for reproducible tests.
	plain := fmt.Sprintf("%064s", userID) // pad to 64 chars
	hash := apikey.HashKey(plain)
	store.Issue(context.Background(), userID, baseFolderID, hash, plain[:8]) //nolint:errcheck

	p := dynamo.NewProvider(nil)
	app := &App{
		noteHandler:   handler.NewNoteHandler(p, jwtSecret),
		searchHandler: handler.NewSearchHandler(p, jwtSecret),
		exportHandler: handler.NewExportHandler(p, jwtSecret),
		tagHandler:    handler.NewTagHandler(p, jwtSecret),
		graphHandler:  handler.NewGraphHandler(p, jwtSecret),
		apiKeyHandler: handler.NewAPIKeyHandler(store, jwtSecret),
		apiKeys:       store,
		jwtSecret:     jwtSecret,
	}
	return apiKeyTestApp{app: app, keyPlain: plain}
}

func TestAPIKey_CorrectKey(t *testing.T) {
	const jwtSecret = "test-jwt-secret"
	ta := newAPIKeyTestApp("112233445566", "", jwtSecret)

	resp, err := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + ta.keyPlain},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("correct key: want 200, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestAPIKey_WrongKey(t *testing.T) {
	ta := newAPIKeyTestApp("sub123", "", "test-secret")

	resp, err := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + fmt.Sprintf("%064s", "wrong")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong key: want 401, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestAPIKey_Unconfigured(t *testing.T) {
	// An empty Store means no key is configured; a junk bearer should still fail JWT parse.
	p := dynamo.NewProvider(nil)
	emptyApp := &App{
		noteHandler:   handler.NewNoteHandler(p, "test-secret"),
		searchHandler: handler.NewSearchHandler(p, "test-secret"),
		exportHandler: handler.NewExportHandler(p, "test-secret"),
		tagHandler:    handler.NewTagHandler(p, "test-secret"),
		graphHandler:  handler.NewGraphHandler(p, "test-secret"),
		apiKeyHandler: handler.NewAPIKeyHandler(apikey.NewInMemoryStore(), "test-secret"),
		apiKeys:       apikey.NewInMemoryStore(), // empty — no key registered
		jwtSecret:     "test-secret",
	}

	resp, err := emptyApp.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer junk-token"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unconfigured: want 401, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestAPIKey_NoAuthHeader(t *testing.T) {
	ta := newAPIKeyTestApp("sub123", "", "test-secret")

	resp, err := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth header: want 401, got %d", resp.StatusCode)
	}
}

func TestAPIKey_NonDemoUser(t *testing.T) {
	// Agent sub must not start with "demo-user-" so the dynamo adapter treats
	// the agent as a regular (uncapped, non-TTL) user. Verify ListNotes succeeds.
	ta := newAPIKeyTestApp("google-oauth2|999888777", "", "jwt-secret")

	resp, err := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + ta.keyPlain},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("non-demo sub: want 200, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestAPIKey_ScopeIsolation(t *testing.T) {
	// The translated JWT must carry the correct userID so created notes land in
	// the key owner's scope and are not visible to a different user.
	const jwtSecret = "iso-secret"

	taAgent := newAPIKeyTestApp("agent-sub-aaa", "", jwtSecret)
	taOther := newAPIKeyTestApp("other-sub-bbb", "", jwtSecret)
	ctx := context.Background()

	// Create a note as the agent.
	createResp, err := taAgent.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + taAgent.keyPlain},
		Body:       `{"name":"agent-note","content":"hello"}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
		t.Fatalf("create: want 200/201, got %d body=%s", createResp.StatusCode, createResp.Body)
	}

	// ListNotes via agent key must return the note.
	listResp, err := taAgent.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + taAgent.keyPlain},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d body=%s", listResp.StatusCode, listResp.Body)
	}
	if listResp.Body == "[]" || listResp.Body == "" {
		t.Error("expected at least one note in agent scope, got empty list")
	}

	// The other user's app (different key, different sub) must see an empty list.
	otherListResp, err := taOther.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + taOther.keyPlain},
	})
	if err != nil {
		t.Fatalf("other list: %v", err)
	}
	if otherListResp.StatusCode != http.StatusOK {
		t.Fatalf("other list: want 200, got %d", otherListResp.StatusCode)
	}
	if otherListResp.Body != "[]" && otherListResp.Body != "null" {
		t.Errorf("other sub should see empty list, got %s", otherListResp.Body)
	}
}

func TestAPIKey_FolderPlacement(t *testing.T) {
	// Verifies that the gophmem CLI's actual wire payload ("parentId") correctly
	// routes a created note into the specified folder — both the CLI and the handler
	// must agree on the field name.
	const jwtSecret = "placement-secret"
	ta := newAPIKeyTestApp("placement-sub", "", jwtSecret)
	ctx := context.Background()

	// Create a destination folder.
	folderResp, err := ta.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/folders",
		Headers:    map[string]string{"Authorization": "Bearer " + ta.keyPlain},
		Body:       `{"name":"AI Memory","parents":[]}`,
	})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if folderResp.StatusCode != http.StatusOK && folderResp.StatusCode != http.StatusCreated {
		t.Fatalf("create folder: want 200/201, got %d body=%s", folderResp.StatusCode, folderResp.Body)
	}
	var folderMeta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(folderResp.Body), &folderMeta); err != nil || folderMeta.ID == "" {
		t.Fatalf("parse folder response: %v body=%s", err, folderResp.Body)
	}

	// Create a note using parentId — gophmem's actual wire payload shape.
	noteResp, err := ta.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + ta.keyPlain},
		Body:       fmt.Sprintf(`{"name":"test.md","content":"hello","parentId":%q}`, folderMeta.ID),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if noteResp.StatusCode != http.StatusCreated && noteResp.StatusCode != http.StatusOK {
		t.Fatalf("create note: want 200/201, got %d body=%s", noteResp.StatusCode, noteResp.Body)
	}

	// List notes in that specific folder — the note must appear there.
	listResp, err := ta.app.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod:            "GET",
		Path:                  "/api/notes",
		QueryStringParameters: map[string]string{"folderId": folderMeta.ID},
		Headers:               map[string]string{"Authorization": "Bearer " + ta.keyPlain},
	})
	if err != nil {
		t.Fatalf("list folder: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list folder: want 200, got %d body=%s", listResp.StatusCode, listResp.Body)
	}
	if listResp.Body == "[]" || listResp.Body == "" {
		t.Errorf("note not found in folder %s: list returned %s", folderMeta.ID, listResp.Body)
	}
}

// ---- translateAPIKey unit tests ----

func TestTranslateAPIKey_CorrectKey(t *testing.T) {
	store := apikey.NewInMemoryStore()
	const plain, jwtSecret = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234", "jwt-secret"
	store.Issue(context.Background(), "sub-123", "", apikey.HashKey(plain), plain[:8]) //nolint:errcheck

	tok, ok := translateAPIKey(
		context.Background(),
		map[string]string{"Authorization": "Bearer " + plain},
		store, jwtSecret,
	)
	if !ok {
		t.Fatal("expected ok=true for correct key")
	}
	if tok == "" {
		t.Error("expected non-empty session JWT")
	}
	if strings.HasPrefix(tok, "Bearer ") {
		t.Errorf("expected plain JWT (no Bearer prefix), got %q", tok)
	}
}

func TestTranslateAPIKey_CaseInsensitiveHeader(t *testing.T) {
	store := apikey.NewInMemoryStore()
	const plain = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	store.Issue(context.Background(), "sub-123", "", apikey.HashKey(plain), plain[:8]) //nolint:errcheck

	_, ok := translateAPIKey(
		context.Background(),
		map[string]string{"authorization": "Bearer " + plain}, // lowercase header name
		store, "jwt-secret",
	)
	if !ok {
		t.Error("expected ok=true when Authorization header is lowercase")
	}
}

func TestTranslateAPIKey_CaseInsensitiveBearer(t *testing.T) {
	store := apikey.NewInMemoryStore()
	const plain = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	store.Issue(context.Background(), "sub-123", "", apikey.HashKey(plain), plain[:8]) //nolint:errcheck

	for _, scheme := range []string{"bearer ", "BEARER ", "Bearer "} {
		_, ok := translateAPIKey(
			context.Background(),
			map[string]string{"Authorization": scheme + plain},
			store, "jwt-secret",
		)
		if !ok {
			t.Errorf("expected ok=true for scheme %q", scheme)
		}
	}
}

func TestTranslateAPIKey_WrongKey(t *testing.T) {
	store := apikey.NewInMemoryStore()
	store.Issue(context.Background(), "sub-123", "", apikey.HashKey("real-key"), "real-key"[:8]) //nolint:errcheck

	_, ok := translateAPIKey(
		context.Background(),
		map[string]string{"Authorization": "Bearer wrong-key"},
		store, "jwt-secret",
	)
	if ok {
		t.Error("expected ok=false for wrong key")
	}
}

func TestTranslateAPIKey_MissingHeader(t *testing.T) {
	store := apikey.NewInMemoryStore()
	_, ok := translateAPIKey(context.Background(), map[string]string{}, store, "jwt-secret")
	if ok {
		t.Error("expected ok=false for missing Authorization header")
	}
}

func TestTranslateAPIKey_NilStore(t *testing.T) {
	_, ok := translateAPIKey(
		context.Background(),
		map[string]string{"Authorization": "Bearer some-key"},
		nil, "jwt-secret",
	)
	if ok {
		t.Error("expected ok=false when store is nil")
	}
}

func TestTranslateAPIKey_EmptyStore(t *testing.T) {
	_, ok := translateAPIKey(
		context.Background(),
		map[string]string{"Authorization": "Bearer some-key"},
		apikey.NewInMemoryStore(), "jwt-secret",
	)
	if ok {
		t.Error("expected ok=false when store has no keys")
	}
}

// errStore simulates a DynamoDB failure on every operation.
type errStore struct{}

func (errStore) Issue(_ context.Context, _, _, _, _ string) error {
	return errors.New("simulated DDB error")
}
func (errStore) Lookup(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, errors.New("simulated DDB error")
}
func (errStore) StatusFor(_ context.Context, _ string) (bool, string, int64, int64, error) {
	return false, "", 0, 0, errors.New("simulated DDB error")
}
func (errStore) Revoke(_ context.Context, _ string) error {
	return errors.New("simulated DDB error")
}

func TestTranslateAPIKey_StoreError(t *testing.T) {
	// A store error must be treated as a miss so human sessions are unaffected.
	_, ok := translateAPIKey(
		context.Background(),
		map[string]string{"Authorization": "Bearer some-key"},
		errStore{}, "jwt-secret",
	)
	if ok {
		t.Error("expected ok=false when store returns error")
	}
}

func TestAPIKey_RevokedKeyReturns401(t *testing.T) {
	const jwtSecret = "revoke-test-secret"
	store := apikey.NewInMemoryStore()
	plain := fmt.Sprintf("%064s", "revoked-sub")
	hash := apikey.HashKey(plain)
	store.Issue(context.Background(), "revoked-sub", "", hash, plain[:8]) //nolint:errcheck

	p := dynamo.NewProvider(nil)
	app := &App{
		noteHandler:   handler.NewNoteHandler(p, jwtSecret),
		searchHandler: handler.NewSearchHandler(p, jwtSecret),
		exportHandler: handler.NewExportHandler(p, jwtSecret),
		tagHandler:    handler.NewTagHandler(p, jwtSecret),
		graphHandler:  handler.NewGraphHandler(p, jwtSecret),
		apiKeyHandler: handler.NewAPIKeyHandler(store, jwtSecret),
		apiKeys:       store,
		jwtSecret:     jwtSecret,
	}

	// Key must work before revocation.
	preResp, err := app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + plain},
	})
	if err != nil {
		t.Fatalf("pre-revoke HandleRequest: %v", err)
	}
	if preResp.StatusCode != http.StatusOK {
		t.Fatalf("before revoke: want 200, got %d body=%s", preResp.StatusCode, preResp.Body)
	}

	store.Revoke(context.Background(), "revoked-sub") //nolint:errcheck

	// Same key must now be rejected.
	postResp, err := app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + plain},
	})
	if err != nil {
		t.Fatalf("post-revoke HandleRequest: %v", err)
	}
	if postResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("after revoke: want 401, got %d body=%s", postResp.StatusCode, postResp.Body)
	}
}

func TestAPIKey_RequiresOriginVerify(t *testing.T) {
	const gateway = "test-gw-secret"
	store := apikey.NewInMemoryStore()
	p := dynamo.NewProvider(nil)
	gatedApp := &App{
		noteHandler:      handler.NewNoteHandler(p, "test-secret"),
		searchHandler:    handler.NewSearchHandler(p, "test-secret"),
		exportHandler:    handler.NewExportHandler(p, "test-secret"),
		tagHandler:       handler.NewTagHandler(p, "test-secret"),
		graphHandler:     handler.NewGraphHandler(p, "test-secret"),
		apiKeyHandler:    handler.NewAPIKeyHandler(store, "test-secret"),
		apiKeys:          store,
		jwtSecret:        "test-secret",
		apiGatewaySecret: gateway,
	}

	resp, err := gatedApp.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/api/api-keys",
		Headers:    map[string]string{},
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("missing X-Origin-Verify: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

// ---- injectSessionCookie unit tests ----

func TestInjectSessionCookie_NilHeaders(t *testing.T) {
	req := events.APIGatewayProxyRequest{Headers: nil}
	injectSessionCookie(&req, "tok-abc")
	if req.Headers["Cookie"] != "session_token=tok-abc" {
		t.Errorf("nil-headers: got %q, want session_token=tok-abc", req.Headers["Cookie"])
	}
}

func TestInjectSessionCookie_NoExistingCookie(t *testing.T) {
	req := events.APIGatewayProxyRequest{Headers: map[string]string{"X-Custom": "val"}}
	injectSessionCookie(&req, "tok-123")
	if req.Headers["Cookie"] != "session_token=tok-123" {
		t.Errorf("no-prior-cookie: got %q", req.Headers["Cookie"])
	}
	// Original header must be preserved.
	if req.Headers["X-Custom"] != "val" {
		t.Errorf("X-Custom was dropped: %v", req.Headers)
	}
}

func TestInjectSessionCookie_ExistingCookiePreserved(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{"Cookie": "other=value"},
	}
	injectSessionCookie(&req, "tok-xyz")
	got := req.Headers["Cookie"]
	if !strings.Contains(got, "other=value") || !strings.Contains(got, "session_token=tok-xyz") {
		t.Errorf("existing cookie not preserved: got %q", got)
	}
	// Canonical key only — no duplicate.
	if _, hasLower := req.Headers["cookie"]; hasLower {
		t.Error("duplicate lowercase 'cookie' key left in map")
	}
}

func TestInjectSessionCookie_LowercaseCookieNormalised(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{"cookie": "prior=1"}, // lowercase
	}
	injectSessionCookie(&req, "tok-lower")
	// The old lowercase key must be gone.
	if _, hasLower := req.Headers["cookie"]; hasLower {
		t.Error("lowercase 'cookie' key not removed after normalisation")
	}
	got := req.Headers["Cookie"]
	if !strings.Contains(got, "prior=1") || !strings.Contains(got, "session_token=tok-lower") {
		t.Errorf("lowercase cookie not preserved: got %q", got)
	}
}

// Phase 3c regression: Authorization: Bearer <session-JWT> without a Cookie
// must no longer authenticate (Bearer read path has been removed).
func TestBearerSessionJWT_NoLongerAuthenticates(t *testing.T) {
	ta := newAPIKeyTestApp("bearer-jwt-user", "", "jwt-secret")

	// Mint a valid session JWT directly (not via API key exchange).
	tok, err := handler.SignSession(handler.SessionClaims{UserID: "bearer-jwt-user"}, 5*60*1e9, "jwt-secret")
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	resp, reqErr := ta.app.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/api/notes",
		Headers:    map[string]string{"Authorization": "Bearer " + tok},
	})
	if reqErr != nil {
		t.Fatalf("HandleRequest: %v", reqErr)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Bearer-only session JWT: want 401, got %d (Phase 3c regression)", resp.StatusCode)
	}
}
