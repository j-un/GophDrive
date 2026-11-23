package apikey_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"

	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const issue220TablePrefix = "APIKeyHashesIssue220"

const issue220JWTSecret = "issue220-test-secret"

func dialDynamoLocal(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:8000", 500*time.Millisecond)
	if err != nil {
		t.Skipf("DynamoDB Local is not accepting connections on 127.0.0.1:8000: %v", err)
	}
	conn.Close()
}

func newIssue220DDB(t *testing.T) (*apikey.DDBStore, *dynamodb.Client, string) {
	t.Helper()
	dialDynamoLocal(t)

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
	)
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://127.0.0.1:8000")
	})

	name := fmt.Sprintf("%s_%s_%d", issue220TablePrefix, t.Name(), time.Now().UnixNano())
	if name == "APIKeyHashes" || !strings.HasPrefix(name, issue220TablePrefix) {
		t.Fatalf("refusing table %q", name)
	}

	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(name),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{{
			IndexName: aws.String("user_id-index"),
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
			},
			Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly},
		}},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("CreateTable %s: %v", name, err)
	}
	t.Cleanup(func() {
		_, delErr := client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(name),
		})
		if delErr != nil {
			t.Errorf("DeleteTable %s: %v", name, delErr)
		}
	})
	if err := waitIssue220Table(ctx, client, name); err != nil {
		t.Fatal(err)
	}
	return apikey.NewDDBStore(client, name), client, name
}

