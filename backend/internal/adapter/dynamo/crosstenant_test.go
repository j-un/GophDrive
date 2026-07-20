package dynamo

import (
	"context"
	"errors"
	"testing"

	"github.com/jun/gophdrive/backend/internal/adapter"
)

// seedVictim creates a fresh shared fake + victim/attacker adapters, and
// seeds a single owner-a note. Returns the fake for direct inspection, the
// two adapters, and the seeded file ID.
func seedVictim(t *testing.T) (*fakeDDB, *Adapter, *Adapter, string) {
	t.Helper()
	fake := newFakeDDB()
	victim := NewAdapter(fake, "user-a", "")
	attacker := NewAdapter(fake, "user-b", "")

	f, err := victim.CreateFile(context.Background(), "secret-note", []byte("owner-a content"), "")
	if err != nil {
		t.Fatalf("seed CreateFile: %v", err)
	}
	return fake, victim, attacker, f.ID
}

// assertVictimContentUnchanged is the positive-control half of each attack
// test: the victim must still own the row with its original content.
func assertVictimContentUnchanged(t *testing.T, victim *Adapter, fileID string) {
	t.Helper()
	f, err := victim.GetFile(context.Background(), fileID)
	if err != nil {
		t.Fatalf("victim.GetFile after attack: %v", err)
	}
	if string(f.Content) != "owner-a content" {
		t.Errorf("victim content changed: got %q, want %q", string(f.Content), "owner-a content")
	}
}

func TestCrossTenant_GetFile(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	_, err := attacker.GetFile(context.Background(), id)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.GetFile: got %v, want ErrNotFound", err)
	}
	assertVictimContentUnchanged(t, victim, id)
}

func TestCrossTenant_DeleteFile(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	err := attacker.DeleteFile(context.Background(), id)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.DeleteFile: got %v, want ErrNotFound", err)
	}
	if _, err := victim.GetFile(context.Background(), id); err != nil {
		t.Fatalf("victim.GetFile after attacker.Delete: %v (expected still readable)", err)
	}
}

func TestCrossTenant_RenameFile(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	_, err := attacker.RenameFile(context.Background(), id, "stolen")
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.RenameFile: got %v, want ErrNotFound", err)
	}
	f, err := victim.GetFile(context.Background(), id)
	if err != nil {
		t.Fatalf("victim.GetFile: %v", err)
	}
	if f.Name != "secret-note" {
		t.Errorf("victim.Name changed: got %q, want %q", f.Name, "secret-note")
	}
}

func TestCrossTenant_SetStarred(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	_, err := attacker.SetStarred(context.Background(), id, true)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.SetStarred: got %v, want ErrNotFound", err)
	}
	f, err := victim.GetFile(context.Background(), id)
	if err != nil {
		t.Fatalf("victim.GetFile: %v", err)
	}
	if f.Starred {
		t.Errorf("victim.Starred flipped: got true, want false")
	}
}

func TestCrossTenant_DuplicateFile(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)
	ctx := context.Background()

	_, err := attacker.DuplicateFile(ctx, id)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.DuplicateFile: got %v, want ErrNotFound", err)
	}
	attFiles, err := attacker.ListFiles(ctx, "")
	if err != nil {
		t.Fatalf("attacker.ListFiles: %v", err)
	}
	if len(attFiles) != 0 {
		t.Errorf("attacker.ListFiles: got %d items, want 0", len(attFiles))
	}
	vicFiles, err := victim.ListFiles(ctx, "")
	if err != nil {
		t.Fatalf("victim.ListFiles: %v", err)
	}
	if len(vicFiles) != 1 {
		t.Errorf("victim.ListFiles: got %d items, want 1", len(vicFiles))
	}
}

func TestCrossTenant_SaveFile(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	_, err := attacker.SaveFile(context.Background(), id, []byte("overwrite"), "")
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.SaveFile: got %v, want ErrNotFound", err)
	}
	assertVictimContentUnchanged(t, victim, id)
}

func TestCrossTenant_MoveFile_Source(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)

	_, err := attacker.MoveFile(context.Background(), id, "")
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("attacker.MoveFile: got %v, want ErrNotFound", err)
	}
	f, err := victim.GetFile(context.Background(), id)
	if err != nil {
		t.Fatalf("victim.GetFile: %v", err)
	}
	if len(f.Parents) != 1 || f.Parents[0] != "root" {
		t.Errorf("victim.Parents changed: got %v, want [root]", f.Parents)
	}
}

