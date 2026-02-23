package session

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jun/gophdrive/backend/internal/model"
)

const DefaultTTL = 5 * time.Minute

// LockManager handles session locking for files using DynamoDB TTL.
type LockManager struct {
	client      *dynamodb.Client
	tableName   string
	ttlDuration time.Duration
}

// NewLockManager creates a new LockManager.
func NewLockManager(client *dynamodb.Client, tableName string) *LockManager {
	return &LockManager{
		client:      client,
		tableName:   tableName,
		ttlDuration: DefaultTTL,
	}
}

// AcquireLock attempts to acquire a lock on a file for the given user.
// It succeeds if:
// 1. No lock exists for the file.
// 2. The existing lock has expired (TTL < now).
// 3. The existing lock belongs to the same user (refresh).
func (m *LockManager) AcquireLock(ctx context.Context, fileID string, userID string) (*model.EditingSession, error) {
	now := time.Now().Unix()
	expiresAt := now + int64(m.ttlDuration.Seconds())

	session := model.EditingSession{
		FileID:    fileID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	// Condition: (attribute_not_exists(file_id)) OR (expires_at < :now) OR (user_id = :user_id)
	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(m.tableName),
		Item:      item,
		ConditionExpression: aws.String(
			"attribute_not_exists(file_id) OR expires_at < :now OR user_id = :user_id",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now)},
			":user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})

	if err != nil {
		if string(err.Error()) == "ConditionalCheckFailedException" {
			// In AWS SDK v2, use errors.As(err, &condCheckFailed)
			return nil, fmt.Errorf("file is locked by another user")
		}
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return &session, nil
}

// Heartbeat extends the lock TTL if the user owns the lock.
func (m *LockManager) Heartbeat(ctx context.Context, fileID string, userID string) (*model.EditingSession, error) {
	now := time.Now().Unix()
	expiresAt := now + int64(m.ttlDuration.Seconds())

	// We only update if user_id matches and lock is not expired (safety check, though if it's expired we could re-acquire).
	// But strictly heartbeat implies active session.
	// If it expired, maybe we should re-acquire? Let's strictly update only if we own it.

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(m.tableName),
		Key: map[string]types.AttributeValue{
			"file_id": &types.AttributeValueMemberS{Value: fileID},
		},
		UpdateExpression:    aws.String("SET expires_at = :expires_at"),
		ConditionExpression: aws.String("user_id = :user_id"), // Only if we own it
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expires_at": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)},
			":user_id":    &types.AttributeValueMemberS{Value: userID},
		},
		ReturnValues: types.ReturnValueAllNew,
	}

	out, err := m.client.UpdateItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to send heartbeat: %w", err)
	}

	var session model.EditingSession
	err = attributevalue.UnmarshalMap(out.Attributes, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// ReleaseLock removes the lock if the user owns it.
func (m *LockManager) ReleaseLock(ctx context.Context, fileID string, userID string) error {
	_, err := m.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(m.tableName),
		Key: map[string]types.AttributeValue{
			"file_id": &types.AttributeValueMemberS{Value: fileID},
		},
		ConditionExpression: aws.String("user_id = :user_id"), // Only if we own it
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}

// GetLockStatus retrieves the current lock status.
func (m *LockManager) GetLockStatus(ctx context.Context, fileID string) (*model.EditingSession, error) {
	out, err := m.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(m.tableName),
		Key: map[string]types.AttributeValue{
			"file_id": &types.AttributeValueMemberS{Value: fileID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get lock status: %w", err)
	}
	if out.Item == nil {
		return nil, nil // No lock
	}

	var session model.EditingSession
	err = attributevalue.UnmarshalMap(out.Item, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check expiry
	now := time.Now().Unix()
	if session.ExpiresAt < now {
		return nil, nil // Expired
	}

	return &session, nil
}
