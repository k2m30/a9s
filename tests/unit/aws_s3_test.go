package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T041 - Test S3 bucket listing
// ---------------------------------------------------------------------------

func TestFetchS3Buckets_ParsesMultipleBuckets(t *testing.T) {
	creationDate := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	listMock := &mockS3ListBucketsClient{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("my-data-bucket"), CreationDate: &creationDate},
				{Name: aws.String("my-logs-bucket"), CreationDate: &creationDate},
				{Name: aws.String("my-config-bucket"), CreationDate: &creationDate},
			},
		},
	}

	resources, err := awsclient.FetchS3Buckets(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	r0 := resources[0]
	if r0.ID != "my-data-bucket" {
		t.Errorf("resource[0].ID: expected %q, got %q", "my-data-bucket", r0.ID)
	}
	if r0.Fields["name"] != "my-data-bucket" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "my-data-bucket", r0.Fields["name"])
	}
	if r0.Fields["creation_date"] != "2025-01-15T10:30:00Z" {
		t.Errorf("resource[0].Fields[\"creation_date\"] = %q, want %q", r0.Fields["creation_date"], "2025-01-15T10:30:00Z")
	}
}

func TestFetchS3Buckets_ErrorResponse(t *testing.T) {
	listMock := &mockS3ListBucketsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchS3Buckets(context.Background(), listMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchS3Buckets_EmptyResponse(t *testing.T) {
	listMock := &mockS3ListBucketsClient{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{},
		},
	}

	resources, err := awsclient.FetchS3Buckets(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T042 - Test S3 object listing
// ---------------------------------------------------------------------------

func TestFetchS3Objects_ParsesFoldersAndFiles(t *testing.T) {
	lastModified := time.Date(2025, 3, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockS3ListObjectsV2Client{
		output: &s3.ListObjectsV2Output{
			CommonPrefixes: []s3types.CommonPrefix{
				{Prefix: aws.String("folder1/")},
				{Prefix: aws.String("folder2/")},
			},
			Contents: []s3types.Object{
				{
					Key:          aws.String("file1.txt"),
					Size:         aws.Int64(1024),
					LastModified: &lastModified,
					StorageClass: s3types.ObjectStorageClassStandard,
				},
				{
					Key:          aws.String("file2.json"),
					Size:         aws.Int64(2048576),
					LastModified: &lastModified,
					StorageClass: s3types.ObjectStorageClassStandardIa,
				},
				{
					Key:          aws.String("file3.log"),
					Size:         aws.Int64(5368709120),
					LastModified: &lastModified,
					StorageClass: s3types.ObjectStorageClassGlacier,
				},
			},
		},
	}

	resources, err := awsclient.FetchS3Objects(context.Background(), mock, "my-bucket", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 5 {
		t.Fatalf("expected 5 resources, got %d", len(resources))
	}

	r0 := resources[0]
	if r0.ID != "folder1/" {
		t.Errorf("resource[0].ID: expected %q, got %q", "folder1/", r0.ID)
	}

	r2 := resources[2]
	if r2.ID != "file1.txt" {
		t.Errorf("resource[2].ID: expected %q, got %q", "file1.txt", r2.ID)
	}

	fileFields := []string{"key", "size", "last_modified", "storage_class"}
	for i := 2; i < 5; i++ {
		for _, key := range fileFields {
			if _, ok := resources[i].Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r2.Fields["storage_class"] != "STANDARD" {
		t.Errorf("resource[2].storage_class: expected %q, got %q", "STANDARD", r2.Fields["storage_class"])
	}
}

func TestFetchS3Objects_ErrorResponse(t *testing.T) {
	mock := &mockS3ListObjectsV2Client{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchS3Objects(context.Background(), mock, "my-bucket", "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchS3Objects_EmptyResponse(t *testing.T) {
	mock := &mockS3ListObjectsV2Client{
		output: &s3.ListObjectsV2Output{
			CommonPrefixes: []s3types.CommonPrefix{},
			Contents:       []s3types.Object{},
		},
	}

	resources, err := awsclient.FetchS3Objects(context.Background(), mock, "my-bucket", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
