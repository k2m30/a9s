// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_issue_enrichment.go — Wave 2 issue enrichment for the athena resource type.
package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// athena canonical FindingCodes.
const (
	athenaCodeGovernanceMisconfigured domain.FindingCode = "athena.governance-misconfigured"
)

// EnrichAthenaWorkGroup calls GetWorkGroup per workgroup (capped at EnrichmentCap) to
// surface governance and security findings.
//
// Findings:
//   - WorkGroup.Configuration.EnforceWorkGroupConfiguration == false → "~" severity,
//     "EnforceWorkGroupConfiguration disabled (callers can bypass)".
//   - WorkGroup.Configuration.ResultConfiguration.EncryptionConfiguration == nil → "~" severity,
//     "result encryption not configured".
//
// Per-WG errors mark Truncated=true and are skipped.
// Skip when clients.Athena == nil.
func EnrichAthenaWorkGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.Athena == nil {
		return result, nil
	}
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		wgName := r.Fields["workgroup_name"]
		if wgName == "" {
			wgName = r.ID
		}
		if wgName == "" {
			return
		}
		out, err := clients.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{
			WorkGroup: aws.String(wgName),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.WorkGroup == nil || out.WorkGroup.Configuration == nil {
			return
		}
		cfg := out.WorkGroup.Configuration
		key := r.ID
		if key == "" {
			key = wgName
		}
		var rows []domain.DetailRow
		// EnforceWorkGroupConfiguration defaults to true; false means callers can bypass settings.
		if cfg.EnforceWorkGroupConfiguration != nil && !*cfg.EnforceWorkGroupConfiguration {
			rows = append(rows, domain.DetailRow{
				Label: "EnforceWorkGroupConfiguration",
				Value: "false",
				Tier:  "~",
			})
		}
		// Missing encryption on result configuration is a security concern.
		if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.EncryptionConfiguration == nil {
			rows = append(rows, domain.DetailRow{
				Label: "ResultConfiguration.EncryptionConfiguration",
				Value: "nil",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			return
		}
		summary := rows[0].Label
		if len(rows) > 1 {
			summary = fmt.Sprintf("%s (%d findings)", rows[0].Label, len(rows))
		}
		setWave2Finding(&result, key, athenaCodeGovernanceMisconfigured, summary, "~", "athena", rows, "")
		// "~" severity does not contribute to IssueCount.
	})
	result.IssueCount = 0
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, nil
}
