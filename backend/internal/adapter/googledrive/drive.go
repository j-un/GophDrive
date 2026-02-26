package googledrive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jun/gophdrive/backend/internal/adapter"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const mdExt = ".md"

// toDriveName appends .md extension for storage on Google Drive.
func toDriveName(name string) string {
	if strings.HasSuffix(name, mdExt) {
		return name
	}
	return name + mdExt
}

// fromDriveName strips .md extension when returning names to the API.
func fromDriveName(name string) string {
	return strings.TrimSuffix(name, mdExt)
}

// DriveAdapter implements adapter.StorageAdapter for Google Drive.
type DriveAdapter struct {
	service      *drive.Service
	BaseFolderID string
}

// NewDriveAdapter creates a new DriveAdapter.
// standardClient should be an authenticated http.Client with specific user credentials.
func NewDriveAdapter(ctx context.Context, client *http.Client, baseFolderID string) (*DriveAdapter, error) {
	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}
	return &DriveAdapter{service: srv, BaseFolderID: baseFolderID}, nil
}

// ListFiles lists files in a specific folder.
func (d *DriveAdapter) ListFiles(ctx context.Context, folderID string) ([]adapter.FileMetadata, error) {
	targetFolderID := folderID
	if targetFolderID == "" {
		if d.BaseFolderID != "" {
			targetFolderID = d.BaseFolderID
		} else {
			targetFolderID = "root"
		}
	}

	q := fmt.Sprintf("'%s' in parents and trashed = false and (name contains '%s' or mimeType = 'application/vnd.google-apps.folder')", targetFolderID, mdExt)
	// Only fetch necessary fields
	fields := "nextPageToken, files(id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred)"

	r, err := d.service.Files.List().
		Q(q).
		Fields(googleapi.Field(fields)).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list files: %v", err)
	}

	files := []adapter.FileMetadata{}
	for _, f := range r.Files {
		if f.MimeType != "application/vnd.google-apps.folder" && !strings.HasSuffix(f.Name, mdExt) {
			continue
		}
		name := f.Name
		if f.MimeType != "application/vnd.google-apps.folder" {
			name = fromDriveName(name)
		}
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		files = append(files, adapter.FileMetadata{
			ID:           f.Id,
			Name:         name,
			MIMEType:     f.MimeType,
			ModifiedTime: modTime,
			Size:         f.Size,
			ETag:         f.Md5Checksum,
			Parents:      f.Parents,
			Starred:      f.Starred,
		})
	}
	return files, nil
}

// CreateFolder creates a new folder.
func (d *DriveAdapter) CreateFolder(ctx context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	if len(parents) == 0 {
		if d.BaseFolderID != "" {
			parents = []string{d.BaseFolderID}
		} else {
			parents = []string{"root"}
		}
	}

	f := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  parents,
	}

	res, err := d.service.Files.Create(f).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to create folder: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         res.Name,
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
	}, nil
}

// EnsureRootFolder ensures a root folder exists and returns its ID.
// Deprecated: Logic moved to client-side Base Folder selection, but kept for compatibility.
func (d *DriveAdapter) EnsureRootFolder(ctx context.Context, name string) (string, error) {
	// 1. Search for the folder in 'root'
	q := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and 'root' in parents and trashed = false", name)
	r, err := d.service.Files.List().Q(q).Fields("files(id)").Do()
	if err != nil {
		return "", fmt.Errorf("unable to search for root folder: %v", err)
	}

	if len(r.Files) > 0 {
		return r.Files[0].Id, nil
	}

	// 2. Create if not exists
	folder, err := d.CreateFolder(ctx, name, []string{"root"})
	if err != nil {
		return "", fmt.Errorf("unable to create root folder: %v", err)
	}

	return folder.ID, nil
}

// ListRootFolders lists folders that are direct children of 'root', used for setup.
func (d *DriveAdapter) ListRootFolders(ctx context.Context) ([]adapter.FileMetadata, error) {
	// Explicitly list from 'root', ignoring BaseFolderID
	q := "'root' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false"
	fields := "nextPageToken, files(id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred)"

	r, err := d.service.Files.List().
		Q(q).
		Fields(googleapi.Field(fields)).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list root folders: %v", err)
	}

	files := []adapter.FileMetadata{}
	for _, f := range r.Files {
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		files = append(files, adapter.FileMetadata{
			ID:           f.Id,
			Name:         f.Name,
			MIMEType:     f.MimeType,
			ModifiedTime: modTime,
			Size:         f.Size,
			ETag:         f.Md5Checksum,
			Parents:      f.Parents,
			Starred:      f.Starred,
		})
	}
	return files, nil
}

