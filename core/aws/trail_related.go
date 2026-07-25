// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_related.go contains CloudTrail trail related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkTrailS3 searches the s3 cache for the bucket this trail writes logs to.
// Pattern C — match S3BucketName from RawStruct against s3 cache IDs (bucket names).
func checkTrailS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.S3BucketName == nil || *trail.S3BucketName == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	bucketName := *trail.S3BucketName

	s3List, truncated, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return resource.ErrorRelated("s3", err)
	}
	if s3List == nil {
		return resource.UnknownRelated("s3")
	}

	var ids []string
	for _, s3Res := range s3List {
		if s3Res.ID == bucketName {
			ids = append(ids, s3Res.ID)
		}
	}
	return relatedResultTrunc("s3", ids, truncated)
}

// checkTrailLogs searches the logs cache for the CloudWatch log group associated
// with this trail via CloudWatchLogsLogGroupArn.
// Pattern C — parse log group name from ARN, match against logs cache IDs.
// ARN format: arn:aws:logs:REGION:ACCOUNT:log-group:NAME:*
func checkTrailLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.CloudWatchLogsLogGroupArn == nil || *trail.CloudWatchLogsLogGroupArn == "" {
		return resource.KnownRelated("logs", nil, false)
	}

	logGroupName := parseTrailLogGroupName(*trail.CloudWatchLogsLogGroupArn)
	if logGroupName == "" {
		return resource.KnownRelated("logs", nil, false)
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == logGroupName {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkTrailSNS searches the sns cache for the topic this trail publishes to.
// Pattern C — match SnsTopicARN against sns cache IDs (topic ARNs).
func checkTrailSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.SnsTopicARN == nil || *trail.SnsTopicARN == "" {
		return resource.KnownRelated("sns", nil, false)
	}
	topicARN := *trail.SnsTopicARN

	snsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns")
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if snsList == nil {
		return resource.UnknownRelated("sns")
	}

	var ids []string
	for _, snsRes := range snsList {
		if snsRes.ID == topicARN {
			ids = append(ids, snsRes.ID)
		}
	}
	return relatedResultTrunc("sns", ids, truncated)
}

// checkTrailKMS searches the kms cache for the key used by this trail.
// Pattern C — match KmsKeyId (ARN or alias) against kms cache IDs and key_id field.
func checkTrailKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.KmsKeyId == nil || *trail.KmsKeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	kmsRef := *trail.KmsKeyId

	kmsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "kms")
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if kmsList == nil {
		return resource.UnknownRelated("kms")
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
	return relatedResultTrunc("kms", ids, truncated)
}

// checkTrailRole extracts the IAM role name from the trail's CloudWatchLogsRoleArn.
// ARN format: arn:aws:iam::ACCOUNT:role/ROLE-NAME
// Pattern F — no cache needed.
func checkTrailRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.CloudWatchLogsRoleArn == nil || *trail.CloudWatchLogsRoleArn == "" {
		return resource.KnownRelated("role", nil, false)
	}
	arn := *trail.CloudWatchLogsRoleArn
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
		return relatedResult("role", []string{arn[idx+1:]})
	}
	return resource.KnownRelated("role", nil, false)
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
