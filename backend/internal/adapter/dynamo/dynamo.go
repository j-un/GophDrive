package dynamo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/core/markdown"
)

const mdExt = ".md"

func toStoredName(name string) string {
	if strings.HasSuffix(name, mdExt) {
		return name
	}
	return name + mdExt
}

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

// DDBClient is the subset of *dynamodb.Client methods used by Adapter and
// Provider. Follows the secret.SSMClient pattern so tests can inject a fake.
type DDBClient interface {
	ddbScanner
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

// Adapter implements adapter.StorageAdapter.
// When inMemory is true the adapter uses an in-memory map (for tests); when
// false it drives DynamoDB via client. The flag — rather than a nil-client
// check — is authoritative so a live client can never accidentally opt into
// the fallback path.
type Adapter struct {
	client   DDBClient
	inMemory bool
	userID   string

	files map[string]*adapter.File
	mu    sync.RWMutex

	BaseFolderID string
}

const (
	// maxContentSize bounds a single note body. 256KB stays well under DDB's
	// 400KB item-size cap and leaves room for metadata.
	maxContentSize   = 256 * 1024
	maxTitleLength   = 255
	maxDemoItemCount = 50
)

func (m *Adapter) isDemoUser() bool {
	return strings.HasPrefix(m.userID, adapter.DemoUserIDPrefix)
}

func (m *Adapter) itemTTL() int64 {
	if !m.isDemoUser() {
		return 0
	}
	return time.Now().Add(60 * time.Minute).Unix()
}

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
	if m.inMemory {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return len(m.files), nil
	}
	return countAllPages(ctx, m.client, m.userScanInput())
}

func (m *Adapter) userScanInput() *dynamodb.ScanInput {
	return &dynamodb.ScanInput{
		TableName:        getTableName(),
		FilterExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	}
}

const inlineSizeLimit = 300 * 1024

func (m *Adapter) routeBody(_ context.Context, content []byte) (inline []byte, s3Key string, err error) {
	if len(content) > inlineSizeLimit {
		return nil, "", adapter.ErrPayloadTooLarge
	}
	return content, "", nil
}

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

func (m *Adapter) getOwnedFileItem(ctx context.Context, fileID string) (*FileItem, error) {
	item, err := m.getFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if item.UserID != m.userID {
		return nil, adapter.ErrNotFound
	}
	return item, nil
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
	Headings     []string          `dynamodbav:"headings,omitempty"`
	Aliases      []string          `dynamodbav:"aliases,omitempty"`
	NoteType     string            `dynamodbav:"note_type,omitempty"`
	Status       string            `dynamodbav:"status,omitempty"`
	Links        []adapter.LinkRef `dynamodbav:"links,omitempty"`
	Content      []byte            `dynamodbav:"content,omitempty"`
	BodyS3Key    string            `dynamodbav:"body_s3_key,omitempty"`
	TTL          int64             `dynamodbav:"ttl,omitempty"`
	CreatedTime  time.Time         `dynamodbav:"created_time,omitempty"`
	// ViewedTime records when the note was last opened (read). ListRecent orders
	// by this; it falls back to ModifiedTime when absent (notes never opened
	// since this field was introduced). Updated by TouchViewed without touching
	// ETag or ModifiedTime, so it never affects optimistic concurrency.
	ViewedTime time.Time `dynamodbav:"viewed_time,omitempty"`
}

// NewAdapter returns an Adapter backed by DynamoDB via client. Passing a nil
// interface delegates to NewMemoryAdapter (legacy path — new tests should
// call NewMemoryAdapter directly). A nil-typed *dynamodb.Client would satisfy
// DDBClient without being a nil interface and would incorrectly drive real
// DynamoDB calls; production callers must construct the client via
// dynamodb.NewFromConfig(cfg).
func NewAdapter(client DDBClient, userID string, baseFolderID string) *Adapter {
	if client == nil {
		return NewMemoryAdapter(userID, baseFolderID)
	}
	return &Adapter{
		client:       client,
		userID:       userID,
		files:        make(map[string]*adapter.File),
		BaseFolderID: baseFolderID,
	}
}

// NewMemoryAdapter returns an Adapter backed by an in-memory map. Preferred
// over NewAdapter(nil, ...) in tests: the fallback is opt-in structurally
// (inMemory=true) rather than inferred from a nil client, so a live client
// can never accidentally trip the in-memory branch.
func NewMemoryAdapter(userID string, baseFolderID string) *Adapter {
	return &Adapter{
		inMemory:     true,
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

	if m.inMemory {
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
				Headings:     item.Headings,
				Aliases:      item.Aliases,
				Type:         item.NoteType,
				Status:       item.Status,
			})
		}
	}
	return files, nil
}

