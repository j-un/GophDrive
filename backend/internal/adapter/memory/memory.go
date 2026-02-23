package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/auth"
)

const mdExt = ".md"

// toMemoryName appends .md extension for storage.
func toMemoryName(name string) string {
	if strings.HasSuffix(name, mdExt) {
		return name
	}
	return name + mdExt
}

// fromMemoryName strips .md extension when returning names to the API.
func fromMemoryName(name string) string {
	return strings.TrimSuffix(name, mdExt)
}

func getTableName() *string {
	name := os.Getenv("FILE_STORE_TABLE")
	if name == "" {
		name = "FileStore"
	}
	return aws.String(name)
}

// MemoryAdapter implements adapter.StorageAdapter.
// If client is nil, it uses in-memory map (for tests).
// If client is set, it uses DynamoDB (for dev mode persistence).
type MemoryAdapter struct {
	client *dynamodb.Client
	userID string

	// Fallback for tests
	files map[string]*adapter.File
	mu    sync.RWMutex

	BaseFolderID string
}

const (
	maxDemoContentSize = 256 * 1024 // 256KB
	maxDemoTitleLength = 255
	maxDemoItemCount   = 50
)

func (m *MemoryAdapter) countUserItems(ctx context.Context) (int, error) {
	if m.client == nil {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return len(m.files), nil
	}

	// Scan to count (inefficient for prod, but acceptable for demo/dev mode limit enforcement)
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, err
	}
	return int(out.Count), nil
}

type FileItem struct {
	PK           string    `dynamodbav:"pk"`
	UserID       string    `dynamodbav:"user_id"`
	ID           string    `dynamodbav:"id"`
	Name         string    `dynamodbav:"name"`
	MIMEType     string    `dynamodbav:"mime_type"`
	ModifiedTime time.Time `dynamodbav:"modified_time"`
	Size         int64     `dynamodbav:"size"`
	ETag         string    `dynamodbav:"etag"`
	Parents      []string  `dynamodbav:"parents"`
	Starred      bool      `dynamodbav:"starred"`
	Content      []byte    `dynamodbav:"content"`
	TTL          int64     `dynamodbav:"ttl"`
}

func NewMemoryAdapter(client *dynamodb.Client, userID string, baseFolderID string) *MemoryAdapter {
	return &MemoryAdapter{
		client:       client,
		userID:       userID,
		files:        make(map[string]*adapter.File),
		BaseFolderID: baseFolderID,
	}
}

func (m *MemoryAdapter) ListFiles(ctx context.Context, folderID string) ([]adapter.FileMetadata, error) {
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

	// Scan and filter (inefficient but fine for dev)
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		return nil, err
	}

	var files []adapter.FileMetadata
	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return nil, err
	}

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
				name = fromMemoryName(name)
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
			})
		}
	}
	return files, nil
}

func (m *MemoryAdapter) GetFile(ctx context.Context, fileID string) (*adapter.File, error) {
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

	return &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromMemoryName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
		},
		Content: item.Content,
	}, nil
}

func (m *MemoryAdapter) SaveFile(ctx context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
	if len(content) > maxDemoContentSize {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxDemoContentSize)
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

	f.Content = content
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()
	f.Size = int64(len(content))

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toMemoryName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      f.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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
	meta.Name = fromMemoryName(meta.Name)
	return &meta, nil
}

func (m *MemoryAdapter) CreateFile(ctx context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	if len(name) > maxDemoTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxDemoTitleLength)
	}
	if len(content) > maxDemoContentSize {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxDemoContentSize)
	}

	count, _ := m.countUserItems(ctx)
	if count >= maxDemoItemCount {
		return nil, fmt.Errorf("item limit reached for demo mode (max %d items)", maxDemoItemCount)
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

	id := uuid.New().String()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         toMemoryName(name),
			MIMEType:     "text/markdown",
			ModifiedTime: time.Now(),
			Size:         int64(len(content)),
			ETag:         uuid.New().String(),
			Parents:      []string{targetFolderID},
		},
		Content: content,
	}

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toMemoryName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      f.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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
	meta.Name = fromMemoryName(meta.Name)
	return &meta, nil
}

