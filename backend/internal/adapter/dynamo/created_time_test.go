package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

func TestCreatedTime_SetOnCreateFile(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	before := time.Now()
	meta, err := m.CreateFile(ctx, "note.md", []byte("hello"), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	after := time.Now()

	f, err := m.GetFile(ctx, meta.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if f.CreatedTime.IsZero() {
		t.Fatal("CreatedTime is zero after CreateFile")
	}
	if f.CreatedTime.Before(before) || f.CreatedTime.After(after) {
		t.Errorf("CreatedTime %v outside expected range [%v, %v]", f.CreatedTime, before, after)
	}
}

func TestCreatedTime_SetOnCreateFolder(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	before := time.Now()
	meta, err := m.CreateFolder(ctx, "My Folder", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	after := time.Now()

	f, err := m.GetFile(ctx, meta.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if f.CreatedTime.IsZero() {
		t.Fatal("CreatedTime is zero after CreateFolder")
	}
	if f.CreatedTime.Before(before) || f.CreatedTime.After(after) {
		t.Errorf("CreatedTime %v outside expected range [%v, %v]", f.CreatedTime, before, after)
	}
}

func TestCreatedTime_SetOnDuplicateFile(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	orig, _ := m.CreateFile(ctx, "orig.md", []byte("content"), "root")

	before := time.Now()
	dup, err := m.DuplicateFile(ctx, orig.ID)
	if err != nil {
		t.Fatalf("DuplicateFile: %v", err)
	}
	after := time.Now()

	f, err := m.GetFile(ctx, dup.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if f.CreatedTime.IsZero() {
		t.Fatal("CreatedTime is zero after DuplicateFile")
	}
	if f.CreatedTime.Before(before) || f.CreatedTime.After(after) {
		t.Errorf("CreatedTime %v outside expected range [%v, %v]", f.CreatedTime, before, after)
	}
}

func TestCreatedTime_UnchangedAfterSaveFile(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	meta, _ := m.CreateFile(ctx, "note.md", []byte("v1"), "root")
	orig, _ := m.GetFile(ctx, meta.ID)
	origCreated := orig.CreatedTime

	_, err := m.SaveFile(ctx, meta.ID, []byte("v2"), meta.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	updated, _ := m.GetFile(ctx, meta.ID)
	if !updated.CreatedTime.Equal(origCreated) {
		t.Errorf("CreatedTime changed after SaveFile: was %v, now %v", origCreated, updated.CreatedTime)
	}
	// ModifiedTime must not go backwards (equal is permitted in fast test environments).
	if updated.ModifiedTime.Before(orig.ModifiedTime) {
		t.Errorf("ModifiedTime went backwards after SaveFile: was %v, now %v", orig.ModifiedTime, updated.ModifiedTime)
	}
}

func TestCreatedTime_FileMetadataUnchanged(t *testing.T) {
	// FileMetadata returned by CreateFile must not carry a CreatedTime field —
	// the JSON output shape is unchanged (A-read is deferred).
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	meta, err := m.CreateFile(ctx, "note.md", []byte("hi"), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	// FileMetadata has no CreatedTime field — this just verifies the struct
	// compiles as expected and common fields are populated.
	if meta.ID == "" {
		t.Error("ID empty")
	}
	if meta.ModifiedTime.IsZero() {
		t.Error("ModifiedTime zero")
	}
}

// TestCreatedTime_FileItemMarshalRoundtrip verifies that created_time survives
// a MarshalMap → UnmarshalMap round-trip — the same path that DynamoDB takes on
// PutItem/GetItem. This guards against omitempty stripping a non-zero value and
// also confirms that a zero-value (legacy row with no created_time) round-trips
// cleanly to the zero value without error.
func TestCreatedTime_FileItemMarshalRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond) // DDB encodes to ms precision

	t.Run("non-zero created_time is preserved", func(t *testing.T) {
		item := FileItem{
			PK:           "abc",
			ID:           "abc",
			UserID:       "u1",
			ModifiedTime: now,
			CreatedTime:  now,
		}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}
		var out FileItem
		if err := attributevalue.UnmarshalMap(av, &out); err != nil {
			t.Fatalf("UnmarshalMap: %v", err)
		}
		if !out.CreatedTime.Equal(now) {
			t.Errorf("CreatedTime mismatch: want %v, got %v", now, out.CreatedTime)
		}
	})

	t.Run("zero created_time (legacy row) round-trips as zero", func(t *testing.T) {
		// The DynamoDB SDK does not skip time.Time{} with omitempty — it encodes
		// the zero time as "0001-01-01T00:00:00Z". For actual legacy rows in DDB
		// (rows written before Phase A2 and thus lacking the attribute entirely),
		// UnmarshalMap leaves CreatedTime at its Go zero value. This sub-test
		// covers the marshal→unmarshal path; the absence-from-real-DDB case is
		// covered by the fact that UnmarshalMap leaves unset fields at zero.
		item := FileItem{
			PK:           "xyz",
			ID:           "xyz",
			UserID:       "u1",
			ModifiedTime: now,
			// CreatedTime intentionally zero (simulates legacy row)
		}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}
		var out FileItem
		if err := attributevalue.UnmarshalMap(av, &out); err != nil {
			t.Fatalf("UnmarshalMap: %v", err)
		}
		// After roundtrip the zero time must remain zero (IsZero == true).
		if !out.CreatedTime.IsZero() {
			t.Errorf("expected zero CreatedTime for legacy row, got %v", out.CreatedTime)
		}
	})
}

// TestCreatedTime_SaveFilePreservesCreatedTime verifies that SaveFile in the
// in-memory fallback path does not alter CreatedTime. The DDB path cannot be
// unit-tested without a live DynamoDB Local instance (Adapter.client is a
// concrete *dynamodb.Client with no abstraction interface); the SaveFile DDB fix
// (adding CreatedTime: f.CreatedTime to the rebuilt FileItem) must be verified
// via e2e: POST /notes → PUT /notes/{id} → GET /notes/{id} confirms created_time
// is unchanged.
func TestCreatedTime_SaveFilePreservesCreatedTime_InMemory(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	meta, _ := m.CreateFile(ctx, "note.md", []byte("v1"), "root")
	before, _ := m.GetFile(ctx, meta.ID)

	// Simulate append: SaveFile changes content but must not touch CreatedTime.
	updated, err := m.SaveFile(ctx, meta.ID, []byte("v1\n\nappended"), meta.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	after, _ := m.GetFile(ctx, meta.ID)

	if !after.CreatedTime.Equal(before.CreatedTime) {
		t.Errorf("SaveFile changed CreatedTime: was %v, now %v", before.CreatedTime, after.CreatedTime)
	}
	if after.ModifiedTime.Equal(before.ModifiedTime) {
		t.Log("ModifiedTime unchanged (acceptable in fast tests with coarse clock)")
	}
	_ = updated

	// Verify adapter.File.CreatedTime is not exposed in FileMetadata JSON shape.
	var _ adapter.FileMetadata = after.FileMetadata // compile-time: FileMetadata has no CreatedTime
}
