package dynamo

import (
	"context"
	"testing"
)

// Export should return paths rooted at the base folder, with intermediate
// folder names joined by "/", and skip non-note rows entirely.
func TestAdapter_Export_NestedPaths(t *testing.T) {
	ctx := context.Background()

	root := NewAdapter(nil, "user1", "")
	base, err := root.CreateFolder(ctx, "MyNotes", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder base: %v", err)
	}

	a := NewAdapter(nil, "user1", base.ID)
	sub, err := a.CreateFolder(ctx, "Sub", []string{base.ID})
	if err != nil {
		t.Fatalf("CreateFolder sub: %v", err)
	}
	if _, err := a.CreateFile(ctx, "top", []byte("top body"), base.ID); err != nil {
		t.Fatalf("CreateFile top: %v", err)
	}
	if _, err := a.CreateFile(ctx, "deep", []byte("deep body"), sub.ID); err != nil {
		t.Fatalf("CreateFile deep: %v", err)
	}

	entries, err := a.Export(ctx)
	if err != nil {
		t.Fatalf("Export returned err: %v", err)
	}

	got := map[string]string{}
	for _, e := range entries {
		got[e.Path] = string(e.Content)
	}

	if got["top.md"] != "top body" {
		t.Errorf("top-level note path = missing/wrong, have %v", got)
	}
	if got["Sub/deep.md"] != "deep body" {
		t.Errorf("nested note path = missing/wrong, have %v", got)
	}
	for path := range got {
		if path == "MyNotes/top.md" || path == "MyNotes/Sub/deep.md" {
			t.Errorf("path %q includes the base folder; paths should be relative to it", path)
		}
	}
}

// A note created without a base folder should still get a sensible path.
// Folders are emitted as their name; the note itself sits at the root.
func TestAdapter_Export_NoBaseFolder(t *testing.T) {
	ctx := context.Background()

	a := NewAdapter(nil, "user1", "")
	folder, err := a.CreateFolder(ctx, "Loose", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if _, err := a.CreateFile(ctx, "child", []byte("child"), folder.ID); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := a.CreateFile(ctx, "rooted", []byte("rooted"), "root"); err != nil {
		t.Fatalf("CreateFile rooted: %v", err)
	}

	entries, err := a.Export(ctx)
	if err != nil {
		t.Fatalf("Export err: %v", err)
	}

	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	if !paths["Loose/child.md"] {
		t.Errorf("expected Loose/child.md in %v", paths)
	}
	if !paths["rooted.md"] {
		t.Errorf("expected rooted.md in %v", paths)
	}
}

// Folders themselves must not appear as export entries.
func TestAdapter_Export_FoldersAreNotEntries(t *testing.T) {
	ctx := context.Background()

	a := NewAdapter(nil, "user1", "")
	if _, err := a.CreateFolder(ctx, "EmptyFolder", []string{"root"}); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	entries, err := a.Export(ctx)
	if err != nil {
		t.Fatalf("Export err: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for folders-only adapter, got %d (%+v)", len(entries), entries)
	}
}

// Notes with BodyS3Key set live in S3 — read isn't wired yet, so they must
// not be silently exported with empty content.
func TestAdapter_Export_SkipsSpilloverRows(t *testing.T) {
	a := NewAdapter(nil, "user1", "")

	items := []FileItem{
		{ID: "f1", Name: "inline.md", MIMEType: "text/markdown", Parents: []string{"root"}, Content: []byte("body")},
		{ID: "f2", Name: "spill.md", MIMEType: "text/markdown", Parents: []string{"root"}, BodyS3Key: "users/u/abc"},
	}
	entries, err := a.buildExport(items)
	if err != nil {
		t.Fatalf("buildExport err: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (spillover skipped), got %d", len(entries))
	}
	if entries[0].Path != "inline.md" {
		t.Errorf("entry path = %q, want inline.md", entries[0].Path)
	}
}

// A corrupt parent chain (cycle) must not hang the export.
func TestAdapter_Export_CycleSafe(t *testing.T) {
	a := NewAdapter(nil, "user1", "")

	items := []FileItem{
		{ID: "fa", Name: "A", MIMEType: "application/vnd.google-apps.folder", Parents: []string{"fb"}},
		{ID: "fb", Name: "B", MIMEType: "application/vnd.google-apps.folder", Parents: []string{"fa"}},
		{ID: "n1", Name: "stuck.md", MIMEType: "text/markdown", Parents: []string{"fa"}, Content: []byte("x")},
	}
	entries, err := a.buildExport(items)
	if err != nil {
		t.Fatalf("buildExport err: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Path traversal stops on cycle; we still get a usable filename.
	if entries[0].Content == nil || string(entries[0].Content) != "x" {
		t.Errorf("content = %q, want x", entries[0].Content)
	}
}
