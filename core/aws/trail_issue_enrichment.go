// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_issue_enrichment.go — Wave 2 issue enrichment for the trail resource type.
package aws

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// EnrichTrail reads each trail's logging health and the posture of the bucket
// it delivers to. The two are independent reads, so a refused one leaves only
// its own signals unread.
func EnrichTrail(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result, bucketErr := EnrichTrailLogBucket(ctx, clients, resources, cache)
	statusErr := enrichTrailStatus(ctx, clients, resources, &result)
	return result, errors.Join(statusErr, bucketErr)
}

// enrichTrailStatus calls GetTrailStatus per trail (capped at EnrichmentCap),
// the only source of a trail's logging health, and writes is_logging,
// latest_delivery_error and latest_delivery_time with the findings they raise.
func enrichTrailStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	if clients.CloudTrail == nil {
		return nil
	}
	if result.FieldUpdates == nil {
		result.FieldUpdates = make(map[string]map[string]string)
	}
	resources = capAtEnrichmentCap(result, resources, func(r resource.Resource) bool {
		return r.Fields["trail_arn"] != ""
	}, resourceIDsOf)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		out, err := clients.CloudTrail.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: aws.String(r.Fields["trail_arn"])})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		fields := map[string]string{"is_logging": "", "latest_delivery_error": aws.ToString(out.LatestDeliveryError), "latest_delivery_time": ""}
		if out.IsLogging != nil {
			fields["is_logging"] = strconv.FormatBool(*out.IsLogging)
		}
		if out.LatestDeliveryTime != nil {
			fields["latest_delivery_time"] = out.LatestDeliveryTime.Format(time.RFC3339)
		}
		result.FieldUpdates[r.ID] = fields
		for _, f := range trailStatusFindings(fields["is_logging"], fields["latest_delivery_error"], fields["latest_delivery_time"]) {
			addWave2Finding(result, r.ID, f, nil)
		}
	})
	return errors.Join(loopErr, AggregateFailures("GetTrailStatus", failures, len(resources)))
}

// trailBucketOwned reports whether the session's account owns bucket, read
// off the loaded s3 list: ListBuckets "Returns a list of all buckets owned by
// the authenticated sender of the request", so a bucket on it is the
// account's, and one absent from a complete list belongs to another account.
// known is false when the list is not loaded in full. The account-level
// public access block applies only to the account's own buckets.
func trailBucketOwned(cache resource.ResourceCache, bucket string) (owned, known bool) {
	rows, truncated, loaded := cachedRelatedList(cache, "s3")
	if slices.ContainsFunc(rows, func(r resource.Resource) bool { return r.ID == bucket }) {
		return true, true
	}
	return false, loaded && !truncated
}

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
func EnrichTrailLogBucket(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
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

		pub := s3PublicVerdict(ctx, clients, api, bucket)
		logging, loggingErr := api.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})

		mu.Lock()
		defer mu.Unlock()

		// A read that failed leaves the trail's posture unknown, not clean. A
		// deleted bucket is an answer about the trail rather than a row that
		// went away: there is no bucket posture to report, and
		// GetTrailStatus's delivery error says what it costs the trail.
		for _, err := range append(pub.failures, pub.unreachable) {
			if err != nil && !IsNotFoundErr(err) {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
		// The verdict that holds only if the session's account owns the
		// bucket needs that fact; one both readings agree on does not.
		ownDecided, public := pub.public(false)
		if foreignDecided, foreignPublic := pub.public(true); foreignDecided != ownDecided || foreignPublic != public {
			switch owned, known := trailBucketOwned(cache, bucket); {
			case !known:
				public = false
				markUninspected(&result, r.ID, checkListIncomplete("s3"))
			case !owned:
				public = foreignPublic
			}
		}
		if public {
			setWave2Finding(&result, r.ID, CodeTrailLogBucketPublic, []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: tierOf(CodeTrailLogBucketPublic)}})
		}

		switch {
		case IsNotFoundErr(loggingErr):
		case loggingErr != nil:
			MarkSkipped(&result, r.ID, &failures, loggingErr)
		case logging.LoggingEnabled == nil:
			setWave2Finding(&result, r.ID, CodeTrailLogBucketNoAccessLogging, []domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: tierOf(CodeTrailLogBucketNoAccessLogging)}})
		}
	})
	return result, errors.Join(loopErr, AggregateFailures("log bucket posture", failures, n))
}
