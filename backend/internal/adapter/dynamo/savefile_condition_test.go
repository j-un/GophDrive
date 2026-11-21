package dynamo

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

func TestSaveFile_PreconditionFailedWhenStoredETagChanges(t *testing.T) {
	fake := newFakeDDB()
	a := NewAdapter(fake, "user-1", "")
	ctx := context.Background()

	created, err := a.CreateFile(ctx, "note.md", []byte("original"), "")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	const storedByOther = "stored-by-other-writer"
	fake.afterGetItem = func(items map[string]map[string]types.AttributeValue) {
		row := items[created.ID]
		row["etag"] = &types.AttributeValueMemberS{Value: "etag-moved"}
		row["content"] = &types.AttributeValueMemberB{Value: []byte(storedByOther)}
	}

	_, err = a.SaveFile(ctx, created.ID, []byte("this-save"), created.ETag)
	if !errors.Is(err, adapter.ErrPreconditionFailed) {
		t.Errorf("SaveFile error = %v, want ErrPreconditionFailed", err)
	}

	got, err := a.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got.Content) != storedByOther {
		t.Errorf("content = %q, want the body the hook stored %q", got.Content, storedByOther)
	}
}

func TestSaveFile_CommitsWhenStoredETagMatches(t *testing.T) {
	fake := newFakeDDB()
	a := NewAdapter(fake, "user-1", "")
	ctx := context.Background()

	created, err := a.CreateFile(ctx, "note.md", []byte("original"), "")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	updated, err := a.SaveFile(ctx, created.ID, []byte("next"), created.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if updated.ETag == created.ETag {
		t.Fatal("SaveFile kept the ETag it was given")
	}

	got, err := a.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got.Content) != "next" {
		t.Errorf("content = %q, want %q", got.Content, "next")
	}
	if got.ETag != updated.ETag {
		t.Errorf("stored ETag = %q, want %q", got.ETag, updated.ETag)
	}
}

func TestSaveFile_EmptyETagOverwrites(t *testing.T) {
	fake := newFakeDDB()
	a := NewAdapter(fake, "user-1", "")
	ctx := context.Background()

	created, err := a.CreateFile(ctx, "note.md", []byte("original"), "")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	fake.afterGetItem = func(items map[string]map[string]types.AttributeValue) {
		row := items[created.ID]
		row["etag"] = &types.AttributeValueMemberS{Value: "etag-moved"}
		row["content"] = &types.AttributeValueMemberB{Value: []byte("stored-by-other-writer")}
	}

	if _, err := a.SaveFile(ctx, created.ID, []byte("forced"), ""); err != nil {
		t.Fatalf("SaveFile with an empty etag: %v", err)
	}

	got, err := a.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got.Content) != "forced" {
		t.Errorf("content = %q, want %q", got.Content, "forced")
	}
}
