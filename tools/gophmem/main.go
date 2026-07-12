package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	baseURL := resolveSetting("GOPHMEM_BASE_URL", "http://localhost:8080")
	apiKey := resolveSetting("GOPHMEM_API_KEY", "")

	client := NewClient(baseURL, apiKey)

	var err error
	switch os.Args[1] {
	case "write":
		err = runWrite(client, os.Args[2:])
	case "append":
		err = runAppend(client, os.Args[2:])
	case "read":
		err = runRead(client, os.Args[2:], os.Stdout)
	case "search":
		err = runSearch(client, os.Args[2:], os.Stdout)
	case "list":
		err = runList(client, os.Args[2:])
	case "tags":
		err = runTags(client, os.Args[2:])
	case "links":
		err = runLinks(client, os.Args[2:], os.Stdout)
	case "backlinks":
		err = runBacklinks(client, os.Args[2:], os.Stdout)
	case "graph":
		err = runGraph(client, os.Args[2:], os.Stdout)
	case "unresolved":
		err = runUnresolved(client, os.Args[2:], os.Stdout)
	case "setup":
		err = runSetup(client, os.Args[2:])
	case "config":
		err = runConfig(os.Args[2:], os.Stdout)
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `gophmem — GophDrive agent memory CLI

Usage:
  gophmem write <title> [--tags a,b] [--stdin]              Create note in AI Memory folder
  gophmem append <id|title>                                  Append stdin to an existing note
  gophmem read <id> [--section <heading>]                    Print note content (or one section)
  gophmem search <query> [--tag t]... [--type t] [--since D] [--until D] [--limit N] [--no-snippet] [--in titles|headings|all]
                                                              Search notes (Vault-wide; --until DATE includes that whole day)
  gophmem list [--folder <id>]                               List notes (default: AI Memory)
  gophmem tags                                               List all tags with counts
  gophmem links <id|title>                                   Outbound wikilinks of a note
  gophmem backlinks <id|title>                               Notes linking to a note
  gophmem graph [--center <id|title>] [--depth N]            Adjacency view (default: whole vault)
  gophmem unresolved                                         Wikilinks whose target note does not exist
  gophmem setup                                              Print sub/base_folder_id for SSM setup
  gophmem config set [--base-url URL] [--api-key KEY]        Save settings to config file (0600)
  gophmem config show                                        Show resolved settings and their source

Configuration (priority: env > config file > default):
  GOPHMEM_BASE_URL    API base URL (default: http://localhost:8080; production: https://<domain>/api)
  GOPHMEM_API_KEY     Agent API key
  Config file:        ~/.config/gophmem/config  (override: GOPHMEM_CONFIG_DIR)`)
}

// ---- write ----

