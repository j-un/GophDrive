package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FileMetadata mirrors the backend adapter.FileMetadata JSON shape.
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
	Snippet      string    `json:"snippet,omitempty"`
	Aliases      []string  `json:"aliases,omitempty"`
	Type         string    `json:"type,omitempty"`
}

// NoteResponse is the shape returned by GET /notes/{id}.
type NoteResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Content   string          `json:"content"`
	Modified  string          `json:"modified"`
	ETag      string          `json:"etag"`
	Parents   []string        `json:"parents,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Links     []LinkRef       `json:"links,omitempty"`
	Backlinks []BacklinkEntry `json:"backlinks,omitempty"`
}

// LinkRef describes a single [[wiki-link]] within a note, mirroring
// adapter.LinkRef. CurrentTitle and Resolved are derived at read time from
// the live note set and are never persisted server-side.
type LinkRef struct {
	Title        string `json:"title"`
	TargetID     string `json:"targetId,omitempty"`
	CurrentTitle string `json:"currentTitle,omitempty"`
	Resolved     bool   `json:"resolved"`
}

// BacklinkEntry identifies a note that contains a resolved [[wiki-link]]
// pointing at another note, mirroring adapter.BacklinkEntry.
type BacklinkEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GraphNode is a lightweight representation of a note returned by GET /graph,
// mirroring adapter.GraphNode. Body content is excluded.
type GraphNode struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags,omitempty"`
	Links     []LinkRef `json:"links,omitempty"`
	Backlinks []string  `json:"backlinks,omitempty"`
	Modified  time.Time `json:"modified"`
}

// TagCount mirrors adapter.TagCount.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// UserProfile is the shape returned by GET /auth/user.
type UserProfile struct {
	ID           string `json:"id"`
	BaseFolderID string `json:"base_folder_id"`
}

// ErrConflict is returned by UpdateNote when the server responds 412.
var ErrConflict = errors.New("ETag conflict (412 Precondition Failed)")

// ErrNotFound is returned when the server responds 404.
var ErrNotFound = errors.New("not found (404)")

// Client is a thin HTTP wrapper around the GophDrive REST API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient constructs a Client. Pass a custom *http.Client as the optional
// third argument to override the default (useful for tests with httptest).
func NewClient(baseURL, apiKey string, httpClient ...*http.Client) *Client {
	hc := &http.Client{Timeout: 30 * time.Second}
	if len(httpClient) > 0 && httpClient[0] != nil {
		hc = httpClient[0]
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    hc,
	}
}

func (c *Client) do(method, path string, body any, extraHeaders map[string]string) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

func readJSON[T any](resp *http.Response) (T, error) {
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decode response: %w", err)
	}
	return v, nil
}

func (c *Client) GetUser() (UserProfile, error) {
	resp, err := c.do("GET", "/auth/user", nil, nil)
	if err != nil {
		return UserProfile{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return UserProfile{}, fmt.Errorf("GET /auth/user: status %d", resp.StatusCode)
	}
	return readJSON[UserProfile](resp)
}

func (c *Client) ListNotes(folderID string) ([]FileMetadata, error) {
	path := "/notes"
	if folderID != "" {
		path += "?folderId=" + url.QueryEscape(folderID)
	}
	resp, err := c.do("GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET /notes: status %d", resp.StatusCode)
	}
	files, err := readJSON[[]FileMetadata](resp)
	if err != nil {
		return nil, err
	}
	if files == nil {
		files = []FileMetadata{}
	}
	return files, nil
}

func (c *Client) GetNote(id string) (NoteResponse, error) {
	resp, err := c.do("GET", "/notes/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return NoteResponse{}, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return NoteResponse{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return NoteResponse{}, fmt.Errorf("GET /notes/%s: status %d", id, resp.StatusCode)
	}
	return readJSON[NoteResponse](resp)
}

