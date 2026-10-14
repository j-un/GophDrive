package dynamo

import (
	"strings"

	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/core/markdown"
)

func normalizeTitle(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

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

func resolveLinks(content []byte, items []FileItem, prevLinks []adapter.LinkRef) []adapter.LinkRef {
	titles := markdown.ExtractWikiLinks(content)
	if len(titles) == 0 {
		return nil
	}

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
			cands = aliasCandidates[key]
		}
		switch len(cands) {
		case 0:
			result = append(result, adapter.LinkRef{Title: title, Resolved: false})
		case 1:
			result = append(result, adapter.LinkRef{Title: title, TargetID: cands[0].ID, Resolved: true})
		default:
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

func buildGraph(items []FileItem) []adapter.GraphNode {
	byID, titleToID, aliasToID := buildLookupMaps(items)

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
