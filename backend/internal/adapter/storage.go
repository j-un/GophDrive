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
	Tags         []string  `json:"tags,omitempty"`
	Links        []LinkRef `json:"links,omitempty"`
	ViewedTime   time.Time `json:"viewedTime,omitempty"`
}

// TagCount is a tag name paired with how many notes carry it.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// LinkRef describes a single [[wiki-link]] within a note.
//
// Title is the text as written in the source. TargetID is the resolved note
// UUID — stable across renames. CurrentTitle is the target's present display
// name, derived at read time from the live note set; it is never persisted.
// Resolved is false when the target did not exist at the last write time (an
// unresolved link may be re-resolved at read time if the target was created
// later).
type LinkRef struct {
	Title        string `json:"title" dynamodbav:"title"`
	TargetID     string `json:"targetId,omitempty" dynamodbav:"target_id,omitempty"`
	CurrentTitle string `json:"currentTitle,omitempty" dynamodbav:"-"`
	Resolved     bool   `json:"resolved" dynamodbav:"resolved"`
}

// BacklinkEntry identifies a note that contains a resolved [[wiki-link]]
// pointing at another note.
type BacklinkEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GraphNode is a lightweight representation of a note used by GET /graph.
// Body content is intentionally excluded to keep the payload small; callers
// fetch bodies individually via GET /notes/{id}.
type GraphNode struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags,omitempty"`
	Links     []LinkRef `json:"links,omitempty"`
	Backlinks []string  `json:"backlinks,omitempty"`
	Modified  time.Time `json:"modified"`
}

// File represents a file with its content.
type File struct {
	FileMetadata
	Content []byte `json:"content"`
}

// ExportEntry is a single note flattened for archive packaging.
//
// Path is a POSIX-style path relative to the user's base folder
// (e.g., "Folder/Sub/note.md"). Folders are not emitted as separate entries —
// the archive writer infers them from path components. ModifiedTime feeds
// the archive's per-entry mtime so round-trips through tar/zip preserve it.
type ExportEntry struct {
	Path         string
	Content      []byte
	ModifiedTime time.Time
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

	// EnsureRootFolder ensures a root folder exists and returns its ID.
	EnsureRootFolder(ctx context.Context, name string) (string, error)

	// DeleteFile deletes a file or folder by its ID.
	DeleteFile(ctx context.Context, fileID string) error

	// DuplicateFile duplicates a file by its ID.
	DuplicateFile(ctx context.Context, fileID string) (*FileMetadata, error)

	// RenameFile renames a file by its ID.
	RenameFile(ctx context.Context, fileID string, newName string) (*FileMetadata, error)

	// MoveFile changes the parent folder of a file or folder.
	// newParentID == "" is resolved to the user's root/base folder.
	// Returns ErrNotFound if the item or destination does not exist,
	// ErrInvalidMove if the destination is the item itself or one of its descendants.
	MoveFile(ctx context.Context, fileID string, newParentID string) (*FileMetadata, error)

	// SetStarred sets the starred status of a file.
	SetStarred(ctx context.Context, fileID string, starred bool) (*FileMetadata, error)

	// ListStarred lists all starred files/folders.
	ListStarred(ctx context.Context) ([]FileMetadata, error)

	// ListRecent lists recently viewed files.
	ListRecent(ctx context.Context, limit int) ([]FileMetadata, error)

	// SearchFiles searches for files matching the query.
	SearchFiles(ctx context.Context, query string) ([]FileMetadata, error)

	// SearchFilesWithTags searches for files matching the query AND all of the
	// provided tags (AND semantics). An empty tags slice applies no tag filter.
	SearchFilesWithTags(ctx context.Context, query string, tags []string) ([]FileMetadata, error)

	// ListAllTags returns every distinct tag across the user's notes with counts.
	ListAllTags(ctx context.Context) ([]TagCount, error)

	// Export returns every note owned by the caller, each annotated with
	// the full folder path leading to it (rooted at the user's base folder).
	// Folders are not returned as separate entries; they are implied by the
	// path components on each note.
	Export(ctx context.Context) ([]ExportEntry, error)

	// EnrichNoteLinks fills CurrentTitle on each resolved link (using the
	// target's present display name), re-resolves unresolved links where the
	// target now exists, and collects backlinks — all in a single user scan.
	EnrichNoteLinks(ctx context.Context, noteID string, links []LinkRef) (enriched []LinkRef, backlinks []BacklinkEntry, err error)

	// Graph returns a lightweight knowledge-graph view of all the caller's
	// notes: IDs, titles, tags, outbound links (with currentTitle), and
	// backlink IDs. Body content is excluded; callers fetch it per-note.
	Graph(ctx context.Context) ([]GraphNode, error)
}