func TestCrossTenant_MoveFile_Destination(t *testing.T) {
	_, victim, attacker, id := seedVictim(t)
	ctx := context.Background()

	// Attacker creates their own folder, then victim tries to move victim's
	// own file into it — the destination guard must fail because the folder
	// belongs to another tenant.
	attFolder, err := attacker.CreateFolder(ctx, "attacker-folder", nil)
	if err != nil {
		t.Fatalf("attacker.CreateFolder: %v", err)
	}
	_, err = victim.MoveFile(ctx, id, attFolder.ID)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("victim.MoveFile into attacker folder: got %v, want ErrNotFound", err)
	}
	f, err := victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("victim.GetFile: %v", err)
	}
	if len(f.Parents) != 1 || f.Parents[0] != "root" {
		t.Errorf("victim.Parents changed: got %v, want [root]", f.Parents)
	}
}

func TestCrossTenant_TouchViewed_DemoTTLContamination(t *testing.T) {
	fake, victim, _, id := seedVictim(t)
	ctx := context.Background()

	// Attacker is a demo user: without the user_id condition guard on
	// TouchViewed, a demo-user attacker could rewrite the victim's ttl and
	// have the row auto-expire.
	attacker := NewAdapter(fake, "demo-user-evil", "")

	// Capture victim's ViewedTime before the attack.
	before, err := victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("victim.GetFile (before): %v", err)
	}

	// TouchViewed swallows CCFE — a documented no-op — so err == nil is
	// the correct assertion.
	if err := attacker.TouchViewed(ctx, id); err != nil {
		t.Fatalf("attacker.TouchViewed: got %v, want nil (documented no-op)", err)
	}

	after, err := victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("victim.GetFile (after): %v", err)
	}
	if !after.ViewedTime.Equal(before.ViewedTime) {
		t.Errorf("victim.ViewedTime changed: got %v, want %v", after.ViewedTime, before.ViewedTime)
	}

	// Persistence peek: the stored row must have NO "ttl" attribute — victim
	// is a real (non-demo) user, so writing ttl would flag them for the
	// 60-minute demo expiry.
	fake.mu.Lock()
	stored := fake.items[id]
	fake.mu.Unlock()
	if stored == nil {
		t.Fatalf("victim row missing from fake store")
	}
	if _, ok := stored["ttl"]; ok {
		t.Errorf("victim row acquired ttl attribute from attacker.TouchViewed — demo TTL contamination")
	}
}

// TestSameTenant_HappyPath is the positive control for the fake: it proves
// the condition guard passes for legitimate owners, so the cross-tenant
// failures above are meaningful rather than the fake rejecting everything.
func TestSameTenant_HappyPath(t *testing.T) {
	_, victim, _, id := seedVictim(t)
	ctx := context.Background()

	// Rename
	if _, err := victim.RenameFile(ctx, id, "renamed"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	got, err := victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("GetFile after Rename: %v", err)
	}
	if got.Name != "renamed" {
		t.Errorf("Name after Rename: got %q, want %q", got.Name, "renamed")
	}

	// SetStarred(true)
	if _, err := victim.SetStarred(ctx, id, true); err != nil {
		t.Fatalf("SetStarred: %v", err)
	}
	got, err = victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("GetFile after SetStarred: %v", err)
	}
	if !got.Starred {
		t.Errorf("Starred after SetStarred(true) was false")
	}

	// TouchViewed sets a non-zero ViewedTime
	if err := victim.TouchViewed(ctx, id); err != nil {
		t.Fatalf("TouchViewed: %v", err)
	}
	got, err = victim.GetFile(ctx, id)
	if err != nil {
		t.Fatalf("GetFile after TouchViewed: %v", err)
	}
	if got.ViewedTime.IsZero() {
		t.Errorf("ViewedTime is zero after TouchViewed")
	}

	// SaveFile with the current ETag
	saved, err := victim.SaveFile(ctx, id, []byte("v2"), got.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if saved.ETag == got.ETag {
		t.Errorf("ETag should change after SaveFile")
	}

	// DeleteFile, then GetFile → ErrNotFound
	if err := victim.DeleteFile(ctx, id); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := victim.GetFile(ctx, id); !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("GetFile after Delete: got %v, want ErrNotFound", err)
	}
}
