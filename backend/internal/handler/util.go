package handler

import (
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
)

// GetUserID extracts the user ID from the Authorization header or session cookie.
func GetUserID(req events.APIGatewayProxyRequest, jwtSecret string) (string, error) {
	tokenString := GetTokenString(req)
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

// GetTokenString extracts the raw JWT string from Authorization header or session_token cookie.
func GetTokenString(req events.APIGatewayProxyRequest) string {
	// Helper for case-insensitive header lookup
	getHeader := func(name string) string {
		for k, v := range req.Headers {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		// Also check multi-value headers (API Gateway v1)
		for k, v := range req.MultiValueHeaders {
			if strings.EqualFold(k, name) && len(v) > 0 {
				return v[0] // Return first occurrence
			}
		}
		return ""
	}

	// 1. Check Authorization Header (Bearer <token>)
	authHeader := getHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. Check Cookie
	// Cookie format: session_token=xxx; ...
	// We check both req.Headers and req.MultiValueHeaders for "Cookie"
	cookieHeaders := []string{}
	if h := getHeader("Cookie"); h != "" {
		cookieHeaders = append(cookieHeaders, h)
	}
	// Specifically check all occurrences in MultiValueHeaders
	for k, v := range req.MultiValueHeaders {
		if strings.EqualFold(k, "Cookie") {
			cookieHeaders = append(cookieHeaders, v...)
		}
	}

	for _, cookies := range cookieHeaders {
		parts := strings.Split(cookies, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "session_token=") {
				return strings.TrimPrefix(part, "session_token=")
			}
		}
	}

	return ""
}
