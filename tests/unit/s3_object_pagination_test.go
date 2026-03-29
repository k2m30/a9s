package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestFetchS3Objects_Paginated verifies the two-call pagination flow for S3
// objects. FetchS3Objects makes exactly one API call per invocation (single-page
// pagination contract). Call 1 returns page 0 with IsTruncated=true. Call 2
// (using NextToken from call 1) returns page 1 with IsTruncated=false.
func TestFetchS3Objects_Paginated(t *testing.T) {
	page0 := []*s3.ListObjectsV2Output{
		{
			IsTruncated:           aws.Bool(true),
			NextContinuationToken: aws.String("token1"),
			CommonPrefixes: []s3types.CommonPrefix{
				{Prefix: aws.String("folder1/")},
			},
			Contents: []s3types.Object{
				{
					Key:          aws.String("file1.txt"),
					Size:         aws.Int64(100),
					StorageClass: s3types.ObjectStorageClassStandard,
				},
			},
		},
	}
	page1 := []*s3.ListObjectsV2Output{
		{
			IsTruncated: aws.Bool(false),
			// No NextContinuationToken — last page
			CommonPrefixes: []s3types.CommonPrefix{
				{Prefix: aws.String("folder2/")},
			},
			Contents: []s3types.Object{
				{
					Key:          aws.String("file2.txt"),
					Size:         aws.Int64(200),
					StorageClass: s3types.ObjectStorageClassStandard,
				},
				{
					Key:          aws.String("file3.txt"),
					Size:         aws.Int64(300),
					StorageClass: s3types.ObjectStorageClassStandard,
				},
			},
		},
	}

	// Call 1: no continuation token — returns page 0 with IsTruncated=true
	mock1 := &mockPaginatedS3ListObjectsV2Client{pages: page0}
	result1, err := awsclient.FetchS3Objects(context.Background(), mock1, "test-bucket", "", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	// Page 0: 1 folder + 1 file = 2 resources
	if len(result1.Resources) != 2 {
		t.Errorf("call 1: expected 2 resources (1 folder + 1 file), got %d", len(result1.Resources))
	}
	if mock1.calls != 1 {
		t.Errorf("call 1: expected 1 API call, got %d", mock1.calls)
	}
	if result1.Pagination == nil {
		t.Fatal("call 1: Pagination is nil")
	}
	if !result1.Pagination.IsTruncated {
		t.Error("call 1: expected IsTruncated=true")
	}
	if result1.Pagination.NextToken == "" {
		t.Error("call 1: expected non-empty NextToken")
	}

	// Call 2: use NextToken from call 1 — returns page 1 with IsTruncated=false
	mock2 := &mockPaginatedS3ListObjectsV2Client{pages: page1}
	result2, err := awsclient.FetchS3Objects(context.Background(), mock2, "test-bucket", "", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	// Page 1: 1 folder + 2 files = 3 resources
	if len(result2.Resources) != 3 {
		t.Errorf("call 2: expected 3 resources (1 folder + 2 files), got %d", len(result2.Resources))
	}
	if mock2.calls != 1 {
		t.Errorf("call 2: expected 1 API call, got %d", mock2.calls)
	}
	if result2.Pagination == nil {
		t.Fatal("call 2: Pagination is nil")
	}
	if result2.Pagination.IsTruncated {
		t.Error("call 2: expected IsTruncated=false")
	}

	// Verify total resources across both calls = 5
	total := len(result1.Resources) + len(result2.Resources)
	if total != 5 {
		t.Errorf("expected 5 total resources across 2 calls, got %d", total)
	}

	// Verify all expected resource IDs are present across both pages
	ids := map[string]bool{}
	for _, r := range result1.Resources {
		ids[r.ID] = true
	}
	for _, r := range result2.Resources {
		ids[r.ID] = true
	}
	expected := []string{"folder1/", "file1.txt", "folder2/", "file2.txt", "file3.txt"}
	for _, e := range expected {
		if !ids[e] {
			t.Errorf("missing resource %q", e)
		}
	}
}
