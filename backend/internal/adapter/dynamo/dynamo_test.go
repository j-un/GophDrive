package dynamo

import (
	"context"
	"testing"

	"github.com/jun/gophdrive/backend/internal/adapter"
)

func TestAdapter_CreateAndListFiles(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Create a file in root
	file, err := m.CreateFile(ctx, "note.md", []byte("hello"), "")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if file.Name != "note.md" {
		t.Errorf("Expected name 'note.md', got '%s'", file.Name)
	}
	if file.MIMEType != "text/markdown" {
		t.Errorf("Expected mimeType 'text/markdown', got '%s'", file.MIMEType)
	}

	// List files in root
	files, err := m.ListFiles(ctx, "root")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0].ID != file.ID {
		t.Errorf("File ID mismatch")
	}
}

func TestAdapter_GetFile_NotFound(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	_, err := m.GetFile(ctx, "nonexistent-id")
	if err != adapter.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestAdapter_SaveFile_ETagMatch(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	file, _ := m.CreateFile(ctx, "note.md", []byte("v1"), "root")
	originalETag := file.ETag

	// Update with matching ETag
	updated, err := m.SaveFile(ctx, file.ID, []byte("v2"), originalETag)
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	if updated.ETag == originalETag {
		t.Error("Expected ETag to change after update")
	}

	// Verify content changed
	f, _ := m.GetFile(ctx, file.ID)
	if string(f.Content) != "v2" {
		t.Errorf("Expected content 'v2', got '%s'", string(f.Content))
	}
}

func TestAdapter_SaveFile_ETagMismatch(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	file, _ := m.CreateFile(ctx, "note.md", []byte("v1"), "root")

	_, err := m.SaveFile(ctx, file.ID, []byte("v2"), "wrong-etag")
	if err != adapter.ErrPreconditionFailed {
		t.Errorf("Expected ErrPreconditionFailed, got %v", err)
	}
}

func TestAdapter_CreateFolder(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	folder, err := m.CreateFolder(ctx, "MyFolder", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}
	if folder.Name != "MyFolder" {
		t.Errorf("Expected name 'MyFolder', got '%s'", folder.Name)
	}
	if folder.MIMEType != "application/vnd.google-apps.folder" {
		t.Errorf("Expected folder mimeType, got '%s'", folder.MIMEType)
	}
}

func TestAdapter_EnsureRootFolder_Idempotent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	id1, err := m.EnsureRootFolder(ctx, "GophDrive")
	if err != nil {
		t.Fatalf("EnsureRootFolder failed: %v", err)
	}

	id2, err := m.EnsureRootFolder(ctx, "GophDrive")
	if err != nil {
		t.Fatalf("EnsureRootFolder second call failed: %v", err)
	}

	if id1 != id2 {
		t.Errorf("EnsureRootFolder should be idempotent: got different IDs %s vs %s", id1, id2)
	}
}

func TestAdapter_DuplicateFile(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	orig, _ := m.CreateFile(ctx, "orig.md", []byte("content"), "root")

	dup, err := m.DuplicateFile(ctx, orig.ID)
	if err != nil {
		t.Fatalf("DuplicateFile failed: %v", err)
	}
	if dup.ID == orig.ID {
		t.Error("Duplicated file should have a different ID")
	}
	if dup.Name != "Copy of orig" {
		t.Errorf("Expected name 'Copy of orig', got '%s'", dup.Name)
	}

	// Verify content
	f, _ := m.GetFile(ctx, dup.ID)
	if string(f.Content) != "content" {
		t.Errorf("Duplicated content mismatch")
	}
}

func TestAdapter_RenameFile(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Test renaming a file
	file, _ := m.CreateFile(ctx, "old.md", []byte("data"), "root")
	originalETag := file.ETag

	renamed, err := m.RenameFile(ctx, file.ID, "new.md")
	if err != nil {
		t.Fatalf("RenameFile(file) failed: %v", err)
	}
	if renamed.Name != "new.md" {
		t.Errorf("Expected file name 'new.md', got '%s'", renamed.Name)
	}
	if renamed.ETag == originalETag {
		t.Error("Expected ETag to change after file rename")
	}

	// Test renaming a folder (should not add .md)
	folder, _ := m.CreateFolder(ctx, "OldFolder", []string{"root"})
	folderRenamed, err := m.RenameFile(ctx, folder.ID, "NewFolder")
	if err != nil {
		t.Fatalf("RenameFile(folder) failed: %v", err)
	}
	if folderRenamed.Name != "NewFolder" {
		t.Errorf("Expected folder name 'NewFolder', got '%s'", folderRenamed.Name)
	}
}