func runWrite(client *Client, args []string) error {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	tagsFlag := fs.String("tags", "", "comma-separated tags (prepended as YAML frontmatter)")
	stdinFlag := fs.Bool("stdin", false, "read note body from stdin")

	// Separate the title (first non-flag arg) from the flag args so that
	// flags placed after the title are parsed correctly.
	var title string
	var flagArgs []string
	for _, a := range args {
		if title == "" && !strings.HasPrefix(a, "-") {
			title = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}
	if title == "" {
		return fmt.Errorf("usage: gophmem write <title> [--tags a,b] [--stdin]")
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if !strings.HasSuffix(title, ".md") {
		title += ".md"
	}

	var body string
	if *stdinFlag {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		body = string(b)
	}

	if *tagsFlag != "" {
		parts := strings.Split(*tagsFlag, ",")
		fm := "---\ntags:\n"
		for _, t := range parts {
			fm += "  - " + strings.TrimSpace(t) + "\n"
		}
		fm += "---\n\n"
		body = fm + body
	}

	folderID, err := ResolveAIMemoryFolder(client)
	if err != nil {
		return fmt.Errorf("resolve AI Memory folder: %w", err)
	}
	file, err := client.CreateNote(title, body, folderID)
	if err != nil {
		return fmt.Errorf("create note: %w", err)
	}
	fmt.Printf("created: %s  (id: %s)\n", file.Name, file.ID)
	return nil
}

// ---- append ----

const appendMaxRetries = 3

func runAppend(client *Client, args []string) error {
	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gophmem append <id|title>")
	}
	idOrTitle := strings.Join(fs.Args(), " ")

	addition, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	id, err := resolveNoteID(client, idOrTitle)
	if err != nil {
		return err
	}

	noteName, err := appendToNote(client, id, string(addition))
	if err != nil {
		return err
	}
	fmt.Printf("appended to: %s  (id: %s)\n", noteName, id)
	return nil
}

// appendToNote performs a GET → append → PUT cycle with up to appendMaxRetries
// retries on 412 ETag conflicts. Returns the note name on success.
func appendToNote(client *Client, id, addition string) (string, error) {
	for i := range appendMaxRetries {
		note, err := client.GetNote(id)
		if err != nil {
			return "", fmt.Errorf("get note: %w", err)
		}
		newContent := strings.TrimRight(note.Content, "\n") + "\n\n" + strings.TrimSpace(addition) + "\n"
		_, err = client.UpdateNote(id, newContent, note.ETag)
		if errors.Is(err, ErrConflict) {
			if i == appendMaxRetries-1 {
				return "", fmt.Errorf("append failed after %d retries (ETag conflict)", appendMaxRetries)
			}
			continue
		}
		if err != nil {
			return "", fmt.Errorf("update note: %w", err)
		}
		return note.Name, nil
	}
	return "", fmt.Errorf("appendToNote: unreachable retry exit")
}

// resolveNoteID returns a note ID given either a UUID string or a title
// (resolved via search, exact-name match, falling back to alias match).
func resolveNoteID(client *Client, idOrTitle string) (string, error) {
	if looksLikeUUID(idOrTitle) {
		return idOrTitle, nil
	}
	results, err := client.Search(idOrTitle, SearchOpts{})
	if err != nil {
		return "", fmt.Errorf("search for %q: %w", idOrTitle, err)
	}
	for _, f := range results {
		name := strings.TrimSuffix(f.Name, ".md")
		if strings.EqualFold(name, idOrTitle) || strings.EqualFold(f.Name, idOrTitle) {
			return f.ID, nil
		}
		for _, alias := range f.Aliases {
			if strings.EqualFold(alias, idOrTitle) {
				return f.ID, nil
			}
		}
	}
	return "", fmt.Errorf("note not found: %q", idOrTitle)
}

func looksLikeUUID(s string) bool {
	// UUID v4: 8-4-4-4-12 hex chars with hyphens at fixed positions.
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// ---- read ----

func runRead(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	sectionFlag := fs.String("section", "", "print only the named section (case-insensitive partial match)")

	// Separate the note ID (first non-flag arg) from flag args so that flags
	// placed after the ID are parsed correctly (same pattern as runWrite).
	var id string
	var flagArgs []string
	for _, a := range args {
		if id == "" && !strings.HasPrefix(a, "-") {
			id = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("usage: gophmem read <id> [--section <heading>]")
	}
	note, err := client.GetNote(id)
	if err != nil {
		return err
	}
	if *sectionFlag != "" {
		body, found := extractSection(note.Content, *sectionFlag)
		if !found {
			return fmt.Errorf("note %s: section %q not found", note.ID, *sectionFlag)
		}
		fmt.Fprint(out, body)
		return nil
	}
	fmt.Fprintf(out, "# %s\n", note.Name)
	fmt.Fprintf(out, "id: %s | modified: %s | etag: %s\n", note.ID, note.Modified, note.ETag)
	if len(note.Tags) > 0 {
		fmt.Fprintf(out, "tags: %s\n", strings.Join(note.Tags, ", "))
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, note.Content)
	return nil
}

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// extractSection returns the content of the first heading whose text
// case-insensitively contains needle, up to (but not including) the next
// heading at the same or shallower level. Code fences are skipped.
func extractSection(body, needle string) (string, bool) {
	lines := strings.Split(body, "\n")
	inFence := false
	startLine := -1
	startLevel := 0
	var out []string

	for i, line := range lines {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
		}
		if inFence {
			if startLine >= 0 {
				out = append(out, line)
			}
			continue
		}

		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			if startLine >= 0 {
				out = append(out, line)
			}
			continue
		}

		level := len(m[1])
		text := m[2]

		if startLine < 0 {
			// Not yet found target heading
			if strings.Contains(strings.ToLower(text), strings.ToLower(needle)) {
				startLine = i
				startLevel = level
				out = append(out, line)
			}
		} else {
			// Inside target section — stop when we hit same or shallower level
			if level <= startLevel {
				break
			}
			out = append(out, line)
		}
	}

	if startLine < 0 {
		return "", false
	}
	return strings.Join(out, "\n") + "\n", true
}

// ---- search ----

// stringSlice implements flag.Value, appending each occurrence so a flag can
// be repeated on the command line (e.g. --tag a --tag b).
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// parseDate parses a date-only string (YYYY-MM-DD, interpreted in Local time)
// or an RFC3339 timestamp. dateOnly reports whether the date-only format
// matched, which callers use to decide whether to treat the value as covering
// a whole day.
func parseDate(s string) (t time.Time, dateOnly bool, err error) {
	if t, perr := time.ParseInLocation("2006-01-02", s, time.Local); perr == nil {
		return t, true, nil
	}
	if t, perr := time.Parse(time.RFC3339, s); perr == nil {
		return t, false, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid date %q (want YYYY-MM-DD or RFC3339)", s)
}

// formatModifiedAfter converts a --since input into the modifiedAfter query
// value. An empty input means "no filter" and returns "".
func formatModifiedAfter(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	t, _, err := parseDate(s)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339), nil
}

// formatModifiedBefore converts a --until input into the modifiedBefore query
// value. A date-only input is advanced to the start of the next local day so
// the named day is fully included; an RFC3339 input passes through unchanged
// as the exclusive bound. An empty input means "no filter" and returns "".
func formatModifiedBefore(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	t, dateOnly, err := parseDate(s)
	if err != nil {
		return "", err
	}
	if dateOnly {
		t = t.AddDate(0, 0, 1)
	}
	return t.UTC().Format(time.RFC3339), nil
}

func runSearch(client *Client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	var tags stringSlice
	fs.Var(&tags, "tag", "filter by tag (repeatable, AND)")
	limitFlag := fs.Int("limit", 10, "max results (1-100)")
	noSnippetFlag := fs.Bool("no-snippet", false, "suppress snippet lines")
	inFlag := fs.String("in", "", "search scope: titles, headings, or all (default: all)")
	typeFlag := fs.String("type", "", "filter by note type")
	sinceFlag := fs.String("since", "", "only notes modified on/after this date (YYYY-MM-DD or RFC3339)")
	untilFlag := fs.String("until", "", "only notes modified through this date (YYYY-MM-DD or RFC3339); DATE includes that whole day")

	// Separate query words from flags so flags may appear anywhere.
	// Value-taking flags are paired with their next arg.
	valueTakingFlags := map[string]bool{"tag": true, "limit": true, "in": true, "type": true, "since": true, "until": true}
	var queryWords, flagArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			queryWords = append(queryWords, a)
			continue
		}
		flagArgs = append(flagArgs, a)
		// Peek at next arg: if this flag takes a value, grab it.
		if strings.Contains(a, "=") {
			continue // -flag=value form; value already embedded
		}
		name := strings.TrimLeft(a, "-")
		if valueTakingFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if len(queryWords) == 0 && len(tags) == 0 && *typeFlag == "" {
		return fmt.Errorf("usage: gophmem search <query> [--tag t]... [--type t] [--since D] [--until D] [--limit N] [--no-snippet] [--in titles|headings|all]")
	}
	query := strings.Join(queryWords, " ")
	modifiedAfter, err := formatModifiedAfter(*sinceFlag)
	if err != nil {
		return err
	}
	modifiedBefore, err := formatModifiedBefore(*untilFlag)
	if err != nil {
		return err
	}
	results, err := client.Search(query, SearchOpts{
		Tags:           tags,
		Limit:          *limitFlag,
		Scope:          strings.ToLower(*inFlag),
		Type:           *typeFlag,
		ModifiedAfter:  modifiedAfter,
		ModifiedBefore: modifiedBefore,
	})
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "(no results)")
		return nil
	}
	for _, f := range results {
		tagsStr := "[-]"
		if len(f.Tags) > 0 {
			tagsStr = "[" + strings.Join(f.Tags, ",") + "]"
		}
		date := f.ModifiedTime.Format("2006-01-02")
		name := strings.TrimSuffix(f.Name, ".md")
		fmt.Fprintf(out, "%s  %s  %s  %s\n", f.ID, name, tagsStr, date)
		if !*noSnippetFlag && f.Snippet != "" {
			fmt.Fprintf(out, "        > %s\n", f.Snippet)
		}
	}
	return nil
}

