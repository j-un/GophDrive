package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

func TestTouchViewed_MovesNoteToFrontOfRecent(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
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

func TestTouchViewed_DoesNotChangeETagOrModifiedTime(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
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

func TestTouchViewed_MissingNoteIsNoop(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	if err := m.TouchViewed(context.Background(), "does-not-exist"); err != nil {
		t.Fatalf("TouchViewed on missing note should be a no-op, got %v", err)
	}
}

func TestViewedTime_FileItemMarshalRoundtrip(t *testing.T) {
	now := time.Now().UTC()

	t.Run("set viewed_time round-trips intact", func(t *testing.T) {
		item := FileItem{PK: "x", ID: "x", UserID: "u1", ModifiedTime: now, ViewedTime: now}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}
		var out FileItem
		if err := attributevalue.UnmarshalMap(av, &out); err != nil {
			t.Fatalf("UnmarshalMap: %v", err)
		}
		if !out.ViewedTime.Equal(now) {
			t.Errorf("ViewedTime mismatch: want %v, got %v", now, out.ViewedTime)
		}
	})

	t.Run("zero viewed_time round-trips as zero", func(t *testing.T) {
		// omitempty does NOT skip time.Time{}; the SDK encodes it as
		// "0001-01-01T00:00:00Z", which must still read back as IsZero().
		item := FileItem{PK: "y", ID: "y", UserID: "u1", ModifiedTime: now}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}
		var out FileItem
		if err := attributevalue.UnmarshalMap(av, &out); err != nil {
			t.Fatalf("UnmarshalMap: %v", err)
		}
		if !out.ViewedTime.IsZero() {
			t.Errorf("expected zero ViewedTime to round-trip as IsZero, got %v", out.ViewedTime)
		}
	})
}
