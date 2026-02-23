package session

import (
	"context"
	"testing"
	"time"
)

func TestMockLocker_AcquireAndRelease(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	s, err := m.AcquireLock(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	if s.FileID != "file1" || s.UserID != "user1" {
		t.Errorf("Session mismatch: got %+v", s)
	}

	err = m.ReleaseLock(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	status, _ := m.GetLockStatus(ctx, "file1")
	if status != nil {
		t.Error("Expected nil lock status after release")
	}
}

func TestMockLocker_DoubleAcquire_SameUser(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	_, err := m.AcquireLock(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	_, err = m.AcquireLock(ctx, "file1", "user1")
	if err != nil {
		t.Errorf("Same user should be able to re-acquire: %v", err)
	}
}

func TestMockLocker_DoubleAcquire_DifferentUser(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	_, err := m.AcquireLock(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	_, err = m.AcquireLock(ctx, "file1", "user2")
	if err == nil {
		t.Error("Expected error when different user tries to acquire existing lock")
	}
}

func TestMockLocker_Heartbeat(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	s, _ := m.AcquireLock(ctx, "file1", "user1")
	originalExpiry := s.ExpiresAt

	// Wait a bit so time.Now() gives a different second
	time.Sleep(1100 * time.Millisecond)

	updated, err := m.Heartbeat(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if updated.ExpiresAt <= originalExpiry {
		t.Errorf("Expected heartbeat to extend expiry: original=%d, updated=%d", originalExpiry, updated.ExpiresAt)
	}
}

func TestMockLocker_ExpiredLock(t *testing.T) {
	m := NewMockLocker()
	m.ttlDuration = -1 * time.Second // already expired
	ctx := context.Background()

	_, err := m.AcquireLock(ctx, "file1", "user1")
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	_, err = m.AcquireLock(ctx, "file1", "user2")
	if err != nil {
		t.Errorf("Should acquire expired lock: %v", err)
	}
}

func TestMockLocker_GetLockStatus_Active(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	m.AcquireLock(ctx, "file1", "user1")

	status, err := m.GetLockStatus(ctx, "file1")
	if err != nil {
		t.Fatalf("GetLockStatus failed: %v", err)
	}
	if status == nil {
		t.Fatal("Expected non-nil lock status")
	}
	if status.UserID != "user1" {
		t.Errorf("Expected userID 'user1', got '%s'", status.UserID)
	}
}

func TestMockLocker_GetLockStatus_Nonexistent(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	status, err := m.GetLockStatus(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetLockStatus unexpected error: %v", err)
	}
	if status != nil {
		t.Error("Expected nil for nonexistent lock")
	}
}

func TestMockLocker_ReleaseLock_WrongUser(t *testing.T) {
	m := NewMockLocker()
	ctx := context.Background()

	m.AcquireLock(ctx, "file1", "user1")

	err := m.ReleaseLock(ctx, "file1", "user2")
	if err == nil {
		t.Error("Expected error when releasing lock owned by another user")
	}
}
