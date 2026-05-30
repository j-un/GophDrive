package dynamo

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/core/markdown"
)

const mdExt = ".md"

// toStoredName appends .md extension for storage.
func toStoredName(name string) string {
	if strings.HasSuffix(name, mdExt) {
		return name
	}
	return name + mdExt
}

// fromStoredName strips .md extension when returning names to the API.
func fromStoredName(name string) string {
	return strings.TrimSuffix(name, mdExt)
}

func getTableName() *string {
	name := os.Getenv("FILE_STORE_TABLE")
	if name == "" {
		name = "FileStore"
	}
	return aws.String(name)
}

// Adapter implements adapter.StorageAdapter.
// If client is nil, it uses in-memory map (for tests).
// If client is set, it uses DynamoDB (for dev mode persistence).
type Adapter struct {
	client *dynamodb.Client
	userID string

	// Fallback for tests
	files map[string]*adapter.File
	mu    sync.RWMutex

	BaseFolderID string
}

const (
	// maxContentSize bounds a single note body. 256KB stays well under DDB's
	// 400KB item-size cap and leaves room for metadata.
	maxContentSize = 256 * 1024
	// maxTitleLength caps how long a file or folder name can be.
	maxTitleLength = 255
	// maxDemoItemCount limits demo-user data only — real (Google-authenticated)
	// users have no item-count limit.
	maxDemoItemCount = 50
	demoUserPrefix   = "demo-user-"
)

// isDemoUser reports whether ephemeral demo-user limits/TTL apply.
func (m *Adapter) isDemoUser() bool {
	return strings.HasPrefix(m.userID, demoUserPrefix)
}

// itemTTL returns the DynamoDB TTL for a newly-written item. Demo users get
// a 60-minute expiry so their data auto-cleans; real users get 0 (omitted by
// omitempty), meaning the item never expires.
func (m *Adapter) itemTTL() int64 {
	if !m.isDemoUser() {
		return 0
	}
	return time.Now().Add(60 * time.Minute).Unix()
}

// checkItemLimit enforces the demo-user item count cap. Real users are
// uncapped at this layer.
func (m *Adapter) checkItemLimit(ctx context.Context) error {
	if !m.isDemoUser() {
		return nil
	}
	count, _ := m.countUserItems(ctx)
	if count >= maxDemoItemCount {
		return fmt.Errorf("item limit reached for demo mode (max %d items)", maxDemoItemCount)
	}
	return nil
}

func (m *Adapter) countUserItems(ctx context.Context) (int, error) {
	if m.client == nil {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return len(m.files), nil
	}
	return countAllPages(ctx, m.client, m.userScanInput())
}

// userScanInput returns the canonical ScanInput used by every per-user scan.
// Concentrating it here means new callers can't accidentally drop the
// user_id filter, and pagination is handled uniformly via scanAllPages.
func (m *Adapter) userScanInput() *dynamodb.ScanInput {
	return &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	}
}

// inlineSizeLimit caps how large a body can be before storage must spill to
// S3. Set well below DynamoDB's 400KB single-item limit to leave room for
// metadata and JSON encoding overhead.
//
// Today every body comes through under this limit (text-only, capped further
// by maxContentSize at the API layer). The constant exists so the spillover
// branch in routeBody is reachable as soon as a future feature lifts the
// API-level cap to allow non-text content.
const inlineSizeLimit = 300 * 1024

// routeBody decides whether a body lives inline (DDB) or in S3.
//
// Phase 3 implements only the inline path. The S3 spillover path is
// reserved: when a future feature enables image/file uploads, this is the
// single place that learns to upload to S3 and return a non-empty key.
// Until then, oversized bodies surface ErrPayloadTooLarge so a regression
// that lifts the API cap fails loudly instead of silently corrupting items.
func (m *Adapter) routeBody(_ context.Context, content []byte) (inline []byte, s3Key string, err error) {
	if len(content) > inlineSizeLimit {
		return nil, "", adapter.ErrPayloadTooLarge
	}
	return content, "", nil
}

// getFileItem reads the raw persisted FileItem (vs the adapter.File facade
// returned by GetFile). Used by metadata-only writers (rename, star) so they
// can preserve every attribute — including BodyS3Key once spillover lands —
// without re-implementing knowledge of the schema.
func (m *Adapter) getFileItem(ctx context.Context, fileID string) (*FileItem, error) {
	out, err := m.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: getTableName(),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fileID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, adapter.ErrNotFound
	}
	var item FileItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// scanUserItems returns every FileItem owned by the adapter's user, walking