func (m *MemoryAdapter) CreateFolder(ctx context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	if len(name) > maxDemoTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxDemoTitleLength)
	}

	count, _ := m.countUserItems(ctx)
	if count >= maxDemoItemCount {
		return nil, fmt.Errorf("item limit reached for demo mode (max %d items)", maxDemoItemCount)
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
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      nil,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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

func (m *MemoryAdapter) EnsureRootFolder(ctx context.Context, name string) (string, error) {
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

func (m *MemoryAdapter) createRootFolder(ctx context.Context, name string) (string, error) {
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
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      nil,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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

func (m *MemoryAdapter) DeleteFile(ctx context.Context, fileID string) error {
	if m.client == nil {
		return m.deleteFileMap(ctx, fileID)
	}

	// 1. Find children (Recursive delete)
	// Scan and filter (inefficient but fine for dev)
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		return err
	}

	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
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

func (m *MemoryAdapter) DuplicateFile(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	count, _ := m.countUserItems(ctx)
	if count >= maxDemoItemCount {
		return nil, fmt.Errorf("item limit reached for demo mode (max %d items)", maxDemoItemCount)
	}

	if m.client == nil {
		return m.duplicateFileMap(ctx, fileID)
	}

	// 1. Get original
	orig, err := m.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// 2. Create copy
	newName := "Copy of " + orig.Name
	newID := uuid.New().String()
	now := time.Now()

	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           newID,
			Name:         newName, // stripped name
			MIMEType:     orig.MIMEType,
			ModifiedTime: now,
			Size:         orig.Size,
			ETag:         uuid.New().String(),
			Parents:      orig.Parents,
		},
		Content: orig.Content, // Shallow copy of content slice is fine for now as we don't modify it in place usually
	}

	// Deep copy content if needed, but []byte is standard
	newContent := make([]byte, len(orig.Content))
	copy(newContent, orig.Content)
	f.Content = newContent

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toMemoryName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      f.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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
	meta.Name = fromMemoryName(meta.Name)
	return &meta, nil
}

// --- Map Implementations (Fallback) ---

func (m *MemoryAdapter) listFilesMap(ctx context.Context, folderID string) ([]adapter.FileMetadata, error) {
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
				meta.Name = fromMemoryName(meta.Name)
			}
			files = append(files, meta)
		}
	}
	return files, nil
}

func (m *MemoryAdapter) getFileMap(ctx context.Context, fileID string) (*adapter.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}
	meta := f.FileMetadata
	if meta.MIMEType != "application/vnd.google-apps.folder" {
		meta.Name = fromMemoryName(meta.Name)
	}
	return &adapter.File{
		FileMetadata: meta,
		Content:      f.Content,
	}, nil
}

func (m *MemoryAdapter) saveFileMap(ctx context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
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
	f.Name = toMemoryName(f.Name)
	return &f.FileMetadata, nil
}

func (m *MemoryAdapter) createFileMap(ctx context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         toMemoryName(name),
			MIMEType:     "text/markdown",
			ModifiedTime: time.Now(),
			Size:         int64(len(content)),
			ETag:         uuid.New().String(),
			Parents:      []string{folderID},
		},
		Content: content,
	}
	m.files[id] = f
	return &f.FileMetadata, nil
}

func (m *MemoryAdapter) createFolderMap(ctx context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           id,
			Name:         name,
			MIMEType:     "application/vnd.google-apps.folder",
			ModifiedTime: time.Now(),
			Size:         0,
			ETag:         uuid.New().String(),
			Parents:      parents,
		},
	}
	m.files[id] = f
	return &f.FileMetadata, nil
}

func (m *MemoryAdapter) ensureRootFolderMap(ctx context.Context, name string) (string, error) {
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
	m.files[id] = f
	return id, nil
}

func (m *MemoryAdapter) deleteFileMap(ctx context.Context, fileID string) error {
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

func (m *MemoryAdapter) duplicateFileMap(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
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
			Name:         toMemoryName(newName),
			MIMEType:     orig.MIMEType,
			ModifiedTime: now,
			Size:         int64(len(newContent)),
			ETag:         uuid.New().String(),
			Parents:      orig.Parents,
		},
		Content: newContent,
	}
	m.files[newID] = f
	meta := f.FileMetadata
	meta.Name = fromMemoryName(meta.Name)
	return &meta, nil
}

