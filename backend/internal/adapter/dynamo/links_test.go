package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/jun/gophdrive/backend/internal/adapter"
)

func TestResolveLinks_NewNote(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	// Create target note first.
	target, _ := m.CreateFile(ctx, "Auth Design", []byte("# Auth Design"), "root")

	// Create a note linking to it.
	src, err := m.CreateFile(ctx, "Overview", []byte("see [[Auth Design]]"), "root")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	if len(src.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(src.Links))
	}
	l := src.Links[0]
	if l.Title != "Auth Design" {
		t.Errorf("title = %q, want %q", l.Title, "Auth Design")
	}
	if !l.Resolved {
		t.Error("link should be resolved")
	}
	if l.TargetID != target.ID {
		t.Errorf("targetId = %q, want %q", l.TargetID, target.ID)
	}
}

func TestResolveLinks_Unresolved(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Does Not Exist]]"), "root")

	if len(src.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(src.Links))
	}
	if src.Links[0].Resolved {
		t.Error("link should be unresolved")
	}
}

func TestResolveLinks_CarryForwardOnResave(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	// Create target, then link to it.
	target, _ := m.CreateFile(ctx, "Auth Design", []byte("# Auth"), "root")
	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Auth Design]]"), "root")

	if src.Links[0].TargetID != target.ID {
		t.Fatalf("initial link not resolved")
	}

	// Rename target — in the map adapter there is no dedicated rename method
	// for this test; simulate by changing the stored Name directly via SaveFile
	// on the source to trigger re-resolve while the target name has changed.
	// Instead just verify that re-saving the source preserves the targetId.
	updated, err := m.SaveFile(ctx, src.ID, []byte("updated: see [[Auth Design]]"), src.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if len(updated.Links) != 1 {
		t.Fatalf("expected 1 link after re-save, got %d", len(updated.Links))
	}
	if updated.Links[0].TargetID != target.ID {
		t.Errorf("carry-forward failed: targetId = %q, want %q", updated.Links[0].TargetID, target.ID)
	}
}

func TestResolveLinks_LateTargetCreation(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	// Create source with unresolved link.
	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Future Note]]"), "root")
	if src.Links[0].Resolved {
		t.Fatal("should be unresolved initially")
	}

	// Create the target later.
	future, _ := m.CreateFile(ctx, "Future Note", []byte("# Future"), "root")

	// EnrichNoteLinks re-resolves on read.
	enriched, backs, err := m.EnrichNoteLinks(ctx, src.ID, src.Links)
	if err != nil {
		t.Fatalf("EnrichNoteLinks: %v", err)
	}
	if len(enriched) != 1 || !enriched[0].Resolved {
		t.Errorf("expected resolved link after enrichment, got %+v", enriched)
	}
	if enriched[0].TargetID != future.ID {
		t.Errorf("targetId = %q, want %q", enriched[0].TargetID, future.ID)
	}
	_ = backs
}

func TestEnrichNoteLinks_Backlinks(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	target, _ := m.CreateFile(ctx, "Core", []byte("# Core"), "root")
	srcA, _ := m.CreateFile(ctx, "A", []byte("[[Core]]"), "root")
	srcB, _ := m.CreateFile(ctx, "B", []byte("[[Core]]"), "root")

	_, backs, err := m.EnrichNoteLinks(ctx, target.ID, target.Links)
	if err != nil {
		t.Fatalf("EnrichNoteLinks: %v", err)
	}
	if len(backs) != 2 {
		t.Fatalf("expected 2 backlinks, got %d: %+v", len(backs), backs)
	}
	gotIDs := map[string]bool{backs[0].ID: true, backs[1].ID: true}
	if !gotIDs[srcA.ID] || !gotIDs[srcB.ID] {
		t.Errorf("backlink IDs mismatch: got %v, want %v and %v", gotIDs, srcA.ID, srcB.ID)
	}
}

func TestEnrichNoteLinks_CurrentTitle(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	target, _ := m.CreateFile(ctx, "Auth Design", []byte("# Auth"), "root")
	src, _ := m.CreateFile(ctx, "Overview", []byte("[[Auth Design]]"), "root")

	enriched, _, err := m.EnrichNoteLinks(ctx, src.ID, src.Links)
	if err != nil {
		t.Fatalf("EnrichNoteLinks: %v", err)
	}
	if len(enriched) == 0 {
		t.Fatal("expected enriched links")
	}
	if enriched[0].CurrentTitle != "Auth Design" {
		t.Errorf("currentTitle = %q, want %q", enriched[0].CurrentTitle, "Auth Design")
	}
	_ = target
}

