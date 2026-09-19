package unit_test

// r53→s3 takes the bucket from the record's own name.
//
// A Route 53 alias to an S3 static website targets the bare regional endpoint
// (s3-website-<region>.amazonaws.com); the bucket name is never inside it.
// S3 routes the request by Host header, which is why AWS requires the record
// name to equal the bucket name. Deriving the bucket from the alias target
// therefore reports every zone as having no buckets.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	row3ZoneID     = "Z0AC9ME123456EXAMPLE"
	row3SiteBucket = "acme-site.example.com"
)

// row3R53Fake answers ListResourceRecordSets with the configured record sets.
// Every other Route53API call is left to the embedded nil interface: the
// r53→s3 pivot must reach exactly one call.
type row3R53Fake struct {
	awsclient.Route53API
	sets []r53types.ResourceRecordSet
}

func (f *row3R53Fake) ListResourceRecordSets(
	_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.sets}, nil
}

// row3AliasRecord is an A-record alias, the shape Route 53 returns for an
// S3 website or a CloudFront distribution.
func row3AliasRecord(name, target string) r53types.ResourceRecordSet {
	return r53types.ResourceRecordSet{
		Name: aws.String(name),
		Type: r53types.RRTypeA,
		AliasTarget: &r53types.AliasTarget{
			DNSName:              aws.String(target),
			HostedZoneId:         aws.String("Z3AQBSTGFYJSTF"),
			EvaluateTargetHealth: false,
		},
	}
}

func row3Run(t *testing.T, sets []r53types.ResourceRecordSet, cache resource.ResourceCache) resource.RelatedCheckResult {
	t.Helper()
	clients := &awsclient.ServiceClients{Route53: &row3R53Fake{sets: sets}}
	zone := resource.Resource{ID: row3ZoneID, Name: "example.com."}
	return checkerByTarget(t, "r53", "s3")(context.Background(), clients, zone, cache)
}

func row3S3Cache(truncated bool, bucketIDs ...string) resource.ResourceCache {
	list := make([]resource.Resource, 0, len(bucketIDs))
	for _, id := range bucketIDs {
		list = append(list, resource.Resource{ID: id, Name: id, Fields: map[string]string{"name": id}})
	}
	return resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: list, IsTruncated: truncated}}
}

func TestS3_0916_Row3_LegacyHyphenEndpointResolvesByRecordName(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{row3AliasRecord(row3SiteBucket+".", "s3-website-us-east-1.amazonaws.com.")},
		row3S3Cache(false, row3SiteBucket, "acme-archive"),
	)
	if got.Count() != 1 {
		t.Fatalf("Count = %d, want 1", got.Count())
	}
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != row3SiteBucket {
		t.Fatalf("ResourceIDs = %v, want [%s]", ids, row3SiteBucket)
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false")
	}
}

func TestS3_0916_Row3_DotEndpointResolvesByRecordName(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{row3AliasRecord(row3SiteBucket+".", "s3-website.eu-west-1.amazonaws.com.")},
		row3S3Cache(false, row3SiteBucket),
	)
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != row3SiteBucket {
		t.Fatalf("ResourceIDs = %v, want [%s]", ids, row3SiteBucket)
	}
}

func TestS3_0916_Row3_CloudFrontAliasIsNotABucket(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{row3AliasRecord("cdn.example.com.", "d111111abcdef8.cloudfront.net.")},
		row3S3Cache(false, row3SiteBucket, "cdn.example.com"),
	)
	if got.Count() != 0 {
		t.Fatalf("Count = %d, want 0 — a CloudFront alias names no bucket", got.Count())
	}
	if len(got.ResourceIDs()) != 0 {
		t.Fatalf("ResourceIDs = %v, want none", got.ResourceIDs())
	}
}

func TestS3_0916_Row3_BucketAbsentFromListCarriesItsTruncation(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{row3AliasRecord(row3SiteBucket+".", "s3-website-us-east-1.amazonaws.com.")},
		row3S3Cache(true, "acme-archive"),
	)
	if got.Count() != 0 {
		t.Fatalf("Count = %d, want 0", got.Count())
	}
	if !got.Truncated() {
		t.Error("Truncated = false, want true — the s3 list was truncated, so the bucket may be unseen rather than absent")
	}
}