func (m *Adapter) GetFile(ctx context.Context, fileID string) (*adapter.File, error) {
	if m.inMemory {
		return m.getFileMap(ctx, fileID)
	}

	item, err := m.getOwnedFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if item.BodyS3Key != "" {
		return nil, adapter.ErrPayloadTooLarge
	}

	return &adapter.File{
		FileMetadata: adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromStoredName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			ViewedTime:   item.ViewedTime,
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
			Tags:         item.Tags,
			Aliases:      item.Aliases,
			Type:         item.NoteType,
			Status:       item.Status,
			Links:        item.Links,
		},
		Content:     item.Content,
		CreatedTime: item.CreatedTime,
	}, nil
}

// TouchViewed records that a note was opened (viewed), updating only its
// viewed_time so ListRecent can order by recency-of-access. It deliberately
// leaves content, etag and modified_time untouched, so it never triggers a
// conflict on the optimistic-concurrency save path. A missing row (note
// deleted between read and touch) is treated as a no-op rather than an error.
func (m *Adapter) TouchViewed(ctx context.Context, fileID string) error {
	now := time.Now()
	if m.inMemory {
		return m.touchViewedMap(fileID, now)
	}

	nowAV, err := attributevalue.Marshal(now)
	if err != nil {
		return err
	}
	updateExpr := "SET viewed_time = :v"
	values := map[string]types.AttributeValue{
		":v":   nowAV,
		":uid": &types.AttributeValueMemberS{Value: m.userID},
	}
	var names map[string]string
	if ttl := m.itemTTL(); ttl != 0 {
		updateExpr += ", #ttl = :t"
		values[":t"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(ttl, 10)}
		names = map[string]string{"#ttl": "ttl"}
	}
	_, err = m.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: getTableName(),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fileID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ConditionExpression:       aws.String("attribute_exists(pk) AND user_id = :uid"),
		ExpressionAttributeValues: values,
		ExpressionAttributeNames:  names,
	})
	if err != nil {
		var notFound *types.ConditionalCheckFailedException
		if errors.As(err, &notFound) {
			return nil
		}
		return err
	}
	return nil
}

func (m *Adapter) touchViewedMap(fileID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.files[fileID]; ok {
		f.ViewedTime = now
	}
	return nil
}

