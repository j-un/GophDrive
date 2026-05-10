package auth

import "testing"

// GoogleVerifier.Verify is a thin wrapper over google.golang.org/api/idtoken,
// which performs network calls to Google's JWKS endpoint. The verifier is
// validated end-to-end at the handler layer using an injected fake Verifier
// (see internal/handler/auth_test.go). No unit tests are placed here to
// avoid mocking the underlying SDK or hitting the network.

func TestNewGoogleVerifier_StoresAudience(t *testing.T) {
	v := NewGoogleVerifier("client-abc")
	if v.audience != "client-abc" {
		t.Errorf("audience = %q, want %q", v.audience, "client-abc")
	}
}
