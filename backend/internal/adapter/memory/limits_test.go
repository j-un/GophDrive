package memory

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryAdapter_Limits(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryAdapter(nil, "user1", "")

	t.Run("Title length limit", func(t *testing.T) {
		longName := strings.Repeat("a", maxDemoTitleLength+1)
		_, err := m.CreateFile(ctx, longName, []byte("content"), "")
		if err == nil || !strings.Contains(err.Error(), "name too long") {
			t.Errorf("Expected error about name length, got: %v", err)
		}
	})

	t.Run("Content size limit", func(t *testing.T) {
		largeContent := make([]byte, maxDemoContentSize+1)
		_, err := m.CreateFile(ctx, "test.md", largeContent, "")
		if err == nil || !strings.Contains(err.Error(), "content too large") {
			t.Errorf("Expected error about content size, got: %v", err)
		}
	})

	t.Run("Item count limit", func(t *testing.T) {
		m2 := NewMemoryAdapter(nil, "user2", "")
		// Create max items
		for i := 0; i < maxDemoItemCount; i++ {
			_, err := m2.CreateFile(ctx, "note.md", []byte("ok"), "")
			if err != nil {
				t.Fatalf("Failed to create item %d: %v", i, err)
			}
		}
		// Create one more
		_, err := m2.CreateFile(ctx, "overflow.md", []byte("ok"), "")
		if err == nil || !strings.Contains(err.Error(), "item limit reached") {
			t.Errorf("Expected error about item limit, got: %v", err)
		}
	})
}