func (m *Adapter) SaveFile(ctx context.Context, fileID string, content []byte, etag string) (*adapter.FileMetadata, error) {
	if len(content) > maxContentSize {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxContentSize)
	}

	if m.inMemory {
		return m.saveFileMap(ctx, fileID, content, etag)
	}

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

	meta, _, _ := markdown.ParseNoteMeta(content)

	f.Content = inline
	f.ModifiedTime = time.Now()
	f.ETag = uuid.New().String()
	f.Size = int64(len(content))
	f.Tags = markdown.ExtractTags(content)
	f.Headings = markdown.ExtractHeadings(content)
	f.Aliases = meta.Aliases
	f.Type = meta.Type
	f.Status = meta.Status

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
		Headings:     f.Headings,
		Aliases:      f.Aliases,
		NoteType:     f.Type,
		Status:       f.Status,
		Links:        f.Links,
		Content:      inline,
		BodyS3Key:    s3Key,
		TTL:          m.itemTTL(),
		ViewedTime:   f.ModifiedTime,
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

	if m.inMemory {
		return m.createFileMap(ctx, name, content, targetFolderID)
	}

	if targetFolderID != m.BaseFolderID && targetFolderID != "root" {
		it, err := m.getOwnedFileItem(ctx, targetFolderID)
		if err != nil {
			if errors.Is(err, adapter.ErrNotFound) {
				return nil, fmt.Errorf("parent folder %q: %w", targetFolderID, adapter.ErrNotFound)
			}
			return nil, err
		}
		if it.MIMEType != "application/vnd.google-apps.folder" {
			return nil, fmt.Errorf("parent folder %q: %w", targetFolderID, adapter.ErrNotFound)
		}
	}

	inline, s3Key, err := m.routeBody(ctx, content)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	tags := markdown.ExtractTags(content)
	headings := markdown.ExtractHeadings(content)
	noteMeta, _, _ := markdown.ParseNoteMeta(content)

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
			Aliases:      noteMeta.Aliases,
			Type:         noteMeta.Type,
			Status:       noteMeta.Status,
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
		Headings:     headings,
		Aliases:      noteMeta.Aliases,
		NoteType:     noteMeta.Type,
		Status:       noteMeta.Status,
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

	if m.inMemory {
		return m.createFolderMap(ctx, name, targetParents)
	}

	parentID := targetParents[0]
	if parentID != m.BaseFolderID && parentID != "root" {
		it, err := m.getOwnedFileItem(ctx, parentID)
		if err != nil {
			if errors.Is(err, adapter.ErrNotFound) {
				return nil, fmt.Errorf("parent folder %q: %w", parentID, adapter.ErrNotFound)
			}
			return nil, err
		}
		if it.MIMEType != "application/vnd.google-apps.folder" {
			return nil, fmt.Errorf("parent folder %q: %w", parentID, adapter.ErrNotFound)
		}
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

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func itemNoteType(noteType string, content []byte) string {
	if noteType != "" {
		return noteType
	}
	if len(content) == 0 {
		return ""
	}
	meta, _, _ := markdown.ParseNoteMeta(content)
	return meta.Type
}

func scoreMatch(name string, aliases []string, content []byte, headings, tags []string, query string, scope adapter.SearchScope) (hit bool, score int, snippet string) {
	if query == "" {
		return false, 0, ""
	}
	titleScore := 0
	if containsIgnoreCase(name, query) {
		titleScore = 100
		if strings.EqualFold(strings.TrimSuffix(name, mdExt), query) {
			titleScore += 50
		}
	}

	aliasContains, aliasExact := false, false
	for _, a := range aliases {
		if containsIgnoreCase(a, query) {
			aliasContains = true
		}
		if strings.EqualFold(a, query) {
			aliasExact = true
		}
	}
	aliasScore := 0
	if aliasContains {
		aliasScore += 80
	}
	if aliasExact {
		aliasScore += 40
	}

	if scope == adapter.ScopeTitles {
		s := titleScore + aliasScore
		return s > 0, s, ""
	}

	hs := headings
	if len(hs) == 0 && len(content) > 0 {
		hs = markdown.ExtractHeadings(content)
	}
	headingScore := 0
	for _, h := range hs {
		if containsIgnoreCase(h, query) {
			headingScore = 40
			break
		}
	}

	if scope == adapter.ScopeHeadings {
		return headingScore > 0, headingScore, ""
	}

	tagScore := 0
	for _, t := range tags {
		if containsIgnoreCase(t, query) {
			tagScore = 30
			break
		}
	}

	n := strings.Count(strings.ToLower(string(content)), strings.ToLower(query))
	cappedN := n
	if cappedN > 5 {
		cappedN = 5
	}
	bodyScore := 10 * cappedN

	total := titleScore + aliasScore + headingScore + tagScore + bodyScore
	if total == 0 {
		return false, 0, ""
	}
	snip := ""
	if n > 0 {
		snip = makeSnippet(content, query, 60)
	}
	return true, total, snip
}

func makeSnippet(content []byte, query string, window int) string {
	if len(query) == 0 || len(content) == 0 {
		return ""
	}
	lower := strings.ToLower(string(content))
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		return ""
	}
	start := idx - window
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + window
	if end > len(content) {
		end = len(content)
	}
	// Snap to UTF-8 rune boundaries so multibyte characters (e.g. Japanese) are not split.
	for start > 0 && !utf8.RuneStart(content[start]) {
		start--
	}
	for end < len(content) && !utf8.RuneStart(content[end]) {
		end++
	}
	snippet := strings.ReplaceAll(string(content[start:end]), "\n", " ")
	for strings.Contains(snippet, "  ") {
		snippet = strings.ReplaceAll(snippet, "  ", " ")
	}
	snippet = strings.TrimSpace(snippet)
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(content) {
		snippet = snippet + "…"
	}
	return snippet
}

