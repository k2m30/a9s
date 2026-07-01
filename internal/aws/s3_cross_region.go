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
// Two call sites use this classifier:
//
//   1. Related-def checkers in s3_related.go — return resource.ApproximateZero
//      (renders as "0+") instead of the unknown-marker Count:-1. The bucket
//      exists and was scanned; we just cannot see across regions for the
//      per-bucket detail call. The honest answer is "0 known matches, more
//      may exist beyond what we could see."
//
//   2. EnrichS3PublicAccessBlock in s3_issue_enrichment.go — mark the bucket's
//      ID in TruncatedIDs (row "?") but skip the failure-aggregate entry so
//      the `!` log stays quiet on multi-region accounts.
//
// This regression class was latent: the only checker that classified
// cross-region (the issue enricher) kept its detection inline, so the four
// related-def checkers fell through to Count:-1. Extracting the classifier
// into one helper makes future call-site additions (more per-bucket S3 calls)
// consistent by default.
package aws

import (
	"errors"

	smithy "github.com/aws/smithy-go"
)

// isS3CrossRegionErr reports whether err is the S3 cross-region rejection
// pair: PermanentRedirect (301) or IllegalLocationConstraintException (400).
// Both indicate the configured S3 client's region does not match the target
// bucket's region — not a bug, just multi-region account topology.
func isS3CrossRegionErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.ErrorCode()
	return code == "PermanentRedirect" || code == "IllegalLocationConstraintException"
}
