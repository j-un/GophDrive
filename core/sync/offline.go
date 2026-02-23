package sync

import "time"

// OfflineChange represents a change made while offline.
type OfflineChange struct {
	NoteID    string `json:"noteId"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// NewOfflineChange creates a new offline change.
func NewOfflineChange(noteID, content string) OfflineChange {
	return OfflineChange{
		NoteID:    noteID,
		Content:   content,
		Timestamp: time.Now().Unix(),
	}
}
