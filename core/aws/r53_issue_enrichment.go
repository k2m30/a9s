// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// r53_issue_enrichment.go — Wave 2 issue enrichment for the r53 resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53svc "github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// r53 canonical FindingCodes.
const (
	r53CodeOrphanPrivateZone domain.FindingCode = "r53.orphan-private-zone"
)

// EnrichRoute53Zone calls GetHostedZone per zone (cap EnrichmentCap) and raises a finding
// for private zones that have no VPC associations (orphaned private zone).
//
// Findings:
//   - HostedZone.Config.PrivateZone == true AND VPCs[] empty → "~" finding
//     "private zone with no VPC associations (orphan)"
//
// Skip if clients.Route53 == nil. Per-zone errors → Truncated.
func EnrichRoute53Zone(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.Route53 == nil {
		return result, nil
	}
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		zoneID := r.Fields["zone_id"]
		if zoneID == "" {
			zoneID = r.ID
		}
		if zoneID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.GetHostedZoneOutput, error) {
			return clients.Route53.GetHostedZone(ctx, &r53svc.GetHostedZoneInput{
				Id: aws.String(zoneID),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.HostedZone == nil {
			return
		}
		// Only raise a finding for private zones — public zones cannot have VPC associations.
		if out.HostedZone.Config == nil || !out.HostedZone.Config.PrivateZone {
			return
		}
		if len(out.VPCs) > 0 {
			return
		}
		setWave2Finding(&result, r.ID, r53CodeOrphanPrivateZone, "private zone with no VPC associations (orphan)", "~", "r53", []domain.DetailRow{
			{Label: "Zone ID", Value: zoneID, Tier: "~"},
			{Label: "Issue", Value: "private zone with no VPC associations (orphan)", Tier: "~"},
		}, "")
	})
	sort.Strings(failures)
	// All Route53 findings are severity "~" (informational).
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result,
		AggregateFailures("r53-enrich: GetHostedZone", failures, total)
}
