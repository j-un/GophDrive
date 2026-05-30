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
//
// Race note: the GSI lookup is eventually consistent. Two simultaneous
// first-Issue calls from the same user could both see "no prior key" and
// attempt PutItem; ConditionExpression "attribute_not_exists(pk)" ensures only
// one succeeds. The regenerate path uses TransactWriteItems so Delete(old) and
// Put(new) are atomic. A narrow window remains where two simultaneous regenerate
// calls both read the same oldPK from the GSI and race on the TransactWrite—one
// will fail with TransactionCanceledException and the caller should surface a 5xx.
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

// keyItem is the DynamoDB row shape.
type keyItem struct {
	PK            string `dynamodbav:"pk"`
	UserID        string `dynamodbav:"user_id"`
	BaseFolderID  string `dynamodbav:"base_folder_id"`
	CreatedAt     int64  `dynamodbav:"created_at"`
	FirstIssuedAt int64  `dynamodbav:"first_issued_at,omitempty"`
	KeyPrefix     string `dynamodbav:"key_prefix"`
}

// DDBStore implements Store against DynamoDB.
// Table schema:
//   - pk (S): sha256(plaintext) — partition key
//   - user_id (S): Google sub — GSI "user_id-index" hash key (KEYS_ONLY)
//   - base_folder_id, created_at, first_issued_at, key_prefix: plain attributes
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
	oldPK, err := s.queryUserPK(ctx, userID)
	if err != nil {
		return fmt.Errorf("apikey Issue: query existing key: %w", err)
	}

	now := time.Now().Unix()
	item := keyItem{
		PK:           hash,
		UserID:       userID,
		BaseFolderID: baseFolderID,
		CreatedAt:    now,
		KeyPrefix:    prefix,
	}

	if oldPK == "" {
		// First issue: record the original creation time and guard against
		// simultaneous clicks with a conditional write.
		item.FirstIssuedAt = now
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("apikey Issue: marshal: %w", err)
		}
		_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(s.table),
			Item:                av,
			ConditionExpression: aws.String("attribute_not_exists(pk)"),
		})
		return err
	}

	// Regenerate: preserve the original first_issued_at from the old row.
	oldOut, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: oldPK}},
	})
	if err != nil {
		return fmt.Errorf("apikey Issue: fetch old item: %w", err)
	}
	if oldOut.Item != nil {
		var oldItem keyItem
		if merr := attributevalue.UnmarshalMap(oldOut.Item, &oldItem); merr == nil && oldItem.FirstIssuedAt != 0 {
			item.FirstIssuedAt = oldItem.FirstIssuedAt
		}
	}
	if item.FirstIssuedAt == 0 {
		item.FirstIssuedAt = now // fallback for rows written before this column existed
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("apikey Issue: marshal: %w", err)
	}

	// Atomically delete the old hash and write the new one.
	_, err = s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Delete: &types.Delete{
					TableName:           aws.String(s.table),
					Key:                 map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: oldPK}},
					ConditionExpression: aws.String("attribute_exists(pk)"),
				},
			},
			{
				Put: &types.Put{
					TableName:           aws.String(s.table),
					Item:                av,
					ConditionExpression: aws.String("attribute_not_exists(pk)"),
				},
			},
		},
	})
	return err
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
	pk, err := s.queryUserPK(ctx, userID)
	if err != nil {
		return false, "", 0, 0, err
	}
	if pk == "" {
		return false, "", 0, 0, nil
	}

	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return false, "", 0, 0, err
	}
	if out.Item == nil {
		return false, "", 0, 0, nil
	}
	var item keyItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return false, "", 0, 0, err
	}
	return true, item.KeyPrefix, item.CreatedAt, item.FirstIssuedAt, nil
}

func (s *DDBStore) Revoke(ctx context.Context, userID string) error {
	pk, err := s.queryUserPK(ctx, userID)
	if err != nil {
		return fmt.Errorf("apikey Revoke: query existing key: %w", err)
	}
	if pk == "" {
		return nil // no-op
	}
	_, err = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	return err
}

// queryUserPK queries the user_id-index GSI and returns the pk for the user,
// or "" if no key exists.
func (s *DDBStore) queryUserPK(ctx context.Context, userID string) (string, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		IndexName:              aws.String("user_id-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return "", err
	}
	if len(out.Items) == 0 {
		return "", nil
	}
	pkAttr, ok := out.Items[0]["pk"]
	if !ok {
		return "", nil
	}
	pkVal, ok := pkAttr.(*types.AttributeValueMemberS)
	if !ok {
		return "", nil
	}
	return pkVal.Value, nil
}

// InMemoryStore is a thread-safe in-memory Store for tests.
type InMemoryStore struct {
	mu     sync.RWMutex
	byHash map[string]*memRecord
	byUser map[string]string // userID → hash
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
			firstIssuedAt = prev.firstIssuedAt // preserve original creation time across regenerate
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
