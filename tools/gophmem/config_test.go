package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfig sets GOPHMEM_CONFIG_DIR to a temp dir and clears the env vars
// so tests never read from the real ~/.config or the host environment.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GOPHMEM_CONFIG_DIR", t.TempDir())
	t.Setenv("GOPHMEM_BASE_URL", "")
	t.Setenv("GOPHMEM_API_KEY", "")
}

// ---- configPath ----

func TestConfigPath_UsesConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOPHMEM_CONFIG_DIR", dir)
	got := configPath()
	want := filepath.Join(dir, "gophmem", "config")
	if got != want {
		t.Errorf("configPath = %q, want %q", got, want)
	}
}

func TestConfigPath_FallsBackToXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOPHMEM_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	got := configPath()
	if !strings.HasPrefix(got, dir) {
		t.Errorf("configPath %q should start with XDG_CONFIG_HOME %q", got, dir)
	}
}

// ---- loadConfigFile ----

func TestLoadConfigFile_Missing(t *testing.T) {
	isolateConfig(t)
	m := loadConfigFile()
	if len(m) != 0 {
		t.Errorf("expected empty map for missing config, got %v", m)
	}
}

func TestLoadConfigFile_BasicParsing(t *testing.T) {
	isolateConfig(t)
	content := "# comment\n\nGOPHMEM_BASE_URL=https://example.com/api\nGOPHMEM_API_KEY=secret\n"
	writeConfigFile(t, content)

	m := loadConfigFile()
	if m["GOPHMEM_BASE_URL"] != "https://example.com/api" {
		t.Errorf("BASE_URL: got %q", m["GOPHMEM_BASE_URL"])
	}
	if m["GOPHMEM_API_KEY"] != "secret" {
		t.Errorf("API_KEY: got %q", m["GOPHMEM_API_KEY"])
	}
}

func TestLoadConfigFile_WhitespaceTrimmed(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "  GOPHMEM_API_KEY  =  trimmed  \n")

	m := loadConfigFile()
	if m["GOPHMEM_API_KEY"] != "trimmed" {
		t.Errorf("expected trimmed, got %q", m["GOPHMEM_API_KEY"])
	}
}

func TestLoadConfigFile_ValueWithEquals(t *testing.T) {
	isolateConfig(t)
	// Value contains '=' — only the first '=' splits key from value.
	writeConfigFile(t, "GOPHMEM_API_KEY=abc=def=ghi\n")

	m := loadConfigFile()
	if m["GOPHMEM_API_KEY"] != "abc=def=ghi" {
		t.Errorf("expected abc=def=ghi, got %q", m["GOPHMEM_API_KEY"])
	}
}

func TestLoadConfigFile_SkipsMalformedLines(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "NOEQUALSSIGN\nGOPHMEM_API_KEY=valid\n")

	m := loadConfigFile()
	if m["GOPHMEM_API_KEY"] != "valid" {
		t.Errorf("valid key lost: %v", m)
	}
	if _, ok := m["NOEQUALSSIGN"]; ok {
		t.Error("malformed line should be skipped")
	}
}

// ---- resolveSetting ----

func TestResolveSetting_EnvWins(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "GOPHMEM_API_KEY=fromfile\n")
	t.Setenv("GOPHMEM_API_KEY", "fromenv")

	got := resolveSetting("GOPHMEM_API_KEY", "default")
	if got != "fromenv" {
		t.Errorf("env should win, got %q", got)
	}
}

func TestResolveSetting_FileWhenEnvEmpty(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "GOPHMEM_API_KEY=fromfile\n")

	got := resolveSetting("GOPHMEM_API_KEY", "default")
	if got != "fromfile" {
		t.Errorf("file should win when env empty, got %q", got)
	}
}

func TestResolveSetting_DefaultWhenBothEmpty(t *testing.T) {
	isolateConfig(t)

	got := resolveSetting("GOPHMEM_BASE_URL", "http://localhost:8080")
	if got != "http://localhost:8080" {
		t.Errorf("expected default, got %q", got)
	}
}

// ---- saveConfig ----

func TestSaveConfig_RoundTrip(t *testing.T) {
	isolateConfig(t)
	updates := map[string]string{
		"GOPHMEM_BASE_URL": "https://save.example/api",
		"GOPHMEM_API_KEY":  "mykey",
	}
	if err := saveConfig(updates); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	m := loadConfigFile()
	if m["GOPHMEM_BASE_URL"] != "https://save.example/api" {
		t.Errorf("BASE_URL: %q", m["GOPHMEM_BASE_URL"])
	}
	if m["GOPHMEM_API_KEY"] != "mykey" {
		t.Errorf("API_KEY: %q", m["GOPHMEM_API_KEY"])
	}
}

