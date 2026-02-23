package memory

import (
	"context"
	"testing"
)

func TestMemoryAdapter_SearchFiles_Recursive(t *testing.T) {
	ctx := context.Background()

	// 1. Create Base Folder
	m_root := NewMemoryAdapter(nil, "user1", "")
	baseFolder, err := m_root.CreateFolder(ctx, "BaseFolder", []string{"root"})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// 2. Setup Adapter with BaseFolderID
	m := NewMemoryAdapter(nil, "user1", baseFolder.ID)

	// 3. Create Subfolder inside BaseFolder
	subFolder, err := m.CreateFolder(ctx, "SubFolder", []string{baseFolder.ID})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// 4. Create files at different levels
	// Level 1: Direct child of BaseFolder
	m.CreateFile(ctx, "level1.md", []byte("match me"), baseFolder.ID)
	// Level 2: Child of SubFolder
	m.CreateFile(ctx, "level2.md", []byte("match me"), subFolder.ID)
	// Outside: Not in BaseFolder
	m_root.CreateFile(ctx, "outside.md", []byte("match me"), "root")

	// 5. Search for "match"
	results, err := m.SearchFiles(ctx, "match")
	if err != nil {
		t.Fatalf("SearchFiles failed: %v", err)
	}

	// CURRENT EXPECTATION: Only level1.md is found because it is a direct child
	// DESIRED EXPECTATION: Both level1.md and level2.md are found

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
