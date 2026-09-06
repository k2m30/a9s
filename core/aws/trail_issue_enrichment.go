// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_issue_enrichment.go — Wave 2 issue enrichment for the trail resource type.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// trailBucketSignals maps an s3 wave-2 finding code to the trail finding it
// raises on whichever trail delivers to that bucket.
var trailBucketSignals = []struct { //nolint:gochecknoglobals // static table: intentional package-level var
	s3Code   domain.FindingCode
	code     domain.FindingCode
	phrase   string
	severity domain.Severity
	tier     string
	detail   string
}{
	{s3CodePublic, CodeTrailLogBucketPublic, "log bucket is publicly accessible", domain.SevBroken, "!", trailLogBucketPublicDetail},
	{s3CodeAccessLoggingOff, CodeTrailLogBucketNoAccessLogging, "log bucket has no access logging", domain.SevWarn, "~", trailLogBucketNoAccessLoggingDetail},
}

// EnrichTrailLogBucket reports what the s3 cache already knows about the
// bucket each trail delivers to: a publicly readable log bucket, and one that
// records no access logging.
//
// Cache-only — it makes no AWS call. The s3 rows carry their own wave-2
// findings, so this reads them by code string rather than importing the s3
// enricher's symbols. Registered at Priority 200 so s3 (100) runs first.
//
// Three states mean "not known" and emit nothing rather than a false
// negative: no s3 cache, a truncated one (the bucket may be on a page nobody
// loaded), and one whose rows carry no wave-2 finding at all, which means the
// s3 wave-2 pass has not run yet. Cache order across types is not guaranteed.
func EnrichTrailLogBucket(_ context.Context, _ *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	entry, ok := cache["s3"]
	if !ok || entry.IsTruncated || !anyWave2Finding(entry.Resources) {
		return result, nil
	}

	codesByBucket := make(map[string]map[domain.FindingCode]bool, len(entry.Resources))
	for _, b := range entry.Resources {
		codes := make(map[domain.FindingCode]bool, len(b.Findings))
		for _, f := range b.Findings {
			codes[f.Code] = true
		}
		codesByBucket[b.ID] = codes
	}

	for _, r := range resources {
		bucket := r.Fields["s3_bucket"]
		codes, known := codesByBucket[bucket]
		if bucket == "" || !known {
			continue
		}
		for _, sig := range trailBucketSignals {
			if !codes[sig.s3Code] {
				continue
			}
			setWave2Finding(&result, r.ID, sig.code, sig.phrase, sig.tier, "trail",
				[]domain.DetailRow{{Label: "Bucket", Value: bucket, Tier: sig.tier}})
		}
	}
	return result, nil
}

// anyWave2Finding reports whether any cached row carries a finding the wave-2
// pass produced. An all-wave1 cache means that pass has not run yet.
func anyWave2Finding(rs []domain.Resource) bool {
	for _, r := range rs {
		for _, f := range r.Findings {
			if len(f.Source) >= 6 && f.Source[:6] == "wave2:" {
				return true
			}
		}
	}
	return false
}
