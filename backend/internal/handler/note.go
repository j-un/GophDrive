package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

// NoteHandler handles CRUD operations for notes.
type NoteHandler struct {
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

// NewNoteHandler creates a new NoteHandler.
func NewNoteHandler(provider adapter.StorageProvider, jwtSecret string) *NoteHandler {
	return &NoteHandler{storageProvider: provider, jwtSecret: jwtSecret}
}

// getStorageAdapter creates a new storage adapter for the authenticated user.
func (h *NoteHandler) getStorageAdapter(ctx context.Context, req events.APIGatewayProxyRequest) (adapter.StorageAdapter, error) {
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	storage, err := h.storageProvider.GetAdapter(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage adapter: %w", err)
	}

	return storage, nil
}

// ListNotes lists all notes in the specified folder (or root "GophDrive" folder if not specified).
func (h *NoteHandler) ListNotes(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	folderID := req.QueryStringParameters["folderId"]
	if folderID == "" {

	}

	files, err := storage.ListFiles(ctx, folderID)
	if err != nil {
		fmt.Printf("ListFiles error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to list notes: %v", err)}, nil
	}

	body, _ := json.Marshal(files)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// CreateFolder creates a new folder.
func (h *NoteHandler) CreateFolder(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	var payload struct {
		Name     string `json:"name"`
		ParentID string `json:"parentId"`
	}
	if err := json.Unmarshal([]byte(req.Body), &payload); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	if payload.Name == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Folder name is required"}, nil
	}

	parents := []string{}
	if payload.ParentID != "" {
		parents = append(parents, payload.ParentID)
	} else {
	}

	folder, err := storage.CreateFolder(ctx, payload.Name, parents)
	if err != nil {
		fmt.Printf("CreateFolder error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to create folder: %v", err)}, nil
	}

	body, _ := json.Marshal(folder)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// GetNote retrieves a simplified note representation.
func (h *NoteHandler) GetNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	file, err := storage.GetFile(ctx, id)
	if err != nil {
		if errors.Is(err, adapter.ErrNotFound) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
		}
		fmt.Printf("GetFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to get note: %v", err)}, nil
	}

	// For MVP, just return content as string in body, or JSON if model.Note
	// Let's return JSON wrapping content.
	type NoteResponse struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Content  string   `json:"content"`
		Modified string   `json:"modified"`
		ETag     string   `json:"etag"`
		Parents  []string `json:"parents"`
	}

	resp := NoteResponse{
		ID:       file.ID,
		Name:     file.Name,
		Content:  string(file.Content),
		Modified: file.ModifiedTime.Format(time.RFC3339),
		ETag:     file.ETag,
		Parents:  file.Parents,
	}

	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
			"ETag":         file.ETag,
		},
	}, nil
}

// CreateNote creates a new note.
func (h *NoteHandler) CreateNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	var input struct {
		Name     string `json:"name"`
		Content  string `json:"content"`
		ParentID string `json:"parentId"`
	}
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	folderID := input.ParentID
	if folderID == "" {

	}

	file, err := storage.CreateFile(ctx, input.Name, []byte(input.Content), folderID)
	if err != nil {
		fmt.Printf("CreateFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to create note: %v", err)}, nil
	}

	body, _ := json.Marshal(file)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// UpdateNote updates an existing note.
func (h *NoteHandler) UpdateNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	var input struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	// Verify ETag from header (If-Match)
	etag := req.Headers["If-Match"]
	// If etag is empty, we force update (last writer wins) or reject.
	// For optimistic locking, client SHOULD send If-Match.

	file, err := storage.SaveFile(ctx, id, []byte(input.Content), etag)
	if err != nil {
		if errors.Is(err, adapter.ErrPreconditionFailed) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusPreconditionFailed, Body: "ETag mismatch"}, nil
		}
		if errors.Is(err, adapter.ErrNotFound) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
		}
		fmt.Printf("SaveFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to update note: %v", err)}, nil
	}

	body, _ := json.Marshal(file)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// DeleteNote deletes a note.
func (h *NoteHandler) DeleteNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	err = storage.DeleteFile(ctx, id)
	if err != nil {
		fmt.Printf("DeleteFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to delete note: %v", err)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent}, nil
}

// DuplicateNote duplicates a note.
func (h *NoteHandler) DuplicateNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	newFile, err := storage.DuplicateFile(ctx, id)
	if err != nil {
		if errors.Is(err, adapter.ErrNotFound) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
		}
		fmt.Printf("DuplicateFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to duplicate note: %v", err)}, nil
	}

	body, _ := json.Marshal(newFile)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// RenameNote renames a note.
func (h *NoteHandler) RenameNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	if input.Name == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Name is required"}, nil
	}

	updatedFile, err := storage.RenameFile(ctx, id, input.Name)
	if err != nil {
		if errors.Is(err, adapter.ErrNotFound) {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
		}
		fmt.Printf("RenameFile error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to rename note: %v", err)}, nil
	}

	body, _ := json.Marshal(updatedFile)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// PatchNote handles partial updates to a note (e.g. starring).
func (h *NoteHandler) PatchNote(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	id := req.PathParameters["id"]
	if id == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing note ID"}, nil
	}

	var input struct {
		Name    *string `json:"name"`
		Starred *bool   `json:"starred"`
	}
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	var updatedFile *adapter.FileMetadata

	// Handle Rename
	if input.Name != nil {
		if *input.Name == "" {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Name cannot be empty"}, nil
		}
		var err error
		updatedFile, err = storage.RenameFile(ctx, id, *input.Name)
		if err != nil {
			if errors.Is(err, adapter.ErrNotFound) {
				return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
			}
			fmt.Printf("RenameFile error: %v\n", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to rename note: %v", err)}, nil
		}
	}

	// Handle Starred update
	// Note: If both Name and Starred are present, we process both.
	// Since RenameFile and SetStarred both return updated metadata, we simply update `updatedFile`.
	// In a real generic implementation we might want to fetch once, apply all changes, and save once.
	// But our StorageAdapter is granular.
	// If Name was updated, `updatedFile` holds the new state.
	// If we then call SetStarred, we modify the backend again.
	// This is slightly inefficient (2 DB calls) but safe for now.
	if input.Starred != nil {
		var err error
		updatedFile, err = storage.SetStarred(ctx, id, *input.Starred)
		if err != nil {
			if errors.Is(err, adapter.ErrNotFound) {
				return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Note not found"}, nil
			}
			fmt.Printf("SetStarred error: %v\n", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: fmt.Sprintf("Failed to update starred status: %v", err)}, nil
		}
	}

	// If no fields to update were found (or handled), we could return 400 or just current file.
	// For now, if updatedFile is nil, it means nothing changed.
	if updatedFile == nil {
		// Just return the file as is? Or error?
		// Let's assume we wanted to do something.
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "No valid fields to update"}, nil
	}

	body, _ := json.Marshal(updatedFile)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// ListStarredNotes lists all starred notes/folders.
func (h *NoteHandler) ListStarredNotes(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	storage, err := h.getStorageAdapter(ctx, req)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: err.Error()}, nil
	}

	files, err := storage.ListStarred(ctx)
	if err != nil {
		fmt.Printf("ListStarred error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to list starred notes"}, nil
	}

	body, _ := json.Marshal(files)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