// GetGraph fetches a lightweight knowledge-graph view of every note the
// caller owns (GET /graph).
func (c *Client) GetGraph() ([]GraphNode, error) {
	resp, err := c.do("GET", "/graph", nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET /graph: status %d", resp.StatusCode)
	}
	nodes, err := readJSON[[]GraphNode](resp)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []GraphNode{}
	}
	return nodes, nil
}

// CacheKey returns a string unique to this (baseURL, apiKey) pair for use as a
// folder cache key. The API key is hashed so it never appears in the cache file.
// GET /notes/{id} resolves both files and folders — callers depend on this.
func (c *Client) CacheKey() string {
	digest := "anonymous"
	if c.apiKey != "" {
		sum := sha256.Sum256([]byte(c.apiKey))
		digest = hex.EncodeToString(sum[:4]) // 4 bytes → 8 hex chars (32-bit digest)
	}
	return c.baseURL + "#" + digest
}

type createNoteReq struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	ParentID string `json:"parentId"`
}

func (c *Client) CreateNote(name, content, parentID string) (FileMetadata, error) {
	resp, err := c.do("POST", "/notes", createNoteReq{Name: name, Content: content, ParentID: parentID}, nil)
	if err != nil {
		return FileMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return FileMetadata{}, fmt.Errorf("POST /notes: status %d: %s", resp.StatusCode, body)
	}
	return readJSON[FileMetadata](resp)
}

type updateNoteReq struct {
	Content string `json:"content"`
}

// UpdateNote calls PUT /notes/{id} with If-Match. Returns ErrConflict on 412.
func (c *Client) UpdateNote(id, content, etag string) (FileMetadata, error) {
	resp, err := c.do("PUT", "/notes/"+url.PathEscape(id), updateNoteReq{Content: content},
		map[string]string{"If-Match": etag})
	if err != nil {
		return FileMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return FileMetadata{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return FileMetadata{}, fmt.Errorf("PUT /notes/%s: status %d: %s", id, resp.StatusCode, body)
	}
	return readJSON[FileMetadata](resp)
}

type createFolderReq struct {
	Name    string   `json:"name"`
	Parents []string `json:"parents"`
}

func (c *Client) CreateFolder(name string, parents []string) (FileMetadata, error) {
	resp, err := c.do("POST", "/folders", createFolderReq{Name: name, Parents: parents}, nil)
	if err != nil {
		return FileMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return FileMetadata{}, fmt.Errorf("POST /folders: status %d: %s", resp.StatusCode, body)
	}
	return readJSON[FileMetadata](resp)
}

// SearchOpts holds the optional filters accepted by GET /search.
type SearchOpts struct {
	Tags           []string
	Limit          int
	Scope          string // "in" param
	Type           string
	ModifiedAfter  string // RFC3339, already formatted
	ModifiedBefore string
}

func (c *Client) Search(query string, opts SearchOpts) ([]FileMetadata, error) {
	v := url.Values{}
	if query != "" {
		v.Set("q", query)
	}
	for _, t := range opts.Tags {
		v.Add("tag", t)
	}
	if opts.Limit > 0 {
		v.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Scope != "" {
		v.Set("in", opts.Scope)
	}
	if opts.Type != "" {
		v.Set("type", opts.Type)
	}
	if opts.ModifiedAfter != "" {
		v.Set("modifiedAfter", opts.ModifiedAfter)
	}
	if opts.ModifiedBefore != "" {
		v.Set("modifiedBefore", opts.ModifiedBefore)
	}
	resp, err := c.do("GET", "/search?"+v.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET /search: status %d", resp.StatusCode)
	}
	files, err := readJSON[[]FileMetadata](resp)
	if err != nil {
		return nil, err
	}
	if files == nil {
		files = []FileMetadata{}
	}
	return files, nil
}

func (c *Client) ListTags() ([]TagCount, error) {
	resp, err := c.do("GET", "/tags", nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET /tags: status %d", resp.StatusCode)
	}
	return readJSON[[]TagCount](resp)
}
