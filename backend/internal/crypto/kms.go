package crypto

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Encryptor defines the interface for encryption and decryption.
type Encryptor interface {
	Encrypt(ctx context.Context, plaintext string) (string, error)
	Decrypt(ctx context.Context, ciphertext string) (string, error)
}

// KMSService implements Encryptor using AWS KMS.
type KMSService struct {
	client *kms.Client
	keyID  string
}

// NewKMSService creates a new KMSService.
// keyID can be a key ID, key ARN, or alias name (e.g., "alias/gophdrive-token-key").
func NewKMSService(client *kms.Client, keyID string) *KMSService {
	return &KMSService{
		client: client,
		keyID:  keyID,
	}
}

// Encrypt encrypts the plaintext using the configured KMS key.
// Returns base64 encoded ciphertext.
func (s *KMSService) Encrypt(ctx context.Context, plaintext string) (string, error) {
	input := &kms.EncryptInput{
		KeyId:     aws.String(s.keyID),
		Plaintext: []byte(plaintext),
	}

	result, err := s.client.Encrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

// Decrypt decrypts the base64 encoded ciphertext using KMS.
func (s *KMSService) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	input := &kms.DecryptInput{
		CiphertextBlob: decoded,
		KeyId:          aws.String(s.keyID), // Optional, but good practice
	}

	result, err := s.client.Decrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %w", err)
	}

	return string(result.Plaintext), nil
}
