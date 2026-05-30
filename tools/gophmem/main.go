package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	baseURL := os.Getenv("GOPHMEM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // local dev: direct backend (Next.js is a static export, no proxy)
	}
	apiKey := os.Getenv("GOPHMEM_API_KEY")

	client := NewClient(baseURL, apiKey)

	var err error
	switch os.Args[1] {
	case "write":
		err = runWrite(client, os.Args[2:])
	case "append":
		err = runAppend(client, os.Args[2:])
	case "read":
		err = runRead(client, os.Args[2:])
	case "search":
		err = runSearch(client, os.Args[2:])
	case "list":
		err = runList(client, os.Args[2:])
	case "tags":
		err = runTags(client, os.Args[2:])
	case "setup":
		err = runSetup(client, os.Args[2:])
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
  gophmem write <title> [--tags a,b] [--stdin]   Create note in AI Memory folder
  gophmem append <id|title>                       Append stdin to an existing note
  gophmem read <id>                               Print note content and metadata
  gophmem search <query> [--tag t]                Search notes (Vault-wide)
  gophmem list [--folder <id>]                    List notes (default: AI Memory)
  gophmem tags                                    List all tags with counts
  gophmem setup                                   Print sub/base_folder_id for SSM setup

Environment:
  GOPHMEM_BASE_URL    API base URL (default: http://localhost:8080; production: https://<domain>/api)
  GOPHMEM_API_KEY     Agent API key`)
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
// (resolved via search, exact-name match).
func resolveNoteID(client *Client, idOrTitle string) (string, error) {
	if looksLikeUUID(idOrTitle) {
		return idOrTitle, nil
	}
	results, err := client.Search(idOrTitle, nil)
	if err != nil {
		return "", fmt.Errorf("search for %q: %w", idOrTitle, err)
	}
	for _, f := range results {
		name := strings.TrimSuffix(f.Name, ".md")
		if strings.EqualFold(name, idOrTitle) || strings.EqualFold(f.Name, idOrTitle) {
			return f.ID, nil
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

func runRead(client *Client, args []string) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gophmem read <id>")
	}
	note, err := client.GetNote(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Printf("# %s\n", note.Name)
	fmt.Printf("id: %s | modified: %s | etag: %s\n", note.ID, note.Modified, note.ETag)
	if len(note.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(note.Tags, ", "))
	}
	fmt.Println()
	fmt.Print(note.Content)
	return nil
}

// ---- search ----

func runSearch(client *Client, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	tagFlag := fs.String("tag", "", "filter by tag")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 && *tagFlag == "" {
		return fmt.Errorf("usage: gophmem search <query> [--tag t]")
	}
	query := strings.Join(fs.Args(), " ")
	var tags []string
	if *tagFlag != "" {
		tags = []string{*tagFlag}
	}
	results, err := client.Search(query, tags)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("(no results)")
		return nil
	}
	for _, f := range results {
		fmt.Printf("%s\t%s\n", f.ID, f.Name)
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
