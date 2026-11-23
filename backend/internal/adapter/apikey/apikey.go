package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Store manages hashed API keys. The plaintext key is never persisted;
// only its SHA-256 hex is stored as the partition key.
type Store interface {
	// Issue creates (or atomically replaces) the caller's API key. hash is
	// sha256(plaintext), prefix is the first 8 chars of plaintext for UI display.
	Issue(ctx context.Context, userID, baseFolderID, hash, prefix string) error

	// Lookup returns the userID and baseFolderID for a given key hash.
	Lookup(ctx context.Context, hash string) (userID, baseFolderID string, ok bool, err error)

	// StatusFor returns current key metadata for a user without exposing the hash.
	// firstIssuedAt is the original creation epoch, preserved across Regenerate.
	// createdAt reflects the last Issue time.
	StatusFor(ctx context.Context, userID string) (hasKey bool, prefix string, createdAt, firstIssuedAt int64, err error)

	// Revoke deletes the user's key. No-op if the user has no key.
	Revoke(ctx context.Context, userID string) error
}

// HashKey returns the hex-encoded SHA-256 of plaintext. Callers should use
// this to produce consistent hashes for both Issue and Lookup.
func HashKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

type keyItem struct {
	PK            string `dynamodbav:"pk"`
	UserID        string `dynamodbav:"user_id"`
	BaseFolderID  string `dynamodbav:"base_folder_id"`
	CreatedAt     int64  `dynamodbav:"created_at"`
	FirstIssuedAt int64  `dynamodbav:"first_issued_at,omitempty"`
	KeyPrefix     string `dynamodbav:"key_prefix"`
}

// userItem has no user_id attribute, so the GSI does not project this row as a key.
type userItem struct {
	PK       string `dynamodbav:"pk"`
	LiveHash string `dynamodbav:"live_hash"`
}

// DDBStore implements Store against DynamoDB.
type DDBStore struct {
	client *dynamodb.Client
	table  string
}

// NewDDBStore returns a DDBStore. table defaults to the API_KEY_HASHES_TABLE
// env var, then "APIKeyHashes".
func NewDDBStore(client *dynamodb.Client, table string) *DDBStore {
	if table == "" {
		table = os.Getenv("API_KEY_HASHES_TABLE")
	}
	if table == "" {
		table = "APIKeyHashes"
	}
	return &DDBStore{client: client, table: table}
}

func (s *DDBStore) Issue(ctx context.Context, userID, baseFolderID, hash, prefix string) error {
	st, err := s.resolve(ctx, userID)
	if err != nil {
		return fmt.Errorf("apikey Issue: resolve: %w", err)
	}

	now := time.Now().Unix()
	item := keyItem{
		PK:            hash,
		UserID:        userID,
		BaseFolderID:  baseFolderID,
		CreatedAt:     now,
		FirstIssuedAt: now,
		KeyPrefix:     prefix,
	}
	old := st.oldHashes()
	if len(old) > 0 {
		prev, ok, err := s.oldestIssued(ctx, old)
		if err != nil {
			return fmt.Errorf("apikey Issue: read old keys: %w", err)
		}
		if ok {
			item.FirstIssuedAt = prev.FirstIssuedAt
		}
	}

	lockPK := userItemPK(userID)
	ops := newWriteSet(len(old) + 2)
	if st.hasUser {
		ops.add(lockPK, s.updateLiveHash(lockPK, st.liveHash, hash))
	} else {
		put, err := s.putAbsent(userItem{PK: lockPK, LiveHash: hash})
		if err != nil {
			return fmt.Errorf("apikey Issue: marshal user: %w", err)
		}
		ops.add(lockPK, put)
	}

	kept := false
	for _, pk := range old {
		if pk == hash {
			kept = true
			continue
		}
		ops.add(pk, s.deletePK(pk))
	}
	if !kept {
		put, err := s.putAbsent(item)
		if err != nil {
			return fmt.Errorf("apikey Issue: marshal: %w", err)
		}
		ops.add(hash, put)
	}

	_, err = s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: ops.items,
	})
	if err != nil {
		return fmt.Errorf("apikey Issue: %w", err)
	}
	return nil
}