func waitIssue220Table(ctx context.Context, client *dynamodb.Client, name string) error {
	deadline := time.Now().Add(20 * time.Second)
	for {
		out, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(name)})
		if err == nil && out.Table != nil && out.Table.TableStatus == types.TableStatusActive && len(out.Table.GlobalSecondaryIndexes) == 1 {
			ready := true
			for _, gsi := range out.Table.GlobalSecondaryIndexes {
				if gsi.IndexStatus != types.IndexStatusActive {
					ready = false
				}
			}
			if ready {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("table %s not active: %v", name, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func userIndexCount(ctx context.Context, client *dynamodb.Client, table, userID string) (int, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		IndexName:              aws.String("user_id-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, err
	}
	return int(out.Count), nil
}

func mintIssue220Session(t *testing.T, userID, folder string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            userID,
		"base_folder_id": folder,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(issue220JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestDDBStore_TwoFirstIssuesOneHash(t *testing.T) {
	store, _, _ := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-two-first-issues"
	hashes := []string{apikey.HashKey("first-key-a"), apikey.HashKey("first-key-b")}

	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(len(hashes))
	errs := make([]error, len(hashes))
	var wg sync.WaitGroup
	wg.Add(len(hashes))
	for i, hash := range hashes {
		go func(i int, hash string) {
			defer wg.Done()
			ready.Done()
			<-start
			errs[i] = store.Issue(ctx, userID, "folder-1", hash, "prefix12")
		}(i, hash)
	}
	ready.Wait()
	close(start)
	wg.Wait()

	oks := make([]bool, len(hashes))
	for i, hash := range hashes {
		_, _, ok, err := store.Lookup(ctx, hash)
		if err != nil {
			t.Fatalf("Lookup %d: %v", i, err)
		}
		oks[i] = ok
	}

	live := 0
	for i, ok := range oks {
		if ok {
			live++
			if errs[i] != nil {
				t.Errorf("live hash %d Issue returned %v", i, errs[i])
			}
			continue
		}
		if errs[i] == nil {
			t.Errorf("losing Issue %d returned nil", i)
		}
	}
	if live != 1 {
		t.Fatalf("want one live hash, got %d lookups=%v errs=%v", live, oks, errs)
	}
}

func TestDDBStore_RevokeWhenIndexEmpty(t *testing.T) {
	store, client, table := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-revoke-empty-index"
	const folderID = "folder-revoke"
	hash := apikey.HashKey("revoke-plaintext")
	if err := store.Issue(ctx, userID, folderID, hash, "revoke-p"); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	updated, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: hash},
		},
		UpdateExpression: aws.String("REMOVE user_id"),
		ReturnValues:     types.ReturnValueAllNew,
	})
	if err != nil {
		t.Fatalf("REMOVE user_id: %v", err)
	}
	if _, still := updated.Attributes["user_id"]; still {
		t.Fatal("arrangement: user_id still present on the base item")
	}

	indexCount, err := userIndexCount(ctx, client, table, userID)
	if err != nil {
		t.Fatalf("query user_id-index: %v", err)
	}
	base, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: hash},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	t.Logf("after REMOVE user_id: index_count=%d base_item=%v", indexCount, base.Item != nil)
	if indexCount != 0 || base.Item == nil {
		t.Fatalf("arrangement failed: index_count=%d base_item=%v", indexCount, base.Item != nil)
	}

	if err := store.Revoke(ctx, userID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	h := handler.NewAPIKeyHandler(store, issue220JWTSecret)
	resp, err := h.Delete(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "DELETE",
		Path:       "/api-keys",
		Headers:    map[string]string{"Cookie": "session_token=" + mintIssue220Session(t, userID, folderID)},
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, ok, err := store.Lookup(ctx, hash)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent || ok {
		t.Fatalf("Delete status=%d lookup_ok=%v, want 204 and lookup not ok", resp.StatusCode, ok)
	}
}

func TestDDBStore_RegeneratePreservesFirstIssuedAt(t *testing.T) {
	store, _, _ := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-regenerate"
	h1 := apikey.HashKey("key-v1")
	h2 := apikey.HashKey("key-v2")
	if err := store.Issue(ctx, userID, "folder-1", h1, "key-v1_"); err != nil {
		t.Fatalf("first Issue: %v", err)
	}
	_, _, _, firstIssuedAt, err := store.StatusFor(ctx, userID)
	if err != nil {
		t.Fatalf("StatusFor: %v", err)
	}
	if firstIssuedAt == 0 {
		t.Fatal("first_issued_at is 0 after first Issue")
	}

	// first_issued_at is unix seconds. Wait until the clock ticks so a rewrite to now is visible.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Unix() <= firstIssuedAt {
		if time.Now().After(deadline) {
			t.Fatal("clock did not advance past first_issued_at")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := store.Issue(ctx, userID, "folder-1", h2, "key-v2_"); err != nil {
		t.Fatalf("second Issue: %v", err)
	}

	_, _, ok1, err := store.Lookup(ctx, h1)
	if err != nil {
		t.Fatalf("Lookup old: %v", err)
	}
	_, _, ok2, err := store.Lookup(ctx, h2)
	if err != nil {
		t.Fatalf("Lookup new: %v", err)
	}
	if ok1 || !ok2 {
		t.Fatalf("want a single hash, old_ok=%v new_ok=%v", ok1, ok2)
	}
	_, _, _, firstAgain, err := store.StatusFor(ctx, userID)
	if err != nil {
		t.Fatalf("StatusFor after regenerate: %v", err)
	}
	if firstAgain != firstIssuedAt {
		t.Fatalf("first_issued_at changed from %d to %d", firstIssuedAt, firstAgain)
	}
}

func putHashOnly(t *testing.T, client *dynamodb.Client, table, userID, folder, hash, prefix string, firstIssuedAt int64) {
	t.Helper()
	_, err := client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]types.AttributeValue{
			"pk":              &types.AttributeValueMemberS{Value: hash},
			"user_id":         &types.AttributeValueMemberS{Value: userID},
			"base_folder_id":  &types.AttributeValueMemberS{Value: folder},
			"created_at":      &types.AttributeValueMemberN{Value: strconv.FormatInt(firstIssuedAt, 10)},
			"first_issued_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(firstIssuedAt, 10)},
			"key_prefix":      &types.AttributeValueMemberS{Value: prefix},
		},
	})
	if err != nil {
		t.Fatalf("PutItem %s: %v", hash, err)
	}
}

