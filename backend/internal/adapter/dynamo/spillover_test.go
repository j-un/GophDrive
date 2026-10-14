package dynamo

import (
	"context"
	"errors"
	"testing"

	"github.com/jun/gophdrive/backend/internal/adapter"
)

func TestRouteBody_SmallContentStaysInline(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	body := []byte("hello world")

	inline, key, err := m.routeBody(context.Background(), body)
	if err != nil {
		t.Fatalf("routeBody returned err: %v", err)
	}
	if string(inline) != "hello world" {
		t.Errorf("inline = %q, want %q", inline, "hello world")
	}
	if key != "" {
		t.Errorf("s3Key = %q, want empty (no spillover yet)", key)
	}
}

func TestRouteBody_AtBoundaryStaysInline(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	body := make([]byte, inlineSizeLimit)

	inline, key, err := m.routeBody(context.Background(), body)
	if err != nil {
		t.Fatalf("routeBody at boundary returned err: %v", err)
	}
	if len(inline) != inlineSizeLimit {
		t.Errorf("inline len = %d, want %d", len(inline), inlineSizeLimit)
	}
	if key != "" {
		t.Errorf("s3Key = %q, want empty", key)
	}
}

func TestRouteBody_OverBoundaryReturnsErrPayloadTooLarge(t *testing.T) {
	m := NewMemoryAdapter("user1", "")
	body := make([]byte, inlineSizeLimit+1)

	_, _, err := m.routeBody(context.Background(), body)
	if !errors.Is(err, adapter.ErrPayloadTooLarge) {
		t.Errorf("err = %v, want adapter.ErrPayloadTooLarge", err)
	}
}

func TestFileItem_BodyS3KeyOmittedWhenEmpty(t *testing.T) {
	item := FileItem{Content: []byte("data")}
	if item.BodyS3Key != "" {
		t.Errorf("default BodyS3Key = %q, want empty", item.BodyS3Key)
	}
}
