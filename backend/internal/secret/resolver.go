// Package secret provides an abstraction for retrieving secrets from
// different backends (SSM Parameter Store, environment variables, etc.).
package secret

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMClient is the subset of *ssm.Client methods used by SSMResolver.
type SSMClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Resolver retrieves secret values by name.
type Resolver interface {
	GetSecret(ctx context.Context, name string) (string, error)
}

// SSMResolver fetches secrets from AWS Systems Manager Parameter Store.
type SSMResolver struct {
	client SSMClient
}

// NewSSMResolver returns a Resolver backed by SSM Parameter Store.
func NewSSMResolver(client SSMClient) Resolver {
	return &SSMResolver{client: client}
}

// GetSecret retrieves a SecureString parameter from SSM with decryption.
func (r *SSMResolver) GetSecret(ctx context.Context, name string) (string, error) {
	out, err := r.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("ssm get parameter %q: %w", name, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("ssm parameter %q has no value", name)
	}
	return *out.Parameter.Value, nil
}

// EnvResolver fetches secrets from environment variables.
// The parameter name is converted from SSM path format (e.g. "/gophdrive/jwt-secret")
// to the corresponding environment variable name (e.g. "JWT_SECRET") by taking the
// last segment, uppercasing, and replacing hyphens with underscores.
type EnvResolver struct{}

// NewEnvResolver returns a Resolver that reads from environment variables.
func NewEnvResolver() Resolver {
	return &EnvResolver{}
}

// GetSecret reads from the environment variable derived from the parameter name.
func (r *EnvResolver) GetSecret(_ context.Context, name string) (string, error) {
	envName := paramNameToEnvVar(name)
	val := os.Getenv(envName)
	if val == "" {
		return "", fmt.Errorf("environment variable %q (from param %q) is not set", envName, name)
	}
	return val, nil
}

// paramNameToEnvVar converts an SSM parameter name to an environment variable name.
// "/gophdrive/jwt-secret" -> "JWT_SECRET"
// "/gophdrive/google-client-secret" -> "GOOGLE_CLIENT_SECRET"
func paramNameToEnvVar(name string) string {
	parts := strings.Split(name, "/")
	last := parts[len(parts)-1]
	return strings.ToUpper(strings.ReplaceAll(last, "-", "_"))
}