func TestAdapter_SetStarred(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	file, _ := m.CreateFile(ctx, "note.md", []byte("data"), "root")

	// Star
	starred, err := m.SetStarred(ctx, file.ID, true)
	if err != nil {
		t.Fatalf("SetStarred(true) failed: %v", err)
	}
	if !starred.Starred {
		t.Error("Expected file to be starred")
	}

	// Unstar
	unstarred, err := m.SetStarred(ctx, file.ID, false)
	if err != nil {
		t.Fatalf("SetStarred(false) failed: %v", err)
	}
	if unstarred.Starred {
		t.Error("Expected file to be unstarred")
	}
}

func TestAdapter_ListStarred(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	f1, _ := m.CreateFile(ctx, "starred.md", []byte("a"), "root")
	m.CreateFile(ctx, "normal.md", []byte("b"), "root")
	m.SetStarred(ctx, f1.ID, true)

	starred, err := m.ListStarred(ctx)
	if err != nil {
		t.Fatalf("ListStarred failed: %v", err)
	}
	if len(starred) != 1 {
		t.Fatalf("Expected 1 starred file, got %d", len(starred))
	}
	if starred[0].ID != f1.ID {
		t.Errorf("Starred file ID mismatch")
	}
}

func TestAdapter_SearchFiles(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	m.CreateFile(ctx, "hello-world.md", []byte("greeting"), "root")
	m.CreateFile(ctx, "other.md", []byte("nothing"), "root")
	m.CreateFile(ctx, "notes.md", []byte("hello from content"), "root")

	// Search by name
	results, err := m.SearchFiles(ctx, "hello")
	if err != nil {
		t.Fatalf("SearchFiles failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results (name match + content match), got %d", len(results))
	}

	// Case-insensitive
	results2, _ := m.SearchFiles(ctx, "HELLO")
	if len(results2) != 2 {
		t.Errorf("Expected case-insensitive search to find 2 results, got %d", len(results2))
	}
}

func TestAdapter_DeleteFile_Recursive(t *testing.T) {
	// Setup in-memory adapter
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// 1. Create Parent Folder
	parent, err := m.CreateFolder(ctx, "Parent", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// 2. Create Child Folder
	childFolder, err := m.CreateFolder(ctx, "ChildFolder", []string{parent.ID})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// 3. Create Child File
	childFile, err := m.CreateFile(ctx, "ChildFile", []byte("content"), childFolder.ID)
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// 4. Delete Parent
	err = m.DeleteFile(ctx, parent.ID)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// 5. Verify Parent Gone
	_, err = m.GetFile(ctx, parent.ID)
	if err != adapter.ErrNotFound {
		t.Errorf("Parent should be deleted, got error: %v", err)
	}

	// 6. Verify Child Folder Gone
	_, err = m.GetFile(ctx, childFolder.ID)
	if err != adapter.ErrNotFound {
		t.Errorf("Child Folder should be deleted, got error: %v", err)
	}

	// 7. Verify Child File Gone
	_, err = m.GetFile(ctx, childFile.ID)
	if err != adapter.ErrNotFound {
		t.Errorf("Child File should be deleted, got error: %v", err)
	}
}

func TestAdapter_ListRecent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Create files with different modified times (simulated by sequence)
	f1, _ := m.CreateFile(ctx, "old.md", []byte("v1"), "root")
	f2, _ := m.CreateFile(ctx, "new.md", []byte("v1"), "root")

	recent, err := m.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	if len(recent) != 2 {
		t.Fatalf("Expected 2 recent files, got %d", len(recent))
	}

	// Should be ordered by viewed time desc (f2 should be most recent)
	if recent[0].ID != f2.ID {
		t.Errorf("Expected most recent file to be %s, got %s (f1=%s, f2=%s)", f2.ID, recent[0].ID, f1.ID, f2.ID)
	}
}