// GetFile retrieves a file's content and metadata by its ID.
func (d *DriveAdapter) GetFile(ctx context.Context, fileID string) (*adapter.File, error) {
	// 1. Get Metadata
	f, err := d.service.Files.Get(fileID).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get file metadata: %v", err)
	}

	// 2. Get Content (only if not a folder)
	var content []byte
	if f.MimeType != "application/vnd.google-apps.folder" {
		resp, err := d.service.Files.Get(fileID).Download()
		if err != nil {
			return nil, fmt.Errorf("unable to download file: %v", err)
		}
		defer resp.Body.Close()

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read file content: %v", err)
		}
	}

	modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)

	return &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           f.Id,
			Name:         fromDriveName(f.Name),
			MIMEType:     f.MimeType,
			ModifiedTime: modTime,
			Size:         f.Size,
			ETag:         f.Md5Checksum,
			Parents:      f.Parents,
			Starred:      f.Starred,
		},
		Content: content,
	}, nil
}

// SaveFile updates an existing file's content.
func (d *DriveAdapter) SaveFile(ctx context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
	// If etag is provided, use If-Match header for optimistic locking.
	f := &drive.File{}
	call := d.service.Files.Update(fileID, f).
		Media(bytes.NewReader(content)).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred")

	if etag != "" {
		call.Header().Set("If-Match", etag)
	}

	res, err := call.Do()
	if err != nil {
		if isPreconditionFailed(err) {
			return nil, adapter.ErrPreconditionFailed
		}
		if isNotFound(err) {
			return nil, adapter.ErrNotFound
		}
		return nil, fmt.Errorf("unable to update file: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         fromDriveName(res.Name),
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
	}, nil
}

func isPreconditionFailed(err error) bool {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == 412
	}
	return false
}

func isNotFound(err error) bool {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == 404
	}
	return false
}

// CreateFile creates a new file in the specified folder.
func (d *DriveAdapter) CreateFile(ctx context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	parents := []string{folderID}
	if folderID == "" {
		if d.BaseFolderID != "" {
			parents = []string{d.BaseFolderID}
		} else {
			parents = []string{"root"}
		}
	}

	f := &drive.File{
		Name:    toDriveName(name),
		Parents: parents,
	}
	res, err := d.service.Files.Create(f).
		Media(bytes.NewReader(content)).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         fromDriveName(res.Name),
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
	}, nil
}

// DeleteFile deletes a file by its ID.
func (d *DriveAdapter) DeleteFile(ctx context.Context, fileID string) error {
	if err := d.service.Files.Delete(fileID).SupportsAllDrives(true).Do(); err != nil {
		return fmt.Errorf("unable to delete file: %v", err)
	}
	return nil
}

// DuplicateFile duplicates a file by its ID.
func (d *DriveAdapter) DuplicateFile(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	// 1. Get original file to generate new name
	orig, err := d.service.Files.Get(fileID).SupportsAllDrives(true).Fields("name, parents").Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get file for duplication: %v", err)
	}

	newName := fmt.Sprintf("Copy of %s", fromDriveName(orig.Name))
	f := &drive.File{
		Name:    toDriveName(newName),
		Parents: orig.Parents,
	}

	// 2. Copy
	res, err := d.service.Files.Copy(fileID, f).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to duplicate file: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         fromDriveName(res.Name),
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
	}, nil
}

// RenameFile renames a file.
func (d *DriveAdapter) RenameFile(ctx context.Context, fileID string, newName string) (*adapter.FileMetadata, error) {
	// 1. Get current metadata to check if it's a folder
	current, err := d.service.Files.Get(fileID).Fields("mimeType").Do()
	if err != nil {
		if isNotFound(err) {
			return nil, adapter.ErrNotFound
		}
		return nil, fmt.Errorf("unable to fetch file metadata for rename: %v", err)
	}

	name := newName
	if current.MimeType != "application/vnd.google-apps.folder" {
		name = toDriveName(newName)
	}

	f := &drive.File{
		Name: name,
	}

	res, err := d.service.Files.Update(fileID, f).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to rename file: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         fromDriveName(res.Name),
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
	}, nil
}

