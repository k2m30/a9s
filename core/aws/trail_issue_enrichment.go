// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_issue_enrichment.go — Wave 2 issue enrichment for the trail resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// trailLogBucketAPI is the pair of read-only S3 calls the log-bucket rows
// need. Asserted off clients.S3 rather than folded into the aggregate, so a
// client or fake that predates these rows still satisfies it.
type trailLogBucketAPI interface {
	S3GetBucketPolicyStatusAPI
	S3GetBucketLoggingAPI
}

// EnrichTrailLogBucket reports on the bucket each trail delivers to: one that
// AWS evaluates as public, and one that records no access logging.
//
// It asks S3 directly for the trail's own bucket rather than joining the s3
// resource cache. A join only answers once the operator has opened the bucket
// list, so in production it is silent exactly when nobody has looked — which
// is when the audit trail sitting in a public bucket matters most.
//
// One or two calls per trail, capped by EnrichmentCap. A bucket that cannot be
// read marks its trail truncated rather than reporting it clean.
func EnrichTrailLogBucket(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	api, ok := clients.S3.(trailLogBucketAPI)
	if !ok {
		return result, nil
	}

	var failures []Failure
	var mu sync.Mutex
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		bucket := r.Fields["s3_bucket"]
		if bucket == "" {
			return
		}

		status, statusErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketPolicyStatusOutput, error) {
			return api.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: aws.String(bucket)})
		})
		logging, loggingErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLoggingOutput, error) {
			return api.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
		})

		mu.Lock()
		defer mu.Unlock()

		// A bucket with no policy cannot be public by policy; anything else
		// that fails leaves the trail's posture unknown, not clean.
		public := statusErr == nil && status.PolicyStatus != nil && aws.ToBool(status.PolicyStatus.IsPublic)
		switch {
		case statusErr == nil || isS3APIErrCode(statusErr, "NoSuchBucketPolicy"):
			// No policy at all is an answer, not a gap.
		case IsNotFoundErr(statusErr):
			// The log bucket is gone: a race with whoever deleted it, not a
			// failure to log.
			result.TruncatedIDs[r.ID] = true
		default:
			MarkSkipped(&result, r.ID, &failures, statusErr)
		}
		if public {
			setWave2Finding(&result, r.ID, CodeTrailLogBucketPublic, "!", "trail", []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: "!"}})
		}

		switch {
		case loggingErr != nil && IsNotFoundErr(loggingErr):
			// The log bucket is gone: a race, not a failure to log.
			result.TruncatedIDs[r.ID] = true
		case loggingErr != nil:
			MarkSkipped(&result, r.ID, &failures, loggingErr)
		case logging.LoggingEnabled == nil:
			setWave2Finding(&result, r.ID, CodeTrailLogBucketNoAccessLogging, "~", "trail", []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: "~"}})
		}
	})
	return result, AggregateFailures("log bucket posture", failures, n)
}