// every page of a DynamoDB scan. Without this, a single Scan caps at 1MB and
// missing data goes silently undetected — see scan.go for details.
func (m *Adapter) scanUserItems(ctx context.Context) ([]FileItem, error) {
	raw, _, err := scanAllPages(ctx, m.client, m.userScanInput())
	if err != nil {
		return nil, err
	}
	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// FileItem is the persisted shape of a note or folder in DynamoDB.
//
// Body storage is split between two mutually-exclusive fields so the storage
// strategy can flip without re-versioning the schema:
//
//   - Content holds small bodies inline, capped by inlineSizeLimit. This is
//     the only path used today (text-only notes well under the limit).
//   - BodyS3Key points to an S3 object holding the body. Reserved for the
//     future spillover path; never written by the current code.
//
// Exactly one of the two should be set on a given row.
type FileItem struct {
	PK           string            `dynamodbav:"pk"`
	UserID       string            `dynamodbav:"user_id"`
	ID           string            `dynamodbav:"id"`
	Name         string            `dynamodbav:"name"`
	MIMEType     string            `dynamodbav:"mime_type"`
	ModifiedTime time.Time         `dynamodbav:"modified_time"`
	Size         int64             `dynamodbav:"size"`
	ETag         string            `dynamodbav:"etag"`
	Parents      []string          `dynamodbav:"parents"`
	Starred      bool              `dynamodbav:"starred"`
	Tags         []string          `dynamodbav:"tags,omitempty"`
	Links        []adapter.LinkRef `dynamodbav:"links,omitempty"`
	Content      []byte            `dynamodbav:"content,omitempty"`
	BodyS3Key    string            `dynamodbav:"body_s3_key,omitempty"`
	TTL          int64             `dynamodbav:"ttl,omitempty"`
	CreatedTime  time.Time         `dynamodbav:"created_time,omitempty"`
}

func NewAdapter(client *dynamodb.Client, userID string, baseFolderID string) *Adapter {
	return &Adapter{
		client:       client,
		userID:       userID,
		files:        make(map[string]*adapter.File),
		BaseFolderID: baseFolderID,
	}
}

func (m *Adapter) ListFiles(ctx context.Context, folderID string) ([]adapter.FileMetadata, error) {
	targetFolderID := folderID
	if targetFolderID == "" {
		if m.BaseFolderID != "" {
			targetFolderID = m.BaseFolderID
		} else {
			targetFolderID = "root"
		}
	}

	if m.client == nil {
		return m.listFilesMap(ctx, targetFolderID)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}

	var files []adapter.FileMetadata
	for _, item := range items {
		if item.MIMEType != "application/vnd.google-apps.folder" && !strings.HasSuffix(item.Name, ".md") {
			continue
		}
		// Filter by parent
		isChild := false
		for _, p := range item.Parents {
			if p == targetFolderID {
				isChild = true
				break
			}
		}
		if targetFolderID == "root" && len(item.Parents) == 0 {
			isChild = true
		}
		if folderID == "root" && len(item.Parents) == 0 {
			isChild = true
		}

		if isChild {
			name := item.Name
			if item.MIMEType != "application/vnd.google-apps.folder" {
				name = fromStoredName(name)
			}
			files = append(files, adapter.FileMetadata{
				ID:           item.ID,
				Name:         name,
				MIMEType:     item.MIMEType,
				ModifiedTime: item.ModifiedTime,
				Size:         item.Size,
				ETag:         item.ETag,
				Parents:      item.Parents,
				Starred:      item.Starred,
				Tags:         item.Tags,
			})
		}
	}
	return files, nil
}

func (m *Adapter) GetFile(ctx context.Context, fileID string) (*adapter.File, error) {
	if m.client == nil {
		return m.getFileMap(ctx, fileID)
	}

	out, err := m.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: getTableName(),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fileID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, adapter.ErrNotFound
	}

	var item FileItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, err
	}

	// Defensive: a non-empty BodyS3Key means this row was written by a
	// future-Phase spillover path that the read side hasn't learned to fetch.
	// Surfacing this loudly avoids returning silently-empty Content.
	if item.BodyS3Key != "" {
		return nil, adapter.ErrPayloadTooLarge
	}

	return &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromStoredName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
			Tags:         item.Tags,
			Links:        item.Links,
		},
		Content:     item.Content,
		CreatedTime: item.CreatedTime,
	}, nil
}

func (m *Adapter) SaveFile(ctx context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
	if len(content) > maxContentSize {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxContentSize)
	}

	if m.client == nil {
		return m.saveFileMap(ctx, fileID, content, etag)
	}

	// Get existing to check ETag and get Metadata
	f, err := m.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if etag != "" && f.ETag != etag {
		return nil, adapter.ErrPreconditionFailed
	}

	inline, s3Key, err := m.routeBody(ctx, content)
	if err != nil {
		return nil, err
	}

	f.Content = inline
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()
	f.Size = int64(len(content))
	f.Tags = markdown.ExtractTags(content)

	f.Links, err = resolveLinksLazy(content, f.Links, func() ([]FileItem, error) {
		return m.scanUserItems(ctx)
	})
	if err != nil {
		return nil, err
	}

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toStoredName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		CreatedTime:  f.CreatedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Starred:      f.Starred,
		Tags:         f.Tags,
		Links:        f.Links,
		Content:      inline,
		BodyS3Key:    s3Key,
		TTL:          m.itemTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &f.FileMetadata, nil
}

