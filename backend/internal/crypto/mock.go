package crypto

import "context"

// MockEncryptor implements Encryptor for local development (no KMS required).
// It acts as a pass-through or simple base64 encoder (if needed), but here just returns plaintext or trivial transformation.
type MockEncryptor struct{}

func NewMockEncryptor() *MockEncryptor {
	return &MockEncryptor{}
}

func (m *MockEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	// For dev, just return plaintext or prefix it to know it's mocked
	return "mock:" + plaintext, nil
}

func (m *MockEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	// Remove prefix
	if len(ciphertext) > 5 && ciphertext[:5] == "mock:" {
		return ciphertext[5:], nil
	}
	return ciphertext, nil
}
