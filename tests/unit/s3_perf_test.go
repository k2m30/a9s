package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchS3Buckets should only call ListBuckets — no GetBucketLocation.
// It should accept only a ListBuckets API, not a location API.
func TestFetchS3Buckets_NoGetBucketLocation(t *testing.T) {
	buckets := make([]s3types.Bucket, 100)
	for i := range buckets {
		created := time.Now()
		buckets[i] = s3types.Bucket{
			Name:         aws.String(fmt.Sprintf("bucket-%03d", i)),
			CreationDate: &created,
		}
	}
	listClient := &fakeS3ListBuckets{Output: &s3.ListBucketsOutput{Buckets: buckets}}

	start := time.Now()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPageWithNotifications(context.Background(), listClient, nil, token)
	})
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