func (m *Adapter) EnsureRootFolder(ctx context.Context, name string) (string, error) {
	if m.inMemory {
		return m.ensureRootFolderMap(ctx, name)
	}

	files, err := m.ListFiles(ctx, "root")

	if err == nil {
		for _, f := range files {
			if f.Name == name && f.MIMEType == "application/vnd.google-apps.folder" {
				return f.ID, nil
			}
		}
	}

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
	if m.inMemory {
		return m.deleteFileMap(ctx, fileID)
	}

	if _, err := m.getOwnedFileItem(ctx, fileID); err != nil {
		return err
	}

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
			if err := m.DeleteFile(ctx, item.ID); err != nil {
				return err
			}
		}
	}

	_, err = m.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: getTableName(),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fileID},
		},
		ConditionExpression: aws.String("attribute_exists(pk) AND user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return adapter.ErrNotFound
		}
		return err
	}
	return nil
}

func (m *Adapter) DuplicateFile(ctx context.Context, fileID string) (*adapter.FileMetadata, error) {
	if err := m.checkItemLimit(ctx); err != nil {
		return nil, err
	}

	if m.inMemory {
		return m.duplicateFileMap(ctx, fileID)
	}

	orig, err := m.getOwnedFileItem(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if orig.BodyS3Key != "" {
		return nil, adapter.ErrPayloadTooLarge
	}

	newID := uuid.New().String()
	newName := toStoredName("Copy of " + fromStoredName(orig.Name))
	now := time.Now()

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
		Headings:     orig.Headings,
		Aliases:      orig.Aliases,
		NoteType:     orig.NoteType,
		Status:       orig.Status,
		Links:        orig.Links,
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
		Aliases:      item.Aliases,
		Type:         item.NoteType,
		Status:       item.Status,
	}, nil
}

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
	meta, _, _ := markdown.ParseNoteMeta(content)
	f.Content = content
	f.ModifiedTime = time.Now()
	f.ViewedTime = f.ModifiedTime
	f.ETag = uuid.New().String()
	f.Size = int64(len(content))
	f.Tags = markdown.ExtractTags(content)
	f.Headings = markdown.ExtractHeadings(content)
	f.Aliases = meta.Aliases
	f.Type = meta.Type
	f.Status = meta.Status
	f.Links, _ = resolveLinksLazy(content, f.Links, func() ([]FileItem, error) {
		return m.mapItems(), nil
	})
	return &f.FileMetadata, nil
}

func (m *Adapter) createFileMap(_ context.Context, name string, content []byte, folderID string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if folderID != "root" && folderID != m.BaseFolderID && folderID != "" {
		f, ok := m.files[folderID]
		if !ok || f.MIMEType != "application/vnd.google-apps.folder" {
			return nil, fmt.Errorf("parent folder %q: %w", folderID, adapter.ErrNotFound)
		}
	}
	id := uuid.New().String()
	now := time.Now()
	links, _ := resolveLinksLazy(content, nil, func() ([]FileItem, error) {
		return m.mapItems(), nil
	})
	meta, _, _ := markdown.ParseNoteMeta(content)
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
			Headings:     markdown.ExtractHeadings(content),
			Aliases:      meta.Aliases,
			Type:         meta.Type,
			Status:       meta.Status,
			Links:        links,
		},
		Content:     content,
		CreatedTime: now,
	}
	m.files[id] = f
	return &f.FileMetadata, nil
}

func (m *Adapter) createFolderMap(_ context.Context, name string, parents []string) (*adapter.FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	parentID := parents[0]
	if parentID != "root" && parentID != m.BaseFolderID {
		f, ok := m.files[parentID]
		if !ok || f.MIMEType != "application/vnd.google-apps.folder" {
			return nil, fmt.Errorf("parent folder %q: %w", parentID, adapter.ErrNotFound)
		}
	}
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

func (m *Adapter) deleteFileMap(_ context.Context, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleteFileMapLocked(fileID)
}

func (m *Adapter) deleteFileMapLocked(fileID string) error {
	if _, ok := m.files[fileID]; !ok {
		return adapter.ErrNotFound
	}

	var children []string
	for id, f := range m.files {
		for _, p := range f.Parents {
			if p == fileID {
				children = append(children, id)
				break
			}
		}
	}
	for _, childID := range children {
		if err := m.deleteFileMapLocked(childID); err != nil {
			return err
		}
	}
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
			Headings:     orig.Headings,
			Aliases:      orig.Aliases,
			Type:         orig.Type,
			Status:       orig.Status,
			Links:        orig.Links,
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

	if m.inMemory {
		return m.renameFileMap(ctx, fileID, newName)
	}

	item, err := m.getOwnedFileItem(ctx, fileID)
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
		TableName:           getTableName(),
		Item:                av,
		ConditionExpression: aws.String("attribute_exists(pk) AND user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return nil, adapter.ErrNotFound
		}
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
		Headings:     item.Headings,
		Aliases:      item.Aliases,
		Type:         item.NoteType,
		Status:       item.Status,
	}, nil
}

