// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_issue_enrichment.go — Wave 2 issue enrichment for the trail resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// trailLogBucketAPI is the read-only S3 calls the log-bucket rows need: the
// public verdict's reads and the access-logging read. Asserted off the
// bucket's own S3 client rather than folded into the aggregate, so a client
// without these calls still satisfies the aggregate.
type trailLogBucketAPI interface {
	s3PublicAPI
	S3GetBucketLoggingAPI
}

// EnrichTrailLogBucket reports on the bucket each trail delivers to: one that
// is public by s3PublicVerdict, the verdict the bucket's own row carries, and
// one that records no access logging.
//
// It asks S3 directly for the trail's own bucket rather than joining the s3
// resource cache. A join only answers once the operator has opened the bucket
// list, so in production it is silent exactly when nobody has looked — which
// is when the audit trail sitting in a public bucket matters most.
//
// Up to four calls per trail, capped by EnrichmentCap. A bucket that cannot be
// read marks its trail truncated rather than reporting it clean.
func EnrichTrailLogBucket(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients.S3 == nil {
		return result, nil
	}

	var failures []Failure
	var mu sync.Mutex
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		bucket := r.Fields["s3_bucket"]
		if bucket == "" {
			return
		}
		// A trail commonly delivers to a bucket in another region than the
		// one the session is browsing.
		api, ok := clients.s3For(ctx, bucket).(trailLogBucketAPI)
		if !ok {
			return
		}

		pub := s3PublicVerdict(ctx, api, bucket)
		logging, loggingErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLoggingOutput, error) {
			return api.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
		})

		mu.Lock()
		defer mu.Unlock()

		// A read that failed leaves the trail's posture unknown, not clean.
		for _, err := range append(pub.failures, pub.unreachable) {
			if err != nil {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
		if pub.rows != nil {
			setWave2Finding(&result, r.ID, CodeTrailLogBucketPublic, []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: tierOf(CodeTrailLogBucketPublic)}})
		}

		switch {
		case loggingErr != nil:
			MarkSkipped(&result, r.ID, &failures, loggingErr)
		case logging.LoggingEnabled == nil:
			setWave2Finding(&result, r.ID, CodeTrailLogBucketNoAccessLogging, []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: tierOf(CodeTrailLogBucketNoAccessLogging)}})
		}
	})
	return result, errors.Join(loopErr, AggregateFailures("log bucket posture", failures, n))
}