func (m *Adapter) CreateFile(ctx context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	if len(name) > maxTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxTitleLength)
	}
	if len(content) > maxContentSize {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxContentSize)
	}

	if err := m.checkItemLimit(ctx); err != nil {
		return nil, err
	}

	targetFolderID := folderID
	if targetFolderID == "" {
		if m.BaseFolderID != "" {
			targetFolderID = m.BaseFolderID
		} else {
			targetFolderID = "root"
		}
	}

	if m.client == nil {
		return m.createFileMap(ctx, name, content, targetFolderID)
	}

	inline, s3Key, err := m.routeBody(ctx, content)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	tags := markdown.ExtractTags(content)

	links, err := resolveLinksLazy(content, nil, func() ([]FileItem, error) {
		return m.scanUserItems(ctx)
	})
	if err != nil {
		return nil, err
	}

	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "text/markdown",
			ModifiedTime: time.Now(),
			Size:         int64(len(content)),
			ETag:         uuid.New().String(),
			Parents:      []string{targetFolderID},
			Tags:         tags,
			Links:        links,
		},
		Content: inline,
	}

	now := f.ModifiedTime
	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toStoredName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: now,
		CreatedTime:  now,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Tags:         tags,
		Links:        links,
		Content:      inline,
		BodyS3Key:    s3Key,
		TTL:          m.itemTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	meta := f.FileMetadata
	meta.Name = fromStoredName(meta.Name)
	return &meta, nil
}

func (m *Adapter) CreateFolder(ctx context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	if len(name) > maxTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxTitleLength)
	}

	if err := m.checkItemLimit(ctx); err != nil {
		return nil, err
	}

	targetParents := parents
	if len(targetParents) == 0 {
		if m.BaseFolderID != "" {
			targetParents = []string{m.BaseFolderID}
		} else {
			targetParents = []string{"root"}
		}
	}

	if m.client == nil {
		return m.createFolderMap(ctx, name, targetParents)
	}

	id := uuid.New().String()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "application/vnd.google-apps.folder",
			ModifiedTime: time.Now(),
			Size:         0,
			ETag:         uuid.New().String(),
			Parents:      targetParents,
		},
	}

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         f.Name,
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		CreatedTime:  f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      nil,
		TTL:          m.itemTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}
	return &f.FileMetadata, nil
}

// Helper for case-insensitive check
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func (m *Adapter) EnsureRootFolder(ctx context.Context, name string) (string, error) {
	if m.client == nil {
		return m.ensureRootFolderMap(ctx, name)
	}

	// 1. Search existing (Scan)
	files, err := m.ListFiles(ctx, "root") // Helper scan? No, ListFiles filters by parent "root".
	// But EnsureRootFolder checks for Name + MimeType + Parent=root
	// Let's just Scan all and find it.

	if err == nil {
		for _, f := range files {
			if f.Name == name && f.MIMEType == "application/vnd.google-apps.folder" {
				return f.ID, nil
			}
		}
	}

	// 2. Create
	return m.createRootFolder(ctx, name)
}

func (m *Adapter) createRootFolder(ctx context.Context, name string) (string, error) {
	id := uuid.New().String()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "application/vnd.google-apps.folder",
			ModifiedTime: time.Now(),
			Size:         0,
			ETag:         uuid.New().String(),
			Parents:      []string{"root"},
		},
	}

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         f.Name,
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		CreatedTime:  f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      nil,
		TTL:          m.itemTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return "", err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (m *Adapter) DeleteFile(ctx context.Context, fileID string) error {
	if m.client == nil {
		return m.deleteFileMap(ctx, fileID)
	}

	// Find children for recursive delete.
	items, err := m.scanUserItems(ctx)
	if err != nil {
		return err
	}

	for _, item := range items {
		isChild := false
		for _, p := range item.Parents {
			if p == fileID {
				isChild = true
				break
			}
		}
		if isChild {
			// Recursively delete child
			if err := m.DeleteFile(ctx, item.ID); err != nil {
				return err
			}
		}
	}

	// 2. Delete the item itself
	_, err = m.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: getTableName(),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fileID},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (m *Adapter) DuplicateFile(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	if err := m.checkItemLimit(ctx); err != nil {
		return nil, err
	}

	if m.client == nil {
		return m.duplicateFileMap(ctx, fileID)
	}

	// Read the raw FileItem so spillover-mode rows (BodyS3Key set) wouldn't
	// be silently truncated to an empty body on copy. Phase 3 doesn't write
	// such rows, but the helper means a future "duplicate also copies the
	// S3 object" implementation has only one place to extend.
	orig, err := m.getFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if orig.BodyS3Key != "" {
		// Spillover not wired yet; refuse to duplicate a row we couldn't
		// faithfully copy. Phase 3 never produces such rows.
		return nil, adapter.ErrPayloadTooLarge
	}

	newID := uuid.New().String()
	newName := toStoredName("Copy of " + fromStoredName(orig.Name))
	now := time.Now()

	// Deep-copy the inline body so the new row never shares backing memory.
	dupContent := append([]byte(nil), orig.Content...)

	item := FileItem{
		PK:           newID,
		UserID:       m.userID,
		ID:           newID,
		Name:         newName,
		MIMEType:     orig.MIMEType,
		ModifiedTime: now,
		CreatedTime:  now,
		Size:         orig.Size,
		ETag:         uuid.New().String(),
		Parents:      orig.Parents,
		Tags:         orig.Tags,
		Content:      dupContent,
		TTL:          m.itemTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &adapter.FileMetadata{
		ID:           item.ID,
		Name:         fromStoredName(item.Name),
		MIMEType:     item.MIMEType,
		ModifiedTime: item.ModifiedTime,
		Size:         item.Size,
		ETag:         item.ETag,
		Parents:      item.Parents,
		Starred:      item.Starred,
		Tags:         item.Tags,
	}, nil
}

// --- Map Implementations (Fallback) ---

func (m *Adapter) listFilesMap(ctx context.Context, folderID string) ([]adapter.FileMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []adapter.FileMetadata
	for _, f := range m.files {
		isChild := false
		for _, p := range f.Parents {
			if p == folderID {
				isChild = true
				break
			}
		}
		if folderID == "root" && len(f.Parents) == 0 {
			isChild = true
		}
		if isChild {
			meta := f.FileMetadata
			if meta.MIMEType != "application/vnd.google-apps.folder" {
				meta.Name = fromStoredName(meta.Name)
			}
			files = append(files, meta)
		}
	}
	return files, nil
}

func (m *Adapter) getFileMap(ctx context.Context, fileID string) (*adapter.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}
	meta := f.FileMetadata
	if meta.MIMEType != "application/vnd.google-apps.folder" {
		meta.Name = fromStoredName(meta.Name)
	}
	return &adapter.File{
		FileMetadata: meta,
		Content:      f.Content,
		CreatedTime:  f.CreatedTime,
	}, nil
}

