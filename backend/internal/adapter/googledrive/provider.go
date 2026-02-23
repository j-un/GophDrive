package googledrive

import (
	"context"
	"fmt"

	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/auth"
)

// Provider implements adapter.StorageProvider for Google Drive.
type Provider struct {
	authService *auth.AuthService
}

// NewProvider creates a new Google Drive provider.
func NewProvider(authService *auth.AuthService) *Provider {
	return &Provider{authService: authService}
}

// GetAdapter returns a DriveAdapter for the given user ID.
func (p *Provider) GetAdapter(ctx context.Context, userID string) (adapter.StorageAdapter, error) {
	// Get base folder ID from user token
	var baseFolderID string
	if token, err := p.authService.GetUserToken(ctx, userID); err == nil {
		baseFolderID = token.BaseFolderID
	}

	client, err := p.authService.GetClient(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	storage, err := NewDriveAdapter(ctx, client, baseFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create drive adapter: %w", err)
	}

	return storage, nil
}
