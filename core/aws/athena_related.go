// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_related.go contains Athena WorkGroup related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// athenaWorkGroupConfig fetches Configuration for a workgroup by name.
// Returns nil on any failure so callers can emit an unknown result.
func athenaWorkGroupConfig(ctx context.Context, clients any, wgName string) *athenatypes.WorkGroupConfiguration {
	if wgName == "" {
		return nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Athena == nil {
		return nil
	}
	out, err := c.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(wgName)})
	// Every checkAthena* caller turns this nil into UnknownRelated, so the
	// pivot renders "?" rather than a count nobody read, and the workgroup row
	// itself was never in question.
	// no finding: the related panel's own unknown is the mechanism.
	if err != nil || out == nil || out.WorkGroup == nil {
		return nil
	}
	return out.WorkGroup.Configuration
}

// checkAthenaS3 calls athena:GetWorkGroup and extracts the result-output bucket
// from Configuration.ResultConfiguration.OutputLocation (form: s3://bucket/prefix).
func checkAthenaS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return NotRead("s3")
	}
	if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.OutputLocation == nil {
		return foundNone("s3", "cfg.ResultConfiguration.OutputLocation")
	}
	return relatedRefs("s3", []string{*cfg.ResultConfiguration.OutputLocation}, refContext(clients, cache, "s3"))
}

// checkAthenaKMS calls athena:GetWorkGroup and extracts the KMS key ID from
// Configuration.ResultConfiguration.EncryptionConfiguration.KmsKey.
func checkAthenaKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return NotRead("kms")
	}
	if cfg.ResultConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == nil ||
		*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == "" {
		return foundNone("kms", "cfg")
	}
	keyID := kmsRefFromField(*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkAthenaLogs calls athena:GetWorkGroup and links the log group named by
// Configuration.MonitoringConfiguration.CloudWatchLoggingConfiguration when it
// is enabled — the only place Athena records where a workgroup writes logs.
func checkAthenaLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return NotRead("logs")
	}
	if cfg.MonitoringConfiguration == nil || cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration == nil {
		return foundNone("logs", "cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration")
	}
	cw := cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration
	if !aws.ToBool(cw.Enabled) {
		return foundNone("logs", "cfg.MonitoringConfiguration.CloudWatchLoggingConfiguration.Enabled")
	}
	// Logging on with no LogGroup leaves the group to Athena, which names none
	// in the configuration.
	if aws.ToString(cw.LogGroup) == "" {
		return NotRead("logs")
	}
	return relatedResultTrunc("logs", []string{*cw.LogGroup}, false)
}

// checkAthenaRole calls athena:GetWorkGroup and extracts the ExecutionRole for
// Spark workgroups from Configuration.ExecutionRole.
func checkAthenaRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return NotRead("role")
	}
	if cfg.ExecutionRole == nil || *cfg.ExecutionRole == "" {
		return foundNone("role", "cfg.ExecutionRole")
	}
	return relatedRefs("role", []string{*cfg.ExecutionRole}, refContext(clients, cache, "role"))
}