func (m *Adapter) saveFileMap(_ context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}
	if etag != "" && f.ETag != etag {
		return nil, adapter.ErrPreconditionFailed
	}
	f.Content = content
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()
	f.Size = int64(len(content))
	f.Tags = markdown.ExtractTags(content)
	f.Links, _ = resolveLinksLazy(content, f.Links, func() ([]FileItem, error) {
		return m.mapItems(), nil
	})
	return &f.FileMetadata, nil
}

func (m *Adapter) createFileMap(_ context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	now := time.Now()
	links, _ := resolveLinksLazy(content, nil, func() ([]FileItem, error) {
		return m.mapItems(), nil
	})
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         toStoredName(name),
			MIMEType:     "text/markdown",
			ModifiedTime: now,
			Size:         int64(len(content)),
			ETag:         uuid.New().String(),
			Parents:      []string{folderID},
			Tags:         markdown.ExtractTags(content),
			Links:        links,
		},
		Content:     content,
		CreatedTime: now,
	}
	m.files[id] = f
	return &f.FileMetadata, nil
}

func (m *Adapter) createFolderMap(ctx context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	now := time.Now()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "application/vnd.google-apps.folder",
			ModifiedTime: now,
			Size:         0,
			ETag:         uuid.New().String(),
			Parents:      parents,
		},
		CreatedTime: now,
	}
	m.files[id] = f
	return &f.FileMetadata, nil
}

func (m *Adapter) ensureRootFolderMap(ctx context.Context, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, f := range m.files {
		if f.Name == name && f.MIMEType == "application/vnd.google-apps.folder" {
			isRoot := len(f.Parents) == 0
			for _, p := range f.Parents {
				if p == "root" {
					isRoot = true
					break
				}
			}
			if isRoot {
				return f.ID, nil
			}
		}
	}
	// Create
	id := uuid.New().String()
	now := time.Now()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "application/vnd.google-apps.folder",
			ModifiedTime: now,
			Size:         0,
			ETag:         uuid.New().String(),
			Parents:      []string{"root"},
		},
		CreatedTime: now,
	}
	m.files[id] = f
	return id, nil
}

func (m *Adapter) deleteFileMap(ctx context.Context, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check existence
	if _, ok := m.files[fileID]; !ok {
		return adapter.ErrNotFound
	}

	// Find children to delete (avoid modifying map while iterating)
	var childrenToDelete []string
	for id, f := range m.files {
		isChild := false
		for _, p := range f.Parents {
			if p == fileID {
				isChild = true
				break
			}
		}
		if isChild {
			childrenToDelete = append(childrenToDelete, id)
		}
	}

	// Unlock to allow recursive calls
	m.mu.Unlock()
	for _, childID := range childrenToDelete {
		// Recursion (will lock again)
		_ = m.deleteFileMap(ctx, childID)
	}
	m.mu.Lock()

	delete(m.files, fileID)
	return nil
}

