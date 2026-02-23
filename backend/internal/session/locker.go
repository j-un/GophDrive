package session

import (
	"context"

	"github.com/jun/gophdrive/backend/internal/model"
)

// Locker defines the interface for file lock management.
// Implementations manage session-based locking to prevent concurrent edit conflicts.
type Locker interface {
	// AcquireLock attempts to acquire a lock on a file for the given user.
	AcquireLock(ctx context.Context, fileID, userID string) (*model.EditingSession, error)

	// Heartbeat extends the lock TTL if the user owns the lock.
	Heartbeat(ctx context.Context, fileID, userID string) (*model.EditingSession, error)

	// ReleaseLock removes the lock if the user owns it.
	ReleaseLock(ctx context.Context, fileID, userID string) error

	// GetLockStatus retrieves the current lock status.
	GetLockStatus(ctx context.Context, fileID string) (*model.EditingSession, error)
}
