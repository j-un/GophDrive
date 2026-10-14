package dynamo

import (
	"context"
	"testing"
)

func TestAdapter_SearchFiles_Recursive(t *testing.T) {
	ctx := context.Background()

	m_root := NewMemoryAdapter("user1", "")
	baseFolder, err := m_root.CreateFolder(ctx, "BaseFolder", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	m := NewMemoryAdapter("user1", baseFolder.ID)

	subFolder, err := m.CreateFolder(ctx, "SubFolder", []string{baseFolder.ID})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	m.CreateFile(ctx, "level1.md", []byte("match me"), baseFolder.ID)
	m.CreateFile(ctx, "level2.md", []byte("match me"), subFolder.ID)
	m_root.CreateFile(ctx, "outside.md", []byte("match me"), "root")

	results, err := m.SearchFiles(ctx, "match")
	if err != nil {
		t.Fatalf("SearchFiles failed: %v", err)
	}

	foundLevel1 := false
	foundLevel2 := false
	foundOutside := false

	for _, r := range results {
		if r.Name == "level1" {
			foundLevel1 = true
		}
		if r.Name == "level2" {
			foundLevel2 = true
		}
		if r.Name == "outside" {
			foundOutside = true
		}
	}

	if !foundLevel1 {
		t.Errorf("level1 (direct child) should be found")
	}
	if !foundLevel2 {
		t.Errorf("level2 (recursive child) should be found")
	}
	if foundOutside {
		t.Errorf("outside.md should NOT be found")
	}

	t.Logf("Found %d results", len(results))
}