func (m *Adapter) duplicateFileMap(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	orig, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}

	newID := uuid.New().String()
	newName := "Copy of " + orig.Name
	now := time.Now()

	newContent := make([]byte, len(orig.Content))
	copy(newContent, orig.Content)

	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           newID,
			Name:         toStoredName(newName),
			MIMEType:     orig.MIMEType,
			ModifiedTime: now,
			Size:         int64(len(newContent)),
			ETag:         uuid.New().String(),
			Parents:      orig.Parents,
			Tags:         orig.Tags,
		},
		Content:     newContent,
		CreatedTime: now,
	}
	m.files[newID] = f
	meta := f.FileMetadata
	meta.Name = fromStoredName(meta.Name)
	return &meta, nil
}

func (m *Adapter) RenameFile(ctx context.Context, fileID string, newName string) (*adapter.FileMetadata, error) {
	if len(newName) > maxTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxTitleLength)
	}

	if m.client == nil {
		return m.renameFileMap(ctx, fileID, newName)
	}

	// Read the raw row so we re-PUT every attribute, including any
	// future-Phase BodyS3Key, instead of going through GetFile (which
	// strips spillover state).
	item, err := m.getFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}

	storedName := newName
	if item.MIMEType != "application/vnd.google-apps.folder" {
		storedName = toStoredName(newName)
	}
	item.Name = storedName
	item.ModifiedTime = time.Now()
	item.ETag = uuid.New().String()
	item.TTL = m.itemTTL()

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &adapter.FileMetadata{
		ID:           item.ID,
		Name:         fromStoredName(item.Name),
		MIMEType:     item.MIMEType,
		ModifiedTime: item.ModifiedTime,
		Size:         item.Size,
		ETag:         item.ETag,
		Parents:      item.Parents,
		Starred:      item.Starred,
		Tags:         item.Tags,
	}, nil
}

func (m *Adapter) SetStarred(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
	if m.client == nil {
		return m.setStarredMap(ctx, fileID, starred)
	}

	// See RenameFile for why this reads the raw FileItem rather than GetFile.
	item, err := m.getFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}

	item.Starred = starred
	item.ModifiedTime = time.Now()
	item.ETag = uuid.New().String()
	item.TTL = m.itemTTL()

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &adapter.FileMetadata{
		ID:           item.ID,
		Name:         fromStoredName(item.Name),
		MIMEType:     item.MIMEType,
		ModifiedTime: item.ModifiedTime,
		Size:         item.Size,
		ETag:         item.ETag,
		Parents:      item.Parents,
		Starred:      item.Starred,
		Tags:         item.Tags,
	}, nil
}

func (m *Adapter) setStarredMap(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}

	f.Starred = starred
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()

	return &f.FileMetadata, nil
}

func (m *Adapter) renameFileMap(ctx context.Context, fileID string, newName string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}

	name := newName
	if f.MIMEType != "application/vnd.google-apps.folder" {
		name = toStoredName(newName)
	}
	f.Name = name
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()

	return &f.FileMetadata, nil
}

func (m *Adapter) MoveFile(ctx context.Context, fileID string, newParentID string) (*adapter.FileMetadata, error) {
	if m.client == nil {
		return m.moveFileMap(ctx, fileID, newParentID)
	}

	item, err := m.getFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if item.UserID != m.userID {
		return nil, adapter.ErrNotFound
	}

	resolvedParentID := newParentID
	if resolvedParentID == "" {
		if m.BaseFolderID != "" {
			resolvedParentID = m.BaseFolderID
		} else {
			resolvedParentID = "root"
		}
	}

	if resolvedParentID == fileID {
		return nil, adapter.ErrInvalidMove
	}

	if resolvedParentID != "root" {
		dest, err := m.getFileItem(ctx, resolvedParentID)
		if err != nil {
			return nil, err
		}
		if dest.UserID != m.userID {
			return nil, adapter.ErrNotFound
		}
		if dest.MIMEType != "application/vnd.google-apps.folder" {
			return nil, adapter.ErrInvalidMove
		}
	}

	// Only folders can create a cycle; notes have no children.
	if item.MIMEType == "application/vnd.google-apps.folder" {
		items, err := m.scanUserItems(ctx)
		if err != nil {
			return nil, err
		}
		parentOf := make(map[string]string, len(items))
		for _, it := range items {
			if len(it.Parents) > 0 {
				parentOf[it.ID] = it.Parents[0]
			}
		}
		visited := make(map[string]bool)
		cur := resolvedParentID
		for cur != "" && cur != "root" {
			if visited[cur] {
				break
			}
			visited[cur] = true
			if cur == fileID {
				return nil, adapter.ErrInvalidMove
			}
			cur = parentOf[cur]
		}
	}

	item.Parents = []string{resolvedParentID}
	item.ModifiedTime = time.Now()
	item.ETag = uuid.New().String()
	item.TTL = m.itemTTL()

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: getTableName(),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &adapter.FileMetadata{
		ID:           item.ID,
		Name:         fromStoredName(item.Name),
		MIMEType:     item.MIMEType,
		ModifiedTime: item.ModifiedTime,
		Size:         item.Size,
		ETag:         item.ETag,
		Parents:      item.Parents,
		Starred:      item.Starred,
		Tags:         item.Tags,
	}, nil
}

