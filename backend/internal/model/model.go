package model

import "time"

// UserToken represents the user's OAuth2 token stored in DynamoDB.
type UserToken struct {
	UserID                string    `json:"user_id" dynamodbav:"user_id"`
	EncryptedRefreshToken string    `json:"encrypted_refresh_token" dynamodbav:"encrypted_refresh_token"`
	BaseFolderID          string    `json:"base_folder_id" dynamodbav:"base_folder_id"` // Root folder for the app
	UpdatedAt             time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

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
