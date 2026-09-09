// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_related.go contains Athena WorkGroup related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// athenaWorkGroupConfig fetches Configuration for a workgroup by name (Pattern
// C helper). Returns nil on any failure so callers can emit an unknown result.
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
// Pattern C — single API call per checker.
func checkAthenaS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("s3")
	}
	if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.OutputLocation == nil {
		return resource.KnownRelated("s3", nil, false)
	}
	bucket := bucketFromS3URI(*cfg.ResultConfiguration.OutputLocation)
	if bucket == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	return relatedResult("s3", []string{bucket})
}

// checkAthenaKMS calls athena:GetWorkGroup and extracts the KMS key ID from
// Configuration.ResultConfiguration.EncryptionConfiguration.KmsKey. Pattern C.
func checkAthenaKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("kms")
	}
	if cfg.ResultConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == nil ||
		*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsKeyIDFromField(*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey, res.Type)
	return relatedResult("kms", []string{keyID})
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
		return resource.KnownRelated("logs", nil, false)
	}
	// Athena publishes metrics but the log group is implicit (/aws/athena/<WG>).
	// Emit the conventional log-group name so detail-view drill-through works.
	lg := "/aws/athena/" + res.ID
	return relatedResult("logs", []string{lg})
}

// checkAthenaRole calls athena:GetWorkGroup and extracts the ExecutionRole for
// Spark workgroups from Configuration.ExecutionRole. Pattern C.
func checkAthenaRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.UnknownRelated("role")
	}
	if cfg.ExecutionRole == nil || *cfg.ExecutionRole == "" {
		return resource.KnownRelated("role", nil, false)
	}
	roleARN := *cfg.ExecutionRole
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}
	return relatedResult("role", []string{roleName})
}

// bucketFromS3URI extracts the bucket name from an s3:// URI.
// Returns "" for non-s3 URIs or malformed input.
func bucketFromS3URI(uri string) string {
	const prefix = "s3://"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	rest := uri[len(prefix):]
	if bucket, _, ok := strings.Cut(rest, "/"); ok {
		return bucket
	}
	return rest
}