func (m *MemoryAdapter) duplicateFileDynamo(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	// 1. Get original
	orig, err := m.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// 2. Create copy
	newName := "Copy of " + orig.Name
	newID := uuid.New().String()
	now := time.Now()

	f := &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           newID,
			Name:         newName,
			MIMEType:     orig.MIMEType,
			ModifiedTime: now,
			Size:         orig.Size,
			ETag:         uuid.New().String(),
			Parents:      orig.Parents,
		},
		Content: orig.Content,
	}

	item := FileItem{
		PK:           f.ID,
		UserID:       m.userID,
		ID:           f.ID,
		Name:         toMemoryName(f.Name),
		MIMEType:     f.MIMEType,
		ModifiedTime: f.ModifiedTime,
		Size:         f.Size,
		ETag:         f.ETag,
		Parents:      f.Parents,
		Content:      f.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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
	meta.Name = fromMemoryName(meta.Name)
	return &meta, nil
}

func (m *MemoryAdapter) RenameFile(ctx context.Context, fileID string, newName string) (*adapter.FileMetadata, error) {
	if len(newName) > maxDemoTitleLength {
		return nil, fmt.Errorf("name too long (max %d characters)", maxDemoTitleLength)
	}

	if m.client == nil {
		return m.renameFileMap(ctx, fileID, newName)
	}

	// 1. Get original to check existence and current state
	orig, err := m.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// 2. Update Name
	orig.Name = toMemoryName(newName)
	orig.ModifiedTime = time.Now()
	// ETag should probably change on rename? Yes.
	orig.ETag = uuid.New().String()

	item := FileItem{
		PK:           orig.ID,
		UserID:       m.userID,
		ID:           orig.ID,
		Name:         orig.Name,
		MIMEType:     orig.MIMEType,
		ModifiedTime: orig.ModifiedTime,
		Size:         orig.Size,
		ETag:         orig.ETag,
		Parents:      orig.Parents,
		Starred:      orig.Starred,
		Content:      orig.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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

	return &orig.FileMetadata, nil
}

func (m *MemoryAdapter) SetStarred(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
	if m.client == nil {
		return m.setStarredMap(ctx, fileID, starred)
	}

	// 1. Get original
	orig, err := m.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// 2. Update Starred
	orig.Starred = starred
	orig.ModifiedTime = time.Now()
	orig.ETag = uuid.New().String()

	item := FileItem{
		PK:           orig.ID,
		UserID:       m.userID,
		ID:           orig.ID,
		Name:         orig.Name,
		MIMEType:     orig.MIMEType,
		ModifiedTime: orig.ModifiedTime,
		Size:         orig.Size,
		ETag:         orig.ETag,
		Parents:      orig.Parents,
		Starred:      orig.Starred,
		Content:      orig.Content,
		TTL:          time.Now().Add(60 * time.Minute).Unix(),
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

	return &orig.FileMetadata, nil
}

func (m *MemoryAdapter) setStarredMap(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
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

func (m *MemoryAdapter) renameFileMap(ctx context.Context, fileID string, newName string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[fileID]
	if !ok {
		return nil, adapter.ErrNotFound
	}

	f.Name = toMemoryName(newName)
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()

	return &f.FileMetadata, nil
}

// Provider implements adapter.StorageProvider backed by DynamoDB (or Memory if nil).
type Provider struct {
	client      *dynamodb.Client
	authService *auth.AuthService
	stores      map[string]*MemoryAdapter
	mu          sync.Mutex
}

func NewProvider(client *dynamodb.Client, authService *auth.AuthService) *Provider {
	return &Provider{
		client:      client,
		authService: authService,
		stores:      make(map[string]*MemoryAdapter),
	}
}

func (p *Provider) GetAdapter(ctx context.Context, userID string) (adapter.StorageAdapter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.stores[userID]; !ok {
		// Fetch BaseFolderID from auth service if available
		var baseFolderID string
		if p.authService != nil {
			if token, err := p.authService.GetUserToken(ctx, userID); err == nil {
				baseFolderID = token.BaseFolderID
			}
		}
		p.stores[userID] = NewMemoryAdapter(p.client, userID, baseFolderID)
	}
	// Update BaseFolderID if it changed (simple approach: always update on get?)
	// For now, let's just update it if we have the service.
	if p.authService != nil {
		if token, err := p.authService.GetUserToken(ctx, userID); err == nil {
			p.stores[userID].BaseFolderID = token.BaseFolderID
		}
	}
	return p.stores[userID], nil
}

// ListRootFolders lists "actual" root folders (parents=[] or parents=["root"])
func (m *MemoryAdapter) ListRootFolders(ctx context.Context) ([]adapter.FileMetadata, error) {
	if m.client == nil {
		return m.listRootFoldersMap(ctx)
	}
	// Scan and filter
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return nil, err
	}

	var files []adapter.FileMetadata
	for _, item := range items {
		if item.MIMEType == "application/vnd.google-apps.folder" {
			targetRootID := "root"
			if m.BaseFolderID != "" {
				targetRootID = m.BaseFolderID
			}

			isRoot := len(item.Parents) == 0
			for _, p := range item.Parents {
				if p == targetRootID || p == "root" {
					isRoot = true
					break
				}
			}
			if isRoot {
				files = append(files, adapter.FileMetadata{
					ID:           item.ID,
					Name:         item.Name,
					MIMEType:     item.MIMEType,
					ModifiedTime: item.ModifiedTime,
					Size:         item.Size,
					ETag:         item.ETag,
					Parents:      item.Parents,
					Starred:      item.Starred,
				})
			}
		}
	}
	return files, nil
}

func (m *MemoryAdapter) listRootFoldersMap(ctx context.Context) ([]adapter.FileMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var files []adapter.FileMetadata
	for _, f := range m.files {
		if f.MIMEType == "application/vnd.google-apps.folder" {
			isRoot := len(f.Parents) == 0
			for _, p := range f.Parents {
				if p == "root" {
					isRoot = true
					break
				}
			}
			if isRoot {
				files = append(files, f.FileMetadata)
			}
		}
	}
	return files, nil
}

// isDescendant checks recursively if targetFolderID is an ancestor of the file.
func (m *MemoryAdapter) isDescendant(fileParents []string, targetFolderID string, allItems map[string][]string) bool {
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

func (m *MemoryAdapter) ListStarred(ctx context.Context) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.client == nil {
		return m.listStarredMap(ctx)
	}

	// Scan and filter
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
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
					name = fromMemoryName(name)
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
				})
			}
		}
	}
	return files, nil
}

