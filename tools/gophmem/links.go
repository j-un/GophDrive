package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---- links ----

func runLinks(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("links", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gophmem links <id|title>")
	}
	idOrTitle := strings.Join(fs.Args(), " ")

	id, err := resolveNoteID(client, idOrTitle)
	if err != nil {
		return err
	}
	note, err := client.GetNote(id)
	if err != nil {
		return err
	}

	if len(note.Links) == 0 {
		fmt.Fprintln(out, "(no links)")
		return nil
	}
	for _, l := range note.Links {
		if l.Resolved {
			fmt.Fprintf(out, "%s\t%s\n", l.TargetID, linkDisplay(l))
		} else {
			fmt.Fprintf(out, "-\t%s\t[unresolved]\n", l.Title)
		}
	}
	return nil
}

// linkDisplay returns the preferred display name for a LinkRef: the target's
// current title if known, else the title as written at the source.
func linkDisplay(l LinkRef) string {
	if l.CurrentTitle != "" {
		return l.CurrentTitle
	}
	return l.Title
}

// ---- backlinks ----

func runBacklinks(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("backlinks", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gophmem backlinks <id|title>")
	}
	idOrTitle := strings.Join(fs.Args(), " ")

	id, err := resolveNoteID(client, idOrTitle)
	if err != nil {
		return err
	}
	note, err := client.GetNote(id)
	if err != nil {
		return err
	}

	if len(note.Backlinks) == 0 {
		fmt.Fprintln(out, "(no backlinks)")
		return nil
	}
	for _, b := range note.Backlinks {
		fmt.Fprintf(out, "%s\t%s\n", b.ID, b.Name)
	}
	return nil
}

// ---- graph ----

func runGraph(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	centerFlag := fs.String("center", "", "center the view on this note (id or title)")
	depthFlag := fs.Int("depth", 1, "BFS depth from --center")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var depthSet bool
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "depth" {
			depthSet = true
		}
	})
	if depthSet && *centerFlag == "" {
		return fmt.Errorf("--depth requires --center")
	}

	nodes, err := client.GetGraph()
	if err != nil {
		return err
	}
	byID := indexGraphNodesByID(nodes)

	if *centerFlag == "" {
		sorted := make([]GraphNode, len(nodes))
		copy(sorted, nodes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Modified.After(sorted[j].Modified)
		})
		for _, n := range sorted {
			printGraphNode(out, n, byID)
		}
		return nil
	}

	center, err := resolveGraphCenter(nodes, *centerFlag)
	if err != nil {
		return err
	}
	for _, id := range bfsGraph(byID, center.ID, *depthFlag) {
		printGraphNode(out, byID[id], byID)
	}
	return nil
}

func indexGraphNodesByID(nodes []GraphNode) map[string]GraphNode {
	m := make(map[string]GraphNode, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n
	}
	return m
}

// resolveGraphCenter resolves a --center argument against the already-fetched
// node set: a UUID-shaped input matches node.ID, otherwise a case-insensitive
// title match (also tried with a ".md" suffix trimmed from the input).
func resolveGraphCenter(nodes []GraphNode, idOrTitle string) (GraphNode, error) {
	if looksLikeUUID(idOrTitle) {
		for _, n := range nodes {
			if n.ID == idOrTitle {
				return n, nil
			}
		}
		return GraphNode{}, fmt.Errorf("note not found: %q", idOrTitle)
	}
	trimmed := strings.TrimSuffix(idOrTitle, ".md")
	for _, n := range nodes {
		if strings.EqualFold(n.Title, idOrTitle) || strings.EqualFold(n.Title, trimmed) {
			return n, nil
		}
	}
	return GraphNode{}, fmt.Errorf("note not found: %q", idOrTitle)
}

// bfsGraph walks the undirected graph formed by resolved link targets plus
// backlinks, starting at centerID, up to depth hops. Returns visited node IDs
// in BFS order (centerID first).
func bfsGraph(byID map[string]GraphNode, centerID string, depth int) []string {
	visited := map[string]bool{centerID: true}
	order := []string{centerID}
	frontier := []string{centerID}

	for d := 0; d < depth; d++ {
		var next []string
		for _, id := range frontier {
			n, ok := byID[id]
			if !ok {
				continue
			}
			for _, nb := range graphNeighbors(n) {
				if visited[nb] {
					continue
				}
				visited[nb] = true
				order = append(order, nb)
				next = append(next, nb)
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	return order
}

// graphNeighbors returns the undirected edges touching n: its own resolved
// link targets plus its own backlinks.
func graphNeighbors(n GraphNode) []string {
	var out []string
	for _, l := range n.Links {
		if l.Resolved && l.TargetID != "" {
			out = append(out, l.TargetID)
		}
	}
	out = append(out, n.Backlinks...)
	return out
}

// printGraphNode renders one node block: header line (id, title, tags),
// then one line per outbound link (resolved/unresolved) and per backlink.
func printGraphNode(out io.Writer, n GraphNode, byID map[string]GraphNode) {
	tagsStr := "[-]"
	if len(n.Tags) > 0 {
		tagsStr = "[" + strings.Join(n.Tags, ",") + "]"
	}
	fmt.Fprintf(out, "%s\t%s\t%s\n", n.ID, n.Title, tagsStr)
	for _, l := range n.Links {
		if l.Resolved {
			fmt.Fprintf(out, "\t-> %s\n", linkDisplay(l))
		} else {
			fmt.Fprintf(out, "\t->? %s\n", l.Title)
		}
	}
	for _, b := range n.Backlinks {
		title := b
		if src, ok := byID[b]; ok {
			title = src.Title
		}
		fmt.Fprintf(out, "\t<- %s\n", title)
	}
}

// ---- unresolved ----

type unresolvedLink struct {
	title   string // first-seen original title (display casing)
	count   int
	sources []string // titles of notes referencing this unresolved target
}

func runUnresolved(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("unresolved", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	nodes, err := client.GetGraph()
	if err != nil {
		return err
	}

	agg := map[string]*unresolvedLink{}
	var keys []string
	for _, n := range nodes {
		for _, l := range n.Links {
			if l.Resolved {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(l.Title))
			u, ok := agg[key]
			if !ok {
				u = &unresolvedLink{title: l.Title}
				agg[key] = u
				keys = append(keys, key)
			}
			u.count++
			u.sources = append(u.sources, n.Title)
		}
	}

	if len(agg) == 0 {
		fmt.Fprintln(out, "(none)")
		return nil
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := agg[keys[i]], agg[keys[j]]
		if a.count != b.count {
			return a.count > b.count
		}
		return strings.ToLower(a.title) < strings.ToLower(b.title)
	})

	for _, k := range keys {
		u := agg[k]
		fmt.Fprintf(out, "%s\t%d\t%s\n", u.title, u.count, strings.Join(u.sources, ","))
	}
	return nil
}
