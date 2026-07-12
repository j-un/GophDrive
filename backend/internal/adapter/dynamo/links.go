package dynamo

import (
	"strings"

	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/core/markdown"
)

// normalizeTitle lowercases and trims a note display name for fuzzy matching.
func normalizeTitle(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// itemAliases returns the note's aliases, lazily parsing frontmatter for rows
// persisted before the aliases attribute existed (mirrors the tags fallback).
func itemAliases(it FileItem) []string {
	if len(it.Aliases) > 0 {
		return it.Aliases
	}
	if len(it.Content) == 0 {
		return nil
	}
	meta, _, _ := markdown.ParseNoteMeta(it.Content)
	return meta.Aliases
}

// resolveLinks extracts [[wiki-link]] tokens from content and resolves each to
// a stored note UUID.
//
// Carry-forward rule: if a token already has a resolved entry in prevLinks and
// its targetId still points to a live note, that targetId is preserved without
// re-resolving by title. This makes links stable across target renames — the
// source body keeps the original text, but the stored id remains correct.
//
// Resolution order per token: carry-forward → exact-name candidates → alias
// candidates → unresolved. Alias candidates are consulted only when there are
// zero name candidates, so an exact-name match always beats an alias match.
// Within each bucket:
//   - 0 candidates → try the next bucket (or unresolved)
//   - 1 candidate  → resolved
//   - >1 candidates → most-recently-modified wins, lexically-smallest ID tiebreak
func resolveLinks(content []byte, items []FileItem, prevLinks []adapter.LinkRef) []adapter.LinkRef {
	titles := markdown.ExtractWikiLinks(content)
	if len(titles) == 0 {
		return nil
	}

	// Build lookup maps from the current user note set.
	byID := make(map[string]FileItem, len(items))
	candidates := make(map[string][]FileItem, len(items))
	aliasCandidates := make(map[string][]FileItem, len(items))
	for _, it := range items {
		if it.MIMEType != "text/markdown" {
			continue
		}
		byID[it.ID] = it
		key := normalizeTitle(fromStoredName(it.Name))
		candidates[key] = append(candidates[key], it)
		for _, a := range itemAliases(it) {
			ak := normalizeTitle(a)
			if ak == "" {
				continue
			}
			aliasCandidates[ak] = append(aliasCandidates[ak], it)
		}
	}

	// Index existing resolved links by normalized written title for
	// carry-forward. The key is normalized so a case-only edit of the written
	// token (e.g. [[API]] → [[api]]) still matches its prior targetId instead
	// of silently re-resolving to a different note.
	carried := make(map[string]string, len(prevLinks))
	for _, l := range prevLinks {
		if l.Resolved && l.TargetID != "" {
			if _, alive := byID[l.TargetID]; alive {
				carried[normalizeTitle(l.Title)] = l.TargetID
			}
		}
	}

	result := make([]adapter.LinkRef, 0, len(titles))
	for _, title := range titles {
		if id, ok := carried[normalizeTitle(title)]; ok {
			result = append(result, adapter.LinkRef{Title: title, TargetID: id, Resolved: true})
			continue
		}
		key := normalizeTitle(title)
		cands := candidates[key]
		if len(cands) == 0 {
			// Only fall back to aliases when no note carries this exact name.
			cands = aliasCandidates[key]
		}
		switch len(cands) {
		case 0:
			result = append(result, adapter.LinkRef{Title: title, Resolved: false})
		case 1:
			result = append(result, adapter.LinkRef{Title: title, TargetID: cands[0].ID, Resolved: true})
		default:
			// Most-recently-modified wins; ties broken by lexically smallest
			// ID so the result is deterministic at sub-second timestamps.
			best := cands[0]
			for _, c := range cands[1:] {
				if c.ModifiedTime.After(best.ModifiedTime) ||
					(c.ModifiedTime.Equal(best.ModifiedTime) && c.ID < best.ID) {
					best = c
				}
			}
			result = append(result, adapter.LinkRef{Title: title, TargetID: best.ID, Resolved: true})
		}
	}
	return result
}

// resolveLinksLazy resolves the [[wiki-links]] in content, but invokes scan
// only when the content actually contains link tokens. A link-free save (the
// common case) thus skips the potentially large user-item scan entirely; it
// also correctly clears any previously stored links by returning nil.
//
// The scan runs outside the optimistic-ETag transaction, so a concurrent
// create of a same-titled note between scan and persist can make collision
// resolution non-deterministic. This is accepted: resolved targetIds are
// stable across re-saves (carry-forward), and read-time enrichment
// re-resolves any link still unresolved at write time.
func resolveLinksLazy(content []byte, prevLinks []adapter.LinkRef, scan func() ([]FileItem, error)) ([]adapter.LinkRef, error) {
	if len(markdown.ExtractWikiLinks(content)) == 0 {
		return nil, nil
	}
	items, err := scan()
	if err != nil {
		return nil, err
	}
	return resolveLinks(content, items, prevLinks), nil
}

// buildLookupMaps constructs three indexes over a flat user-item list:
//   - byID: note UUID → FileItem (all mime types)
//   - titleToID: normalized display name → note UUID (markdown only, last seen wins)
//   - aliasToID: normalized alias → note UUID (markdown only, last seen wins)
//
// titleToID and aliasToID are kept separate so name precedence stays explicit
// at call sites — enrichLinks consults titleToID first, then aliasToID.
func buildLookupMaps(items []FileItem) (byID map[string]FileItem, titleToID map[string]string, aliasToID map[string]string) {
	byID = make(map[string]FileItem, len(items))
	titleToID = make(map[string]string, len(items))
	aliasToID = make(map[string]string, len(items))
	for _, it := range items {
		byID[it.ID] = it
		if it.MIMEType == "text/markdown" {
			titleToID[normalizeTitle(fromStoredName(it.Name))] = it.ID
			for _, a := range itemAliases(it) {
				ak := normalizeTitle(a)
				if ak == "" {
					continue
				}
				aliasToID[ak] = it.ID
			}
		}
	}
	return
}

// enrichLinks returns a copy of links with CurrentTitle filled from the live
// note set and unresolved links re-resolved where the target now exists.
// Resolved entries' TargetIDs are kept as-is (rename stability). Re-resolution
// tries the exact-name index first, then aliases, so an exact-name match
// always beats an alias match.
func enrichLinks(links []adapter.LinkRef, byID map[string]FileItem, titleToID map[string]string, aliasToID map[string]string) []adapter.LinkRef {
	if len(links) == 0 {
		return nil
	}
	out := make([]adapter.LinkRef, len(links))
	for i, l := range links {
		e := l
		if l.Resolved && l.TargetID != "" {
			if it, ok := byID[l.TargetID]; ok {
				e.CurrentTitle = fromStoredName(it.Name)
			}
		} else if !l.Resolved {
			key := normalizeTitle(l.Title)
			id, ok := titleToID[key]
			if !ok {
				id, ok = aliasToID[key]
			}
			if ok {
				e.TargetID = id
				e.Resolved = true
				if it, ok2 := byID[id]; ok2 {
					e.CurrentTitle = fromStoredName(it.Name)
				}
			}
		}
		out[i] = e
	}
	return out
}

// backlinksOf returns one BacklinkEntry for each markdown note whose links —
// after the same read-time enrichment buildGraph applies — contain a resolved
// reference to noteID. Enriching here (rather than reading stored Links) keeps
// GET /notes/{id} backlinks in agreement with GET /graph for late-created
// targets whose source links were unresolved at write time.
func backlinksOf(noteID string, items []FileItem, byID map[string]FileItem, titleToID map[string]string, aliasToID map[string]string) []adapter.BacklinkEntry {
	var result []adapter.BacklinkEntry
	for _, it := range items {
		if it.MIMEType != "text/markdown" {
			continue
		}
		for _, l := range enrichLinks(it.Links, byID, titleToID, aliasToID) {
			if l.Resolved && l.TargetID == noteID {
				result = append(result, adapter.BacklinkEntry{
					ID:   it.ID,
					Name: fromStoredName(it.Name),
				})
				break
			}
		}
	}
	return result
}

// buildGraph converts a flat item list into GraphNodes with enriched links and
// precomputed backlink IDs.
//
// Backlinks are derived from enriched links so that late-created targets (whose
// links were unresolved at write time but re-resolved by enrichLinks) are
// correctly reflected in the graph.
func buildGraph(items []FileItem) []adapter.GraphNode {
	byID, titleToID, aliasToID := buildLookupMaps(items)

	// First pass: enrich all notes so re-resolved links feed into backlinks.
	type noteRow struct {
		item     FileItem
		enriched []adapter.LinkRef
	}
	rows := make([]noteRow, 0, len(items))
	for _, it := range items {
		if it.MIMEType != "text/markdown" {
			continue
		}
		rows = append(rows, noteRow{item: it, enriched: enrichLinks(it.Links, byID, titleToID, aliasToID)})
	}

	// Build backlink map from enriched links.
	backlinksMap := make(map[string][]string)
	for _, r := range rows {
		for _, l := range r.enriched {
			if l.Resolved && l.TargetID != "" {
				backlinksMap[l.TargetID] = append(backlinksMap[l.TargetID], r.item.ID)
			}
		}
	}

	nodes := make([]adapter.GraphNode, 0, len(rows))
	for _, r := range rows {
		nodes = append(nodes, adapter.GraphNode{
			ID:        r.item.ID,
			Title:     fromStoredName(r.item.Name),
			Tags:      r.item.Tags,
			Links:     r.enriched,
			Backlinks: backlinksMap[r.item.ID],
			Modified:  r.item.ModifiedTime,
		})
	}
	return nodes
}
