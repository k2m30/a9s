// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"net/url"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func colorSecrets(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	findings := secretStateFindings(r.Fields["status"])
	if len(findings) == 0 {
		findings = secretStructuralFindings(r.Fields["rotation_enabled"], r.Fields["last_changed"])
	}
	return colorFromFindings(findings)
}

func colorSSM(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	var lastModified *time.Time
	if t, err := time.Parse("2006-01-02 15:04", r.Fields["last_modified"]); err == nil {
		lastModified = &t
	}
	return colorFromFindings(ssmColorFindings(r.Fields["name"], r.Fields["type"], lastModified))
}

func colorKMS(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(kmsStateFindings(
		kmstypes.KeyState(r.Fields["status"]), r.Fields["status"]))
}

var secretsTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Secrets Manager",
		ShortName:     "secrets",
		Aliases:       []string{"secrets", "secretsmanager", "sm"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:Fields.arn",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "secretsmanager/secret?region="+region+"&name="+url.QueryEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "secret_name", Title: "Secret Name", Width: 36, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "last_accessed", Title: "Last Accessed", Width: 18, Sortable: true},
			{Key: "last_changed", Title: "Last Changed", Width: 18, Sortable: true},
			{Key: "rotation_enabled", Title: "Rotation", Width: 10, Sortable: true},
		},
		Color: colorSecrets,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchSecretsPage(ctx, c.SecretsManager, continuationToken)
		}),
		Reveal: revealWithClients(func(ctx context.Context, c *ServiceClients, resourceID string) (string, error) {
			return RevealSecret(ctx, c.SecretsManager, resourceID)
		}),
		Wave2:     IssueEnricher{Fn: EnrichSecretsPolicy, Priority: 100},
		FieldKeys: []string{"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled", "arn", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkSecretsKMS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda (rotation)", Checker: checkSecretsLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSecretsCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkSecretsDBI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkSecretsCB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "codeartifact", DisplayName: "CodeArtifact Domains", Checker: checkSecretsCodeArtifact},
			{TargetType: "eb", DisplayName: "Elastic Beanstalk", Checker: checkSecretsEB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkSecretsECSTask, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSecretsLogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkSecretsRole},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSecretsSNS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("secrets")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
			{FieldPath: "RotationLambdaARN", TargetType: "lambda"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeSecretStateDeleted, Phrase: "deleted", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSecretStateRotationOverdue, Phrase: "rotation overdue", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSecretStateDormant, Phrase: "dormant", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSecretRotationDisabled, Phrase: "rotation not enabled", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSecretStaleValue, Phrase: "value unchanged in over 365 days", Severity: domain.SevWarn, Source: "wave1"},
			{Code: secretsCodePublicPolicy, Phrase: "resource policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The secret's resource policy allows a wildcard principal, so any AWS account can read the credential this secret holds. Remove the \"*\" principal from the resource policy, or scope it with a condition naming the accounts that need it."},
			{Code: secretsCodeCrossAccountPolicy, Phrase: "resource policy grants another account", Severity: domain.SevWarn, Source: "wave2", Detail: "The secret's resource policy names a principal in another AWS account, so that account can read the credential. Confirm the grant is intended and still needed, and remove the account from the resource policy otherwise."},
		},
	},
	{
		Name:          "SSM Parameters",
		ShortName:     "ssm",
		Aliases:       []string{"ssm", "parameters", "parameter-store"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			name := strings.TrimPrefix(r.ID, "/")
			return consolelink.Regional(region, "systems-manager/parameters/"+name+"/description?region="+region)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 40, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "risk", Title: "Status", Width: 10, Sortable: true},
			{Key: "version", Title: "Version", Width: 8, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorSSM,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchSSMParametersPage(ctx, c.SSM, continuationToken)
		}),
		Reveal: revealWithClients(func(ctx context.Context, c *ServiceClients, resourceID string) (string, error) {
			return RevealSSMParameter(ctx, c.SSM, resourceID)
		}),
		FieldKeys: []string{"name", "type", "version", "last_modified", "description", "risk"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSSMKMS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ssm")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: ssmCodePlaintextSensitive, Phrase: "plaintext value looks like a credential", Severity: domain.SevBroken, Source: "wave1"},
			{Code: ssmCodeStaleValue, Phrase: "not modified in over 365 days", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "KMS Keys",
		ShortName:     "kms",
		Aliases:       []string{"kms", "keys"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "kms/home?region="+region+"#/kms/keys/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "alias", Title: "Alias", Width: 32, Sortable: true},
			{Key: "key_id", Title: "Key ID", Width: 38, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color:                  colorKMS,
		Fetcher:                fetcherWithClients(FetchKMSKeysPage),
		Wave2:                  IssueEnricher{Fn: EnrichKMSRotation, Priority: 100},
		FetchByIDs:             fetchByIDsWithClients(FetchKMSKeysByIDs),
		FieldKeys:              []string{"alias", "key_id", "status", "description"},
		IssueEnricherFieldKeys: []string{"rotation_enabled"},
		Related: []domain.RelatedDef{
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkKMSEBS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkKMSRDS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkKMSSecrets, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles (grants)", Checker: checkKMSRole, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("kms")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeKMSStatePendingDeletion, Phrase: "pending deletion", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeKMSStateDisabled, Phrase: "disabled", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeKMSStateUnavailable, Phrase: "<key state>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeKMSAccessDenied, Phrase: "access denied (kms:DescribeKey)", Severity: domain.SevBroken, Source: "wave1"},
			{Code: kmsCodeRotationDisabled, Phrase: "key rotation disabled", Severity: domain.SevWarn, Source: "wave2", Detail: "This customer-managed key never rotates its backing material, so every ciphertext ever written under it depends on one key that has been in use since creation. Enable automatic key rotation on the key."},
			{Code: kmsCodePublicPolicy, Phrase: "key policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The key policy allows a wildcard principal, so any AWS account can use this key to decrypt data encrypted with it. Replace the \"*\" principal with the specific accounts or roles that need the key, or add a condition scoping the grant."},
		},
	},
}
