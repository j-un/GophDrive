package adapter

import (
	"context"
	"time"
)

// FileMetadata represents metadata about a file stored in the cloud storage.
type FileMetadata struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MIMEType     string    `json:"mimeType"`
	ModifiedTime time.Time `json:"modifiedTime"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	Parents      []string  `json:"parents,omitempty"`
	Starred      bool      `json:"starred"`
}

// File represents a file with its content.
type File struct {
	FileMetadata
	Content []byte `json:"content"`
}

// StorageAdapter defines the interface for interacting with cloud storage services.
// This abstraction allows switching between different providers (e.g., Google Drive, OneDrive)
// without changing the core business logic.
type StorageAdapter interface {
	// ListFiles lists files in a specific folder.
	ListFiles(ctx context.Context, folderID string) ([]FileMetadata, error)

	// GetFile retrieves a file's content and metadata by its ID.
	GetFile(ctx context.Context, fileID string) (*File, error)

	// SaveFile updates an existing file's content.
	// It should verify the ETag to prevent overwriting changes (optimistic locking).
	// If etag is empty, it forces an overwrite.
	SaveFile(ctx context.Context, fileID string, content []byte, etag string) (*FileMetadata, error)

	// CreateFile creates a new file in the specified folder.
	CreateFile(ctx context.Context, name string, content []byte, folderID string) (*FileMetadata, error)

	// CreateFolder creates a new folder.
	CreateFolder(ctx context.Context, name string, parents []string) (*FileMetadata, error)

	// ListRootFolders lists folders that are direct children of 'root', used for setup.
	ListRootFolders(ctx context.Context) ([]FileMetadata, error)

	// EnsureRootFolder ensures a root folder exists and returns its ID.
	EnsureRootFolder(ctx context.Context, name string) (string, error)

	// DeleteFile deletes a file or folder by its ID.
	DeleteFile(ctx context.Context, fileID string) error

	// DuplicateFile duplicates a file by its ID.
	DuplicateFile(ctx context.Context, fileID string) (*FileMetadata, error)

	// RenameFile renames a file by its ID.
	RenameFile(ctx context.Context, fileID string, newName string) (*FileMetadata, error)

	// SetStarred sets the starred status of a file.
	SetStarred(ctx context.Context, fileID string, starred bool) (*FileMetadata, error)

	// ListStarred lists all starred files/folders.
	ListStarred(ctx context.Context) ([]FileMetadata, error)

	// SearchFiles searches for files matching the query.
	SearchFiles(ctx context.Context, query string) ([]FileMetadata, error)
}