func (m *MemoryAdapter) listStarredMap(ctx context.Context) ([]adapter.FileMetadata, error) {
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
					meta.Name = fromMemoryName(meta.Name)
				}
				files = append(files, meta)
			}
		}
	}
	return files, nil
}

// SearchFiles searches for files matching the query (simple robust scan for dev).
func (m *MemoryAdapter) SearchFiles(ctx context.Context, query string) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.client == nil {
		return m.searchFilesMap(ctx, query)
	}

	// DynamoDB implementation: Scan and filter in Go (inefficient but OK for dev)
	out, err := m.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []FileItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return nil, err
	}

	// Build parent map for recursive check
	parentMap := make(map[string][]string)
	for _, item := range items {
		parentMap[item.ID] = item.Parents
	}

	var files []adapter.FileMetadata
	for _, item := range items {
		// Simple Case-insensitive substring match on Name or Content
		// Note: Content might be large, but for dev it is fine.
		if item.MIMEType == "application/vnd.google-apps.folder" {
			continue // Don't search folders for now to match cloud logic
		}

		if !strings.HasSuffix(item.Name, ".md") {
			continue
		}

		if !m.isDescendant(item.Parents, targetFolderID, parentMap) {
			continue
		}

		match := false
		if containsIgnoreCase(item.Name, query) {
			match = true
		} else if containsIgnoreCase(string(item.Content), query) {
			match = true
		}

		if match {
			name := item.Name
			if item.MIMEType != "application/vnd.google-apps.folder" {
				name = fromMemoryName(name)
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
			})
		}
	}
	return files, nil
}

func (m *MemoryAdapter) searchFilesMap(ctx context.Context, query string) ([]adapter.FileMetadata, error) {
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
		if f.FileMetadata.MIMEType == "application/vnd.google-apps.folder" {
			continue
		}

		if !strings.HasSuffix(f.Name, ".md") {
			continue
		}

		if !m.isDescendant(f.Parents, targetFolderID, parentMap) {
			continue
		}

		match := false
		if containsIgnoreCase(f.Name, query) {
			match = true
		} else if containsIgnoreCase(string(f.Content), query) {
			match = true
		}

		if match {
			meta := f.FileMetadata
			if meta.MIMEType != "application/vnd.google-apps.folder" {
				meta.Name = fromMemoryName(meta.Name)
			}
			files = append(files, meta)
		}
	}
	return files, nil
}
