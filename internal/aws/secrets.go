package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("secrets", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSecrets(ctx, c.SecretsManager)
	})
	resource.RegisterFieldKeys("secrets", []string{"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled"})
}

// FetchSecrets calls the SecretsManager ListSecrets API and converts the
// response into a slice of generic Resource structs.
func FetchSecrets(ctx context.Context, api SecretsManagerListSecretsAPI) ([]resource.Resource, error) {
	output, err := api.ListSecrets(ctx, &secretsmanager.ListSecretsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching secrets: %w", err)
	}

	var resources []resource.Resource

	for _, secret := range output.SecretList {
		secretName := ""
		if secret.Name != nil {
			secretName = *secret.Name
		}

		description := ""
		if secret.Description != nil {
			description = *secret.Description
		}

		lastAccessed := ""
		if secret.LastAccessedDate != nil {
			lastAccessed = secret.LastAccessedDate.Format("2006-01-02")
		}

		lastChanged := ""
		if secret.LastChangedDate != nil {
			lastChanged = secret.LastChangedDate.Format("2006-01-02")
		}

		rotationEnabled := "No"
		if secret.RotationEnabled != nil && *secret.RotationEnabled {
			rotationEnabled = "Yes"
		}

		r := resource.Resource{
			ID:     secretName,
			Name:   secretName,
			Status: "",
			Fields: map[string]string{
				"secret_name":      secretName,
				"description":      description,
				"last_accessed":    lastAccessed,
				"last_changed":     lastChanged,
				"rotation_enabled": rotationEnabled,
			},
			RawStruct:  secret,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

// RevealSecret calls the SecretsManager GetSecretValue API and returns the
// secret string value.
func RevealSecret(ctx context.Context, api SecretsManagerGetSecretValueAPI, secretName string) (string, error) {
	output, err := api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("revealing secret: %w", err)
	}

	if output.SecretString != nil {
		return *output.SecretString, nil
	}

	return "", nil
}
