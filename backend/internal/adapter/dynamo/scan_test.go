package dynamo

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeScanner returns a pre-canned sequence of pages.
type fakeScanner struct {
	pages   []*dynamodb.ScanOutput
	err     error
	calls   int
	lastKey []map[string]types.AttributeValue // captured per-call ExclusiveStartKey
}

func (f *fakeScanner) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	f.lastKey = append(f.lastKey, in.ExclusiveStartKey)
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.pages) {
		return &dynamodb.ScanOutput{}, nil
	}
	out := f.pages[f.calls]
	f.calls++
	return out, nil
}

func item(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: id}}
}

func key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: id}}
}

func TestScanAllPages_SinglePage(t *testing.T) {
	f := &fakeScanner{
		pages: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{item("a"), item("b")}, LastEvaluatedKey: nil},
		},
	}
	got, pages, err := scanAllPages(context.Background(), f, &dynamodb.ScanInput{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pages != 1 {
		t.Errorf("pages = %d, want 1", pages)
	}
	if len(got) != 2 {
		t.Errorf("items = %d, want 2", len(got))
	}
}

func TestScanAllPages_FollowsLastEvaluatedKey(t *testing.T) {
	f := &fakeScanner{
		pages: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{item("a")}, LastEvaluatedKey: key("k1")},
			{Items: []map[string]types.AttributeValue{item("b")}, LastEvaluatedKey: key("k2")},
			{Items: []map[string]types.AttributeValue{item("c")}, LastEvaluatedKey: nil},
		},
	}
	got, pages, err := scanAllPages(context.Background(), f, &dynamodb.ScanInput{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pages != 3 {
		t.Errorf("pages = %d, want 3", pages)
	}
	if len(got) != 3 {
		t.Errorf("items = %d, want 3", len(got))
	}
	// First call has nil ExclusiveStartKey; subsequent calls forward the previous LastEvaluatedKey.
	if f.lastKey[0] != nil {
		t.Errorf("call 0 ExclusiveStartKey = %v, want nil", f.lastKey[0])
	}
	if pk, _ := f.lastKey[1]["pk"].(*types.AttributeValueMemberS); pk == nil || pk.Value != "k1" {
		t.Errorf("call 1 ExclusiveStartKey = %v, want pk=k1", f.lastKey[1])
	}
	if pk, _ := f.lastKey[2]["pk"].(*types.AttributeValueMemberS); pk == nil || pk.Value != "k2" {
		t.Errorf("call 2 ExclusiveStartKey = %v, want pk=k2", f.lastKey[2])
	}
}

func TestScanAllPages_PropagatesError(t *testing.T) {
	want := errors.New("network down")
	f := &fakeScanner{err: want}
	_, _, err := scanAllPages(context.Background(), f, &dynamodb.ScanInput{})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestScanAllPages_DoesNotMutateInput(t *testing.T) {
	base := &dynamodb.ScanInput{}
	f := &fakeScanner{
		pages: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{item("a")}, LastEvaluatedKey: key("k1")},
			{Items: []map[string]types.AttributeValue{item("b")}, LastEvaluatedKey: nil},
		},
	}
	if _, _, err := scanAllPages(context.Background(), f, base); err != nil {
		t.Fatalf("err: %v", err)
	}
	if base.ExclusiveStartKey != nil {
		t.Errorf("base.ExclusiveStartKey was mutated to %v", base.ExclusiveStartKey)
	}
}

func TestCountAllPages_SumsCountsAcrossPages(t *testing.T) {
	f := &fakeScanner{
		pages: []*dynamodb.ScanOutput{
			{Count: 7, LastEvaluatedKey: key("k1")},
			{Count: 5, LastEvaluatedKey: key("k2")},
			{Count: 3, LastEvaluatedKey: nil},
		},
	}
	total, err := countAllPages(context.Background(), f, &dynamodb.ScanInput{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if total != 15 {
		t.Errorf("total = %d, want 15", total)
	}
}
