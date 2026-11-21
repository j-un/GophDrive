// Replay the lost-update counterexample against Adapter.SaveFile.
//
// Exit 0: both SaveFile calls returned success and one body is gone.
// Exit 1: that did not happen. The DynamoDB path may have been fixed, or this
// harness no longer drives SaveFile the way the model describes.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
)

// gated blocks the second step of SaveFile until both calls have finished
// GetFile, so each compare runs against the same snapshot.
type gated struct {
	mu    sync.Mutex
	items map[string]map[string]types.AttributeValue
	armed bool
	gets  int
	gate  chan struct{}
	saves int
	bare  int
}

func newGated() *gated {
	return &gated{
		items: map[string]map[string]types.AttributeValue{},
		gate:  make(chan struct{}),
	}
}

func (g *gated) arm() { g.armed = true }

func pkOf(item map[string]types.AttributeValue) string {
	v, ok := item["pk"]
	if !ok {
		return ""
	}
	s, ok := v.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return s.Value
}

func copyItem(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	out := make(map[string]types.AttributeValue, len(item))
	for k, v := range item {
		out[k] = v
	}
	return out
}

func (g *gated) GetItem(_ context.Context, params *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	g.mu.Lock()
	pk := ""
	if v, ok := params.Key["pk"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			pk = s.Value
		}
	}
	var item map[string]types.AttributeValue
	if found, ok := g.items[pk]; ok {
		item = copyItem(found)
	}
	wait := false
	if g.armed {
		g.gets++
		if g.gets == 2 {
			close(g.gate)
		}
		wait = g.gets <= 2
	}
	g.mu.Unlock()

	if wait {
		select {
		case <-g.gate:
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("GetItem waited for the other SaveFile and timed out")
		}
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (g *gated) PutItem(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	pk := pkOf(params.Item)
	if pk == "" {
		return nil, fmt.Errorf("PutItem missing pk")
	}
	if g.armed {
		g.saves++
		if params.ConditionExpression == nil {
			g.bare++
		}
	}
	g.items[pk] = copyItem(params.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func (g *gated) UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return nil, fmt.Errorf("UpdateItem is not part of this SaveFile replay")
}

func (g *gated) DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return nil, fmt.Errorf("DeleteItem is not part of this SaveFile replay")
}

func (g *gated) Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	return &dynamodb.ScanOutput{}, nil
}

func main() {
	ctx := context.Background()
	store := newGated()
	notes := dynamo.NewAdapter(store, "user-1", "")

	created, err := notes.CreateFile(ctx, "note.md", []byte("original"), "root")
	if err != nil {
		fmt.Fprintf(os.Stderr, "CreateFile: %v\n", err)
		os.Exit(1)
	}
	store.arm()

	bodies := [][]byte{[]byte("from-writer-1"), []byte("from-writer-2")}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = notes.SaveFile(ctx, created.ID, bodies[i], created.ETag)
		}(i)
	}
	wg.Wait()

	if errs[0] != nil || errs[1] != nil {
		fmt.Printf("bug did not reproduce: SaveFile errors %v and %v\n", errs[0], errs[1])
		os.Exit(1)
	}
	if store.saves != 2 || store.bare != 2 {
		fmt.Printf("bug did not reproduce: saves=%d unconditional puts=%d\n", store.saves, store.bare)
		os.Exit(1)
	}

	got, err := notes.GetFile(ctx, created.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetFile: %v\n", err)
		os.Exit(1)
	}
	stored := string(got.Content)
	lost := ""
	switch stored {
	case string(bodies[0]):
		lost = string(bodies[1])
	case string(bodies[1]):
		lost = string(bodies[0])
	default:
		fmt.Printf("bug did not reproduce: stored %q\n", stored)
		os.Exit(1)
	}
	fmt.Printf("lost update reproduced: both SaveFile calls returned success, stored %q, %q is gone\n", stored, lost)
}
