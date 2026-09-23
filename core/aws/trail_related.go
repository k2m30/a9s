// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_related.go contains CloudTrail trail related-resource checker functions.
package aws

import (
	"context"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkTrailS3 searches the s3 cache for the bucket this trail writes logs to.
// Pattern C — match S3BucketName from RawStruct against s3 cache IDs (bucket names).
func checkTrailS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.S3BucketName == nil || *trail.S3BucketName == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("s3")
		}
		return resource.ProvenZero("s3", "trail.S3BucketName")
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
// Pattern C — the log group the ARN names, matched against logs cache IDs.
func checkTrailLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.CloudWatchLogsLogGroupArn == nil || *trail.CloudWatchLogsLogGroupArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("logs")
		}
		return resource.ProvenZero("logs", "trail.CloudWatchLogsLogGroupArn")
	}

	// A trail delivers to a log group in its own home region, which a
	// multi-region trail browsed from elsewhere does not share with the
	// session; the group's ARN names the region holding it.
	groupARN := *trail.CloudWatchLogsLogGroupArn
	region := arnRegionOf(groupARN, "logs")
	logList, rc, truncated, err := relatedListIn(ctx, clients, cache, "logs", region)
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}
	logGroupName, local := resource.ResolveRef("logs", groupARN, rc)
	if !local {
		return relatedResultTrunc("logs", nil, true)
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == logGroupName {
			ids = append(ids, logRes.ID)
		}
	}
	return inRegion(clients, region, relatedResultTrunc("logs", ids, truncated))
}

// checkTrailSNS searches the sns cache for the topic this trail publishes to.
// Pattern C — match SnsTopicARN against sns cache IDs (topic ARNs).
func checkTrailSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.SnsTopicARN == nil || *trail.SnsTopicARN == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("sns")
		}
		return resource.ProvenZero("sns", "trail.SnsTopicARN")
	}
	// A multi-region trail publishes to a topic in its home region, which the
	// topic's ARN names.
	topicARN := *trail.SnsTopicARN
	region := arnRegionOf(topicARN, "sns")
	snsList, rc, truncated, err := relatedListIn(ctx, clients, cache, "sns", region)
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if snsList == nil {
		return resource.UnknownRelated("sns")
	}
	ids, lowerBound := listedRefs("sns", []string{topicARN}, rc, snsList)
	return inRegion(clients, region, relatedResultTrunc("sns", ids, truncated && lowerBound))
}

// checkTrailKMS returns the key in the trail's KmsKeyId (a key ARN or alias).
func checkTrailKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.KmsKeyId == nil || *trail.KmsKeyId == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.ProvenZero("kms", "trail.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*trail.KmsKeyId})
}

// checkTrailRole returns the IAM role in the trail's CloudWatchLogsRoleArn.
// Pattern F — no cache needed.
func checkTrailRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	trail, ok := assertStruct[cloudtrailtypes.Trail](res.RawStruct)
	if !ok || trail.CloudWatchLogsRoleArn == nil || *trail.CloudWatchLogsRoleArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("role")
		}
		return resource.ProvenZero("role", "trail.CloudWatchLogsRoleArn")
	}
	return relatedRefs("role", arnsOnly([]string{*trail.CloudWatchLogsRoleArn}), refContext(clients, cache, "role"))
}
