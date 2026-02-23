package googledrive

import "testing"

func TestToDriveName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"appends .md to plain name", "test", "test.md"},
		{"keeps .md if already present", "test.md", "test.md"},
		{"handles empty string", "", ".md"},
		{"handles name with dots", "my.note.v2", "my.note.v2.md"},
		{"does not double .md", "readme.md", "readme.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toDriveName(tt.in)
			if got != tt.want {
				t.Errorf("toDriveName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFromDriveName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"strips .md extension", "test.md", "test"},
		{"no-op if no .md", "test", "test"},
		{"handles empty string", "", ""},
		{"strips only trailing .md", "my.md.backup.md", "my.md.backup"},
		{"no-op for .markdown", "test.markdown", "test.markdown"},
		{"handles just .md", ".md", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromDriveName(tt.in)
			if got != tt.want {
				t.Errorf("fromDriveName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