func (s *DDBStore) Lookup(ctx context.Context, hash string) (userID, baseFolderID string, ok bool, err error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: hash},
		},
	})
	if err != nil {
		return "", "", false, err
	}
	if out.Item == nil {
		return "", "", false, nil
	}
	var item keyItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return "", "", false, err
	}
	return item.UserID, item.BaseFolderID, true, nil
}

func (s *DDBStore) StatusFor(ctx context.Context, userID string) (hasKey bool, prefix string, createdAt, firstIssuedAt int64, err error) {
	st, err := s.resolve(ctx, userID)
	if err != nil {
		return false, "", 0, 0, err
	}
	if st.liveHash != "" {
		item, ok, err := s.getHash(ctx, st.liveHash)
		if err != nil {
			return false, "", 0, 0, err
		}
		if ok {
			return true, item.KeyPrefix, item.CreatedAt, item.FirstIssuedAt, nil
		}
	}
	if len(st.indexPKs) == 0 {
		return false, "", 0, 0, nil
	}
	item, ok, err := s.oldestIssued(ctx, st.indexPKs)
	if err != nil || !ok {
		return false, "", 0, 0, err
	}
	return true, item.KeyPrefix, item.CreatedAt, item.FirstIssuedAt, nil
}

func (s *DDBStore) Revoke(ctx context.Context, userID string) error {
	st, err := s.resolve(ctx, userID)
	if err != nil {
		return fmt.Errorf("apikey Revoke: resolve: %w", err)
	}
	ops := newWriteSet(len(st.indexPKs) + 2)
	if st.hasUser {
		ops.add(userItemPK(userID), s.deletePK(userItemPK(userID)))
	}
	for _, pk := range st.oldHashes() {
		ops.add(pk, s.deletePK(pk))
	}
	if len(ops.items) == 0 {
		return nil
	}
	_, err = s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: ops.items,
	})
	if err != nil {
		return fmt.Errorf("apikey Revoke: %w", err)
	}
	return nil
}

type resolved struct {
	hasUser  bool
	liveHash string
	indexPKs []string
}

func (r resolved) oldHashes() []string {
	pks := make([]string, 0, len(r.indexPKs)+1)
	if r.liveHash != "" {
		pks = append(pks, r.liveHash)
	}
	pks = append(pks, r.indexPKs...)
	return dedup(pks)
}

func (s *DDBStore) resolve(ctx context.Context, userID string) (resolved, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            pkKey(userItemPK(userID)),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return resolved{}, err
	}
	var st resolved
	if out.Item != nil {
		st.hasUser = true
		if av, ok := out.Item["live_hash"].(*types.AttributeValueMemberS); ok {
			st.liveHash = av.Value
		}
	}
	pks, err := s.queryIndexPKs(ctx, userID)
	if err != nil {
		return resolved{}, err
	}
	st.indexPKs = pks
	return st, nil
}

func (s *DDBStore) queryIndexPKs(ctx context.Context, userID string) ([]string, error) {
	var pks []string
	var start map[string]types.AttributeValue
	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			IndexName:              aws.String("user_id-index"),
			KeyConditionExpression: aws.String("user_id = :uid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":uid": &types.AttributeValueMemberS{Value: userID},
			},
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range out.Items {
			pk, ok := item["pk"].(*types.AttributeValueMemberS)
			if !ok || pk.Value == "" {
				continue
			}
			pks = append(pks, pk.Value)
		}
		if len(out.LastEvaluatedKey) == 0 {
			return dedup(pks), nil
		}
		start = out.LastEvaluatedKey
	}
}

func (s *DDBStore) getHash(ctx context.Context, pk string) (keyItem, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            pkKey(pk),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return keyItem{}, false, err
	}
	if out.Item == nil {
		return keyItem{}, false, nil
	}
	var item keyItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return keyItem{}, false, err
	}
	return item, true, nil
}

func (s *DDBStore) oldestIssued(ctx context.Context, pks []string) (keyItem, bool, error) {
	var best keyItem
	var found bool
	for _, pk := range pks {
		item, ok, err := s.getHash(ctx, pk)
		if err != nil {
			return keyItem{}, false, err
		}
		if !ok || item.FirstIssuedAt == 0 {
			continue
		}
		if !found || item.FirstIssuedAt < best.FirstIssuedAt {
			best = item
			found = true
		}
	}
	return best, found, nil
}

