package unit

// Tests for GitHub issue #220: S3 paginated fetcher issues N+1 API calls
// (GetBucketNotificationConfiguration per bucket) when it should call none.
//
// These tests FAIL against the current implementation and PASS once the fix
// (use FetchS3BucketsPage instead of FetchS3BucketsPageWithNotifications) lands.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"

	// Side-effect import: triggers init() which calls resource.RegisterPaginated("s3", …)
	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// countingS3RoundTripper wraps an inner transport and counts calls to the
// GetBucketNotificationConfiguration endpoint (identified by ?notification in
// the S3 request URL).
type countingS3RoundTripper struct {
	notificationCalls atomic.Int64
	listResponse      string // XML to return for ListBuckets
}

func (t *countingS3RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// GetBucketNotificationConfiguration appends ?notification to the bucket URL.
	if strings.Contains(req.URL.RawQuery, "notification") {
		t.notificationCalls.Add(1)
		// Return a valid empty notification config so the call doesn't error.
		body := `<?xml version="1.0" encoding="UTF-8"?>
<NotificationConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></NotificationConfiguration>`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/xml"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}

	// All other S3 calls (ListBuckets) get the canned bucket list.
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(t.listResponse)),
	}, nil
}

func newS3ClientWithCountingTransport(transport *countingS3RoundTripper) *s3sdk.Client {
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
		HTTPClient:  &http.Client{Transport: transport},
	}
	return s3sdk.NewFromConfig(cfg, func(o *s3sdk.Options) { o.UsePathStyle = true })
}

// TestS3PaginatedFetcher_DoesNotCallNotificationAPI verifies that the registered
// paginated fetcher for "s3" makes zero GetBucketNotificationConfiguration
// calls regardless of how many buckets are returned.
//
// BUG (issue #220): the current implementation calls
// FetchS3BucketsPageWithNotifications which issues one notification API call
// per bucket, turning a single-page list into 1+N API calls.
func TestS3PaginatedFetcher_DoesNotCallNotificationAPI(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("s3")
	if fetcher == nil {
		t.Fatal("paginated fetcher for 's3' not registered — ensure internal/aws package is imported")
	}

	listXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets>
    <Bucket>
      <Name>bucket-alpha</Name>
      <CreationDate>2025-01-01T00:00:00.000Z</CreationDate>
    </Bucket>
    <Bucket>
      <Name>bucket-beta</Name>
      <CreationDate>2025-02-01T00:00:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>`

	transport := &countingS3RoundTripper{listResponse: listXML}
	s3Client := newS3ClientWithCountingTransport(transport)

	clients := &awsclient.ServiceClients{
		S3: s3Client,
	}

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}

	if len(result.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(result.Resources))
	}

	calls := transport.notificationCalls.Load()
	if calls != 0 {
		t.Errorf("paginated S3 fetcher made %d GetBucketNotificationConfiguration call(s); want 0 (N+1 bug: issue #220)", calls)
	}
}

// TestS3PaginatedFetcher_DoesNotCallNotificationAPI_ManyBuckets verifies the
// N+1 fix holds with a larger bucket count (10 buckets → still 0 notification
// calls, not 10).
func TestS3PaginatedFetcher_DoesNotCallNotificationAPI_ManyBuckets(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("s3")
	if fetcher == nil {
		t.Fatal("paginated fetcher for 's3' not registered")
	}

	var bucketEntries strings.Builder
	for i := range 10 {
		bucketEntries.WriteString("<Bucket><Name>bucket-")
		bucketEntries.WriteString(string(rune('a' + i)))
		bucketEntries.WriteString("</Name><CreationDate>2025-01-01T00:00:00.000Z</CreationDate></Bucket>")
	}

	listXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets>` + bucketEntries.String() + `</Buckets>
</ListAllMyBucketsResult>`

	transport := &countingS3RoundTripper{listResponse: listXML}
	s3Client := newS3ClientWithCountingTransport(transport)

	clients := &awsclient.ServiceClients{S3: s3Client}

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}

	if len(result.Resources) != 10 {
		t.Errorf("expected 10 resources, got %d", len(result.Resources))
	}

	calls := transport.notificationCalls.Load()
	if calls != 0 {
		t.Errorf("paginated S3 fetcher made %d GetBucketNotificationConfiguration call(s) for 10 buckets; want 0 (N+1 bug: issue #220)", calls)
	}
}
