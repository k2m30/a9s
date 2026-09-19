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
		return resource.UnknownRelated("s3")
	}
	if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.OutputLocation == nil {
		return resource.ProvenZero("s3", "cfg.ResultConfiguration.OutputLocation")
	}
	return relatedRefs("s3", []string{*cfg.ResultConfiguration.OutputLocation}, refContext(clients, cache, "s3"))
}

// checkAthenaKMS calls athena:GetWorkGroup and extracts the KMS key ID from
// Configuration.ResultConfiguration.EncryptionConfiguration.KmsKey.
func checkAthenaKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("kms")
	}
	if cfg.ResultConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == nil ||
		*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == "" {
		return resource.ProvenZero("kms", "cfg")
	}
	keyID := kmsRefFromField(*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkAthenaLogs calls athena:GetWorkGroup and extracts the CloudWatch log
// group used for Spark driver logs (CustomerContentEncryptionConfiguration is
// storage-side; the spark driver log group is carried on the EngineConfiguration).
// For SQL workgroups there is no log group; Count: 0.
func checkAthenaLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("logs")
	}
	if cfg.PublishCloudWatchMetricsEnabled == nil || !*cfg.PublishCloudWatchMetricsEnabled {
		return resource.ProvenZero("logs", "cfg.PublishCloudWatchMetricsEnabled")
	}
	// Athena publishes metrics but the log group is implicit (/aws/athena/<WG>).
	// Emit the conventional log-group name so detail-view drill-through works.
	lg := "/aws/athena/" + res.ID
	return relatedResultTrunc("logs", []string{lg}, false)
}

// checkAthenaRole calls athena:GetWorkGroup and extracts the ExecutionRole for
// Spark workgroups from Configuration.ExecutionRole.
func checkAthenaRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("role")
	}
	if cfg.ExecutionRole == nil || *cfg.ExecutionRole == "" {
		return resource.ProvenZero("role", "cfg.ExecutionRole")
	}
	return relatedRefs("role", []string{*cfg.ExecutionRole}, refContext(clients, cache, "role"))
}
