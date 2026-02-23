package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jun/gophdrive/backend/internal/crypto"
	"github.com/jun/gophdrive/backend/internal/model"
	"golang.org/x/oauth2"
)

// AuthService handles OAuth2 authentication flows and token management.
type AuthService struct {
	oauthConfig  *oauth2.Config
	dynamoClient *dynamodb.Client
	tableName    string
	kmsService   crypto.Encryptor

	// In-memory fallback
	tokens map[string]model.UserToken
	mu     sync.RWMutex
}

// Config returns the OAuth2 config.
func (s *AuthService) Config() *oauth2.Config {
	return s.oauthConfig
}

// NewAuthService creates a new AuthService.
// The oauthConfig should be constructed by the caller (e.g., from environment variables).
func NewAuthService(oauthConfig *oauth2.Config, dynamoClient *dynamodb.Client, tableName string, kmsService crypto.Encryptor) *AuthService {
	return &AuthService{
		oauthConfig:  oauthConfig,
		dynamoClient: dynamoClient,
		tableName:    tableName,
		kmsService:   kmsService,
		tokens:       make(map[string]model.UserToken),
	}
}

// GenerateAuthURL returns the URL to redirect the user to for Google login.
func (s *AuthService) GenerateAuthURL(state string) string {
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges the authorization code for an access token.
func (s *AuthService) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return s.oauthConfig.Exchange(ctx, code)
}

// SaveToken encrypts the refresh token and stores it in DynamoDB.
func (s *AuthService) SaveToken(ctx context.Context, userID string, token *oauth2.Token) error {
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token in response")
	}

	// Encrypt Refresh Token
	encrypted, err := s.kmsService.Encrypt(ctx, token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Check for existing token to preserve BaseFolderID
	var baseFolderID string
	if existing, err := s.GetUserToken(ctx, userID); err == nil {
		baseFolderID = existing.BaseFolderID
	}

	userToken := model.UserToken{
		UserID:                userID,
		EncryptedRefreshToken: encrypted,
		BaseFolderID:          baseFolderID,
		UpdatedAt:             time.Now(),
	}

	// In-memory fallback
	if s.dynamoClient == nil {
		s.mu.Lock()
		s.tokens[userID] = userToken
		s.mu.Unlock()
		return nil
	}

	item, err := attributevalue.MarshalMap(userToken)
	if err != nil {
		return fmt.Errorf("failed to marshal user token: %w", err)
	}

	_, err = s.dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save token to DynamoDB: %w", err)
	}

	return nil
}

// GetUserToken retrieves the UserToken from DynamoDB.
func (s *AuthService) GetUserToken(ctx context.Context, userID string) (*model.UserToken, error) {
	var userToken model.UserToken

	if s.dynamoClient == nil {
		s.mu.RLock()
		t, ok := s.tokens[userID]
		s.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("user not found")
		}
		userToken = t
	} else {
		out, err := s.dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: userID},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get item from DynamoDB: %w", err)
		}
		if out.Item == nil {
			return nil, fmt.Errorf("user not found")
		}

		err = attributevalue.UnmarshalMap(out.Item, &userToken)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user token: %w", err)
		}
	}
	return &userToken, nil
}

// UpdateBaseFolderID updates the BaseFolderID for a user.
func (s *AuthService) UpdateBaseFolderID(ctx context.Context, userID, folderID string) error {
	// Simple update using UpdateItem to avoid overwriting other fields race (though unlikely here)
	if s.dynamoClient == nil {
		s.mu.Lock()
		if t, ok := s.tokens[userID]; ok {
			t.BaseFolderID = folderID
			s.tokens[userID] = t
		}
		s.mu.Unlock()
		return nil
	}

	_, err := s.dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET base_folder_id = :fid, updated_at = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":fid": &types.AttributeValueMemberS{Value: folderID},
			":now": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update base folder id: %w", err)
	}

	return nil
}

// GetClient returns an authenticated http.Client for the user.
func (s *AuthService) GetClient(ctx context.Context, userID string) (*http.Client, error) {
	var userToken model.UserToken

	if s.dynamoClient == nil {
		s.mu.RLock()
		t, ok := s.tokens[userID]
		s.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("user not found")
		}
		userToken = t
	} else {
		// Get from DynamoDB
		out, err := s.dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: userID},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get item from DynamoDB: %w", err)
		}
		if out.Item == nil {
			return nil, fmt.Errorf("user not found")
		}

		err = attributevalue.UnmarshalMap(out.Item, &userToken)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user token: %w", err)
		}
	}

	// Decrypt Refresh Token
	refreshToken, err := s.kmsService.Decrypt(ctx, userToken.EncryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// Create Token Source
	token := &oauth2.Token{
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(-1 * time.Hour), // Force refresh
	}

	tokenSource := s.oauthConfig.TokenSource(ctx, token)

	return oauth2.NewClient(ctx, tokenSource), nil
}
