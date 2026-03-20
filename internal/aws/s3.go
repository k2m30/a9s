package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("s3", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchS3Buckets(ctx, c.S3)
	})
	resource.RegisterFieldKeys("s3", []string{"name", "bucket_name", "creation_date"})
}

// FetchS3Buckets calls the S3 ListBuckets API and returns a slice of
// generic Resource structs. Region is whatever AWS returns — no extra
// GetBucketLocation calls (region is a pass-through API parameter).
func FetchS3Buckets(ctx context.Context, listAPI S3ListBucketsAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var continuationToken *string

	for {
		output, err := listAPI.ListBuckets(ctx, &s3.ListBucketsInput{
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching S3 buckets: %w", err)
		}

		for _, bucket := range output.Buckets {
		bucketName := ""
		if bucket.Name != nil {
			bucketName = *bucket.Name
		}

		creationDate := ""
		if bucket.CreationDate != nil {
			creationDate = bucket.CreationDate.Format("2006-01-02T15:04:05Z07:00")
		}

		r := resource.Resource{
			ID:     bucketName,
			Name:   bucketName,
			Status: "",
			Fields: map[string]string{
				"name":          bucketName,
				"bucket_name":   bucketName,
				"creation_date": creationDate,
			},
			RawStruct:  bucket,
		}

		resources = append(resources, r)
		}

		// Check for more pages
		if output.ContinuationToken == nil || *output.ContinuationToken == "" {
			break
		}
		continuationToken = output.ContinuationToken
	}

	return resources, nil
}

// FetchS3Objects calls the S3 ListObjectsV2 API with the given bucket and prefix.
// It returns folders (CommonPrefixes) and files (Contents) as Resource structs.
// It paginates using IsTruncated and NextContinuationToken until all pages are fetched.
func FetchS3Objects(ctx context.Context, api S3ListObjectsV2API, bucket, prefix string) ([]resource.Resource, error) {
	var resources []resource.Resource
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			Delimiter:         aws.String("/"),
			ContinuationToken: continuationToken,
		}

		output, err := api.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("fetching S3 objects: %w", err)
		}

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
				RawStruct:  cp,
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
				lastModified = obj.LastModified.Format("2006-01-02T15:04:05Z07:00")
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
				RawStruct:  obj,
			}
			resources = append(resources, r)
		}

		// Check for more pages
		if output.IsTruncated == nil || !*output.IsTruncated || output.NextContinuationToken == nil {
			break
		}
		continuationToken = output.NextContinuationToken
	}

	return resources, nil
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
