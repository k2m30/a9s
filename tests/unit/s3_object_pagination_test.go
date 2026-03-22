package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func TestFetchS3Objects_Paginated(t *testing.T) {
	mock := &mockPaginatedS3ListObjectsV2Client{
		pages: []*s3.ListObjectsV2Output{
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
		},
	}

	resources, err := awsclient.FetchS3Objects(context.Background(), mock, "test-bucket", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Page 1: 1 folder + 1 file = 2
	// Page 2: 1 folder + 2 files = 3
	// Total: 5 resources
	if len(resources) != 5 {
		t.Errorf("expected 5 resources across 2 pages, got %d", len(resources))
	}

	if mock.calls != 2 {
		t.Errorf("expected 2 API calls (2 pages), got %d", mock.calls)
	}

	// Verify all resource IDs are present
	ids := map[string]bool{}
	for _, r := range resources {
		ids[r.ID] = true
	}
	expected := []string{"folder1/", "file1.txt", "folder2/", "file2.txt", "file3.txt"}
	for _, e := range expected {
		if !ids[e] {
			t.Errorf("missing resource %q", e)
		}
	}
}