func (m *Adapter) SetStarred(ctx context.Context, fileID string, starred bool) (*adapter.FileMetadata, error) {
	if m.inMemory {
		return m.setStarredMap(ctx, fileID, starred)
	}

	item, err := m.getOwnedFileItem(ctx, fileID)
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
		TableName:           getTableName(),
		Item:                av,
		ConditionExpression: aws.String("attribute_exists(pk) AND user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: m.userID},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return nil, adapter.ErrNotFound
		}
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
		Headings:     item.Headings,
		Aliases:      item.Aliases,
		Type:         item.NoteType,
		Status:       item.Status,
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
	if m.inMemory {
		return m.moveFileMap(ctx, fileID, newParentID)
	}

	item, err := m.getOwnedFileItem(ctx, fileID)
	if err != nil {
		return nil, err
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
		dest, err := m.getOwnedFileItem(ctx, resolvedParentID)
		if err != nil {
			return nil, err
		}
		if dest.MIMEType != "application/vnd.google-apps.folder" {
			return nil, adapter.ErrInvalidMove
		}
	}

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
		Headings:     item.Headings,
		Aliases:      item.Aliases,
		Type:         item.NoteType,
		Status:       item.Status,
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
// When inMemory is true, GetAdapter hands out in-memory adapters (used by
// tests). The per-user adapter cache lets that fallback retain state across
// calls within a single test or Lambda invocation.
type Provider struct {
	client   DDBClient
	inMemory bool
	stores   map[string]*Adapter
	mu       sync.Mutex
}

// NewProvider returns a Provider whose per-user adapters are backed by
// DynamoDB via client. Passing a nil interface delegates to
// NewMemoryProvider (legacy path — new tests should call NewMemoryProvider
// directly). See NewAdapter's doc comment for the typed-nil caveat.
func NewProvider(client DDBClient) *Provider {
	if client == nil {
		return NewMemoryProvider()
	}
	return &Provider{
		client: client,
		stores: make(map[string]*Adapter),
	}
}

// NewMemoryProvider returns a Provider whose GetAdapter hands out in-memory
// adapters. Preferred over NewProvider(nil) in tests — same rationale as
// NewMemoryAdapter.
func NewMemoryProvider() *Provider {
	return &Provider{
		inMemory: true,
		stores:   make(map[string]*Adapter),
	}
}

// GetAdapter returns the per-user adapter, refreshing its baseFolderID from
// the caller's session every call so the value reflects the JWT.
func (p *Provider) GetAdapter(ctx context.Context, userID, baseFolderID string) (adapter.StorageAdapter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.stores[userID]; !ok {
		if p.inMemory {
			p.stores[userID] = NewMemoryAdapter(userID, baseFolderID)
		} else {
			p.stores[userID] = NewAdapter(p.client, userID, baseFolderID)
		}
	} else {
		p.stores[userID].BaseFolderID = baseFolderID
	}
	return p.stores[userID], nil
}

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

	if m.inMemory {
		return m.listStarredMap(ctx)
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
					Headings:     item.Headings,
					Aliases:      item.Aliases,
					Type:         item.NoteType,
					Status:       item.Status,
				})
			}
		}
	}
	return files, nil
}

func sortByViewedTimeDesc(files []adapter.FileMetadata) {
	sort.SliceStable(files, func(i, j int) bool {
		if !files[i].ViewedTime.Equal(files[j].ViewedTime) {
			return files[i].ViewedTime.After(files[j].ViewedTime)
		}
		return files[i].ID < files[j].ID
	})
}

// ListRecent lists recently viewed files, falling back to modified time for
// notes not opened since viewed_time was introduced.
func (m *Adapter) ListRecent(ctx context.Context, limit int) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.inMemory {
		return m.listRecentMap(ctx, limit)
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
		if item.MIMEType == "application/vnd.google-apps.folder" || !strings.HasSuffix(item.Name, ".md") {
			continue
		}
		if !m.isDescendant(item.Parents, targetFolderID, parentMap) {
			continue
		}
		viewedTime := item.ViewedTime
		if viewedTime.IsZero() {
			viewedTime = item.ModifiedTime
		}
		files = append(files, adapter.FileMetadata{
			ID:           item.ID,
			Name:         fromStoredName(item.Name),
			MIMEType:     item.MIMEType,
			ModifiedTime: item.ModifiedTime,
			ViewedTime:   viewedTime,
			Size:         item.Size,
			ETag:         item.ETag,
			Parents:      item.Parents,
			Starred:      item.Starred,
			Tags:         item.Tags,
			Headings:     item.Headings,
			Aliases:      item.Aliases,
			Type:         item.NoteType,
			Status:       item.Status,
		})
	}

	sortByViewedTimeDesc(files)

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
		if meta.ViewedTime.IsZero() {
			meta.ViewedTime = f.ModifiedTime
		}
		files = append(files, meta)
	}

	sortByViewedTimeDesc(files)

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
	return m.SearchFilesWithTags(ctx, query, nil, adapter.ScopeAll, "")
}

