package dynamo

import (
	"context"
	"testing"
	"time"
)

// Opening a note (TouchViewed) must move it to the front of RECENT, even when
// other notes were created/modified more recently.
func TestTouchViewed_MovesNoteToFrontOfRecent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	a, err := m.CreateFile(ctx, "a.md", []byte("# A"), "root")
	if err != nil {
		t.Fatalf("CreateFile a: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	b, err := m.CreateFile(ctx, "b.md", []byte("# B"), "root")
	if err != nil {
		t.Fatalf("CreateFile b: %v", err)
	}

	// Opening A records a view newer than either creation, so A must lead RECENT.
	time.Sleep(2 * time.Millisecond)
	if err := m.TouchViewed(ctx, a.ID); err != nil {
		t.Fatalf("TouchViewed a: %v", err)
	}
	recent, err := m.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(recent) < 2 {
		t.Fatalf("expected at least 2 recent notes, got %d", len(recent))
	}
	if recent[0].ID != a.ID {
		t.Fatalf("after opening A, recent[0]=%s, want A=%s", recent[0].ID, a.ID)
	}

	// Opening B in turn moves B to the front.
	time.Sleep(2 * time.Millisecond)
	if err := m.TouchViewed(ctx, b.ID); err != nil {
		t.Fatalf("TouchViewed b: %v", err)
	}
	recent, err = m.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if recent[0].ID != b.ID {
		t.Fatalf("after opening B, recent[0]=%s, want B=%s", recent[0].ID, b.ID)
	}
}

// TouchViewed must not alter ETag or ModifiedTime, so it can never trigger a
// false conflict on the optimistic-concurrency save path.
func TestTouchViewed_DoesNotChangeETagOrModifiedTime(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	created, err := m.CreateFile(ctx, "note.md", []byte("# hi"), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	before, err := m.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile before: %v", err)
	}

	time.Sleep(2 * time.Millisecond)
	if err := m.TouchViewed(ctx, created.ID); err != nil {
		t.Fatalf("TouchViewed: %v", err)
	}

	after, err := m.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile after: %v", err)
	}
	if after.ETag != before.ETag {
		t.Errorf("TouchViewed changed ETag: %q -> %q", before.ETag, after.ETag)
	}
	if !after.ModifiedTime.Equal(before.ModifiedTime) {
		t.Errorf("TouchViewed changed ModifiedTime: %v -> %v", before.ModifiedTime, after.ModifiedTime)
	}
}

// Touching a note that no longer exists is a no-op, not an error.
func TestTouchViewed_MissingNoteIsNoop(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	if err := m.TouchViewed(context.Background(), "does-not-exist"); err != nil {
		t.Fatalf("TouchViewed on missing note should be a no-op, got %v", err)
	}
}
