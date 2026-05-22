package unit

// Tests for the S3 related-panel contract: lambda/sns/sqs pivots must resolve
// non-zero when the bucket has a matching notification target. That requires
// the paginated S3 fetcher to call GetBucketNotificationConfiguration per
// bucket and populate Fields["notification_*"].
//
// Prior revision (linked to GitHub issue #220) avoided the notification call
// as an N+1-avoidance measure. That tradeoff was explicitly reversed
// 2026-04-23 per user guidance — "related resources MUST work. if they don't
// it's a bug. simple." — because the quiet alternative was three registered
// pivots (lambda/sns/sqs) that always returned 0. A registered pivot that
// never resolves is a contract bug, not a performance optimization.
//
// The N+1 is bounded: S3 list pages cap at 1000 buckets and most accounts
// hold ≤50 buckets total; GetBucketNotificationConfiguration is a cheap
// unmetered call.

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

	// Side-effect import: triggers init() which calls resource.SetPaginatedForTest("s3", …)
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

// TestS3PaginatedFetcher_FetchesNotificationsPerBucket asserts the new
// contract: exactly one GetBucketNotificationConfiguration call per bucket,
// and the resulting Resource carries notification_* Fields so the lambda,
// sns, and sqs s3-related pivots can resolve via Pattern F cache scan.
func TestS3PaginatedFetcher_FetchesNotificationsPerBucket(t *testing.T) {
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

	clients := &awsclient.ServiceClients{S3: s3Client}

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}
	if len(result.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(result.Resources))
	}

	got := transport.notificationCalls.Load()
	if got != 2 {
		t.Errorf("paginated S3 fetcher made %d GetBucketNotificationConfiguration call(s) for 2 buckets; want exactly 2 — the per-bucket call is required so lambda/sns/sqs s3 pivots resolve", got)
	}

	// Every bucket must carry the notification_* Fields keys (even when
	// empty — the field must exist so checkers can read it without the
	// cache scan misfiring).
	for _, r := range result.Resources {
		for _, key := range []string{"notification_lambda", "notification_sns", "notification_sqs"} {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("bucket %q missing Fields[%q] — required for s3→%s pivot",
					r.ID, key, strings.TrimPrefix(key, "notification_"))
			}
		}
	}
}

// TestS3PaginatedFetcher_NotificationCallScalesLinearly confirms the N+1
// relationship holds at larger bucket counts — a regression guard against
// any future "optimization" that quietly drops notification enrichment.
func TestS3PaginatedFetcher_NotificationCallScalesLinearly(t *testing.T) {
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

	got := transport.notificationCalls.Load()
	if got != 10 {
		t.Errorf("paginated S3 fetcher made %d GetBucketNotificationConfiguration call(s) for 10 buckets; want exactly 10", got)
	}
}