func TestDDBStore_RevokeDeletesEveryHashOnlyRow(t *testing.T) {
	store, client, table := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-two-hash-only"
	const folderID = "folder-hash-only"
	h1 := apikey.HashKey("legacy-a")
	h2 := apikey.HashKey("legacy-b")
	putHashOnly(t, client, table, userID, folderID, h1, "legacy-a", 1_000)
	putHashOnly(t, client, table, userID, folderID, h2, "legacy-b", 1_001)

	if err := store.Revoke(ctx, userID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	for i, hash := range []string{h1, h2} {
		_, _, ok, err := store.Lookup(ctx, hash)
		if err != nil {
			t.Fatalf("Lookup %d: %v", i, err)
		}
		if ok {
			t.Errorf("hash %d still live after Revoke", i)
		}
	}
	if err := store.Revoke(ctx, userID); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
}

func TestDDBStore_IssueReplacesHashOnlyRow(t *testing.T) {
	store, client, table := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-replace-hash-only"
	const folderID = "folder-replace"
	oldHash := apikey.HashKey("legacy-live")
	newHash := apikey.HashKey("replacement")
	const wantFirst int64 = 1_700_000_000
	putHashOnly(t, client, table, userID, folderID, oldHash, "legacy-l", wantFirst)

	if err := store.Issue(ctx, userID, folderID, newHash, "replacem"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	_, _, oldOK, err := store.Lookup(ctx, oldHash)
	if err != nil {
		t.Fatalf("Lookup old: %v", err)
	}
	_, _, newOK, err := store.Lookup(ctx, newHash)
	if err != nil {
		t.Fatalf("Lookup new: %v", err)
	}
	if oldOK || !newOK {
		t.Fatalf("want the new hash only, old_ok=%v new_ok=%v", oldOK, newOK)
	}
	_, _, _, firstIssuedAt, err := store.StatusFor(ctx, userID)
	if err != nil {
		t.Fatalf("StatusFor: %v", err)
	}
	if firstIssuedAt != wantFirst {
		t.Fatalf("first_issued_at=%d, want %d", firstIssuedAt, wantFirst)
	}
}

func TestDDBStore_IssueLeavesOneHashWhenTwoHashOnlyRowsExist(t *testing.T) {
	store, client, table := newIssue220DDB(t)
	ctx := context.Background()
	const userID = "user-two-legacy-issue"
	const folderID = "folder-two-legacy"
	h1 := apikey.HashKey("legacy-1")
	h2 := apikey.HashKey("legacy-2")
	newHash := apikey.HashKey("legacy-new")
	putHashOnly(t, client, table, userID, folderID, h1, "legacy-1", 1_000)
	putHashOnly(t, client, table, userID, folderID, h2, "legacy-2", 2_000)

	if err := store.Issue(ctx, userID, folderID, newHash, "legacy-n"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	want := map[string]bool{h1: false, h2: false, newHash: true}
	for hash, wantOK := range want {
		_, _, ok, err := store.Lookup(ctx, hash)
		if err != nil {
			t.Fatalf("Lookup %s: %v", hash, err)
		}
		if ok != wantOK {
			t.Errorf("Lookup %s ok=%v, want %v", hash, ok, wantOK)
		}
	}
	_, _, _, firstIssuedAt, err := store.StatusFor(ctx, userID)
	if err != nil {
		t.Fatalf("StatusFor: %v", err)
	}
	if firstIssuedAt != 1_000 {
		t.Fatalf("first_issued_at=%d, want 1000", firstIssuedAt)
	}
}

func TestDDBStore_RevokeWithNoKey(t *testing.T) {
	store, _, _ := newIssue220DDB(t)
	if err := store.Revoke(context.Background(), "user-never-issued"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
}
