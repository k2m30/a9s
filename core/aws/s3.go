// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// S3HeadBucketSaysMissing reports whether a HeadBucket error proves the
// bucket does not exist. Only "not found" does. A HeadBucket denied by
// permissions answers about this session's access, not about the bucket:
// AWS returns it precisely because the bucket is there and belongs to someone
// else, so reading it as absence reports every cross-account bucket deleted.
// A nil error is not a missing bucket either.
func S3HeadBucketSaysMissing(err error) bool {
	var notFound *s3types.NotFound
	return errors.As(err, &notFound)
}

// FetchS3BucketsPageWithNotifications returns one page of buckets and, when
// available, enriches each bucket with notification targets.
func FetchS3BucketsPageWithNotifications(
	ctx context.Context,
	listAPI S3ListBucketsAPI,
	notificationAPI S3GetBucketNotificationConfigurationAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &s3.ListBucketsInput{
		MaxBuckets: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.ContinuationToken = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.ListBucketsOutput, error) {
		return listAPI.ListBuckets(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching S3 buckets: %w", err)
	}

	var resources []resource.Resource
	var failures []Failure
	for _, bucket := range output.Buckets {
		bucketName := ""
		if bucket.Name != nil {
			bucketName = *bucket.Name
		}

		creationDate := ""
		if bucket.CreationDate != nil {
			creationDate = bucket.CreationDate.Format("2006-01-02 15:04")
		}
		lambdaArns, sqsArns, snsArns := "", "", ""
		notificationError, notificationTruncated := "", ""
		if notificationAPI != nil && bucketName != "" {
			var notificationErr error
			lambdaArns, sqsArns, snsArns, notificationErr = s3NotificationTargets(ctx, notificationAPI, bucketName)
			switch {
			case notificationErr == nil:
			// A bucket outside this client's region is operational: the
			// destinations are unknown, not absent, and the pivots render a
			// soft-truncated zero rather than a refusal in the `!` log.
			case isS3CrossRegionErr(notificationErr):
				notificationTruncated = "true"
			default:
				notificationError = notificationErr.Error()
				failures = append(failures, FailedCall(bucketName, notificationErr))
			}
		}

		r := resource.Resource{
			ID:   bucketName,
			Name: bucketName,
			Fields: map[string]string{
				"name":                   bucketName,
				"creation_date":          creationDate,
				"notification_lambda":    lambdaArns,
				"notification_sqs":       sqsArns,
				"notification_sns":       snsArns,
				"notification_error":     notificationError,
				"notification_truncated": notificationTruncated,
			},
			RawStruct: bucket,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.ContinuationToken != nil && *output.ContinuationToken != "" {
		nextToken = *output.ContinuationToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, AggregateFailures("GetBucketNotificationConfiguration", failures, len(output.Buckets))
}

// s3NotificationTargets returns every destination of each kind the bucket
// notifies, comma-joined in the order the API listed them. A bucket may fan one
// event out to several functions, queues or topics, and reporting only the
// first hides the rest of the blast radius from every pivot that joins on
// these fields. ARNs carry no commas, so the join is lossless.
//
// A call that did not answer returns the reason: an empty list read as "this
// bucket notifies nothing" is a confident zero drawn from a call nobody
// completed.
func s3NotificationTargets(
	ctx context.Context,
	api S3GetBucketNotificationConfigurationAPI,
	bucket string,
) (lambdaArns, sqsArns, snsArns string, _ error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketNotificationConfigurationOutput, error) {
		return api.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
			Bucket: aws.String(bucket),
		})
	})
	if err != nil {
		return "", "", "", err
	}
	if out == nil {
		return "", "", "", UnusableAnswerErr{
			Call:  "GetBucketNotificationConfiguration",
			Field: "notification configuration",
		}
	}
	var lambdas, queues, topics []string
	for _, c := range out.LambdaFunctionConfigurations {
		if arn := aws.ToString(c.LambdaFunctionArn); arn != "" {
			lambdas = append(lambdas, arn)
		}
	}
	for _, c := range out.QueueConfigurations {
		if arn := aws.ToString(c.QueueArn); arn != "" {
			queues = append(queues, arn)
		}
	}
	for _, c := range out.TopicConfigurations {
		if arn := aws.ToString(c.TopicArn); arn != "" {
			topics = append(topics, arn)
		}
	}
	return strings.Join(lambdas, ","), strings.Join(queues, ","), strings.Join(topics, ","), nil
}

// FetchS3Objects calls the S3 ListObjectsV2 API with the given bucket and prefix.
// It returns folders (CommonPrefixes) and files (Contents) as a FetchResult.
// A single API call is made per invocation; IsTruncated and NextContinuationToken
// are forwarded as pagination metadata for the caller to request the next page.
func FetchS3Objects(ctx context.Context, api S3ListObjectsV2API, bucket, prefix string, continuationToken string) (resource.FetchResult, error) {
	if bucket == "" {
		// No bucket context to list — return an empty result rather than
		// calling ListObjectsV2("") (which surfaces a spurious NoSuchBucket).
		return resource.FetchResult{}, nil
	}
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	if continuationToken != "" {
		input.ContinuationToken = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.ListObjectsV2Output, error) {
		return api.ListObjectsV2(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching S3 objects: %w", err)
	}

	var resources []resource.Resource

	for _, cp := range output.CommonPrefixes {
		folderKey := ""
		if cp.Prefix != nil {
			folderKey = *cp.Prefix
		}

		// Fields["kind"] is the structural folder/file marker the production
		// DrillCondition reads.
		r := resource.Resource{
			ID:   folderKey,
			Name: folderKey,
			Fields: map[string]string{
				"key":           folderKey,
				"size":          domain.NotApplicable,
				"size_raw":      domain.NotApplicable,
				"last_modified": domain.NotApplicable,
				"storage_class": domain.NotApplicable,
				"kind":          "folder",
				"bucket":        bucket,
			},
			RawStruct: cp,
		}
		resources = append(resources, r)
	}

	for _, obj := range output.Contents {
		objKey := ""
		if obj.Key != nil {
			objKey = *obj.Key
		}

		size, sizeRaw := "", ""
		if obj.Size != nil {
			size = formatSize(*obj.Size)
			sizeRaw = strconv.FormatInt(*obj.Size, 10)
		}

		lastModified := ""
		if obj.LastModified != nil {
			lastModified = obj.LastModified.Format("2006-01-02 15:04")
		}

		storageClass := string(obj.StorageClass)

		r := resource.Resource{
			ID:   objKey,
			Name: objKey,
			Fields: map[string]string{
				"key":           objKey,
				"size":          size,
				"size_raw":      sizeRaw,
				"last_modified": lastModified,
				"storage_class": storageClass,
				"kind":          "file",
				"bucket":        bucket,
			},
			RawStruct: obj,
		}
		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.IsTruncated != nil && *output.IsTruncated && output.NextContinuationToken != nil {
		nextToken = *output.NextContinuationToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// formatSize converts a byte count to a human-readable string.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