// SearchFilesWithTags searches by text query AND all provided tags (AND semantics),
// optionally restricted to a single note type.
// scope restricts the text match to title, headings, or all fields.
// Items with no stored tags/headings/type are checked via on-the-fly extraction (lazy fallback).
func (m *Adapter) SearchFilesWithTags(ctx context.Context, query string, tags []string, scope adapter.SearchScope, noteType string) ([]adapter.FileMetadata, error) {
	targetFolderID := "root"
	if m.BaseFolderID != "" {
		targetFolderID = m.BaseFolderID
	}

	if m.inMemory {
		return m.searchFilesWithTagsMap(ctx, query, tags, scope, noteType)
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

		if noteType != "" && !strings.EqualFold(itemNoteType(item.NoteType, item.Content), noteType) {
			continue
		}

		var snippet string
		var score int
		if query != "" {
			hit, sc, snip := scoreMatch(item.Name, itemAliases(item), item.Content, item.Headings, item.Tags, query, scope)
			if !hit {
				continue
			}
			score = sc
			snippet = snip
		}

		if len(tags) > 0 {
			itemTags := item.Tags
			if len(itemTags) == 0 && len(item.Content) > 0 {
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
			Headings:     item.Headings,
			Aliases:      item.Aliases,
			Type:         item.NoteType,
			Status:       item.Status,
			Score:        score,
			Snippet:      snippet,
		})
	}
	return files, nil
}

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
	if m.inMemory {
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
	if m.inMemory {
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

func (m *Adapter) searchFilesWithTagsMap(_ context.Context, query string, tags []string, scope adapter.SearchScope, noteType string) ([]adapter.FileMetadata, error) {
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

		if noteType != "" && !strings.EqualFold(itemNoteType(f.Type, f.Content), noteType) {
			continue
		}

		var snippet string
		var score int
		if query != "" {
			aliases := f.Aliases
			if len(aliases) == 0 && len(f.Content) > 0 {
				meta, _, _ := markdown.ParseNoteMeta(f.Content)
				aliases = meta.Aliases
			}
			hit, sc, snip := scoreMatch(f.Name, aliases, f.Content, f.Headings, f.Tags, query, scope)
			if !hit {
				continue
			}
			score = sc
			snippet = snip
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
		meta.Snippet = snippet
		meta.Score = score
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
			Aliases:      f.Aliases,
			NoteType:     f.Type,
			Status:       f.Status,
			Links:        f.Links,
		})
	}
	return items
}

func (m *Adapter) EnrichNoteLinks(ctx context.Context, noteID string, links []adapter.LinkRef) ([]adapter.LinkRef, []adapter.BacklinkEntry, error) {
	if m.inMemory {
		return m.enrichNoteLinksMap(ctx, noteID, links)
	}
	items, err := m.scanUserItems(ctx)
	if err != nil {
		return nil, nil, err
	}
	byID, titleToID, aliasToID := buildLookupMaps(items)
	enriched := enrichLinks(links, byID, titleToID, aliasToID)
	backs := backlinksOf(noteID, items, byID, titleToID, aliasToID)
	return enriched, backs, nil
}

func (m *Adapter) enrichNoteLinksMap(_ context.Context, noteID string, links []adapter.LinkRef) ([]adapter.LinkRef, []adapter.BacklinkEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.mapItems()
	byID, titleToID, aliasToID := buildLookupMaps(items)
	enriched := enrichLinks(links, byID, titleToID, aliasToID)
	backs := backlinksOf(noteID, items, byID, titleToID, aliasToID)
	return enriched, backs, nil
}

func (m *Adapter) Graph(ctx context.Context) ([]adapter.GraphNode, error) {
	if m.inMemory {
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
