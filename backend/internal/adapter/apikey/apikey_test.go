package apikey_test

import (
	"context"
	"testing"

	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
)

func TestInMemoryStore_IssueAndLookup(t *testing.T) {
	s := apikey.NewInMemoryStore()
	ctx := context.Background()

	h := apikey.HashKey("plaintext-key")
	if err := s.Issue(ctx, "user1", "folder1", h, "plaintex"); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	uid, bid, ok, err := s.Lookup(ctx, h)
	if err != nil || !ok {
		t.Fatalf("Lookup: ok=%v err=%v", ok, err)
	}
	if uid != "user1" || bid != "folder1" {
		t.Errorf("got userID=%q baseFolderID=%q", uid, bid)
	}
}

func TestInMemoryStore_Lookup_NotFound(t *testing.T) {
	s := apikey.NewInMemoryStore()
	_, _, ok, err := s.Lookup(context.Background(), "nonexistent-hash")
	if err != nil || ok {
		t.Errorf("expected ok=false, got ok=%v err=%v", ok, err)
	}
}

func TestInMemoryStore_Regenerate(t *testing.T) {
	s := apikey.NewInMemoryStore()
	ctx := context.Background()

	h1 := apikey.HashKey("key-v1")
	if err := s.Issue(ctx, "user1", "folder1", h1, "key-v1_"); err != nil {
		t.Fatal(err)
	}
	_, _, firstCreatedAt, firstIssuedAt1, _ := s.StatusFor(ctx, "user1")

	h2 := apikey.HashKey("key-v2")
	if err := s.Issue(ctx, "user1", "folder1", h2, "key-v2_"); err != nil {
		t.Fatal(err)
	}

	// Old hash must no longer work.
	_, _, ok, _ := s.Lookup(ctx, h1)
	if ok {
		t.Error("old hash still valid after regenerate")
	}

	// New hash must work.
	uid, _, ok, _ := s.Lookup(ctx, h2)
	if !ok || uid != "user1" {
		t.Error("new hash not found after regenerate")
	}

	// first_issued_at must be preserved; created_at may differ (at least equal).
	_, _, createdAt2, firstIssuedAt2, _ := s.StatusFor(ctx, "user1")
	if firstIssuedAt2 != firstIssuedAt1 {
		t.Errorf("first_issued_at changed after regenerate: %d → %d", firstIssuedAt1, firstIssuedAt2)
	}
	if createdAt2 < firstCreatedAt {
		t.Errorf("created_at after regenerate (%d) is before first created_at (%d)", createdAt2, firstCreatedAt)
	}
}

func TestInMemoryStore_StatusFor(t *testing.T) {
	s := apikey.NewInMemoryStore()
	ctx := context.Background()

	hasKey, _, _, _, _ := s.StatusFor(ctx, "user1")
	if hasKey {
		t.Error("expected hasKey=false for unknown user")
	}

	h := apikey.HashKey("mykey")
	s.Issue(ctx, "user1", "f1", h, "mykey_pr")
	hasKey, prefix, _, _, err := s.StatusFor(ctx, "user1")
	if err != nil || !hasKey || prefix != "mykey_pr" {
		t.Errorf("StatusFor: hasKey=%v prefix=%q err=%v", hasKey, prefix, err)
	}
}

func TestInMemoryStore_Revoke(t *testing.T) {
	s := apikey.NewInMemoryStore()
	ctx := context.Background()

	h := apikey.HashKey("revoke-me")
	s.Issue(ctx, "user1", "f1", h, "revoke-m")
	if err := s.Revoke(ctx, "user1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, _, ok, _ := s.Lookup(ctx, h)
	if ok {
		t.Error("key still valid after Revoke")
	}
	hasKey, _, _, _, _ := s.StatusFor(ctx, "user1")
	if hasKey {
		t.Error("StatusFor still shows key after Revoke")
	}
}

func TestInMemoryStore_Revoke_NoKey(t *testing.T) {
	s := apikey.NewInMemoryStore()
	if err := s.Revoke(context.Background(), "nokey-user"); err != nil {
		t.Errorf("Revoke with no key should be no-op, got: %v", err)
	}
}

func TestInMemoryStore_ScopeIsolation(t *testing.T) {
	s := apikey.NewInMemoryStore()
	ctx := context.Background()

	h1 := apikey.HashKey("key-user1")
	h2 := apikey.HashKey("key-user2")
	s.Issue(ctx, "user1", "f1", h1, "key-user")
	s.Issue(ctx, "user2", "f2", h2, "key-user")

	uid1, _, _, _ := s.Lookup(ctx, h1)
	uid2, _, _, _ := s.Lookup(ctx, h2)
	if uid1 != "user1" || uid2 != "user2" {
		t.Errorf("scope isolation failure: uid1=%q uid2=%q", uid1, uid2)
	}
}

func TestHashKey_Deterministic(t *testing.T) {
	h1 := apikey.HashKey("same-input")
	h2 := apikey.HashKey("same-input")
	if h1 != h2 {
		t.Error("HashKey is not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("HashKey length want 64, got %d", len(h1))
	}
}
