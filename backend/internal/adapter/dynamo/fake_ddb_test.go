package dynamo

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDDB is a shared in-memory DynamoDB table. Multiple Adapters (different
// userIDs) constructed over one fakeDDB share rows, making cross-tenant
// access expressible — the per-Adapter map fallback cannot do this.
type fakeDDB struct {
	mu    sync.Mutex
	items map[string]map[string]types.AttributeValue // pk -> raw marshaled item
}

func newFakeDDB() *fakeDDB {
	return &fakeDDB{items: make(map[string]map[string]types.AttributeValue)}
}

// pkOf extracts the pk string from a marshaled item map. Returns "" if absent
// or not a string attribute; callers treat "" as a programmer error.
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

// stringAttr reads a string attribute, returning "" if missing or wrong type.
func stringAttr(item map[string]types.AttributeValue, name string) string {
	v, ok := item[name]
	if !ok {
		return ""
	}
	s, ok := v.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return s.Value
}

// shallowCopy returns a new outer map sharing the same AttributeValue pointers.
// Sufficient because the adapter never mutates values in place after read.
func shallowCopy(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	out := make(map[string]types.AttributeValue, len(item))
	for k, v := range item {
		out[k] = v
	}
	return out
}

// ccfe returns the bare typed ConditionalCheckFailedException the adapter
// errors.As-matches against.
func ccfe() error {
	return &types.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}
}

// evalCondition evaluates the single condition expression used by the adapter:
// "attribute_exists(pk) AND user_id = :uid". Any other expression fails loudly.
// A nil condExpr returns true (unconditional).
func (f *fakeDDB) evalCondition(condExpr *string, values map[string]types.AttributeValue, pk string) (bool, error) {
	if condExpr == nil {
		return true, nil
	}
	if *condExpr != "attribute_exists(pk) AND user_id = :uid" {
		return false, fmt.Errorf("fakeDDB: unsupported expression %q", *condExpr)
	}
	item, ok := f.items[pk]
	if !ok {
		return false, nil
	}
	uidAV, ok := values[":uid"]
	if !ok {
		return false, fmt.Errorf("fakeDDB: condition references :uid but none provided")
	}
	uidS, ok := uidAV.(*types.AttributeValueMemberS)
	if !ok {
		return false, fmt.Errorf("fakeDDB: :uid must be a string attribute")
	}
	return stringAttr(item, "user_id") == uidS.Value, nil
}

func (f *fakeDDB) GetItem(_ context.Context, params *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pk := ""
	if v, ok := params.Key["pk"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			pk = s.Value
		}
	}
	item, ok := f.items[pk]
	if !ok {
		// Adapter maps nil Item to ErrNotFound; do NOT return an error here.
		return &dynamodb.GetItemOutput{Item: nil}, nil
	}
	return &dynamodb.GetItemOutput{Item: shallowCopy(item)}, nil
}

func (f *fakeDDB) PutItem(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pk := pkOf(params.Item)
	if pk == "" {
		return nil, fmt.Errorf("fakeDDB: PutItem missing pk")
	}
	ok, err := f.evalCondition(params.ConditionExpression, params.ExpressionAttributeValues, pk)
	if err != nil {
		return nil, err
	}
	if !ok && params.ConditionExpression != nil {
		return nil, ccfe()
	}
	f.items[pk] = shallowCopy(params.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDDB) UpdateItem(_ context.Context, params *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pk := ""
	if v, ok := params.Key["pk"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			pk = s.Value
		}
	}
	// Condition is always present from TouchViewed; missing item means the
	// condition fails (no upsert emulation).
	pass, err := f.evalCondition(params.ConditionExpression, params.ExpressionAttributeValues, pk)
	if err != nil {
		return nil, err
	}
	if !pass {
		return nil, ccfe()
	}

	if params.UpdateExpression == nil {
		return nil, fmt.Errorf("fakeDDB: UpdateItem requires UpdateExpression")
	}
	expr := *params.UpdateExpression
	item := f.items[pk] // present because evalCondition passed

	switch expr {
	case "SET viewed_time = :v":
		v, ok := params.ExpressionAttributeValues[":v"]
		if !ok {
			return nil, fmt.Errorf("fakeDDB: UpdateItem missing :v")
		}
		item["viewed_time"] = v
	case "SET viewed_time = :v, #ttl = :t":
		v, ok := params.ExpressionAttributeValues[":v"]
		if !ok {
			return nil, fmt.Errorf("fakeDDB: UpdateItem missing :v")
		}
		t, ok := params.ExpressionAttributeValues[":t"]
		if !ok {
			return nil, fmt.Errorf("fakeDDB: UpdateItem missing :t")
		}
		if got := params.ExpressionAttributeNames["#ttl"]; got != "ttl" {
			return nil, fmt.Errorf("fakeDDB: expected #ttl -> ttl, got %q", got)
		}
		item["viewed_time"] = v
		item["ttl"] = t
	default:
		return nil, fmt.Errorf("fakeDDB: unsupported expression %q", expr)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *fakeDDB) DeleteItem(_ context.Context, params *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pk := ""
	if v, ok := params.Key["pk"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			pk = s.Value
		}
	}
	pass, err := f.evalCondition(params.ConditionExpression, params.ExpressionAttributeValues, pk)
	if err != nil {
		return nil, err
	}
	if !pass && params.ConditionExpression != nil {
		return nil, ccfe()
	}
	delete(f.items, pk)
	return &dynamodb.DeleteItemOutput{}, nil
}

func (f *fakeDDB) Scan(_ context.Context, params *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if params.FilterExpression == nil || *params.FilterExpression != "user_id = :uid" {
		expr := ""
		if params.FilterExpression != nil {
			expr = *params.FilterExpression
		}
		return nil, fmt.Errorf("fakeDDB: unsupported expression %q", expr)
	}
	uidAV, ok := params.ExpressionAttributeValues[":uid"]
	if !ok {
		return nil, fmt.Errorf("fakeDDB: Scan missing :uid")
	}
	uidS, ok := uidAV.(*types.AttributeValueMemberS)
	if !ok {
		return nil, fmt.Errorf("fakeDDB: :uid must be a string attribute")
	}
	var matched []map[string]types.AttributeValue
	for _, item := range f.items {
		if stringAttr(item, "user_id") == uidS.Value {
			matched = append(matched, shallowCopy(item))
		}
	}
	return &dynamodb.ScanOutput{
		Items:            matched,
		Count:            int32(len(matched)),
		LastEvaluatedKey: nil, // single-page: scanAllPages would loop forever otherwise
	}, nil
}
