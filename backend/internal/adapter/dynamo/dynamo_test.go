package dynamo

import (
	"context"
	"errors"
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
	if !errors.Is(err, adapter.ErrNotFound) {
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

func TestAdapter_MoveFile(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Create: root → folderA → folderB
	folderA, _ := m.CreateFolder(ctx, "FolderA", []string{"root"})
	folderB, _ := m.CreateFolder(ctx, "FolderB", []string{folderA.ID})
	note, _ := m.CreateFile(ctx, "note.md", []byte("data"), "root")
	originalETag := note.ETag

	// Move note from root into folderA
	moved, err := m.MoveFile(ctx, note.ID, folderA.ID)
	if err != nil {
		t.Fatalf("MoveFile(note) failed: %v", err)
	}
	if len(moved.Parents) == 0 || moved.Parents[0] != folderA.ID {
		t.Errorf("Expected parent %s, got %v", folderA.ID, moved.Parents)
	}
	if moved.ETag == originalETag {
		t.Error("Expected ETag to change after move")
	}

	// Verify the note appears in folderA's listing
	items, _ := m.ListFiles(ctx, folderA.ID)
	found := false
	for _, it := range items {
		if it.ID == note.ID {
			found = true
		}
	}
	if !found {
		t.Error("Moved note not found in destination folder")
	}

	// Move a folder: folderB into root
	movedFolder, err := m.MoveFile(ctx, folderB.ID, "root")
	if err != nil {
		t.Fatalf("MoveFile(folder) failed: %v", err)
	}
	if len(movedFolder.Parents) == 0 || movedFolder.Parents[0] != "root" {
		t.Errorf("Expected parent 'root', got %v", movedFolder.Parents)
	}

	// ErrNotFound for unknown item
	_, err = m.MoveFile(ctx, "no-such-id", folderA.ID)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for unknown item, got %v", err)
	}

	// ErrNotFound for unknown destination
	_, err = m.MoveFile(ctx, note.ID, "no-such-dest")
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for unknown destination, got %v", err)
	}
}

func TestAdapter_MoveFile_CycleRejected(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Build: root → parent → child
	parent, _ := m.CreateFolder(ctx, "Parent", []string{"root"})
	child, _ := m.CreateFolder(ctx, "Child", []string{parent.ID})

	// Moving parent into itself must be rejected
	_, err := m.MoveFile(ctx, parent.ID, parent.ID)
	if !errors.Is(err, adapter.ErrInvalidMove) {
		t.Errorf("Expected ErrInvalidMove for self-move, got %v", err)
	}

	// Moving parent into its own descendant must be rejected
	_, err = m.MoveFile(ctx, parent.ID, child.ID)
	if !errors.Is(err, adapter.ErrInvalidMove) {
		t.Errorf("Expected ErrInvalidMove for descendant-move, got %v", err)
	}

	// Moving child into parent is fine (not a cycle)
	grandchild, _ := m.CreateFolder(ctx, "Grandchild", []string{child.ID})
	_, err = m.MoveFile(ctx, grandchild.ID, parent.ID)
	if err != nil {
		t.Errorf("Expected no error for valid move, got %v", err)
	}
}

func TestAdapter_MoveFile_ResolvesBaseFolder(t *testing.T) {
	m := NewAdapter(nil, "user1", "base-folder-1")
	ctx := context.Background()

	note, _ := m.CreateFile(ctx, "note.md", []byte("data"), "root")

	// Empty newParentID must resolve to BaseFolderID
	moved, err := m.MoveFile(ctx, note.ID, "")
	if err != nil {
		t.Fatalf("MoveFile with empty parentID failed: %v", err)
	}
	if len(moved.Parents) == 0 || moved.Parents[0] != "base-folder-1" {
		t.Errorf("Expected parent base-folder-1, got %v", moved.Parents)
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
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Parent should be deleted, got error: %v", err)
	}

	// 6. Verify Child Folder Gone
	_, err = m.GetFile(ctx, childFolder.ID)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Child Folder should be deleted, got error: %v", err)
	}

	// 7. Verify Child File Gone
	_, err = m.GetFile(ctx, childFile.ID)
	if !errors.Is(err, adapter.ErrNotFound) {
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

func TestAdapter_CreateFile_InvalidParent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	_, err := m.CreateFile(ctx, "note.md", []byte("content"), "nonexistent-folder-id")
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for invalid parentId, got %v", err)
	}
}

func TestAdapter_CreateFile_SentinelParentsAllowed(t *testing.T) {
	ctx := context.Background()

	// "root" must be accepted without a DB entry
	m1 := NewAdapter(nil, "user1", "")
	f1, err := m1.CreateFile(ctx, "note.md", []byte("x"), "root")
	if err != nil {
		t.Fatalf("Expected no error for root parentId, got %v", err)
	}
	if f1 == nil || f1.ID == "" {
		t.Error("Expected non-nil file with ID for root parentId")
	}

	// BaseFolderID must be accepted even though it has no entry in m.files
	m2 := NewAdapter(nil, "user1", "my-base-folder")
	f2, err := m2.CreateFile(ctx, "note.md", []byte("x"), "my-base-folder")
	if err != nil {
		t.Fatalf("Expected no error for BaseFolderID parentId, got %v", err)
	}
	if f2 == nil || f2.ID == "" {
		t.Error("Expected non-nil file with ID for BaseFolderID parentId")
	}
}

func TestAdapter_CreateFolder_InvalidParent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	_, err := m.CreateFolder(ctx, "NewFolder", []string{"nonexistent-folder-id"})
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for invalid parentId, got %v", err)
	}
}

func TestAdapter_CreateFolder_SentinelParentsAllowed(t *testing.T) {
	ctx := context.Background()

	m1 := NewAdapter(nil, "user1", "")
	f1, err := m1.CreateFolder(ctx, "F", []string{"root"})
	if err != nil {
		t.Fatalf("Expected no error for root parentId, got %v", err)
	}
	if f1 == nil || f1.ID == "" {
		t.Error("Expected non-nil folder with ID for root parentId")
	}

	m2 := NewAdapter(nil, "user1", "base-folder-abc")
	f2, err := m2.CreateFolder(ctx, "F", []string{"base-folder-abc"})
	if err != nil {
		t.Fatalf("Expected no error for BaseFolderID parentId, got %v", err)
	}
	if f2 == nil || f2.ID == "" {
		t.Error("Expected non-nil folder with ID for BaseFolderID parentId")
	}
}

func TestAdapter_CreateFile_NonFolderParentRejected(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	note, _ := m.CreateFile(ctx, "note.md", []byte("x"), "root")
	_, err := m.CreateFile(ctx, "child.md", []byte("x"), note.ID)
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound when parentId is a file (not folder), got %v", err)
	}
}

func TestAdapter_CreateFolder_NonFolderParentRejected(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	note, _ := m.CreateFile(ctx, "note.md", []byte("x"), "root")
	_, err := m.CreateFolder(ctx, "Sub", []string{note.ID})
	if !errors.Is(err, adapter.ErrNotFound) {
		t.Errorf("Expected ErrNotFound when parentId is a file (not folder), got %v", err)
	}
}
