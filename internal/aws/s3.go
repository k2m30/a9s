package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("s3", []string{
		"name",
		"bucket_name",
		"creation_date",
		"notification_lambda",
		"notification_sqs",
		"notification_sns",
	})

	resource.RegisterFieldKeys("s3_objects", []string{
		"key",
		"size",
		"last_modified",
		"storage_class",
	})

	resource.RegisterPaginated("s3", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		// Related-panel contract (docs/resources/s3.md §2): lambda/sns/sqs
		// pivots must resolve non-zero when this bucket has a matching
		// notification target. Those checkers read Fields["notification_*"],
		// which can only be populated by GetBucketNotificationConfiguration
		// — so the list path must run it per-bucket. Accepts N+1 per page
		// (cheap API, typically ≤50 buckets per AWS account) in exchange
		// for having the notification pivots actually work.
		return FetchS3BucketsPageWithNotifications(ctx, c.S3, c.S3, continuationToken)
	})

	// Register S3 objects as a child type with its own fetcher.
	resource.RegisterPaginatedChild("s3_objects", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchS3Objects(ctx, c.S3, parentCtx["bucket"], parentCtx["prefix"], continuationToken)
	})
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
		// RelatedContextFromIDs extracts the bucket name from related IDs encoded as
		// "bucket|key". Used when navigating to s3_objects from the related panel
		// (e.g., from a CloudTrail event detail view).
		RelatedContextFromIDs: func(relatedIDs []string) map[string]string {
			for _, id := range relatedIDs {
				parts := strings.SplitN(id, "|", 2)
				if len(parts) != 2 || parts[0] == "" {
					continue
				}
				bucket := parts[0]
				key := parts[1]
				// Derive the prefix (folder path) from the key so the child view
				// lands on the folder containing the object, not the bucket root.
				// Example: key="prod/config.json" → prefix="prod/"
				// Example: key="landing/2026/04/07/x.parquet" → prefix="landing/2026/04/07/"
				// Example: key="build-4821.tar.gz" → prefix=""
				prefix := ""
				if idx := strings.LastIndex(key, "/"); idx >= 0 {
					prefix = key[:idx+1]
				}
				return map[string]string{"bucket": bucket, "prefix": prefix}
			}
			return map[string]string{"bucket": "", "prefix": ""}
		},
	})
}

// FetchS3Buckets calls the S3 ListBuckets API and returns all pages of buckets.
// Used by tests; the production path uses the per-page fetcher for pagination.
func FetchS3Buckets(ctx context.Context, listAPI S3ListBucketsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchS3BucketsPage(ctx, listAPI, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchS3BucketsPage calls the S3 ListBuckets API and returns a single page
// of buckets. Pass an empty continuationToken for the first page.
func FetchS3BucketsPage(ctx context.Context, listAPI S3ListBucketsAPI, continuationToken string) (resource.FetchResult, error) {
	return FetchS3BucketsPageWithNotifications(ctx, listAPI, nil, continuationToken)
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
	for _, bucket := range output.Buckets {
		bucketName := ""
		if bucket.Name != nil {
			bucketName = *bucket.Name
		}

		creationDate := ""
		if bucket.CreationDate != nil {
			creationDate = bucket.CreationDate.Format("2006-01-02 15:04")
		}
		lambdaArn, sqsArn, snsArn := "", "", ""
		if notificationAPI != nil && bucketName != "" {
			lambdaArn, sqsArn, snsArn, _ = firstS3NotificationTargets(ctx, notificationAPI, bucketName)
		}

		r := resource.Resource{
			ID:     bucketName,
			Name:   bucketName,
			Status: "",
			Fields: map[string]string{
				"name":                bucketName,
				"bucket_name":         bucketName,
				"creation_date":       creationDate,
				"notification_lambda": lambdaArn,
				"notification_sqs":    sqsArn,
				"notification_sns":    snsArn,
			},
			RawStruct: bucket,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
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
	}, nil
}

func firstS3NotificationTargets(
	ctx context.Context,
	api S3GetBucketNotificationConfigurationAPI,
	bucket string,
) (lambdaArn, sqsArn, snsArn string, _ error) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketNotificationConfigurationOutput, error) {
		return api.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
			Bucket: aws.String(bucket),
		})
	})
	if err != nil {
		// Best effort enrichment: keep list results even if this lookup fails.
		return "", "", "", nil
	}
	for _, c := range out.LambdaFunctionConfigurations {
		if c.LambdaFunctionArn != nil && *c.LambdaFunctionArn != "" {
			lambdaArn = *c.LambdaFunctionArn
			break
		}
	}
	for _, c := range out.QueueConfigurations {
		if c.QueueArn != nil && *c.QueueArn != "" {
			sqsArn = *c.QueueArn
			break
		}
	}
	for _, c := range out.TopicConfigurations {
		if c.TopicArn != nil && *c.TopicArn != "" {
			snsArn = *c.TopicArn
			break
		}
	}
	return lambdaArn, sqsArn, snsArn, nil
}

// FetchS3Objects calls the S3 ListObjectsV2 API with the given bucket and prefix.
// It returns folders (CommonPrefixes) and files (Contents) as a FetchResult.
// A single API call is made per invocation; IsTruncated and NextContinuationToken
// are forwarded as pagination metadata for the caller to request the next page.
func FetchS3Objects(ctx context.Context, api S3ListObjectsV2API, bucket, prefix string, continuationToken string) (resource.FetchResult, error) {
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

	// Add folders (CommonPrefixes) first
	for _, cp := range output.CommonPrefixes {
		folderKey := ""
		if cp.Prefix != nil {
			folderKey = *cp.Prefix
		}

		r := resource.Resource{
			ID:     folderKey,
			Name:   folderKey,
			Status: "folder",
			Fields: map[string]string{
				"key":           folderKey,
				"size":          "",
				"last_modified": "",
				"storage_class": "",
			},
			RawStruct: cp,
		}
		resources = append(resources, r)
	}

	// Add files (Contents)
	for _, obj := range output.Contents {
		objKey := ""
		if obj.Key != nil {
			objKey = *obj.Key
		}

		size := ""
		if obj.Size != nil {
			size = formatSize(*obj.Size)
		}

		lastModified := ""
		if obj.LastModified != nil {
			lastModified = obj.LastModified.Format("2006-01-02 15:04")
		}

		storageClass := string(obj.StorageClass)

		r := resource.Resource{
			ID:     objKey,
			Name:   objKey,
			Status: "file",
			Fields: map[string]string{
				"key":           objKey,
				"size":          size,
				"last_modified": lastModified,
				"storage_class": storageClass,
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
