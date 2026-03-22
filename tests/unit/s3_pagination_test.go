package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// BUG: FetchS3Buckets must paginate ListBuckets.
// AWS returns partial results with ContinuationToken when there are many buckets.
// Without pagination, count is wrong and some buckets are missing.

func TestFetchS3Buckets_Paginated(t *testing.T) {
	mock := &mockPaginatedS3ListBucketsClient{
		pages: []*s3.ListBucketsOutput{
			{
				Buckets: []s3types.Bucket{
					{Name: aws.String("bucket-1")},
					{Name: aws.String("bucket-2")},
				},
				ContinuationToken: aws.String("token-1"),
			},
			{
				Buckets: []s3types.Bucket{
					{Name: aws.String("bucket-3")},
				},
				// No ContinuationToken = last page
			},
		},
	}

	resources, err := awsclient.FetchS3Buckets(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 3 {
		t.Errorf("expected 3 buckets across 2 pages, got %d", len(resources))
	}

	if mock.calls != 2 {
		t.Errorf("expected 2 API calls (2 pages), got %d", mock.calls)
	}

	// Verify all bucket names
	names := map[string]bool{}
	for _, r := range resources {
		names[r.ID] = true
	}
	for _, expected := range []string{"bucket-1", "bucket-2", "bucket-3"} {
		if !names[expected] {
			t.Errorf("missing bucket %q", expected)
		}
	}
}
