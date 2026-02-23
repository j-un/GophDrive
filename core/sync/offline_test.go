package sync

import (
	"testing"
	"time"
)

func TestNewOfflineChange(t *testing.T) {
	before := time.Now().Unix()
	change := NewOfflineChange("note-1", "hello world")
	after := time.Now().Unix()

	if change.NoteID != "note-1" {
		t.Errorf("NoteID = %q, want %q", change.NoteID, "note-1")
	}
	if change.Content != "hello world" {
		t.Errorf("Content = %q, want %q", change.Content, "hello world")
	}
	if change.Timestamp < before || change.Timestamp > after {
		t.Errorf("Timestamp %d not in range [%d, %d]", change.Timestamp, before, after)
	}
}

func TestNewOfflineChange_EmptyParams(t *testing.T) {
	change := NewOfflineChange("", "")
	if change.NoteID != "" {
		t.Errorf("NoteID = %q, want empty", change.NoteID)
	}
	if change.Content != "" {
		t.Errorf("Content = %q, want empty", change.Content)
	}
	if change.Timestamp == 0 {
		t.Error("Timestamp should be non-zero even with empty params")
	}
}

func TestNewOfflineChange_TimestampIsCurrentEpoch(t *testing.T) {
	now := time.Now().Unix()
	change := NewOfflineChange("id", "content")

	// Should be within 1 second of now
	diff := change.Timestamp - now
	if diff < 0 || diff > 1 {
		t.Errorf("Timestamp %d is not close to current time %d", change.Timestamp, now)
	}
}
