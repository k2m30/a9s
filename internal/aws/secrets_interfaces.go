package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretsManagerListSecretsAPI defines the interface for the SecretsManager ListSecrets operation.
type SecretsManagerListSecretsAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

// SecretsManagerGetSecretValueAPI defines the interface for the SecretsManager GetSecretValue operation.
type SecretsManagerGetSecretValueAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// SecretsManagerGetResourcePolicyAPI defines the interface for the
// SecretsManager GetResourcePolicy operation. Used by secrets→role to read
// the secret's resource policy and extract allowed principals (role ARNs).
type SecretsManagerGetResourcePolicyAPI interface {
	GetResourcePolicy(ctx context.Context, params *secretsmanager.GetResourcePolicyInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetResourcePolicyOutput, error)
}

// SecretsManagerAPI is the aggregate interface covering all SecretsManager operations used by a9s fetchers.
// *secretsmanager.Client structurally satisfies this interface.
type SecretsManagerAPI interface {
	SecretsManagerListSecretsAPI
	SecretsManagerGetSecretValueAPI
	SecretsManagerGetResourcePolicyAPI
}
