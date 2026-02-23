package adapter

import (
	"context"
)

// StorageProvider defines how to get a StorageAdapter for a specific user.
type StorageProvider interface {
	// GetAdapter returns a StorageAdapter for the given user ID.
	GetAdapter(ctx context.Context, userID string) (StorageAdapter, error)
}
