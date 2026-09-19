package unit_test

// All three cf↔s3 sites read an origin hostname through the one S3 endpoint
// parser and require the bucket to equal the parsed name.
//
// Splitting a hostname on its first ".s3" claims any host that merely carries
// the token: a proxy at assets.s3-proxy.example.com reads as bucket "assets",
// and a bucket whose own name contains an s3 label is cut short. Both give the
// operator a related row pointing at a resource that has nothing to do with
// the distribution.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	row4DistID     = "E1ACME234567EX"
	row4ProxyHost  = "assets.s3-proxy.example.com"
	row4AssetsHost = "my-assets.s3.amazonaws.com"
	row4DottedHost = "my.dotted.bucket.s3.us-east-1.amazonaws.com"
)

func row4Distribution(origins ...string) resource.Resource {
	items := make([]cftypes.Origin, 0, len(origins))
	for i, host := range origins {
		items = append(items, cftypes.Origin{
			Id:         aws.String("origin-" + string(rune('a'+i))),
			DomainName: aws.String(host),
		})
	}
	return resource.Resource{
		ID:     row4DistID,
		Name:   row4DistID,
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Id:         aws.String(row4DistID),
			DomainName: aws.String("d111111abcdef8.cloudfront.net"),
			Origins:    &cftypes.Origins{Items: items, Quantity: aws.Int32(int32(len(items)))},
		},
	}
}

func row4BucketCache(ids ...string) resource.ResourceCache {
	list := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		list = append(list, resource.Resource{ID: id, Name: id, Fields: map[string]string{"name": id}})
	}
	return resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: list}}
}

func row4CFCache(dist resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{dist}}}
}

func TestS3_0916_Row4_CFToS3_OnlyTheParsedBucketMatches(t *testing.T) {
	dist := row4Distribution(row4ProxyHost, row4AssetsHost)
	got := checkerByTarget(t, "cf", "s3")(context.Background(), nil, dist, row4BucketCache("assets", "my-assets"))

	if got.Count() != 1 {
		t.Fatalf("Count = %d, want 1 — %q is not an S3 endpoint", got.Count(), row4ProxyHost)
	}
	if ids := got.ResourceIDs(); ids[0] != "my-assets" {
		t.Fatalf("ResourceIDs = %v, want [my-assets]", ids)
	}
}

func TestS3_0916_Row4_S3ToCF_ProxyOriginIsNotThisBucket(t *testing.T) {
	cache := row4CFCache(row4Distribution(row4ProxyHost, row4AssetsHost))
	checker := checkerByTarget(t, "s3", "cf")

	t.Run("bucket named in the proxy host", func(t *testing.T) {
		bucket := resource.Resource{ID: "assets", Name: "assets"}
		got := checker(context.Background(), nil, bucket, cache)
		if got.Count() != 0 {
			t.Fatalf("Count = %d, want 0 — no origin addresses bucket %q", got.Count(), "assets")
		}
	})

	t.Run("bucket actually served", func(t *testing.T) {
		bucket := resource.Resource{ID: "my-assets", Name: "my-assets"}
		got := checker(context.Background(), nil, bucket, cache)
		if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != row4DistID {
			t.Fatalf("ResourceIDs = %v, want [%s]", ids, row4DistID)
		}
	})
}

func TestS3_0916_Row4_DottedBucketMatchesBothDirections(t *testing.T) {
	const dotted = "my.dotted.bucket"
	dist := row4Distribution(row4DottedHost)

	forward := checkerByTarget(t, "cf", "s3")(context.Background(), nil, dist, row4BucketCache(dotted))
	if ids := forward.ResourceIDs(); len(ids) != 1 || ids[0] != dotted {
		t.Fatalf("cf→s3 ResourceIDs = %v, want [%s]", ids, dotted)
	}

	bucket := resource.Resource{ID: dotted, Name: dotted}
	back := checkerByTarget(t, "s3", "cf")(context.Background(), nil, bucket, row4CFCache(dist))
	if ids := back.ResourceIDs(); len(ids) != 1 || ids[0] != row4DistID {
		t.Fatalf("s3→cf ResourceIDs = %v, want [%s]", ids, row4DistID)
	}
}

// row4LoggingClients wires a distribution config whose logging bucket is the
// given host.
func row4LoggingClients(host string) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{CloudFront: &fakeCloudFrontAPI{
		GetConfigOutput: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				Logging: &cftypes.LoggingConfig{
					Enabled:        aws.Bool(true),
					Bucket:         aws.String(host),
					Prefix:         aws.String("cf/"),
					IncludeCookies: aws.Bool(false),
				},
			},
		},
	}}
}

// TestS3_0916_Row4_LoggingBucketIsTheParsedName: the standard-logging
// bucket counts under S3 Buckets (docs/resources/cf.md), read by the same rule
// as an origin host.
func TestS3_0916_Row4_LoggingBucketIsTheParsedName(t *testing.T) {
	checker := checkerByTarget(t, "cf", "s3")
	dist := row4Distribution(row4ProxyHost)

	for host, want := range map[string]string{
		"logs.acme.s3.amazonaws.com":       "logs.acme",
		"acme.s3-archive.s3.amazonaws.com": "acme.s3-archive",
	} {
		t.Run(host, func(t *testing.T) {
			got := checker(context.Background(), row4LoggingClients(host), dist, row4BucketCache(want, "acme"))
			if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != want {
				t.Fatalf("ResourceIDs = %v, want [%s] — the endpoint marker is the last s3 label, not the first", ids, want)
			}
		})
	}

	t.Run("host that is not an S3 endpoint", func(t *testing.T) {
		got := checker(context.Background(), row4LoggingClients(row4ProxyHost), dist, row4BucketCache("assets"))
		if got.Count() != 0 {
			t.Fatalf("Count = %d, want 0 — %q addresses no bucket", got.Count(), row4ProxyHost)
		}
	})
}
