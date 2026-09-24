// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// s3_cross_region.go — Shared detection of S3 cross-region API rejections.
//
// ListBuckets returns ALL buckets globally regardless of the configured S3
// client's region, but per-bucket calls (GetBucketTagging, GetBucketEncryption,
// GetBucketLogging, GetBucketPolicy, GetPublicAccessBlock, ...) require the
// bucket's own regional endpoint. AWS rejects with PermanentRedirect (301)
// or IllegalLocationConstraintException (400) when the configured client
// region differs from the bucket's region. These are legitimate environmental
// conditions on multi-region accounts, not bugs.
//
// s3For sends each per-bucket call to the bucket's own region. The classifier
// below answers for a bucket whose region could not be learned at all: its
// call goes to the session's endpoint and is rejected there.
//
// Two call sites use this classifier:
//
//  1. Related-def checkers in s3_related.go — return a truncated (0+) result
//     (renders as "0+") instead of an UnknownRelated result. The bucket
//     exists and was scanned; we just cannot see across regions for the
//     per-bucket detail call. The honest answer is "0 known matches, more
//     may exist beyond what we could see."
//
//  2. EnrichS3Posture in s3_issue_enrichment.go — mark the bucket's
//     ID in TruncatedIDs (row "?") but skip the failure-aggregate entry so
//     the `!` log stays quiet on multi-region accounts.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3For returns the S3 client that reads bucket, which is the client of the
// region the bucket lives in.
func (c *ServiceClients) s3For(ctx context.Context, bucket string) S3API {
	return c.InRegion(c.bucketRegion(ctx, bucket)).S3
}

// learnBucketRegions records where each listed bucket lives. ListBuckets is
// answered from any region and names each bucket's own, so the call that
// brings the list in spares every bucket on it a lookup.
func (c *ServiceClients) learnBucketRegions(buckets []s3types.Bucket) {
	c.bucketRegionMu.Lock()
	defer c.bucketRegionMu.Unlock()
	if c.bucketRegions == nil {
		c.bucketRegions = map[string]string{}
	}
	for _, b := range buckets {
		name, region := aws.ToString(b.Name), aws.ToString(b.BucketRegion)
		if name != "" && region != "" {
			c.bucketRegions[name] = region
		}
	}
}

// bucketRegion returns the region bucket lives in, or "" when no answer was
// had. s3:GetBucketLocation is answered from any region, and a bucket cannot
// move, so its answer is kept for the life of the session.
func (c *ServiceClients) bucketRegion(ctx context.Context, bucket string) string {
	if bucket == "" || c.oneRegion() {
		return ""
	}
	c.bucketRegionMu.Lock()
	region, known := c.bucketRegions[bucket]
	c.bucketRegionMu.Unlock()
	if known {
		return region
	}
	api, ok := c.S3.(S3GetBucketLocationAPI)
	if !ok {
		return ""
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLocationOutput, error) {
		return api.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	})
	if err != nil {
		// A call that did not answer is no answer to remember: memoising it
		// would send every later read of this bucket to an endpoint that
		// rejects it, for as long as the session lasts.
		return ""
	}
	// "Buckets in Region us-east-1 have a LocationConstraint of null. Buckets
	// with a LocationConstraint of EU reside in eu-west-1."
	// (https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetBucketLocation.html)
	switch region = string(out.LocationConstraint); region {
	case "":
		region = "us-east-1"
	case string(s3types.BucketLocationConstraintEu):
		region = "eu-west-1"
	}
	c.bucketRegionMu.Lock()
	defer c.bucketRegionMu.Unlock()
	if c.bucketRegions == nil {
		c.bucketRegions = map[string]string{}
	}
	c.bucketRegions[bucket] = region
	return region
}

// s3PerBucket is the S3 client the bucket-list and object-list fetchers take:
// they are handed one client for a call about a bucket named in the request,
// and each such call belongs in that bucket's own region.
type s3PerBucket struct{ c *ServiceClients }

func (p s3PerBucket) ListBuckets(ctx context.Context, in *s3.ListBucketsInput, opts ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	if p.c.S3 == nil {
		return nil, errClientMissing
	}
	out, err := p.c.S3.ListBuckets(ctx, in, opts...)
	if out != nil {
		p.c.learnBucketRegions(out.Buckets)
	}
	return out, err
}

func (p s3PerBucket) GetBucketNotificationConfiguration(ctx context.Context, in *s3.GetBucketNotificationConfigurationInput, opts ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return p.c.s3For(ctx, aws.ToString(in.Bucket)).GetBucketNotificationConfiguration(ctx, in, opts...)
}

func (p s3PerBucket) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return p.c.s3For(ctx, aws.ToString(in.Bucket)).ListObjectsV2(ctx, in, opts...)
}

// isS3CrossRegionErr reports whether err is the S3 cross-region rejection
// pair: PermanentRedirect (301) or IllegalLocationConstraintException (400).
// Both indicate the configured S3 client's region does not match the target
// bucket's region — not a bug, just multi-region account topology.
func isS3CrossRegionErr(err error) bool {
	return ErrCodeIs(err, "PermanentRedirect", "IllegalLocationConstraintException")
}
