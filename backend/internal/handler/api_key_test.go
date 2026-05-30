package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const testKeyJWTSecret = "api-key-test-secret"

// mintJWT produces a valid session JWT for testing.
func mintJWT(t *testing.T, userID, baseFolderID string) string {
	t.Helper()
	mc := jwt.MapClaims{
		"sub":            userID,
		"base_folder_id": baseFolderID,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString([]byte(testKeyJWTSecret))
	if err != nil {
		t.Fatalf("mintJWT: %v", err)
	}
	return tok
}

func newReq(method, path, token, body string) events.APIGatewayProxyRequest {
	req := events.APIGatewayProxyRequest{
		HTTPMethod: method,
		Path:       path,
		Body:       body,
	}
	if token != "" {
		req.Headers = map[string]string{"Authorization": "Bearer " + token}
	}
	return req
}

// failingReader always returns an error, used to exercise the rand-failure path.
type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated rand failure")
}

func TestAPIKeyHandler_Issue_Success(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-real-sub", "folder-root")

	resp, err := h.Issue(context.Background(), newReq("POST", "/api-keys", tok, ""))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", resp.StatusCode, resp.Body)
	}

	var out struct {
		Key       string `json:"key"`
		KeyPrefix string `json:"key_prefix"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(out.Key) != 64 {
		t.Errorf("want 64-char key, got len=%d", len(out.Key))
	}
	if out.KeyPrefix != out.Key[:8] {
		t.Errorf("key_prefix mismatch: %q vs %q", out.KeyPrefix, out.Key[:8])
	}

	// Lookup by hash must succeed.
	hash := apikey.HashKey(out.Key)
	uid, bid, ok, err := store.Lookup(context.Background(), hash)
	if err != nil || !ok {
		t.Fatalf("Lookup: ok=%v err=%v", ok, err)
	}
	if uid != "user-real-sub" || bid != "folder-root" {
		t.Errorf("got userID=%q baseFolderID=%q", uid, bid)
	}
}

func TestAPIKeyHandler_Issue_DemoForbidden(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "demo-user-abc123", "folder-demo")

	resp, err := h.Issue(context.Background(), newReq("POST", "/api-keys", tok, ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("demo user: want 403, got %d body=%s", resp.StatusCode, resp.Body)
	}
}

func TestAPIKeyHandler_Issue_Unauthorized(t *testing.T) {
	h := handler.NewAPIKeyHandler(apikey.NewInMemoryStore(), testKeyJWTSecret)
	resp, err := h.Issue(context.Background(), newReq("POST", "/api-keys", "bad-token", ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestAPIKeyHandler_Issue_RandFailure(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	h.RandReader = failingReader{}
	tok := mintJWT(t, "user-randfail", "folder-randfail")

	resp, err := h.Issue(context.Background(), newReq("POST", "/api-keys", tok, ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("rand failure: want 500, got %d body=%s", resp.StatusCode, resp.Body)
	}
	// store.Issue must not have been called
	hasKey, _, _, _, _ := store.StatusFor(context.Background(), "user-randfail")
	if hasKey {
		t.Error("store.Issue should not have been called when rand fails")
	}
}

func TestAPIKeyHandler_Issue_Regenerate_RevokesOld(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-regen", "folder-regen")
	ctx := context.Background()

	resp1, _ := h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))
	var out1 struct {
		Key string `json:"key"`
	}
	json.Unmarshal([]byte(resp1.Body), &out1)

	resp2, _ := h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("second issue: want 201, got %d", resp2.StatusCode)
	}

	// Old key must be gone.
	_, _, ok, _ := store.Lookup(ctx, apikey.HashKey(out1.Key))
	if ok {
		t.Error("old key still valid after regenerate")
	}
}

func TestAPIKeyHandler_Get_NoKey(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-nokey", "folder-nokey")

	resp, err := h.Get(context.Background(), newReq("GET", "/api-keys", tok, ""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out struct {
		HasKey bool `json:"has_key"`
	}
	json.Unmarshal([]byte(resp.Body), &out)
	if out.HasKey {
		t.Error("expected has_key=false")
	}
}

func TestAPIKeyHandler_Get_DoesNotReturnPlaintext(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-noexp", "folder1")
	ctx := context.Background()

	issueResp, _ := h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))
	var issued struct {
		Key string `json:"key"`
	}
	json.Unmarshal([]byte(issueResp.Body), &issued)

	getResp, _ := h.Get(ctx, newReq("GET", "/api-keys", tok, ""))
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Get: want 200, got %d", getResp.StatusCode)
	}
	// Response must not contain the full plaintext key.
	if len(issued.Key) > 0 && strings.Contains(getResp.Body, issued.Key) {
		t.Error("Get response contains plaintext key")
	}
}

func TestAPIKeyHandler_Get_ReturnsContentType(t *testing.T) {
	h := handler.NewAPIKeyHandler(apikey.NewInMemoryStore(), testKeyJWTSecret)
	tok := mintJWT(t, "user-ct", "folder-ct")

	resp, err := h.Get(context.Background(), newReq("GET", "/api-keys", tok, ""))
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Headers["Content-Type"]; ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}
}

func TestAPIKeyHandler_Get_FirstIssuedAt(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-fia", "folder-fia")
	ctx := context.Background()

	// Issue key once to set first_issued_at.
	h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))

	// Regenerate — first_issued_at must be preserved.
	h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))

	resp, _ := h.Get(ctx, newReq("GET", "/api-keys", tok, ""))
	var out struct {
		CreatedAt     int64 `json:"created_at"`
		FirstIssuedAt int64 `json:"first_issued_at"`
	}
	json.Unmarshal([]byte(resp.Body), &out)
	if out.FirstIssuedAt == 0 {
		t.Error("first_issued_at must be non-zero after issue+regenerate")
	}
	if out.FirstIssuedAt > out.CreatedAt {
		t.Errorf("first_issued_at (%d) must be ≤ created_at (%d)", out.FirstIssuedAt, out.CreatedAt)
	}
}

func TestAPIKeyHandler_Delete_Revokes(t *testing.T) {
	store := apikey.NewInMemoryStore()
	h := handler.NewAPIKeyHandler(store, testKeyJWTSecret)
	tok := mintJWT(t, "user-del", "folder-del")
	ctx := context.Background()

	issueResp, _ := h.Issue(ctx, newReq("POST", "/api-keys", tok, ""))
	var issued struct {
		Key string `json:"key"`
	}
	json.Unmarshal([]byte(issueResp.Body), &issued)

	delResp, err := h.Delete(ctx, newReq("DELETE", "/api-keys", tok, ""))
	if err != nil {
		t.Fatal(err)
	}
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("want 204, got %d body=%s", delResp.StatusCode, delResp.Body)
	}

	// Lookup must fail after delete.
	_, _, ok, _ := store.Lookup(ctx, apikey.HashKey(issued.Key))
	if ok {
		t.Error("key still valid after Delete")
	}
}

// Ensure failingReader satisfies io.Reader at compile time.
var _ io.Reader = failingReader{}
