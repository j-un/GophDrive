package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	aiMemoryFolderName = "AI Memory"
	folderMIMEType     = "application/vnd.google-apps.folder"
)

// folderCache maps "BASE_URL#keyDigest" → AI Memory folder ID.
// keyDigest = hex(sha256(apiKey))[:8]; empty key uses "anonymous".
type folderCache map[string]string

func cachePath() string {
	if override := os.Getenv("GOPHMEM_CACHE_DIR"); override != "" {
		return filepath.Join(override, "folders.json")
	}
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "gophmem", "folders.json")
}

func loadFolderCache() folderCache {
	f, err := os.Open(cachePath())
	if err != nil {
		return folderCache{}
	}
	defer f.Close()
	var c folderCache
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		fmt.Fprintf(os.Stderr, "gophmem: warning: corrupt folder cache (%v); delete %s to reset\n", err, cachePath())
		return folderCache{}
	}
	if c == nil {
		return folderCache{}
	}
	return c
}

func saveFolderCache(c folderCache) error {
	p := cachePath()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "folders-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath) // no-op if rename succeeded
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err := json.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

// ResolveAIMemoryFolder returns the ID of the "AI Memory" folder, creating it
// if absent. Searches the whole Vault so a relocated folder is found.
// The cache is keyed by BASE_URL+API-key digest to isolate different environments.
// A cached ID is verified with GET /notes/{id} before use; a 404 evicts the entry.
func ResolveAIMemoryFolder(client *Client) (string, error) {
	cache := loadFolderCache()
	key := client.CacheKey()

	if cachedID, ok := cache[key]; ok && cachedID != "" {
		_, err := client.GetNote(cachedID)
		switch {
		case err == nil:
			return cachedID, nil
		case errors.Is(err, ErrNotFound):
			delete(cache, key)
			_ = saveFolderCache(cache)
			// fall-through to Search/Create
		default:
			return "", err
		}
	}

	// Search the whole Vault so a folder that was moved still resolves.
	results, err := client.Search(aiMemoryFolderName, nil, 0, "")
	if err != nil {
		return "", fmt.Errorf("search for AI Memory folder: %w", err)
	}
	for _, f := range results {
		if f.MIMEType == folderMIMEType && strings.EqualFold(f.Name, aiMemoryFolderName) {
			cache[key] = f.ID
			_ = saveFolderCache(cache)
			return f.ID, nil
		}
	}

	// Not found — create it. Empty parents lets the server use the JWT base_folder_id.
	fmt.Fprintf(os.Stderr, "gophmem: creating \"AI Memory\" folder in your Vault\n")
	folder, err := client.CreateFolder(aiMemoryFolderName, []string{})
	if err != nil {
		return "", fmt.Errorf("create AI Memory folder: %w", err)
	}
	cache[key] = folder.ID
	_ = saveFolderCache(cache)
	return folder.ID, nil
}