func TestGraph_Basic(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	a, _ := m.CreateFile(ctx, "A", []byte("[[B]]"), "root")
	b, _ := m.CreateFile(ctx, "B", []byte("# B"), "root")

	nodes, err := m.Graph(ctx)
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	byID := make(map[string]adapter.GraphNode)
	for _, n := range nodes {
		byID[n.ID] = n
	}

	nodeA, ok := byID[a.ID]
	if !ok {
		t.Fatal("node A missing from graph")
	}
	if len(nodeA.Links) != 1 || nodeA.Links[0].TargetID != b.ID {
		t.Errorf("node A links = %+v, want link to B", nodeA.Links)
	}
	if nodeA.Links[0].CurrentTitle != "B" {
		t.Errorf("currentTitle in graph = %q, want %q", nodeA.Links[0].CurrentTitle, "B")
	}

	nodeB := byID[b.ID]
	if len(nodeB.Backlinks) != 1 || nodeB.Backlinks[0] != a.ID {
		t.Errorf("node B backlinks = %v, want [%s]", nodeB.Backlinks, a.ID)
	}
}

// TestResolveLinks_CarryForwardCaseChange locks in the case-insensitive
// carry-forward fix: editing a written token's case ([[API]] → [[api]]) while a
// newer same-titled note exists must keep the original targetId rather than
// silently re-resolving to the newer note.
func TestResolveLinks_CarryForwardCaseChange(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	a, _ := m.CreateFile(ctx, "API", []byte("# API v1"), "root")
	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[API]]"), "root")
	if src.Links[0].TargetID != a.ID {
		t.Fatalf("initial resolve: targetId = %q, want %q", src.Links[0].TargetID, a.ID)
	}

	// A second, more-recent note with the same title now exists. Without the
	// normalized carry-forward key, re-saving with a case-only edit would miss
	// the carried entry and re-resolve to b (most-recently-modified wins).
	b, _ := m.CreateFile(ctx, "API", []byte("# API v2"), "root")

	updated, err := m.SaveFile(ctx, src.ID, []byte("see [[api]]"), src.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if len(updated.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(updated.Links))
	}
	if updated.Links[0].TargetID != a.ID {
		t.Errorf("carry-forward across case change failed: targetId = %q, want %q (a), not %q (b)",
			updated.Links[0].TargetID, a.ID, b.ID)
	}
}

// TestResolveLinks_AliasResolution covers the alias fallback: a note whose
// frontmatter aliases include "X" is the target of [[X]] when no note is
// literally named X.
func TestResolveLinks_AliasResolution(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	target, err := m.CreateFile(ctx, "Auth Design",
		[]byte("---\naliases: [Login System]\n---\n# Auth"), "root")
	if err != nil {
		t.Fatalf("create target: %v", err)
	}

	src, err := m.CreateFile(ctx, "Overview", []byte("see [[Login System]]"), "root")
	if err != nil {
		t.Fatalf("create src: %v", err)
	}
	if len(src.Links) != 1 || !src.Links[0].Resolved {
		t.Fatalf("expected resolved link via alias, got %+v", src.Links)
	}
	if src.Links[0].TargetID != target.ID {
		t.Errorf("alias target = %q, want %q", src.Links[0].TargetID, target.ID)
	}
}

// TestResolveLinks_ExactNameBeatsAlias asserts a same-titled note wins over
// another note that lists the token as an alias.
func TestResolveLinks_ExactNameBeatsAlias(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	// Note A carries "Payments" as an alias; note B is literally named
	// "Payments". [[Payments]] must resolve to B regardless of modified-time
	// ordering — exact-name precedence overrides alias-recency.
	aliasNote, _ := m.CreateFile(ctx, "Billing",
		[]byte("---\naliases: [Payments]\n---\n# Billing"), "root")
	time.Sleep(2 * time.Millisecond)
	exact, _ := m.CreateFile(ctx, "Payments", []byte("# Payments"), "root")

	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Payments]]"), "root")
	if src.Links[0].TargetID != exact.ID {
		t.Errorf("exact-name precedence broken: targetId = %q, want %q (exact), not %q (alias owner)",
			src.Links[0].TargetID, exact.ID, aliasNote.ID)
	}
}

