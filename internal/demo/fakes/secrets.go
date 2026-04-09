package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SecretsFake implements aws.SecretsManagerAPI against fixture data loaded at construction time.
type SecretsFake struct {
	fix *fixtures.SecretsFixtures
}

// NewSecrets constructs a SecretsFake backed by fixture data from the fixtures package.
func NewSecrets() *SecretsFake {
	return &SecretsFake{fix: fixtures.NewSecretsFixtures()}
}

func (f *SecretsFake) ListSecrets(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return &secretsmanager.ListSecretsOutput{SecretList: f.fix.Secrets}, nil
}

func (f *SecretsFake) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if input.SecretId == nil {
		return nil, fmt.Errorf("GetSecretValue: SecretId is required")
	}
	val, ok := f.fix.SecretValues[*input.SecretId]
	if !ok {
		// Return a placeholder for secrets without explicit values.
		val = `{"value":"[REDACTED — demo mode]"}`
	}
	return &secretsmanager.GetSecretValueOutput{
		Name:         input.SecretId,
		SecretString: aws.String(val),
	}, nil
}
