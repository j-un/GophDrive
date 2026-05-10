// Package auth verifies Google-issued ID tokens used by GophDrive's login flow.
//
// Refresh tokens are no longer stored — the application uses Google purely as
// an OIDC identity provider. Sessions are established via a self-issued JWT
// after the ID token has been validated.
package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// Claims is the subset of Google ID token claims the application trusts after
// signature and audience validation.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Verifier validates a raw ID token string and returns its trusted claims.
// Implementations must reject tokens with bad signatures, wrong audience,
// or expired/missing required claims.
type Verifier interface {
	Verify(ctx context.Context, rawIDToken string) (*Claims, error)
}

// GoogleVerifier validates ID tokens against Google's published JWKS.
type GoogleVerifier struct {
	audience string
}

// NewGoogleVerifier returns a Verifier that requires the given audience
// (the Google OAuth client ID issued for this application).
func NewGoogleVerifier(audience string) *GoogleVerifier {
	return &GoogleVerifier{audience: audience}
}

// Verify validates the ID token's signature, expiration, and audience using
// google.golang.org/api/idtoken, then projects the claims into the app's
// Claims struct.
func (v *GoogleVerifier) Verify(ctx context.Context, rawIDToken string) (*Claims, error) {
	payload, err := idtoken.Validate(ctx, rawIDToken, v.audience)
	if err != nil {
		return nil, fmt.Errorf("validate id token: %w", err)
	}

	c := &Claims{}
	if s, ok := payload.Claims["sub"].(string); ok {
		c.Subject = s
	}
	if s, ok := payload.Claims["email"].(string); ok {
		c.Email = s
	}
	if b, ok := payload.Claims["email_verified"].(bool); ok {
		c.EmailVerified = b
	}
	if s, ok := payload.Claims["name"].(string); ok {
		c.Name = s
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("id token missing subject claim")
	}
	return c, nil
}