func (m *Adapter) moveFileMap(ctx context.Context, fileID string, newParentID string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}

	resolvedParentID := newParentID
	if resolvedParentID == "" {
		if m.BaseFolderID != "" {
			resolvedParentID = m.BaseFolderID
		} else {
			resolvedParentID = "root"
		}
	}

	if resolvedParentID == fileID {
		return nil, adapter.ErrInvalidMove
	}

	// "root" and BaseFolderID are meta-destinations that always exist for the user.
	if resolvedParentID != "root" && resolvedParentID != m.BaseFolderID {
		dest, ok := m.files[resolvedParentID]
		if !ok {
			return nil, adapter.ErrNotFound
		}
		if dest.MIMEType != "application/vnd.google-apps.folder" {
			return nil, adapter.ErrInvalidMove
		}
	}

	if f.MIMEType == "application/vnd.google-apps.folder" {
		parentOf := make(map[string]string, len(m.files))
		for id, it := range m.files {
			if len(it.Parents) > 0 {
				parentOf[id] = it.Parents[0]
			}
		}
		visited := make(map[string]bool)
		cur := resolvedParentID
		for cur != "" && cur != "root" {
			if visited[cur] {
				break
			}
			visited[cur] = true
			if cur == fileID {
				return nil, adapter.ErrInvalidMove
			}
			cur = parentOf[cur]
		}
	}

	f.Parents = []string{resolvedParentID}
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()

	return &f.FileMetadata, nil
}

// Provider implements adapter.StorageProvider backed by DynamoDB.
//
// When client is nil, adapters use an in-memory map (used by tests). The
// per-user adapter cache lets the in-memory fallback retain state across
// calls within a single test or Lambda invocation.
type Provider struct {
	client *dynamodb.Client
	stores map[string]*Adapter
	mu     sync.Mutex
}

func NewProvider(client *dynamodb.Client) *Provider {
	return &Provider{
		client: client,
		stores: make(map[string]*Adapter),
	}
}

// GetAdapter returns the per-user adapter, refreshing its baseFolderID from
// the caller's session every call so the value reflects the JWT.
func (p *Provider) GetAdapter(ctx context.Context, userID, baseFolderID string) (adapter.StorageAdapter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.stores[userID]; !ok {
		p.stores[userID] = NewAdapter(p.client, userID, baseFolderID)
	} else {
		p.stores[userID].BaseFolderID = baseFolderID
	}
	return p.stores[userID], nil
}

// isDescendant checks recursively if targetFolderID is an ancestor of the file.
func (m *Adapter) isDescendant(fileParents []string, targetFolderID string, allItems map[string][]string) bool {
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
		// Recursive check
		if nextParents, ok := allItems[p]; ok {
			if m.isDescendant(nextParents, targetFolderID, allItems) {
				return true
			}
		}
	}
	return false
}

func (m *Adapter) ListStarred(ctx context.Context) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.client == nil {
		return m.listStarredMap(ctx)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}

	// Build parent map for recursive check
	parentMap := make(map[string][]string)
	for _, item := range items {
		parentMap[item.ID] = item.Parents
	}

	var files []adapter.FileMetadata
	for _, item := range items {
		if item.Starred {
			if item.MIMEType != "application/vnd.google-apps.folder" && !strings.HasSuffix(item.Name, ".md") {
				continue
			}
			if m.isDescendant(item.Parents, targetFolderID, parentMap) {
				name := item.Name
				if item.MIMEType != "application/vnd.google-apps.folder" {
					name = fromStoredName(name)
				}
				files = append(files, adapter.FileMetadata{
					ID:           item.ID,
					Name:         name,
					MIMEType:     item.MIMEType,
					ModifiedTime: item.ModifiedTime,
					Size:         item.Size,
					ETag:         item.ETag,
					Parents:      item.Parents,
					Starred:      item.Starred,
					Tags:         item.Tags,
				})
			}
		}
	}
	return files, nil
}

// ListRecent lists recently modified files (proxy for viewed files in demo mode).
func (m *Adapter) ListRecent(ctx context.Context, limit int) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.client == nil {
		return m.listRecentMap(ctx, limit)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}

	// Build parent map
	parentMap := make(map[string][]string)
	for _, item := range items {
		parentMap[item.ID] = item.Parents
	}

	// Filter
	var files []adapter.FileMetadata
	for _, item := range items {
		if item.MIMEType == "application/vnd.google-apps.folder" || !strings.HasSuffix(item.Name, ".md") {
			continue
		}
		if !m.isDescendant(item.Parents, targetFolderID, parentMap) {
			continue
		}
		files = append(files, adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromStoredName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			ViewedTime:   item.ModifiedTime, // Use modifiedTime as proxy
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
			Tags:         item.Tags,
		})
	}

	// Sort by ViewedTime desc
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].ViewedTime.Before(files[j].ViewedTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	if len(files) > limit {
		files = files[:limit]
	}

	return files, nil
}

