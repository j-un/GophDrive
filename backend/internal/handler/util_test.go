package handler_test

import (
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jun/gophdrive/backend/internal/handler"
)

const testJWTSecret = "test-secret"

func TestGetUserID_BearerToken(t *testing.T) {
	token := makeToken(testUserID)
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	userID, err := handler.GetUserID(req, testJWTSecret)
	if err != nil {
		t.Fatalf("GetUserID failed: %v", err)
	}
	if userID != testUserID {
		t.Errorf("Expected userID '%s', got '%s'", testUserID, userID)
	}
}

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
			"Authorization": "Bearer invalid-jwt-token",
		},
	}

	_, err := handler.GetUserID(req, testJWTSecret)
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

func TestGetUserID_ExpiredToken(t *testing.T) {
	// Create an expired token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": testUserID,
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	signed, _ := token.SignedString([]byte(testJWTSecret))

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"Authorization": "Bearer " + signed,
		},
	}

	_, err := handler.GetUserID(req, testJWTSecret)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestGetUserID_CaseInsensitiveHeaders(t *testing.T) {
	token := makeToken(testUserID)
	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"authorization": "Bearer " + token, // lowercase
		},
	}

	userID, err := handler.GetUserID(req, testJWTSecret)
	if err != nil {
		t.Fatalf("GetUserID with lowercase header failed: %v", err)
	}
	if userID != testUserID {
		t.Errorf("Expected userID '%s', got '%s'", testUserID, userID)
	}
}
