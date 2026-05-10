package dynamo

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// scanWarnPageThreshold is the page count at which scanAllPages logs a warning.
// Tuned for personal-scale usage; if real workloads consistently exceed this,
// it's a signal to add a GSI rather than to raise the threshold.
const scanWarnPageThreshold = 10

// ddbScanner is the subset of *dynamodb.Client.Scan used by scanAllPages.
// Defined as an interface so unit tests can inject a fake without touching
// the AWS SDK internals.
type ddbScanner interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

// scanAllPages drives a paginated DynamoDB scan to completion and returns the
// concatenated raw items. DynamoDB caps a single response at 1MB, so callers
// that don't paginate silently miss data once the user's data exceeds that
// size — which would never throw an error and would never be noticed.
//
// The base ScanInput is copied per page so the caller's struct is not mutated.
// pageCount is returned so callers can include it in error context if useful.
func scanAllPages(ctx context.Context, c ddbScanner, base *dynamodb.ScanInput) (items []map[string]types.AttributeValue, pageCount int, err error) {
	var lastKey map[string]types.AttributeValue
	for {
		in := *base
		in.ExclusiveStartKey = lastKey
		out, scanErr := c.Scan(ctx, &in)
		if scanErr != nil {
			return nil, pageCount, scanErr
		}
		items = append(items, out.Items...)
		pageCount++
		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}
	if pageCount >= scanWarnPageThreshold {
		log.Printf("warn: dynamo scan returned %d pages (>= %d) — consider a GSI", pageCount, scanWarnPageThreshold)
	}
	return items, pageCount, nil
}

// countAllPages drives a paginated COUNT scan and returns the total count.
// Same rationale as scanAllPages: a single scan only returns up to 1MB worth
// of matched rows; without pagination the count silently undercounts.
func countAllPages(ctx context.Context, c ddbScanner, base *dynamodb.ScanInput) (int, error) {
	total := 0
	var lastKey map[string]types.AttributeValue
	for {
		in := *base
		in.Select = types.SelectCount
		in.ExclusiveStartKey = lastKey
		out, err := c.Scan(ctx, &in)
		if err != nil {
			return 0, err
		}
		total += int(out.Count)
		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}
	return total, nil
}
