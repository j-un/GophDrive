package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configPath returns the path to the gophmem config file.
// Resolution order: GOPHMEM_CONFIG_DIR override → XDG_CONFIG_HOME → ~/.config
// Mirrors cachePath() in folder.go.
func configPath() string {
	if override := os.Getenv("GOPHMEM_CONFIG_DIR"); override != "" {
		return filepath.Join(override, "gophmem", "config")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gophmem", "config")
}

// parseConfigLine parses a single dotenv line and returns the key and value.
// Returns ok=false for blank lines, comment lines (# prefix), lines with no '=',
// and lines with an empty key after trimming. The first '=' splits key from value;
// subsequent '=' characters belong to the value. Neither quotes nor inline comments
// are supported — the raw trimmed value is returned as-is.
func parseConfigLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:idx])
	if k == "" {
		return "", "", false
	}
	v := strings.TrimSpace(line[idx+1:])
	return k, v, true
}

// loadConfigFile parses the config file and returns a key→value map.
// Missing file → empty map. Malformed lines are skipped silently.
func loadConfigFile() map[string]string {
	f, err := os.Open(configPath())
	if err != nil {
		return map[string]string{}
	}
	defer f.Close()

	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := parseConfigLine(sc.Text()); ok {
			m[k] = v
		}
	}
	return m
}

// resolveSettingWithSource returns the effective value for key plus its source:
//   - "env"     — non-empty environment variable (empty env is treated as unset;
//     this keeps config show and live resolution always in sync)
//   - "file"    — non-empty value from the config file
//   - "default" — def is non-empty and neither env nor file provided a value
//   - "none"    — no value available; returns ""
func resolveSettingWithSource(key, def string) (val, source string) {
	if v := os.Getenv(key); v != "" {
		return v, "env"
	}
	if v := loadConfigFile()[key]; v != "" {
		return v, "file"
	}
	if def != "" {
		return def, "default"
	}
	return "", "none"
}

// resolveSetting returns the effective value for key (priority: env > file > def).
// An environment variable explicitly set to "" is treated as unset.
func resolveSetting(key, def string) string {
	v, _ := resolveSettingWithSource(key, def)
	return v
}

// saveConfig merges updates into the config file using a lossless line-oriented
// strategy: existing comment lines, blank lines, and unrecognised keys are
// preserved verbatim; only the lines whose keys appear in updates are replaced
// in-place. Keys in updates that have no existing line are appended in the fixed
// order BASE_URL → API_KEY. When the file does not yet exist a short header
// comment is prepended.
//
// The write is atomic: MkdirAll(0700) → CreateTemp → Chmod(0600) → Rename,
// mirroring saveFolderCache in folder.go.
func saveConfig(updates map[string]string) error {
	p := configPath()

	// Read existing content as raw lines (preserves comments and unknown keys).
	var rawLines []string
	existing, err := os.ReadFile(p)
	isNew := os.IsNotExist(err)
	if err != nil && !isNew {
		return fmt.Errorf("read config: %w", err)
	}
	if !isNew {
		rawLines = strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	}

	// Pass 1: replace in-place where the key matches.
	handled := map[string]bool{}
	for i, line := range rawLines {
		k, _, ok := parseConfigLine(line)
		if !ok {
			continue
		}
		if newVal, want := updates[k]; want {
			rawLines[i] = k + "=" + newVal
			handled[k] = true
		}
	}

	// Pass 2: append new keys in fixed order.
	newKeyOrder := []string{"GOPHMEM_BASE_URL", "GOPHMEM_API_KEY"}
	for _, k := range newKeyOrder {
		if v, want := updates[k]; want && !handled[k] {
			rawLines = append(rawLines, k+"="+v)
		}
	}
	// Append any updates key not in the fixed order (forward compat).
	for k, v := range updates {
		if !handled[k] {
			inOrder := false
			for _, ok := range newKeyOrder {
				if ok == k {
					inOrder = true
					break
				}
			}
			if !inOrder {
				rawLines = append(rawLines, k+"="+v)
			}
		}
	}

	// Build final content.
	var sb strings.Builder
	if isNew {
		sb.WriteString("# gophmem config (dotenv) — managed by 'gophmem config set'.\n")
		sb.WriteString("# Hand edits (comments, extra keys) are preserved by 'config set'.\n")
	}
	for _, l := range rawLines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}

	// Atomic write.
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(sb.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

// maskAPIKey returns the key with all but the last 4 characters replaced by '*'.
// Keys of 4 characters or fewer are fully masked.
func maskAPIKey(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}
