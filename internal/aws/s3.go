package aws

import (
	"context"
	"encoding/json"
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
			return nil, err
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

		detail := map[string]string{
			"Bucket Name":   bucketName,
			"Creation Date": creationDate,
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(bucket, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
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
			return nil, err
		}

		// Add folders (CommonPrefixes) first
		for _, cp := range output.CommonPrefixes {
			folderKey := ""
			if cp.Prefix != nil {
				folderKey = *cp.Prefix
			}

			// Build DetailData for folder
			detail := map[string]string{
				"Key": folderKey,
			}

			// Build RawJSON for folder
			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(cp, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
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
				DetailData: detail,
				RawJSON:    rawJSON,
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

			etag := ""
			if obj.ETag != nil {
				etag = *obj.ETag
			}

			// Build DetailData for file
			detail := map[string]string{
				"Key":           objKey,
				"Size":          size,
				"Last Modified": lastModified,
				"Storage Class": storageClass,
				"ETag":          etag,
			}

			// Build RawJSON for file
			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(obj, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
			}

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
				DetailData: detail,
				RawJSON:    rawJSON,
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