func (m *Adapter) listRecentMap(ctx context.Context, limit int) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	parentMap := make(map[string][]string)
	for _, f := range m.files {
		parentMap[f.ID] = f.Parents
	}

	var files []adapter.FileMetadata
	for _, f := range m.files {
		if f.MIMEType == "application/vnd.google-apps.folder" || !strings.HasSuffix(f.Name, ".md") {
			continue
		}
		if !m.isDescendant(f.Parents, targetFolderID, parentMap) {
			continue
		}
		meta := f.FileMetadata
		meta.Name = fromStoredName(meta.Name)
		meta.ViewedTime = f.ModifiedTime // Proxy
		files = append(files, meta)
	}

	// Sort desc
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].ViewedTime.Before(files[j].ViewedTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	if len(files) > limit {
		files = files[:limit]
	}

	return files, nil
}

func (m *Adapter) listStarredMap(ctx context.Context) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build parent map
	parentMap := make(map[string][]string)
	for _, f := range m.files {
		parentMap[f.ID] = f.Parents
	}

	var files []adapter.FileMetadata
	for _, f := range m.files {
		if f.Starred {
			if f.MIMEType != "application/vnd.google-apps.folder" && !strings.HasSuffix(f.Name, ".md") {
				continue
			}
			if m.isDescendant(f.Parents, targetFolderID, parentMap) {
				meta := f.FileMetadata
				if meta.MIMEType != "application/vnd.google-apps.folder" {
					meta.Name = fromStoredName(meta.Name)
				}
				files = append(files, meta)
			}
		}
	}
	return files, nil
}

// SearchFiles searches for files matching the query (delegates to SearchFilesWithTags).
func (m *Adapter) SearchFiles(ctx context.Context, query string) ([]adapter.FileMetadata, error) {
	return m.SearchFilesWithTags(ctx, query, nil)
}

// SearchFilesWithTags searches by text query AND all provided tags (AND semantics).
// Items with no stored tags are checked via on-the-fly extraction (lazy fallback).
func (m *Adapter) SearchFilesWithTags(ctx context.Context, query string, tags []string) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.client == nil {
		return m.searchFilesWithTagsMap(ctx, query, tags)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}

	parentMap := make(map[string][]string)
	for _, item := range items {
		parentMap[item.ID] = item.Parents
	}

	var files []adapter.FileMetadata
	for _, item := range items {
		if item.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}
		if !strings.HasSuffix(item.Name, ".md") {
			continue
		}
		if !m.isDescendant(item.Parents, targetFolderID, parentMap) {
			continue
		}

		// Text match (skip if query is empty)
		if query != "" {
			if !containsIgnoreCase(item.Name, query) && !containsIgnoreCase(string(item.Content), query) {
				continue
			}
		}

		// Tag filter (AND semantics); lazy fallback for items with no stored tags
		if len(tags) > 0 {
			itemTags := item.Tags
			if len(itemTags) == 0 && len(item.Content) > 0 {
				// Lazy: item predates tag feature — extract on the fly for this query
				itemTags = markdown.ExtractTags(item.Content)
			}
			if !hasAllTags(itemTags, tags) {
				continue
			}
		}

		files = append(files, adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromStoredName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
			Tags:         item.Tags,
		})
	}
	return files, nil
}

