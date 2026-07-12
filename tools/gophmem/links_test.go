package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// noteUUID is a UUID-shaped test note ID. resolveNoteID short-circuits on
// UUID-shaped input (see looksLikeUUID), so tests can point runLinks /
// runBacklinks straight at a note without also faking a /search response.
const noteUUID = "550e8400-e29b-41d4-a716-446655440000"

// ---- runLinks ----

func TestRunLinks_ResolvedUnresolvedAndCurrentTitlePreference(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/"+noteUUID) {
			writeJSON(w, 200, NoteResponse{
				ID:   noteUUID,
				Name: "source.md",
				Links: []LinkRef{
					{Title: "Old Title", TargetID: "id-a", CurrentTitle: "New Title", Resolved: true},
					{Title: "No Rename", TargetID: "id-b", Resolved: true},
					{Title: "Missing Note", Resolved: false},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer close()

	var buf strings.Builder
	if err := runLinks(client, []string{noteUUID}, &buf); err != nil {
		t.Fatalf("runLinks: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "id-a\tNew Title\n") {
		t.Errorf("expected CurrentTitle to win over Title, got:\n%s", out)
	}
	if !strings.Contains(out, "id-b\tNo Rename\n") {
		t.Errorf("expected fallback to Title when CurrentTitle empty, got:\n%s", out)
	}
	if !strings.Contains(out, "-\tMissing Note\t[unresolved]\n") {
		t.Errorf("expected unresolved link line, got:\n%s", out)
	}
}

func TestRunLinks_NoLinks(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, NoteResponse{ID: noteUUID, Name: "empty.md"})
	}))
	defer close()

	var buf strings.Builder
	if err := runLinks(client, []string{noteUUID}, &buf); err != nil {
		t.Fatalf("runLinks: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "(no links)" {
		t.Errorf("expected (no links), got: %q", buf.String())
	}
}

// ---- runBacklinks ----

func TestRunBacklinks_Empty(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, NoteResponse{ID: noteUUID, Name: "lonely.md"})
	}))
	defer close()

	var buf strings.Builder
	if err := runBacklinks(client, []string{noteUUID}, &buf); err != nil {
		t.Fatalf("runBacklinks: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "(no backlinks)" {
		t.Errorf("expected (no backlinks), got: %q", buf.String())
	}
}

func TestRunBacklinks_Renders(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, NoteResponse{
			ID:   noteUUID,
			Name: "target.md",
			Backlinks: []BacklinkEntry{
				{ID: "src-1", Name: "Source One"},
				{ID: "src-2", Name: "Source Two"},
			},
		})
	}))
	defer close()

	var buf strings.Builder
	if err := runBacklinks(client, []string{noteUUID}, &buf); err != nil {
		t.Fatalf("runBacklinks: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "src-1\tSource One\n") || !strings.Contains(out, "src-2\tSource Two\n") {
		t.Errorf("unexpected backlinks output:\n%s", out)
	}
}

// ---- runGraph: whole vault ----

func TestRunGraph_WholeVault_SortedByModifiedDesc(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graph" {
			http.NotFound(w, r)
			return
		}
		// Server returns oldest first; command must re-sort newest first.
		writeJSON(w, 200, []GraphNode{
			{ID: "old-id", Title: "Old Note", Modified: older},
			{ID: "new-id", Title: "New Note", Tags: []string{"a", "b"}, Modified: newer},
		})
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, nil, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()

	newIdx := strings.Index(out, "new-id")
	oldIdx := strings.Index(out, "old-id")
	if newIdx == -1 || oldIdx == -1 || newIdx > oldIdx {
		t.Errorf("expected newer node first, got:\n%s", out)
	}
	if !strings.Contains(out, "new-id\tNew Note\t[a,b]\n") {
		t.Errorf("expected tags rendered for new-id, got:\n%s", out)
	}
	if !strings.Contains(out, "old-id\tOld Note\t[-]\n") {
		t.Errorf("expected [-] placeholder for no tags, got:\n%s", out)
	}
}

// ---- runGraph: --center BFS ----

// fourNodeChain builds A -> B -> C -> D (resolved links forward, backlinks
// pointing back), so BFS from A discovers one extra hop per depth level.
func fourNodeChain() []GraphNode {
	return []GraphNode{
		{
			ID:    "a-id",
			Title: "Node A",
			Links: []LinkRef{{Title: "Node B", TargetID: "b-id", CurrentTitle: "Node B", Resolved: true}},
		},
		{
			ID:        "b-id",
			Title:     "Node B",
			Links:     []LinkRef{{Title: "Node C", TargetID: "c-id", CurrentTitle: "Node C", Resolved: true}},
			Backlinks: []string{"a-id"},
		},
		{
			ID:        "c-id",
			Title:     "Node C",
			Links:     []LinkRef{{Title: "Node D", TargetID: "d-id", CurrentTitle: "Node D", Resolved: true}},
			Backlinks: []string{"b-id"},
		},
		{
			ID:        "d-id",
			Title:     "Node D",
			Backlinks: []string{"c-id"},
		},
	}
}

func TestRunGraph_CenterDepth1(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, fourNodeChain())
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, []string{"--center", "Node A"}, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "a-id\tNode A\t[-]") {
		t.Errorf("expected center node A in output, got:\n%s", out)
	}
	if !strings.Contains(out, "b-id\tNode B\t[-]") {
		t.Errorf("expected depth-1 neighbor B in output, got:\n%s", out)
	}
	if strings.Contains(out, "c-id\tNode C") {
		t.Errorf("depth 1 must not include node C, got:\n%s", out)
	}
	if strings.Contains(out, "d-id\tNode D") {
		t.Errorf("depth 1 must not include node D, got:\n%s", out)
	}
}