// TestResolveLinks_AliasCollision_MostRecentWins asserts the standard
// ambiguity rule applies within the alias bucket: two notes sharing the same
// alias resolve to the most-recently-modified one.
func TestResolveLinks_AliasCollision_MostRecentWins(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	older, _ := m.CreateFile(ctx, "A",
		[]byte("---\naliases: [Shared]\n---\n# A"), "root")
	time.Sleep(2 * time.Millisecond)
	newer, _ := m.CreateFile(ctx, "B",
		[]byte("---\naliases: [Shared]\n---\n# B"), "root")

	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Shared]]"), "root")
	if src.Links[0].TargetID != newer.ID {
		t.Errorf("alias collision: targetId = %q, want %q (newer), not %q (older)",
			src.Links[0].TargetID, newer.ID, older.ID)
	}
}

// TestResolveLinks_CarryForwardBeatsAliasReResolution locks in that
// carry-forward wins over a fresh alias resolution: if a link was already
// resolved to note A, adding a new note that aliases the token must not
// silently redirect an unchanged link.
func TestResolveLinks_CarryForwardBeatsAliasReResolution(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	target, _ := m.CreateFile(ctx, "Original", []byte("# Original"), "root")
	src, _ := m.CreateFile(ctx, "Overview", []byte("see [[Original]]"), "root")
	if src.Links[0].TargetID != target.ID {
		t.Fatalf("initial resolve failed: %+v", src.Links)
	}

	// A newer note now claims "Original" as an alias. Resaving src without
	// changing the token must preserve the original targetId.
	newer, _ := m.CreateFile(ctx, "Fresh",
		[]byte("---\naliases: [Original]\n---\n# Fresh"), "root")

	updated, err := m.SaveFile(ctx, src.ID, []byte("still: see [[Original]]"), src.ETag)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if updated.Links[0].TargetID != target.ID {
		t.Errorf("carry-forward across alias intro: targetId = %q, want %q (target), not %q (newer)",
			updated.Links[0].TargetID, target.ID, newer.ID)
	}
}

// TestResolveLinks_LegacyItemAliasFallback uses the pure resolveLinks function
// with a hand-built FileItem whose Aliases attribute is empty but whose
// Content carries frontmatter aliases — mirroring rows persisted before the
// aliases field existed.
func TestResolveLinks_LegacyItemAliasFallback(t *testing.T) {
	legacy := FileItem{
		ID:           "legacy-1",
		Name:         "Legacy.md",
		MIMEType:     "text/markdown",
		ModifiedTime: time.Now(),
		// Aliases attribute intentionally omitted; alias lives inside Content.
		Content: []byte("---\naliases:\n  - Historic Name\n---\nbody"),
	}
	links := resolveLinks([]byte("see [[Historic Name]]"), []FileItem{legacy}, nil)
	if len(links) != 1 || !links[0].Resolved || links[0].TargetID != "legacy-1" {
		t.Errorf("legacy fallback: got %+v, want one link resolved to legacy-1", links)
	}
}

// TestBacklinks_ParityWithGraph_LateTarget asserts EnrichNoteLinks backlinks
// agree with Graph backlinks for a target created after its referrer. Both
// paths must apply read-time enrichment so the late target's inbound link is
// visible identically through GET /notes/{id} and GET /graph.
func TestBacklinks_ParityWithGraph_LateTarget(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	ctx := context.Background()

	// Source links to a target that does not exist yet → unresolved at write.
	src, _ := m.CreateFile(ctx, "Source", []byte("[[Late Target]]"), "root")
	if src.Links[0].Resolved {
		t.Fatal("link should be unresolved at write time")
	}

	target, _ := m.CreateFile(ctx, "Late Target", []byte("# Late"), "root")

	_, backs, err := m.EnrichNoteLinks(ctx, target.ID, target.Links)
	if err != nil {
		t.Fatalf("EnrichNoteLinks: %v", err)
	}
	noteIDs := make([]string, len(backs))
	for i, b := range backs {
		noteIDs[i] = b.ID
	}

	nodes, err := m.Graph(ctx)
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	var graphBacks []string
	for _, n := range nodes {
		if n.ID == target.ID {
			graphBacks = n.Backlinks
		}
	}

	if len(noteIDs) != 1 || noteIDs[0] != src.ID {
		t.Errorf("EnrichNoteLinks backlinks = %v, want [%s]", noteIDs, src.ID)
	}
	if len(graphBacks) != 1 || graphBacks[0] != src.ID {
		t.Errorf("Graph backlinks = %v, want [%s]", graphBacks, src.ID)
	}
	if len(noteIDs) != len(graphBacks) || (len(noteIDs) == 1 && noteIDs[0] != graphBacks[0]) {
		t.Errorf("parity broken: EnrichNoteLinks=%v Graph=%v", noteIDs, graphBacks)
	}
}
