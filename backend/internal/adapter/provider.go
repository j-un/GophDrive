package adapter

import (
	"context"
)

// StorageProvider returns a per-request StorageAdapter for the given user.
//
// baseFolderID scopes operations (search, list, recent, starred) to the user's
// notes folder. It comes from the verified session JWT — handlers extract it
// once and pass it through. Pass an empty string only during first-login
// bootstrap, where the caller will immediately call EnsureRootFolder to mint
// one.
type StorageProvider interface {
	GetAdapter(ctx context.Context, userID, baseFolderID string) (StorageAdapter, error)
}