// hasAllTags reports whether itemTags contains every tag in required.
func hasAllTags(itemTags, required []string) bool {
	for _, r := range required {
		found := false
		for _, t := range itemTags {
			if strings.EqualFold(t, r) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ListAllTags returns distinct tags across the user's notes with occurrence counts.
func (m *Adapter) ListAllTags(ctx context.Context) ([]adapter.TagCount, error) {
	if m.client == nil {
		return m.listAllTagsMap(ctx)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, item := range items {
		if item.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}
		for _, t := range item.Tags {
			counts[t]++
		}
	}

	result := make([]adapter.TagCount, 0, len(counts))
	for name, count := range counts {
		result = append(result, adapter.TagCount{Name: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Export walks every note (and every folder above it) under the user's base
// folder and returns one ExportEntry per note, with a POSIX path that mirrors
// the in-app folder hierarchy. Notes whose body lives in S3 (BodyS3Key set)
// are skipped — the spillover read path isn't wired yet, so quietly returning
// empty content would be worse than omitting the note.
func (m *Adapter) Export(ctx context.Context) ([]adapter.ExportEntry, error) {
	if m.client == nil {
		return m.exportMap(ctx)
	}

	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}
	return m.buildExport(items)
}

func (m *Adapter) exportMap(_ context.Context) ([]adapter.ExportEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]FileItem, 0, len(m.files))
	for _, f := range m.files {
		items = append(items, FileItem{
			ID:           f.ID,
			Name:         f.Name,
			MIMEType:     f.MIMEType,
			ModifiedTime: f.ModifiedTime,
			Parents:      f.Parents,
			Content:      f.Content,
		})
	}
	return m.buildExport(items)
}

// buildExport is the storage-agnostic core: given a flat list of items, build
// folder-name and parent-id maps, then compose each note's relative path by
// walking parents up to the base folder (or filesystem "root" pseudo-id).
func (m *Adapter) buildExport(items []FileItem) ([]adapter.ExportEntry, error) {
	folderName := make(map[string]string, len(items))
	parentOf := make(map[string]string, len(items))
	for _, it := range items {
		if it.MIMEType == "application/vnd.google-apps.folder" {
			folderName[it.ID] = it.Name
		}
		if len(it.Parents) > 0 {
			parentOf[it.ID] = it.Parents[0]
		}
	}

	stop := m.BaseFolderID
	if stop == "" {
		stop = "root"
	}

	var entries []adapter.ExportEntry
	for _, it := range items {
		if it.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}
		if !strings.HasSuffix(it.Name, mdExt) {
			continue
		}
		if it.BodyS3Key != "" {
			continue
		}

		segments := folderPathSegments(it.Parents, parentOf, folderName, stop)
		segments = append(segments, fromStoredName(it.Name)+mdExt)
		entries = append(entries, adapter.ExportEntry{
			Path:         strings.Join(segments, "/"),
			Content:      append([]byte(nil), it.Content...),
			ModifiedTime: it.ModifiedTime,
		})
	}
	return entries, nil
}

// folderPathSegments walks parent ids from the note's immediate parent up to
// the stop sentinel, returning folder names in root→leaf order. Cycles are
// defended against with a visited set so a corrupt parent chain can't hang
// the export.
func folderPathSegments(parents []string, parentOf map[string]string, folderName map[string]string, stop string) []string {
	if len(parents) == 0 {
		return nil
	}
	current := parents[0]
	visited := map[string]bool{}
	var rev []string
	for current != "" && current != "root" && current != stop {
		if visited[current] {
			break
		}
		visited[current] = true
		name, ok := folderName[current]
		if !ok {
			break
		}
		rev = append(rev, name)
		current = parentOf[current]
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func (m *Adapter) searchFilesWithTagsMap(_ context.Context, query string, tags []string) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	parentMap := make(map[string][]string)
	for _, f := range m.files {
		parentMap[f.ID] = f.Parents
	}

	var files []adapter.FileMetadata
	for _, f := range m.files {
		if f.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}
		if !strings.HasSuffix(f.Name, ".md") {
			continue
		}
		if !m.isDescendant(f.Parents, targetFolderID, parentMap) {
			continue
		}
		if query != "" {
			if !containsIgnoreCase(f.Name, query) && !containsIgnoreCase(string(f.Content), query) {
				continue
			}
		}
		if len(tags) > 0 {
			itemTags := f.Tags
			if len(itemTags) == 0 && len(f.Content) > 0 {
				itemTags = markdown.ExtractTags(f.Content)
			}
			if !hasAllTags(itemTags, tags) {
				continue
			}
		}
		meta := f.FileMetadata
		meta.Name = fromStoredName(meta.Name)
		files = append(files, meta)
	}
	return files, nil
}

func (m *Adapter) listAllTagsMap(_ context.Context) ([]adapter.TagCount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, f := range m.files {
		if f.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}
		for _, t := range f.Tags {
			counts[t]++
		}
	}

	result := make([]adapter.TagCount, 0, len(counts))
	for name, count := range counts {
		result = append(result, adapter.TagCount{Name: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// mapItems converts the in-memory file map to a FileItem slice for use by
// resolveLinks and buildGraph. Must be called while holding m.mu (read or write).
func (m *Adapter) mapItems() []FileItem {
	items := make([]FileItem, 0, len(m.files))
	for _, f := range m.files {
		items = append(items, FileItem{
			ID:           f.ID,
			Name:         f.Name,
			MIMEType:     f.MIMEType,
			ModifiedTime: f.ModifiedTime,
			CreatedTime:  f.CreatedTime,
			Tags:         f.Tags,
			Links:        f.Links,
		})
	}
	return items
}

func (m *Adapter) EnrichNoteLinks(ctx context.Context, noteID string, links []adapter.LinkRef) ([]adapter.LinkRef, []adapter.BacklinkEntry, error) {
	if m.client == nil {
		return m.enrichNoteLinksMap(ctx, noteID, links)
	}
	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, nil, err
	}
	byID, titleToID := buildLookupMaps(items)
	enriched := enrichLinks(links, byID, titleToID)
	backs := backlinksOf(noteID, items, byID, titleToID)
	return enriched, backs, nil
}

func (m *Adapter) enrichNoteLinksMap(_ context.Context, noteID string, links []adapter.LinkRef) ([]adapter.LinkRef, []adapter.BacklinkEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.mapItems()
	byID, titleToID := buildLookupMaps(items)
	enriched := enrichLinks(links, byID, titleToID)
	backs := backlinksOf(noteID, items, byID, titleToID)
	return enriched, backs, nil
}

func (m *Adapter) Graph(ctx context.Context) ([]adapter.GraphNode, error) {
	if m.client == nil {
		return m.graphMap(ctx)
	}
	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, err
	}
	return buildGraph(items), nil
}

func (m *Adapter) graphMap(_ context.Context) ([]adapter.GraphNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return buildGraph(m.mapItems()), nil
}
