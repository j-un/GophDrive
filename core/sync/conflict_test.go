package sync

import "testing"

func TestCheckConflict(t *testing.T) {
	tests := []struct {
		name         string
		localEtag    string
		remoteEtag   string
		wantConflict bool
	}{
		{
			name:         "same ETag — no conflict",
			localEtag:    "abc123",
			remoteEtag:   "abc123",
			wantConflict: false,
		},
		{
			name:         "different ETag — conflict",
			localEtag:    "abc123",
			remoteEtag:   "def456",
			wantConflict: true,
		},
		{
			name:         "both empty — no conflict",
			localEtag:    "",
			remoteEtag:   "",
			wantConflict: false,
		},
		{
			name:         "only local empty — conflict",
			localEtag:    "",
			remoteEtag:   "abc123",
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckConflict(tt.localEtag, tt.remoteEtag)
			if got != tt.wantConflict {
				t.Errorf("CheckConflict(%q, %q) = %v, want %v",
					tt.localEtag, tt.remoteEtag, got, tt.wantConflict)
			}
		})
	}
}