// ---- list ----

func runList(client *Client, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	folderFlag := fs.String("folder", "", "folder ID (default: AI Memory folder)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	folderID := *folderFlag
	if folderID == "" {
		var err error
		folderID, err = ResolveAIMemoryFolder(client)
		if err != nil {
			return err
		}
	}
	files, err := client.ListNotes(folderID)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("(empty)")
		return nil
	}
	for _, f := range files {
		fmt.Printf("%s\t%s\n", f.ID, f.Name)
	}
	return nil
}

// ---- tags ----

func runTags(client *Client, _ []string) error {
	tags, err := client.ListTags()
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		fmt.Println("(no tags)")
		return nil
	}
	for _, t := range tags {
		fmt.Printf("%s\t%d\n", t.Name, t.Count)
	}
	return nil
}

// ---- config ----

func runConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  gophmem config set [--base-url URL] [--api-key KEY]")
		fmt.Fprintln(out, "  gophmem config show")
		return nil
	}
	switch args[0] {
	case "set":
		return runConfigSet(args[1:], out)
	case "show":
		return runConfigShow(out)
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func runConfigSet(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	baseURLFlag := fs.String("base-url", "", "API base URL")
	apiKeyFlag := fs.String("api-key", "", "Agent API key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseURLFlag == "" && *apiKeyFlag == "" {
		return fmt.Errorf("usage: gophmem config set [--base-url URL] [--api-key KEY]")
	}
	updates := map[string]string{}
	if *baseURLFlag != "" {
		updates["GOPHMEM_BASE_URL"] = *baseURLFlag
	}
	if *apiKeyFlag != "" {
		updates["GOPHMEM_API_KEY"] = *apiKeyFlag
	}
	if err := saveConfig(updates); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(out, "saved to: %s\n", configPath())
	return nil
}

func runConfigShow(out io.Writer) error {
	type entry struct {
		key string
		def string
	}
	entries := []entry{
		{"GOPHMEM_BASE_URL", "http://localhost:8080"},
		{"GOPHMEM_API_KEY", ""},
	}
	for _, e := range entries {
		val, source := resolveSettingWithSource(e.key, e.def)
		display := val
		if display == "" {
			display = "(not set)"
		}
		if e.key == "GOPHMEM_API_KEY" && source != "none" {
			display = maskAPIKey(val)
		}
		fmt.Fprintf(out, "%-20s %s  [%s]\n", e.key, display, source)
	}
	fmt.Fprintf(out, "%-20s %s\n", "config file:", configPath())
	return nil
}

// ---- setup ----

func runSetup(client *Client, _ []string) error {
	profile, err := client.GetUser()
	if err != nil {
		return fmt.Errorf("%w\n\nTip: set GOPHMEM_API_KEY to a valid session JWT (from browser DevTools)\nor a configured agent key, then run gophmem setup again.", err)
	}
	fmt.Printf(`# Paste this JSON into SSM Parameter Store (/gophdrive/agent-api-key)
# or set AGENT_API_KEY in your .env file.
# Replace <RANDOM_KEY> with a long random string: openssl rand -hex 32

{"key":"<RANDOM_KEY>","sub":"%s","base_folder_id":"%s"}

sub:            %s
base_folder_id: %s
`, profile.ID, profile.BaseFolderID, profile.ID, profile.BaseFolderID)
	return nil
}
