package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchSecretsPage calls the SecretsManager ListSecrets API and returns a single
// page of secrets. Pass an empty continuationToken for the first page.
func FetchSecretsPage(ctx context.Context, api SecretsManagerListSecretsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &secretsmanager.ListSecretsInput{MaxResults: aws.Int32(DefaultPageSize)}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListSecrets(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching secrets: %w", err)
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

		// Compute status: DELETED > OVERDUE > DORMANT > OK
		secretStatus := "OK"
		switch {
		case secret.DeletedDate != nil:
			secretStatus = "DELETED"
		case secret.RotationEnabled != nil && *secret.RotationEnabled && secret.NextRotationDate != nil && time.Now().After(*secret.NextRotationDate):
			secretStatus = "OVERDUE"
		case secret.LastAccessedDate != nil && time.Since(*secret.LastAccessedDate) > 180*24*time.Hour:
			secretStatus = "DORMANT"
		}

		r := resource.Resource{
			ID:       secretName,
			Name:     secretName,
			Findings: secretStateFindings(secretStatus),
			Fields: map[string]string{
				"secret_name":      secretName,
				"description":      description,
				"last_accessed":    lastAccessed,
				"last_changed":     lastChanged,
				"rotation_enabled": rotationEnabled,
				"arn":              aws.ToString(secret.ARN),
				"status":           secretStatus,
			},
			RawStruct: secret,
		}
		if len(r.Findings) == 0 {
			r.Findings = secretStructuralFindings(rotationEnabled, lastChanged)
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

func secretStateFindings(status string) []domain.Finding {
	switch status {
	case "DELETED":
		return []domain.Finding{{Code: CodeSecretStateDeleted, Phrase: "deleted", Severity: domain.SevBroken, Source: "wave1"}}
	case "OVERDUE":
		return []domain.Finding{{Code: CodeSecretStateRotationOverdue, Phrase: "rotation overdue", Severity: domain.SevWarn, Source: "wave1"}}
	case "DORMANT":
		return []domain.Finding{{Code: CodeSecretStateDormant, Phrase: "dormant", Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}

// secretStructuralFindings mirrors colorSecrets's own precedence (rotation
// disabled, then stale value) for the branches that carry no status-derived
// Finding, so the list Status cell / detail Attention block always explain
// the Warning color.
//
// colorSecrets's "stale access" branch (last_accessed > 180d) is NOT
// represented here: that condition always produces secretStateFindings'
// DORMANT status first (same threshold, same source field), so this
// function — only called when len(r.Findings)==0 — never observes it.
func secretStructuralFindings(rotationEnabled, lastChanged string) []domain.Finding {
	switch {
	case rotationEnabled == "No":
		return []domain.Finding{{
			Code: CodeSecretRotationDisabled, Phrase: "rotation not enabled",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case lastChanged != "":
		if t, err := time.Parse("2006-01-02", lastChanged); err == nil && time.Since(t) > 365*24*time.Hour {
			return []domain.Finding{{
				Code: CodeSecretStaleValue, Phrase: "value unchanged in over 365 days",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}
	}
	return nil
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
