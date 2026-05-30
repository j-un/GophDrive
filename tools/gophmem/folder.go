package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	aiMemoryFolderName = "AI Memory"
	folderMIMEType     = "application/vnd.google-apps.folder"
)

type folderCache struct {
	AIMemoryID string `json:"ai_memory_id"`
}

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
	return c
}

func saveFolderCache(c folderCache) error {
	p := cachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(c)
}

// ResolveAIMemoryFolder returns the ID of the "AI Memory" folder, creating it
// if absent. Searches the whole Vault so a relocated folder is found.
// The result is cached to avoid repeated API round-trips.
func ResolveAIMemoryFolder(client *Client) (string, error) {
	cache := loadFolderCache()
	if cache.AIMemoryID != "" {
		return cache.AIMemoryID, nil
	}

	// Search the whole Vault so a folder that was moved still resolves.
	results, err := client.Search(aiMemoryFolderName, nil)
	if err != nil {
		return "", fmt.Errorf("search for AI Memory folder: %w", err)
	}
	for _, f := range results {
		if f.MIMEType == folderMIMEType && strings.EqualFold(f.Name, aiMemoryFolderName) {
			_ = saveFolderCache(folderCache{AIMemoryID: f.ID})
			return f.ID, nil
		}
	}

	// Not found — create it. Empty parents lets the server use the JWT base_folder_id.
	fmt.Fprintf(os.Stderr, "gophmem: creating \"AI Memory\" folder in your Vault\n")
	folder, err := client.CreateFolder(aiMemoryFolderName, []string{})
	if err != nil {
		return "", fmt.Errorf("create AI Memory folder: %w", err)
	}
	_ = saveFolderCache(folderCache{AIMemoryID: folder.ID})
	return folder.ID, nil
}
