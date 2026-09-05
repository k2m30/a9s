// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// GetResourcePolicy returns the fixture-registered resource policy for the
// requested secret (see SecretsFixtures.ResourcePolicies). Secrets with no
// entry have no resource policy attached, which is what the real API reports
// as a nil ResourcePolicy.
func (f *SecretsFake) GetResourcePolicy(_ context.Context, input *secretsmanager.GetResourcePolicyInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetResourcePolicyOutput, error) {
	secretID := aws.ToString(input.SecretId)
	policy, ok := f.fix.ResourcePolicies[secretID]
	if !ok {
		return &secretsmanager.GetResourcePolicyOutput{Name: input.SecretId}, nil
	}
	return &secretsmanager.GetResourcePolicyOutput{
		Name:           input.SecretId,
		ResourcePolicy: aws.String(policy),
	}, nil
}