func TestSaveConfig_PartialUpdatePreservesOtherKeys(t *testing.T) {
	isolateConfig(t)
	// Write both keys first.
	if err := saveConfig(map[string]string{
		"GOPHMEM_BASE_URL": "https://orig.example/api",
		"GOPHMEM_API_KEY":  "origkey",
	}); err != nil {
		t.Fatal(err)
	}
	// Update only API_KEY.
	if err := saveConfig(map[string]string{"GOPHMEM_API_KEY": "newkey"}); err != nil {
		t.Fatal(err)
	}
	m := loadConfigFile()
	if m["GOPHMEM_BASE_URL"] != "https://orig.example/api" {
		t.Errorf("BASE_URL should be preserved, got %q", m["GOPHMEM_BASE_URL"])
	}
	if m["GOPHMEM_API_KEY"] != "newkey" {
		t.Errorf("API_KEY should be updated, got %q", m["GOPHMEM_API_KEY"])
	}
}

func TestSaveConfig_FilePermissions(t *testing.T) {
	isolateConfig(t)
	if err := saveConfig(map[string]string{"GOPHMEM_API_KEY": "key"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(configPath())
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected 0600 permissions, got %04o", perm)
	}
}

// ---- maskAPIKey ----

func TestMaskAPIKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abcdefgh", "****efgh"},
		{"abcde", "*bcde"}, // 5 chars: 1 masked + last 4 visible
		{"abcd", "****"},   // 4 chars or fewer — fully masked
		{"abc", "***"},
		{"", ""},
	}
	for _, tt := range tests {
		got := maskAPIKey(tt.in)
		if got != tt.want {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---- runConfig show / set ----

func TestRunConfigShow_Sources(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "GOPHMEM_BASE_URL=https://file.example/api\nGOPHMEM_API_KEY=filekey1234\n")

	var buf strings.Builder
	if err := runConfigShow(&buf); err != nil {
		t.Fatalf("runConfigShow: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "https://file.example/api") {
		t.Errorf("expected base URL in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[file]") {
		t.Errorf("expected source=file in output, got:\n%s", out)
	}
	// API key must be masked — plaintext must not appear.
	if strings.Contains(out, "filekey1234") {
		t.Errorf("plaintext API key must not appear in output:\n%s", out)
	}
	// Masked suffix must appear.
	if !strings.Contains(out, "1234") {
		t.Errorf("expected masked suffix 1234 in output, got:\n%s", out)
	}
}

func TestRunConfigShow_EnvSource(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GOPHMEM_API_KEY", "envkey5678")

	var buf strings.Builder
	if err := runConfigShow(&buf); err != nil {
		t.Fatalf("runConfigShow: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[env]") {
		t.Errorf("expected source=env in output, got:\n%s", out)
	}
}

func TestRunConfigShow_DefaultSource(t *testing.T) {
	isolateConfig(t)

	var buf strings.Builder
	if err := runConfigShow(&buf); err != nil {
		t.Fatalf("runConfigShow: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[default]") {
		t.Errorf("expected source=default for BASE_URL when nothing set, got:\n%s", out)
	}
	if !strings.Contains(out, "[none]") {
		t.Errorf("expected source=none for API_KEY when nothing set, got:\n%s", out)
	}
}

func TestRunConfigSet_SavesAndPrintsPath(t *testing.T) {
	isolateConfig(t)

	var buf strings.Builder
	err := runConfigSet([]string{"--base-url", "https://new.example/api", "--api-key", "newkey"}, &buf)
	if err != nil {
		t.Fatalf("runConfigSet: %v", err)
	}
	if !strings.Contains(buf.String(), "saved to:") {
		t.Errorf("expected 'saved to:' in output, got: %s", buf.String())
	}
	m := loadConfigFile()
	if m["GOPHMEM_BASE_URL"] != "https://new.example/api" {
		t.Errorf("BASE_URL not saved: %v", m)
	}
	if m["GOPHMEM_API_KEY"] != "newkey" {
		t.Errorf("API_KEY not saved: %v", m)
	}
}

func TestRunConfigSet_ErrorWithNoFlags(t *testing.T) {
	isolateConfig(t)
	err := runConfigSet([]string{}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error when no flags given")
	}
}

// ---- runConfig dispatch ----

func TestRunConfig_Dispatch_Set(t *testing.T) {
	isolateConfig(t)
	var buf strings.Builder
	err := runConfig([]string{"set", "--api-key", "mykey"}, &buf)
	if err != nil {
		t.Fatalf("runConfig set: %v", err)
	}
	if loadConfigFile()["GOPHMEM_API_KEY"] != "mykey" {
		t.Error("expected API key saved by dispatch to set")
	}
}

func TestRunConfig_Dispatch_Show(t *testing.T) {
	isolateConfig(t)
	var buf strings.Builder
	err := runConfig([]string{"show"}, &buf)
	if err != nil {
		t.Fatalf("runConfig show: %v", err)
	}
	if !strings.Contains(buf.String(), "GOPHMEM_BASE_URL") {
		t.Errorf("expected GOPHMEM_BASE_URL in show output, got: %s", buf.String())
	}
}

func TestRunConfig_Dispatch_UnknownSubcommand(t *testing.T) {
	isolateConfig(t)
	err := runConfig([]string{"bogus"}, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown config subcommand") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunConfig_Dispatch_NoArgs(t *testing.T) {
	isolateConfig(t)
	var buf strings.Builder
	err := runConfig([]string{}, &buf)
	if err != nil {
		t.Fatalf("expected nil error for no-arg config, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Errorf("expected Usage: in no-arg output, got: %s", buf.String())
	}
}

// ---- CRLF handling ----

func TestLoadConfigFile_CRLF(t *testing.T) {
	isolateConfig(t)
	// Windows-style CRLF line endings; TrimSpace on value must absorb the \r.
	writeConfigFile(t, "GOPHMEM_API_KEY=crlfval\r\nGOPHMEM_BASE_URL=https://example.com/api\r\n")

	m := loadConfigFile()
	if m["GOPHMEM_API_KEY"] != "crlfval" {
		t.Errorf("CRLF not absorbed: got %q", m["GOPHMEM_API_KEY"])
	}
	if m["GOPHMEM_BASE_URL"] != "https://example.com/api" {
		t.Errorf("BASE_URL after CRLF: got %q", m["GOPHMEM_BASE_URL"])
	}
}

// ---- empty-env falls through to file (H2 contract) ----

func TestResolveSetting_EmptyEnvFallsThroughToFile(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "GOPHMEM_API_KEY=fromfile\n")
	// Explicitly set env to empty string — must be treated as unset.
	t.Setenv("GOPHMEM_API_KEY", "")

	got := resolveSetting("GOPHMEM_API_KEY", "default")
	if got != "fromfile" {
		t.Errorf("empty env should fall through to file value, got %q", got)
	}
}

// ---- lossless saveConfig (H1 contract) ----

func TestSaveConfig_PreservesCommentsAndUnknownKeys(t *testing.T) {
	isolateConfig(t)
	// Seed: comment line + unknown key + known key.
	writeConfigFile(t, "# my note\nFUTURE_KEY=preserve-me\nGOPHMEM_BASE_URL=https://orig.example/api\n")

	// Update only API_KEY (new key) — BASE_URL should stay, unknown/comment kept.
	if err := saveConfig(map[string]string{"GOPHMEM_API_KEY": "newkey"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	m := loadConfigFile()
	if m["GOPHMEM_BASE_URL"] != "https://orig.example/api" {
		t.Errorf("BASE_URL should be preserved, got %q", m["GOPHMEM_BASE_URL"])
	}
	if m["GOPHMEM_API_KEY"] != "newkey" {
		t.Errorf("API_KEY should be added, got %q", m["GOPHMEM_API_KEY"])
	}
	if m["FUTURE_KEY"] != "preserve-me" {
		t.Errorf("unknown key should be preserved, got %q", m["FUTURE_KEY"])
	}

	// Raw file must still contain the comment.
	raw, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# my note") {
		t.Errorf("comment line should be preserved, raw file:\n%s", raw)
	}
}

func TestSaveConfig_ReplaceExistingKeyInPlace(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "# header\nGOPHMEM_BASE_URL=https://old.example/api\nGOPHMEM_API_KEY=oldkey\n")

	if err := saveConfig(map[string]string{"GOPHMEM_API_KEY": "updatedkey"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	// API_KEY must be updated; BASE_URL must remain; comment must remain.
	raw, _ := os.ReadFile(configPath())
	content := string(raw)
	if !strings.Contains(content, "GOPHMEM_API_KEY=updatedkey") {
		t.Errorf("API_KEY not updated in-place:\n%s", content)
	}
	if strings.Contains(content, "GOPHMEM_API_KEY=oldkey") {
		t.Errorf("old API_KEY still present:\n%s", content)
	}
	if !strings.Contains(content, "GOPHMEM_BASE_URL=https://old.example/api") {
		t.Errorf("BASE_URL removed:\n%s", content)
	}
	if !strings.Contains(content, "# header") {
		t.Errorf("comment removed:\n%s", content)
	}
}

// ---- resolveSettingWithSource ----

func TestResolveSettingWithSource_Env(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GOPHMEM_API_KEY", "envval")
	val, src := resolveSettingWithSource("GOPHMEM_API_KEY", "def")
	if val != "envval" || src != "env" {
		t.Errorf("got (%q, %q)", val, src)
	}
}

func TestResolveSettingWithSource_File(t *testing.T) {
	isolateConfig(t)
	writeConfigFile(t, "GOPHMEM_API_KEY=fileval\n")
	val, src := resolveSettingWithSource("GOPHMEM_API_KEY", "def")
	if val != "fileval" || src != "file" {
		t.Errorf("got (%q, %q)", val, src)
	}
}

func TestResolveSettingWithSource_Default(t *testing.T) {
	isolateConfig(t)
	val, src := resolveSettingWithSource("GOPHMEM_BASE_URL", "http://localhost:8080")
	if val != "http://localhost:8080" || src != "default" {
		t.Errorf("got (%q, %q)", val, src)
	}
}

func TestResolveSettingWithSource_None(t *testing.T) {
	isolateConfig(t)
	val, src := resolveSettingWithSource("GOPHMEM_API_KEY", "")
	if val != "" || src != "none" {
		t.Errorf("got (%q, %q)", val, src)
	}
}

// ---- helpers ----

func writeConfigFile(t *testing.T, content string) {
	t.Helper()
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
