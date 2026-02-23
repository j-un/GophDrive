package handler

import (
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
)

// GetUserID extracts the user ID from the Authorization header or session cookie.
func GetUserID(req events.APIGatewayProxyRequest, jwtSecret string) (string, error) {
	// Helper for case-insensitive header lookup
	getHeader := func(name string) string {
		for k, v := range req.Headers {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		return ""
	}

	// 1. Check Authorization Header (Bearer <token>)
	tokenString := ""
	authHeader := getHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. Check Cookie
	if tokenString == "" {
		// Cookie format: session_token=xxx; ...
		cookies := getHeader("Cookie")
		if cookies != "" {
			parts := strings.Split(cookies, ";")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "session_token=") {
					tokenString = strings.TrimPrefix(part, "session_token=")
					break
				}
			}
		}
	}

	if tokenString == "" {
		return "", fmt.Errorf("no authorization token found")
	}

	// Verify JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
	}

	return "", fmt.Errorf("invalid token claims")
}
