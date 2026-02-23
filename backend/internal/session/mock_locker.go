package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jun/gophdrive/backend/internal/model"
)

// MockLocker implements Locker using an in-memory map for testing.
type MockLocker struct {
	locks       map[string]*model.EditingSession
	mu          sync.Mutex
	ttlDuration time.Duration
}

// NewMockLocker creates a new MockLocker with the default TTL.
func NewMockLocker() *MockLocker {
	return &MockLocker{
		locks:       make(map[string]*model.EditingSession),
		ttlDuration: DefaultTTL,
	}
}

func (m *MockLocker) AcquireLock(ctx context.Context, fileID, userID string) (*model.EditingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	expiresAt := now + int64(m.ttlDuration.Seconds())

	if existing, ok := m.locks[fileID]; ok {
		// Allow if expired or same user
		if existing.ExpiresAt > now && existing.UserID != userID {
			return nil, fmt.Errorf("file is locked by another user")
		}
	}

	session := &model.EditingSession{
		FileID:    fileID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	m.locks[fileID] = session
	return session, nil
}

func (m *MockLocker) Heartbeat(ctx context.Context, fileID, userID string) (*model.EditingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[fileID]
	if !ok || existing.UserID != userID {
		return nil, fmt.Errorf("lock not found or not owned by user")
	}

	expiresAt := time.Now().Unix() + int64(m.ttlDuration.Seconds())
	existing.ExpiresAt = expiresAt
	return existing, nil
}

func (m *MockLocker) ReleaseLock(ctx context.Context, fileID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[fileID]
	if !ok || existing.UserID != userID {
		return fmt.Errorf("lock not found or not owned by user")
	}

	delete(m.locks, fileID)
	return nil
}

func (m *MockLocker) GetLockStatus(ctx context.Context, fileID string) (*model.EditingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[fileID]
	if !ok {
		return nil, nil
	}

	now := time.Now().Unix()
	if existing.ExpiresAt < now {
		return nil, nil // Expired
	}

	return existing, nil
}
