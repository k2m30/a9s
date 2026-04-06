// trail_related.go contains CloudTrail trail related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("trail", []resource.RelatedDef{
		{TargetType: "s3", DisplayName: "S3 Bucket", Checker: checkTrailS3, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTrailLogs, NeedsTargetCache: true},
		{TargetType: "sns", DisplayName: "SNS Topic", Checker: checkTrailSNS, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkTrailKMS, NeedsTargetCache: true},
	})

	resource.RegisterNavigableFields("trail", []resource.NavigableField{
		{FieldPath: "S3BucketName", TargetType: "s3"},
	})
}

// checkTrailS3 searches the s3 cache for the bucket this trail writes logs to.
// Pattern C — match S3BucketName from RawStruct against s3 cache IDs (bucket names).
func checkTrailS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.S3BucketName == nil || *trail.S3BucketName == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	bucketName := *trail.S3BucketName

	s3List, truncated, err := trailRelatedResources(ctx, clients, cache, "s3")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if s3List == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	var ids []string
	for _, s3Res := range s3List {
		if s3Res.ID == bucketName {
			ids = append(ids, s3Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	return relatedResult("s3", ids)
}

// checkTrailLogs searches the logs cache for the CloudWatch log group associated
// with this trail via CloudWatchLogsLogGroupArn.
// Pattern C — parse log group name from ARN, match against logs cache IDs.
// ARN format: arn:aws:logs:REGION:ACCOUNT:log-group:NAME:*
func checkTrailLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.CloudWatchLogsLogGroupArn == nil || *trail.CloudWatchLogsLogGroupArn == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logGroupName := parseTrailLogGroupName(*trail.CloudWatchLogsLogGroupArn)
	if logGroupName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := trailRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == logGroupName {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkTrailSNS searches the sns cache for the topic this trail publishes to.
// Pattern C — match SnsTopicARN against sns cache IDs (topic ARNs).
func checkTrailSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.SnsTopicARN == nil || *trail.SnsTopicARN == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	topicARN := *trail.SnsTopicARN

	snsList, truncated, err := trailRelatedResources(ctx, clients, cache, "sns")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	if snsList == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	var ids []string
	for _, snsRes := range snsList {
		if snsRes.ID == topicARN {
			ids = append(ids, snsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	return relatedResult("sns", ids)
}

// checkTrailKMS searches the kms cache for the key used by this trail.
// Pattern C — match KmsKeyId (ARN or alias) against kms cache IDs and key_id field.
func checkTrailKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.KmsKeyId == nil || *trail.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	kmsRef := *trail.KmsKeyId

	kmsList, truncated, err := trailRelatedResources(ctx, clients, cache, "kms")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if kmsList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}

	var ids []string
	for _, kmsRes := range kmsList {
		// Match by ID (key UUID), by Fields["key_id"], or by ARN suffix containing the key ID.
		if kmsRes.ID == kmsRef ||
			kmsRes.Fields["key_id"] == kmsRef ||
			strings.Contains(kmsRef, kmsRes.ID) ||
			strings.Contains(kmsRef, kmsRes.Fields["key_id"]) {
			ids = append(ids, kmsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	return relatedResult("kms", ids)
}

// trailRelatedResources returns the resource list for target from cache or by fetching the first page.
func trailRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// parseTrailLogGroupName extracts the log group name from a CloudWatch Logs ARN.
// Expected format: arn:aws:logs:REGION:ACCOUNT:log-group:NAME:*
// Returns the NAME portion, or empty string if parsing fails.
func parseTrailLogGroupName(arn string) string {
	const prefix = "log-group:"
	_, rest, found := strings.Cut(arn, prefix)
	if !found {
		return ""
	}
	// Strip trailing ":*" or ":log-stream:..." suffix
	if name, _, ok := strings.Cut(rest, ":"); ok {
		return name
	}
	return rest
}