func (s *DDBStore) putAbsent(item any) (types.TransactWriteItem, error) {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName:           aws.String(s.table),
			Item:                av,
			ConditionExpression: aws.String("attribute_not_exists(pk)"),
		},
	}, nil
}

func (s *DDBStore) deletePK(pk string) types.TransactWriteItem {
	return types.TransactWriteItem{
		Delete: &types.Delete{
			TableName: aws.String(s.table),
			Key:       pkKey(pk),
		},
	}
}

func (s *DDBStore) updateLiveHash(lockPK, prev, next string) types.TransactWriteItem {
	values := map[string]types.AttributeValue{
		":next": &types.AttributeValueMemberS{Value: next},
	}
	cond := "attribute_not_exists(live_hash)"
	if prev != "" {
		cond = "live_hash = :prev"
		values[":prev"] = &types.AttributeValueMemberS{Value: prev}
	}
	return types.TransactWriteItem{
		Update: &types.Update{
			TableName:                 aws.String(s.table),
			Key:                       pkKey(lockPK),
			UpdateExpression:          aws.String("SET live_hash = :next"),
			ConditionExpression:       aws.String(cond),
			ExpressionAttributeValues: values,
		},
	}
}

// writeSet keeps one operation per item. DynamoDB rejects two operations on
// the same item in one transaction.
type writeSet struct {
	items []types.TransactWriteItem
	seen  map[string]struct{}
}

func newWriteSet(n int) writeSet {
	return writeSet{
		items: make([]types.TransactWriteItem, 0, n),
		seen:  make(map[string]struct{}, n),
	}
}

func (w *writeSet) add(pk string, item types.TransactWriteItem) {
	if pk == "" {
		return
	}
	if _, ok := w.seen[pk]; ok {
		return
	}
	w.seen[pk] = struct{}{}
	w.items = append(w.items, item)
}

func userItemPK(userID string) string {
	return "user#" + userID
}

func pkKey(pk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
	}
}

func dedup(pks []string) []string {
	seen := make(map[string]struct{}, len(pks))
	out := make([]string, 0, len(pks))
	for _, pk := range pks {
		if pk == "" {
			continue
		}
		if _, ok := seen[pk]; ok {
			continue
		}
		seen[pk] = struct{}{}
		out = append(out, pk)
	}
	return out
}

// InMemoryStore is a thread-safe in-memory Store for tests.
type InMemoryStore struct {
	mu     sync.RWMutex
	byHash map[string]*memRecord
	byUser map[string]string
}

type memRecord struct {
	userID, baseFolderID, hash, prefix string
	createdAt, firstIssuedAt           int64
}

// NewInMemoryStore returns an empty InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		byHash: make(map[string]*memRecord),
		byUser: make(map[string]string),
	}
}

func (s *InMemoryStore) Issue(_ context.Context, userID, baseFolderID, hash, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	firstIssuedAt := now
	if old, ok := s.byUser[userID]; ok {
		if prev := s.byHash[old]; prev != nil && prev.firstIssuedAt != 0 {
			firstIssuedAt = prev.firstIssuedAt
		}
		delete(s.byHash, old)
	}
	r := &memRecord{
		userID:        userID,
		baseFolderID:  baseFolderID,
		hash:          hash,
		prefix:        prefix,
		createdAt:     now,
		firstIssuedAt: firstIssuedAt,
	}
	s.byHash[hash] = r
	s.byUser[userID] = hash
	return nil
}

func (s *InMemoryStore) Lookup(_ context.Context, hash string) (string, string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byHash[hash]
	if !ok {
		return "", "", false, nil
	}
	return r.userID, r.baseFolderID, true, nil
}

func (s *InMemoryStore) StatusFor(_ context.Context, userID string) (bool, string, int64, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hash, ok := s.byUser[userID]
	if !ok {
		return false, "", 0, 0, nil
	}
	r := s.byHash[hash]
	return true, r.prefix, r.createdAt, r.firstIssuedAt, nil
}

func (s *InMemoryStore) Revoke(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, ok := s.byUser[userID]
	if !ok {
		return nil
	}
	delete(s.byHash, hash)
	delete(s.byUser, userID)
	return nil
}
