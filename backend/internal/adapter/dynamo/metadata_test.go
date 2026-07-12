package dynamo

import (
	"context"
	"testing"
)

// TestCreateFile_PersistsFrontmatterMeta covers the CreateFile write path:
// aliases, type, status parsed from frontmatter must surface on the returned
// FileMetadata and round-trip through GetFile.
func TestCreateFile_PersistsFrontmatterMeta(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	body := "---\ntype: decision\nstatus: active\naliases:\n  - Auth\n  - Login\n---\n# Auth Design"
	created, err := m.CreateFile(ctx, "Auth Design", []byte(body), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if created.Type != "decision" {
		t.Errorf("created.Type = %q, want %q", created.Type, "decision")
	}
	if created.Status != "active" {
		t.Errorf("created.Status = %q, want %q", created.Status, "active")
	}
	if len(created.Aliases) != 2 || created.Aliases[0] != "Auth" || created.Aliases[1] != "Login" {
		t.Errorf("created.Aliases = %v, want [Auth Login]", created.Aliases)
	}

	got, err := m.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if got.Type != "decision" || got.Status != "active" {
		t.Errorf("GetFile round-trip type/status = %q/%q", got.Type, got.Status)
	}
	if len(got.Aliases) != 2 || got.Aliases[0] != "Auth" || got.Aliases[1] != "Login" {
		t.Errorf("GetFile round-trip Aliases = %v", got.Aliases)
	}
}

// TestSaveFile_ReExtractsMetaFromNewContent verifies SaveFile parses meta from
// the incoming content — not from the previously-fetched file — so removing or
// changing frontmatter is reflected on the next read.
func TestSaveFile_ReExtractsMetaFromNewContent(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	initial := "---\ntype: draft\nstatus: wip\naliases: [Foo]\n---\nbody"
	created, err := m.CreateFile(ctx, "Note", []byte(initial), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	updated := "---\ntype: decision\nstatus: active\naliases: [Bar, Baz]\n---\nchanged"
	saved, err := m.SaveFile(ctx, created.ID, []byte(updated), created.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if saved.Type != "decision" || saved.Status != "active" {
		t.Errorf("SaveFile returned type/status = %q/%q, want decision/active", saved.Type, saved.Status)
	}
	if len(saved.Aliases) != 2 || saved.Aliases[0] != "Bar" || saved.Aliases[1] != "Baz" {
		t.Errorf("SaveFile returned Aliases = %v, want [Bar Baz]", saved.Aliases)
	}

	got, err := m.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if got.Type != "decision" || got.Status != "active" {
		t.Errorf("post-save GetFile type/status = %q/%q", got.Type, got.Status)
	}
	if len(got.Aliases) != 2 || got.Aliases[0] != "Bar" || got.Aliases[1] != "Baz" {
		t.Errorf("post-save GetFile Aliases = %v", got.Aliases)
	}
}

// TestCreateFile_NoFrontmatter_LeavesMetaEmpty locks in the round-trip when a
// note has no frontmatter: aliases/type/status stay empty, and omitempty means
// no attribute is written.
func TestCreateFile_NoFrontmatter_LeavesMetaEmpty(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	created, err := m.CreateFile(ctx, "Plain", []byte("# just a title"), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if created.Type != "" || created.Status != "" || len(created.Aliases) != 0 {
		t.Errorf("expected empty meta, got type=%q status=%q aliases=%v",
			created.Type, created.Status, created.Aliases)
	}

	got, err := m.GetFile(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if got.Type != "" || got.Status != "" || len(got.Aliases) != 0 {
		t.Errorf("GetFile round-trip expected empty meta, got type=%q status=%q aliases=%v",
			got.Type, got.Status, got.Aliases)
	}
}

// TestDuplicateFile_CopiesMetaAndDerivedFields verifies a duplicated note
// carries aliases, type, status, headings, and links — matching the original
// byte-for-byte since the copy shares content.
func TestDuplicateFile_CopiesMetaAndDerivedFields(t *testing.T) {
	m := NewAdapter(nil, "user1", "")
	ctx := context.Background()

	// Create a target so the source's [[wiki-link]] is resolved.
	if _, err := m.CreateFile(ctx, "Target", []byte("# Target"), "root"); err != nil {
		t.Fatalf("create target: %v", err)
	}

	body := "---\ntype: howto\nstatus: active\naliases: [Alt]\n---\n# H1\nsee [[Target]]"
	orig, err := m.CreateFile(ctx, "Source", []byte(body), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if len(orig.Headings) == 0 {
		t.Fatalf("orig should have headings, got %v", orig.Headings)
	}
	if len(orig.Links) == 0 || !orig.Links[0].Resolved {
		t.Fatalf("orig should have a resolved link, got %+v", orig.Links)
	}

	dup, err := m.DuplicateFile(ctx, orig.ID)
	if err != nil {
		t.Fatalf("DuplicateFile: %v", err)
	}

	if dup.Type != orig.Type || dup.Status != orig.Status {
		t.Errorf("dup type/status = %q/%q, want %q/%q", dup.Type, dup.Status, orig.Type, orig.Status)
	}
	if len(dup.Aliases) != 1 || dup.Aliases[0] != "Alt" {
		t.Errorf("dup.Aliases = %v, want [Alt]", dup.Aliases)
	}

	// Verify the persisted duplicate also carries headings and links (via GetFile).
	got, err := m.GetFile(ctx, dup.ID)
	if err != nil {
		t.Fatalf("GetFile duplicated: %v", err)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "Alt" {
		t.Errorf("duplicated GetFile Aliases = %v", got.Aliases)
	}
	if got.Type != "howto" || got.Status != "active" {
		t.Errorf("duplicated GetFile type/status = %q/%q", got.Type, got.Status)
	}
	// DuplicateFile in map mode persists via FileMetadata, so the duplicated
	// file's stored Links reflect the copy — must include the same resolved
	// target the original had, not the previous default of dropping links.
	if len(got.Links) != 1 || !got.Links[0].Resolved {
		t.Errorf("duplicated Links = %+v, want one resolved link", got.Links)
	}
}
