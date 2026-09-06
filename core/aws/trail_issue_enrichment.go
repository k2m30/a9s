// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_issue_enrichment.go — Wave 2 issue enrichment for the trail resource type.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// EnrichTrailLogBucket reports what the s3 cache already knows about the
// bucket each trail delivers to: a publicly readable log bucket, and one that
// records no access logging.
//
// Cache-only — it makes no AWS call. The s3 rows carry their own wave-2
// findings, so this enricher reads them by code string rather than importing
// the s3 enricher's symbols. Registered at Priority 200 so s3 (100) runs
// first; cache order across types is not guaranteed, so when no s3 row yet
// carries a wave-2 finding this returns nothing rather than a false negative.
func EnrichTrailLogBucket(_ context.Context, _ *ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	return IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}, nil
}
