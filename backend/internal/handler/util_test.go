package handler_test

import (
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const testJWTSecret = "test-secret"

func TestGetUserID_Cookie(t *testing.T) {
	token := makeToken(testUserID)
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Cookie": "session_token=" + token + "; Path=/",
		},
	}

	userID, err := handler.GetUserID(req, testJWTSecret)
	if err != nil {
		t.Fatalf("GetUserID from cookie failed: %v", err)
	}
	if userID != testUserID {
		t.Errorf("Expected userID '%s', got '%s'", testUserID, userID)
	}
}

func TestGetUserID_NoToken(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{},
	}

	_, err := handler.GetUserID(req, testJWTSecret)
	if err == nil {
		t.Error("Expected error for missing token, got nil")
	}
}

func TestGetUserID_InvalidToken(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Cookie": "session_token=invalid-jwt-token",
		},
	}

	_, err := handler.GetUserID(req, testJWTSecret)
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

func TestGetUserID_ExpiredToken(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": testUserID,
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	signed, _ := token.SignedString([]byte(testJWTSecret))

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Cookie": "session_token=" + signed,
		},
	}

	_, err := handler.GetUserID(req, testJWTSecret)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestGetUserID_CaseInsensitiveCookieHeader(t *testing.T) {
	token := makeToken(testUserID)
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"cookie": "session_token=" + token, // lowercase header name
		},
	}

	userID, err := handler.GetUserID(req, testJWTSecret)
	if err != nil {
		t.Fatalf("GetUserID with lowercase cookie header failed: %v", err)
	}
	if userID != testUserID {
		t.Errorf("Expected userID '%s', got '%s'", testUserID, userID)
	}
}

func TestSignSession_SubAndExpiry(t *testing.T) {
	const sub = "user-123"
	const secret = "sign-secret"
	ttl := 5 * time.Minute

	before := time.Now()
	tok, err := handler.SignSession(handler.SessionClaims{UserID: sub}, ttl, secret)
	after := time.Now()
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	// Parse and verify sub + expiry window.
	parsed, parseErr := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if parseErr != nil {
		t.Fatalf("jwt.Parse: %v", parseErr)
	}
	claims, _ := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != sub {
		t.Errorf("sub: want %q, got %v", sub, claims["sub"])
	}
	// JWT exp is seconds-precision; allow 1s slack on the lower bound.
	exp := time.Unix(int64(claims["exp"].(float64)), 0)
	if exp.Before(before.Add(ttl-time.Second)) || exp.After(after.Add(ttl+time.Second)) {
		t.Errorf("exp %v outside expected range", exp)
	}
}

func TestSignSession_WrongSecret(t *testing.T) {
	tok, _ := handler.SignSession(handler.SessionClaims{UserID: "u"}, time.Minute, "correct-secret")
	_, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Error("expected error with wrong secret, got nil")
	}
}

func TestSignSession_EmptySecret(t *testing.T) {
	// jwt.SigningMethodHS256 accepts empty key — token is technically valid but
	// only verifiable with the same empty key. Ensure it at least doesn't panic.
	tok, err := handler.SignSession(handler.SessionClaims{UserID: "u"}, time.Minute, "")
	if err != nil {
		t.Fatalf("unexpected error with empty secret: %v", err)
	}
	if tok == "" {
		t.Error("expected non-empty token")
	}
}
