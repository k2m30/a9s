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
	return colorFromFindings(secretFindings(
		r.Fields["status"], r.Fields["rotation_enabled"], r.Fields["last_changed"]))
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
		Name:           "Secrets Manager",
		ShortName:      "secrets",
		LifecycleKey:   "status",
		HumanizeFields: []string{"status"},
		Aliases:        []string{"secrets", "secretsmanager", "sm"},
		Category:       "SECRETS & CONFIG",
		CloudTrailKey:  "ResourceName:Fields.arn",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "secretsmanager/secret?region="+region+"&name="+url.QueryEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "secret_name", Title: "Secret Name", Path: "Name", Width: 36},
			{Key: "status", Title: "Status", Path: "Name", Width: 10},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
			{Key: "last_accessed", Title: "Last Accessed", Path: "LastAccessedDate", Width: 18},
			{Key: "last_changed", Title: "Last Changed", Path: "LastChangedDate", Width: 18},
			{Key: "rotation_enabled", Title: "Rotation", Path: "RotationEnabled", Width: 10},
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
			{Code: CodeSecretStateDeleted, Phrase: "scheduled for deletion", Severity: domain.SevBroken, Source: "wave1", Detail: "This secret is inside its recovery window and will be destroyed when the window ends, and applications reading it are failing already. Restore it now if anything still needs the value — once the window closes there is no way to get it back."},
			{Code: CodeSecretStateRotationOverdue, Phrase: "rotation overdue", Severity: domain.SevWarn, Source: "wave1", Detail: "Rotation is configured but the secret is past the interval it should have rotated in, so either the rotation function is failing or it was never invoked. Check the rotation function's logs, and run a rotation by hand to prove it works."},
			{Code: CodeSecretStateDormant, Phrase: "dormant", Severity: domain.SevWarn, Source: "wave1", Detail: "Nothing has read this secret in a long time, so it is either unused or read by something you have lost track of. Confirm which through CloudTrail, then delete it or record its owner — a live credential nobody watches is the one that leaks unnoticed."},
			{Code: CodeSecretRotationDisabled, Phrase: "rotation not enabled", Severity: domain.SevWarn, Source: "wave1", Detail: "The value never changes on a schedule, so a leaked copy stays valid until somebody notices and rotates it by hand. Turn on rotation with a function that can change the credential at its source."},
			{Code: CodeSecretStaleValue, Phrase: "value unchanged in over 365 days", Severity: domain.SevWarn, Source: "wave1", Detail: "The value has not changed in over a year, so anyone who has ever held a copy still holds a working credential. Rotate it, and set up scheduled rotation so the next year does not look the same."},
			{Code: secretsCodePublicPolicy, Phrase: "resource policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The secret's resource policy allows a wildcard principal, so any AWS account can read the credential this secret holds. Remove the \"*\" principal from the resource policy, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing."},
			{Code: secretsCodeCrossAccountPolicy, Phrase: "resource policy grants another account", Severity: domain.SevWarn, Source: "wave2", Detail: "The secret's resource policy names a principal in another AWS account, so that account can read the credential. Confirm the grant is intended and still needed, and remove the account from the resource policy otherwise."},
		},
	},
	{
		Name:           "SSM Parameters",
		ShortName:      "ssm",
		LifecycleKey:   "risk",
		HumanizeFields: []string{"type"},
		Aliases:        []string{"ssm", "parameters", "parameter-store"},
		Category:       "SECRETS & CONFIG",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			name := strings.TrimPrefix(r.ID, "/")
			return consolelink.Regional(region, "systems-manager/parameters/"+name+"/description?region="+region)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Path: "Name", Width: 40},
			{Key: "type", Title: "Type", Path: "Type", Width: 14},
			{Key: "risk", Title: "Status", Width: 10},
			{Key: "version", Title: "Version", Path: "Version", Width: 8},
			{Key: "last_modified", Title: "Last Modified", Path: "LastModifiedDate", Width: 22},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
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
			{Code: ssmCodePlaintextSensitive, Phrase: "plaintext value looks like a credential", Severity: domain.SevBroken, Source: "wave1", Detail: "This parameter holds what looks like a credential in plain text, so it is unencrypted at rest and readable by anyone allowed to read parameters on that path. Store it as an encrypted parameter under a KMS key, update the readers, then rotate the exposed value."},
			{Code: ssmCodeStaleValue, Phrase: "not modified in over 365 days", Severity: domain.SevWarn, Source: "wave1", Detail: "The parameter has not been touched in over a year, and if it holds a credential that credential has been valid all that time. Confirm it is still current, and rotate it if it is a secret."},
		},
	},
	{
		Name:           "KMS Keys",
		ShortName:      "kms",
		HumanizeFields: []string{"status", "KeyManager", "KeySpec", "KeyState", "KeyUsage", "Origin"},
		Aliases:        []string{"kms", "keys"},
		Category:       "SECRETS & CONFIG",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "kms/home?region="+region+"#/kms/keys/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "alias", Title: "Alias", Path: "AliasName", Width: 32},
			{Key: "key_id", Title: "Key ID", Path: "KeyId", Width: 38},
			{Key: "status", Title: "Status", Path: "KeyState", Width: 12},
			{Key: "rotation_enabled", Title: "Rotation", Width: 10},
			{Key: "description", Title: "Description", Path: "Description", Width: 36},
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
			{Code: CodeKMSStatePendingDeletion, Phrase: "pending deletion", Severity: domain.SevBroken, Source: "wave1", Detail: "The key is scheduled for destruction, and when the waiting period ends everything encrypted under it becomes permanently unreadable — snapshots, buckets, secrets and volumes alike. Cancel the deletion unless you have proven nothing still depends on it."},
			{Code: CodeKMSStateDisabled, Phrase: "disabled", Severity: domain.SevWarn, Source: "wave1", Detail: "The key cannot be used while it is disabled, so any read or write needing it fails now even though nothing has been destroyed. Re-enable it if that was not deliberate; the failures show up as permission errors in the services that use it."},
			{Code: CodeKMSStateUnavailable, Phrase: "<key state>", Severity: domain.SevBroken, Source: "wave1", Detail: "The key is not usable in the state it reports, and what to do depends on which state that is. A key still being created or updated only needs waiting out; one awaiting imported material needs the material imported; one whose custom key store is disconnected needs the store reconnected before anything encrypted under it can be read."},
			{Code: CodeKMSAccessDenied, Phrase: "access denied (kms:DescribeKey)", Severity: domain.SevBroken, Source: "wave1", Detail: "This key's state could not be read because the key policy or your own IAM policy denies it, so nothing here can be judged — the key may be healthy or pending deletion and this view cannot tell you which. Grant the role you browse with permission to describe the key."},
			{Code: kmsCodeRotationDisabled, Phrase: "key rotation disabled", Severity: domain.SevWarn, Source: "wave2", Detail: "The key material never changes on a schedule, so anything that has ever been able to decrypt with this key still can. Turn on automatic rotation if this is a symmetric customer managed key with AWS-generated material, the only kind that supports it; for an asymmetric key, a message-authentication key, one with imported material, or one in a custom key store, plan a manual rotation to a new key and re-encrypt what depends on it."},
			{Code: kmsCodePublicPolicy, Phrase: "key policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The key policy allows a wildcard principal, so any AWS account can use this key to decrypt data encrypted with it. Add a condition that requires the caller's account or role to equal one you expect, and keep any condition limiting which service the request may come through: that one narrows the path but not the caller, so it bounds the exposure without closing it."},
		},
	},
}
