package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorSecrets(r domain.Resource) domain.Color {
	if r.Fields["rotation_enabled"] == "No" {
		return domain.ColorWarning
	}
	if la := r.Fields["last_accessed"]; la != "" {
		if t, err := time.Parse("2006-01-02", la); err == nil {
			if time.Since(t) > 180*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	if lc := r.Fields["last_changed"]; lc != "" {
		if t, err := time.Parse("2006-01-02", lc); err == nil {
			if time.Since(t) > 365*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	return domain.ColorHealthy
}

func colorSSM(r domain.Resource) domain.Color {
	sensitiveSuffixes := []string{
		"_password", "_secret", "_token", "_apikey",
		"_api_key", "_credentials", "_passwd",
	}
	name := strings.ToLower(r.Fields["name"])
	if r.Fields["type"] == "String" {
		for _, suffix := range sensitiveSuffixes {
			if strings.HasSuffix(name, suffix) {
				return domain.ColorBroken
			}
		}
	}
	if lm := r.Fields["last_modified"]; lm != "" {
		if t, err := time.Parse("2006-01-02 15:04", lm); err == nil {
			if time.Since(t) > 365*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	return domain.ColorHealthy
}

func colorKMS(r domain.Resource) domain.Color {
	switch r.Fields["key_state"] {
	case "Enabled":
		return domain.ColorHealthy
	case "Disabled":
		return domain.ColorDim
	case "PendingDeletion", "PendingImport":
		return domain.ColorWarning
	case "Unavailable":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

var secretsTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Secrets Manager",
		ShortName:     "secrets",
		Aliases:       []string{"secrets", "secretsmanager", "sm"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "secret_name", Title: "Secret Name", Width: 36, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "last_accessed", Title: "Last Accessed", Width: 18, Sortable: true},
			{Key: "last_changed", Title: "Last Changed", Width: 18, Sortable: true},
			{Key: "rotation_enabled", Title: "Rotation", Width: 10, Sortable: true},
		},
		Color: colorSecrets,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSecretsPage(ctx, c.SecretsManager, continuationToken)
		},
		Reveal: func(ctx context.Context, clients any, resourceID string) (string, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return "", fmt.Errorf("AWS clients not initialized")
			}
			return RevealSecret(ctx, c.SecretsManager, resourceID)
		},
		FieldKeys: []string{"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled", "arn", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkSecretsKMS, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda (rotation)", Checker: checkSecretsLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSecretsCFN, NeedsTargetCache: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkSecretsDBI, NeedsTargetCache: true},
			{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkSecretsCB, NeedsTargetCache: true},
			{TargetType: "codeartifact", DisplayName: "CodeArtifact Domains", Checker: checkSecretsCodeArtifact},
			{TargetType: "eb", DisplayName: "Elastic Beanstalk", Checker: checkSecretsEB, NeedsTargetCache: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkSecretsECSTask, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSecretsLogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkSecretsRole},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSecretsSNS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("secrets")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
			{FieldPath: "RotationLambdaARN", TargetType: "lambda"},
		},
	},
	{
		Name:          "SSM Parameters",
		ShortName:     "ssm",
		Aliases:       []string{"ssm", "parameters", "parameter-store"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 40, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Width: 8, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorSSM,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSSMParametersPage(ctx, c.SSM, continuationToken)
		},
		Reveal: func(ctx context.Context, clients any, resourceID string) (string, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return "", fmt.Errorf("AWS clients not initialized")
			}
			return RevealSSMParameter(ctx, c.SSM, resourceID)
		},
		FieldKeys: []string{"name", "type", "version", "last_modified", "description", "risk"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSSMKMS, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ssm")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "KMS Keys",
		ShortName:     "kms",
		Aliases:       []string{"kms", "keys"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "alias", Title: "Alias", Width: 32, Sortable: true},
			{Key: "key_id", Title: "Key ID", Width: 38, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorKMS,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchKMSKeysPage(ctx, c, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichKMSRotation, Priority: 100},
		FetchByIDs: func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return nil, fmt.Errorf("AWS clients not initialized")
			}
			return FetchKMSKeysByIDs(ctx, c, ids)
		},
		FieldKeys:              []string{"alias", "key_id", "status", "description"},
		IssueEnricherFieldKeys: []string{"rotation_enabled"},
		Related: []domain.RelatedDef{
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkKMSEBS, NeedsTargetCache: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkKMSRDS, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkKMSSecrets, NeedsTargetCache: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkKMSS3, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Roles (grants)", Checker: checkKMSRole, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("kms")},
		},
	},
}
