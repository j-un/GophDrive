package secret

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeSSMClient struct {
	params map[string]string
}

func (f *fakeSSMClient) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	val, ok := f.params[*input.Name]
	if !ok {
		return nil, fmt.Errorf("parameter not found: %s", *input.Name)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  input.Name,
			Value: aws.String(val),
		},
	}, nil
}

func TestSSMResolver_GetSecret_Success(t *testing.T) {
	client := &fakeSSMClient{
		params: map[string]string{
			"/gophdrive/jwt-secret": "super-secret-value",
		},
	}
	resolver := NewSSMResolver(client)

	val, err := resolver.GetSecret(context.Background(), "/gophdrive/jwt-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "super-secret-value" {
		t.Fatalf("expected %q, got %q", "super-secret-value", val)
	}
}

func TestSSMResolver_GetSecret_NotFound(t *testing.T) {
	client := &fakeSSMClient{
		params: map[string]string{},
	}
	resolver := NewSSMResolver(client)

	_, err := resolver.GetSecret(context.Background(), "/gophdrive/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing parameter, got nil")
	}
}

func TestEnvResolver_GetSecret_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "env-secret-value")
	defer os.Unsetenv("JWT_SECRET")

	resolver := NewEnvResolver()

	val, err := resolver.GetSecret(context.Background(), "/gophdrive/jwt-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "env-secret-value" {
		t.Fatalf("expected %q, got %q", "env-secret-value", val)
	}
}

func TestEnvResolver_GetSecret_NotSet(t *testing.T) {
	os.Unsetenv("NONEXISTENT_SECRET")
	resolver := NewEnvResolver()

	_, err := resolver.GetSecret(context.Background(), "/gophdrive/nonexistent-secret")
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}
}

func TestParamNameToEnvVar(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/gophdrive/jwt-secret", "JWT_SECRET"},
		{"/gophdrive/google-client-secret", "GOOGLE_CLIENT_SECRET"},
		{"/gophdrive/api-gateway-secret", "API_GATEWAY_SECRET"},
	}

	for _, tc := range tests {
		got := paramNameToEnvVar(tc.input)
		if got != tc.expected {
			t.Errorf("paramNameToEnvVar(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
