package dynamo

import (
	"context"
	"strings"
	"testing"
)

func TestAdapter_Limits(t *testing.T) {
	ctx := context.Background()
	m := NewAdapter(nil, "demo-user-1", "")

	t.Run("Title length limit", func(t *testing.T) {
		longName := strings.Repeat("a", maxTitleLength+1)
		_, err := m.CreateFile(ctx, longName, []byte("content"), "")
		if err == nil || !strings.Contains(err.Error(), "name too long") {
			t.Errorf("Expected error about name length, got: %v", err)
		}
	})

	t.Run("Content size limit", func(t *testing.T) {
		largeContent := make([]byte, maxContentSize+1)
		_, err := m.CreateFile(ctx, "test.md", largeContent, "")
		if err == nil || !strings.Contains(err.Error(), "content too large") {
			t.Errorf("Expected error about content size, got: %v", err)
		}
	})

	t.Run("Demo user item count limit", func(t *testing.T) {
		demo := NewAdapter(nil, "demo-user-2", "")
		for i := 0; i < maxDemoItemCount; i++ {
			_, err := demo.CreateFile(ctx, "note.md", []byte("ok"), "")
			if err != nil {
				t.Fatalf("Failed to create item %d: %v", i, err)
			}
		}
		_, err := demo.CreateFile(ctx, "overflow.md", []byte("ok"), "")
		if err == nil || !strings.Contains(err.Error(), "item limit reached") {
			t.Errorf("Expected error about item limit, got: %v", err)
		}
	})

	t.Run("Real user has no item count limit", func(t *testing.T) {
		real := NewAdapter(nil, "google-sub-12345", "")
		for i := 0; i < maxDemoItemCount+5; i++ {
			_, err := real.CreateFile(ctx, "note.md", []byte("ok"), "")
			if err != nil {
				t.Fatalf("Real user blocked at item %d: %v", i, err)
			}
		}
	})
}
