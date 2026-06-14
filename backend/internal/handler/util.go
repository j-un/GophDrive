package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
)

// SessionClaims holds the application-trusted fields carried in the session JWT.
//
// base_folder_id is minted on first login (auto-created root folder) and
// preserved across refreshes so handlers never need a separate DB lookup
// just to know which folder scope a request operates in.
type SessionClaims struct {
	UserID       string
	Email        string
	Name         string
	BaseFolderID string
}

// GetSessionClaims parses and verifies the session JWT, returning the trusted
// claims. Expired tokens are rejected.
func GetSessionClaims(req events.APIGatewayProxyRequest, jwtSecret string) (*SessionClaims, error) {
	tokenString := GetTokenString(req)
	if tokenString == "" {
		return nil, fmt.Errorf("no authorization token found")
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("missing subject in token")
	}

	out := &SessionClaims{UserID: sub}
	if v, ok := claims["email"].(string); ok {
		out.Email = v
	}
	if v, ok := claims["name"].(string); ok {
		out.Name = v
	}
	if v, ok := claims["base_folder_id"].(string); ok {
		out.BaseFolderID = v
	}
	return out, nil
}

// GetUserID is a convenience over GetSessionClaims for handlers that only
// need the user identifier.
func GetUserID(req events.APIGatewayProxyRequest, jwtSecret string) (string, error) {
	c, err := GetSessionClaims(req, jwtSecret)
	if err != nil {
		return "", err
	}
	return c.UserID, nil
}

// SignSession produces a signed HS256 JWT for the given claims and TTL.
// Exposed as a package-level function so code outside this package (e.g. app)
// can mint agent-translation tokens without a full AuthHandler instance.
func SignSession(c SessionClaims, ttl time.Duration, jwtSecret string) (string, error) {
	now := time.Now()
	mc := jwt.MapClaims{
		"sub":            c.UserID,
		"email":          c.Email,
		"name":           c.Name,
		"base_folder_id": c.BaseFolderID,
		"iat":            now.Unix(),
		"exp":            now.Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString([]byte(jwtSecret))
}

// GetTokenString extracts the raw JWT string from the session_token cookie.
func GetTokenString(req events.APIGatewayProxyRequest) string {
	// Helper for case-insensitive header lookup
	getHeader := func(name string) string {
		for k, v := range req.Headers {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		for k, v := range req.MultiValueHeaders {
			if strings.EqualFold(k, name) && len(v) > 0 {
				return v[0]
			}
		}
		return ""
	}

	cookieHeaders := []string{}
	if h := getHeader("Cookie"); h != "" {
		cookieHeaders = append(cookieHeaders, h)
	}
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
