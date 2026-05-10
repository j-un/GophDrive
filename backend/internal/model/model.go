package model

import "time"

// EditingSession represents an active editing session (lock) on a file.
type EditingSession struct {
	FileID    string `json:"file_id" dynamodbav:"file_id"`
	UserID    string `json:"user_id" dynamodbav:"user_id"`
	ExpiresAt int64  `json:"expires_at" dynamodbav:"expires_at"` // TTL (Unix timestamp)
}

// Note represents the note structure used in API.
type Note struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MIMEType     string    `json:"mimeType"`
	ModifiedTime time.Time `json:"modifiedTime"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	Content      string    `json:"content,omitempty"`
}