func TestRunGraph_CenterDepth2(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, fourNodeChain())
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, []string{"--center", "Node A", "--depth", "2"}, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "a-id\tNode A") || !strings.Contains(out, "b-id\tNode B") {
		t.Errorf("expected nodes A and B in output, got:\n%s", out)
	}
	if !strings.Contains(out, "c-id\tNode C") {
		t.Errorf("depth 2 must include node C, got:\n%s", out)
	}
	if strings.Contains(out, "d-id\tNode D") {
		t.Errorf("depth 2 must not include node D, got:\n%s", out)
	}
}

func TestRunGraph_CenterDepth0_OnlyCenter(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, fourNodeChain())
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, []string{"--center", "Node A", "--depth", "0"}, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "a-id\tNode A") {
		t.Errorf("expected center node, got:\n%s", out)
	}
	if strings.Contains(out, "b-id") {
		t.Errorf("depth 0 must include only the center, got:\n%s", out)
	}
}

func TestRunGraph_CenterResolvedByTitleCaseInsensitive(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, fourNodeChain())
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, []string{"--center", "node b"}, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "b-id\tNode B") {
		t.Errorf("expected case-insensitive title resolution to center on B, got:\n%s", out)
	}
}

func TestRunGraph_CenterNotFound(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, fourNodeChain())
	}))
	defer close()

	err := runGraph(client, []string{"--center", "does not exist"}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error for unresolvable center, got nil")
	}
	if !strings.Contains(err.Error(), "note not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunGraph_DepthWithoutCenterIsError(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s (should have errored before any call)", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer close()

	err := runGraph(client, []string{"--depth", "2"}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error when --depth is passed without --center")
	}
}

func TestRunGraph_LinksAndBacklinksRendered(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []GraphNode{
			{
				ID:    "a-id",
				Title: "Node A",
				Links: []LinkRef{
					{Title: "Node B", TargetID: "b-id", CurrentTitle: "Node B", Resolved: true},
					{Title: "Ghost Note", Resolved: false},
				},
			},
			{
				ID:        "b-id",
				Title:     "Node B",
				Backlinks: []string{"a-id"},
			},
		})
	}))
	defer close()

	var buf strings.Builder
	if err := runGraph(client, nil, &buf); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\t-> Node B\n") {
		t.Errorf("expected resolved outbound link line, got:\n%s", out)
	}
	if !strings.Contains(out, "\t->? Ghost Note\n") {
		t.Errorf("expected unresolved outbound link line, got:\n%s", out)
	}
	if !strings.Contains(out, "\t<- Node A\n") {
		t.Errorf("expected backlink line with resolved source title, got:\n%s", out)
	}
}

// ---- runUnresolved ----

func TestRunUnresolved_Empty(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []GraphNode{{ID: "a-id", Title: "Node A"}})
	}))
	defer close()

	var buf strings.Builder
	if err := runUnresolved(client, nil, &buf); err != nil {
		t.Fatalf("runUnresolved: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "(none)" {
		t.Errorf("expected (none), got: %q", buf.String())
	}
}

func TestRunUnresolved_AggregationAndOrdering(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []GraphNode{
			{
				ID:    "a-id",
				Title: "Note A",
				Links: []LinkRef{
					{Title: "Popular Ghost", Resolved: false},
					{Title: "  popular ghost  ", Resolved: false}, // same key, different casing/whitespace
					{Title: "Zeta Ghost", Resolved: false},
				},
			},
			{
				ID:    "b-id",
				Title: "Note B",
				Links: []LinkRef{
					{Title: "Popular Ghost", Resolved: false},
					{Title: "Alpha Ghost", Resolved: false},
					{Title: "Real Target", TargetID: "x", Resolved: true}, // resolved: excluded
				},
			},
		})
	}))
	defer close()

	var buf strings.Builder
	if err := runUnresolved(client, nil, &buf); err != nil {
		t.Fatalf("runUnresolved: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 aggregated unresolved titles, got %d:\n%s", len(lines), buf.String())
	}
	// "Popular Ghost" has 3 refs (2 from A, 1 from B) -> must be first (count desc).
	if !strings.HasPrefix(lines[0], "Popular Ghost\t3\t") {
		t.Errorf("expected Popular Ghost with count 3 first, got: %q", lines[0])
	}
	if !strings.Contains(lines[0], "Note A") || !strings.Contains(lines[0], "Note B") {
		t.Errorf("expected both source notes joined, got: %q", lines[0])
	}
	// Remaining two both have count 1 -> tie-broken alphabetically: Alpha before Zeta.
	if !strings.HasPrefix(lines[1], "Alpha Ghost\t1\tNote B") {
		t.Errorf("expected Alpha Ghost second (count tie, alpha asc), got: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "Zeta Ghost\t1\tNote A") {
		t.Errorf("expected Zeta Ghost third, got: %q", lines[2])
	}
}

// ---- Client.GetGraph ----

func TestClient_GetGraph_Success(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graph" || r.Method != "GET" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, []GraphNode{{ID: "n1", Title: "Note One"}})
	}))
	defer close()

	nodes, err := client.GetGraph()
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Errorf("unexpected nodes: %+v", nodes)
	}
}

func TestClient_GetGraph_NonOKStatus(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer close()

	_, err := client.GetGraph()
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestClient_GetGraph_NilBodyBecomesEmptySlice(t *testing.T) {
	client, close := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []GraphNode(nil))
	}))
	defer close()

	nodes, err := client.GetGraph()
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if nodes == nil {
		t.Error("expected non-nil empty slice for nil body, got nil")
	}
	if len(nodes) != 0 {
		t.Errorf("expected empty slice, got %+v", nodes)
	}
}
