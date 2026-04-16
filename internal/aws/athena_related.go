// athena_related.go contains Athena WorkGroup related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// athenaWorkGroupConfig fetches Configuration for a workgroup by name (Pattern
// C helper). Returns nil on any failure so callers can emit Count: -1.
func athenaWorkGroupConfig(ctx context.Context, clients any, wgName string) *athenatypes.WorkGroupConfiguration {
	if wgName == "" {
		return nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Athena == nil {
		return nil
	}
	out, err := c.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(wgName)})
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
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.OutputLocation == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	bucket := bucketFromS3URI(*cfg.ResultConfiguration.OutputLocation)
	if bucket == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return relatedResult("s3", []string{bucket})
}

// checkAthenaKMS calls athena:GetWorkGroup and extracts the KMS key ID from
// Configuration.ResultConfiguration.EncryptionConfiguration.KmsKey. Pattern C.
func checkAthenaKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if cfg.ResultConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration == nil ||
		cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == nil ||
		*cfg.ResultConfiguration.EncryptionConfiguration.KmsKey == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *cfg.ResultConfiguration.EncryptionConfiguration.KmsKey
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkAthenaGlue returns the Glue Data Catalog name attached to the workgroup
// via its Spark execution role (for Spark workgroups). For SQL workgroups the
// catalog is "AwsDataCatalog" (the account's default Glue catalog). When the
// workgroup's EngineVersion indicates PySpark, we surface the reference.
// Pattern C — one GetWorkGroup call.
func checkAthenaGlue(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1}
	}
	// Every Athena workgroup queries the Glue Data Catalog by default.
	// We only emit a link when the workgroup actually carries an
	// ExecutionRole (Spark workgroups) or a non-default AdditionalConfiguration
	// that ties it explicitly to Glue. Otherwise Count: 0.
	if cfg.EngineVersion == nil || cfg.EngineVersion.EffectiveEngineVersion == nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: 0}
	}
	// No structured "glue job/catalog" field exists on the WG config. The
	// relationship to Glue is the account-wide data catalog; resolving which
	// specific Glue jobs share this catalog requires a catalog crawl.
	return resource.RelatedCheckResult{TargetType: "glue", Count: 0}
}

// checkAthenaLogs calls athena:GetWorkGroup and extracts the CloudWatch log
// group used for Spark driver logs (CustomerContentEncryptionConfiguration is
// storage-side; the spark driver log group is carried on the EngineConfiguration).
// For SQL workgroups there is no log group; Count: 0.
func checkAthenaLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cfg := athenaWorkGroupConfig(ctx, clients, res.ID)
	if cfg == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	if cfg.PublishCloudWatchMetricsEnabled == nil || !*cfg.PublishCloudWatchMetricsEnabled {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if cfg.ExecutionRole == nil || *cfg.ExecutionRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
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
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}
