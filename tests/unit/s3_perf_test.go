package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

type mockFastListBucketsClient struct {
	count int
}

func (m *mockFastListBucketsClient) ListBuckets(
	ctx context.Context,
	params *s3.ListBucketsInput,
	optFns ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	buckets := make([]s3types.Bucket, m.count)
	for i := range buckets {
		name := fmt.Sprintf("bucket-%03d", i)
		created := time.Now()
		buckets[i] = s3types.Bucket{
			Name:         aws.String(name),
			CreationDate: &created,
		}
	}
	return &s3.ListBucketsOutput{Buckets: buckets}, nil
}

// FetchS3Buckets should only call ListBuckets — no GetBucketLocation.
// It should accept only a ListBuckets API, not a location API.
func TestFetchS3Buckets_NoGetBucketLocation(t *testing.T) {
	listClient := &mockFastListBucketsClient{count: 100}

	start := time.Now()
	resources, err := awsclient.FetchS3Buckets(context.Background(), listClient)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 100 {
		t.Fatalf("expected 100 resources, got %d", len(resources))
	}
	// Should be near-instant since no GetBucketLocation calls
	if elapsed > 1*time.Second {
		t.Errorf("FetchS3Buckets took %v for 100 buckets — should be instant without GetBucketLocation", elapsed)
	}
	t.Logf("FetchS3Buckets: 100 buckets in %v", elapsed)
}