// SetStarred sets the starred status of a file.
func (d *DriveAdapter) SetStarred(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
	f := &drive.File{
		Starred: starred,
	}
	f.ForceSendFields = []string{"Starred"}

	res, err := d.service.Files.Update(fileID, f).
		SupportsAllDrives(true).
		Fields("id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to update starred status: %v", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	return &adapter.FileMetadata{
		ID:           res.Id,
		Name:         fromDriveName(res.Name),
		MIMEType:     res.MimeType,
		ModifiedTime: modTime,
		Size:         res.Size,
		ETag:         res.Md5Checksum,
		Parents:      res.Parents,
		Starred:      res.Starred,
	}, nil
}

// isDescendant checks recursively if targetFolderID is an ancestor of the file.
// It uses a cache to minimize API calls.
func (d *DriveAdapter) isDescendant(ctx context.Context, fileParents []string, targetFolderID string, cache map[string]bool) bool {
	if targetFolderID == "root" {
		return true
	}
	for _, p := range fileParents {
		if p == targetFolderID {
			return true
		}
		if p == "" || p == "root" {
			continue
		}

		// Check cache for this parent
		if val, ok := cache[p]; ok {
			if val {
				return true
			}
			continue
		}

		// Fetch parent metadata to get its parents
		pf, err := d.service.Files.Get(p).Fields("id, parents").Do()
		if err != nil {
			cache[p] = false
			continue
		}

		if d.isDescendant(ctx, pf.Parents, targetFolderID, cache) {
			cache[p] = true
			return true
		}
		cache[p] = false
	}
	return false
}

// ListStarred lists all starred files/folders within the base folder.
func (d *DriveAdapter) ListStarred(ctx context.Context) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if d.BaseFolderID != "" {
		targetFolderID = d.BaseFolderID
	}

	// Search all starred files (API doesn't support recursive 'in parents')
	q := fmt.Sprintf("starred = true and trashed = false and (name contains '%s' or mimeType = 'application/vnd.google-apps.folder')", mdExt)
	fields := "nextPageToken, files(id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred)"

	r, err := d.service.Files.List().
		Q(q).
		Fields(googleapi.Field(fields)).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list starred files: %v", err)
	}

	ancestorCache := make(map[string]bool)
	files := []adapter.FileMetadata{}
	for _, f := range r.Files {
		if f.MimeType != "application/vnd.google-apps.folder" && !strings.HasSuffix(f.Name, mdExt) {
			continue
		}
		// Recursive check
		if !d.isDescendant(ctx, f.Parents, targetFolderID, ancestorCache) {
			continue
		}

		name := f.Name
		if f.MimeType != "application/vnd.google-apps.folder" {
			name = fromDriveName(name)
		}
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		files = append(files, adapter.FileMetadata{
			ID:           f.Id,
			Name:         name,
			MIMEType:     f.MimeType,
			ModifiedTime: modTime,
			Size:         f.Size,
			ETag:         f.Md5Checksum,
			Parents:      f.Parents,
			Starred:      f.Starred,
		})
	}
	return files, nil
}

// SearchFiles searches for files matching the query within the base folder.
func (d *DriveAdapter) SearchFiles(ctx context.Context, query string) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if d.BaseFolderID != "" {
		targetFolderID = d.BaseFolderID
	}

	// Implement SearchFiles using fullText search
	// Note: We remove the 'in parents' constraint to allow recursive search,
	// then filter results in memory.
	q := fmt.Sprintf("fullText contains '%s' and name contains '%s' and mimeType != 'application/vnd.google-apps.folder' and trashed = false", query, mdExt)
	fields := "nextPageToken, files(id, name, mimeType, modifiedTime, size, md5Checksum, parents, starred)"

	r, err := d.service.Files.List().
		Q(q).
		Fields(googleapi.Field(fields)).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to search files: %v", err)
	}

	ancestorCache := make(map[string]bool)
	files := []adapter.FileMetadata{}
	for _, f := range r.Files {
		if !strings.HasSuffix(f.Name, mdExt) {
			continue
		}
		// Recursive check
		if !d.isDescendant(ctx, f.Parents, targetFolderID, ancestorCache) {
			continue
		}

		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		files = append(files, adapter.FileMetadata{
			ID:           f.Id,
			Name:         fromDriveName(f.Name),
			MIMEType:     f.MimeType,
			ModifiedTime: modTime,
			Size:         f.Size,
			ETag:         f.Md5Checksum,
			Parents:      f.Parents,
			Starred:      f.Starred,
		})
	}
	return files, nil
}
