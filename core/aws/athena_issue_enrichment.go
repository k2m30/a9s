// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_issue_enrichment.go — Wave 2 issue enrichment for the athena resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// athena canonical FindingCodes. Enforcement and result encryption are two
// independent settings, so they are two findings: a workgroup can fail either
// without the other, and one merged phrase could name only the first.
const (
	athenaCodeSettingsNotEnforced domain.FindingCode = "athena.settings-not-enforced"
	athenaCodeResultsUnencrypted  domain.FindingCode = "athena.results-unencrypted"
)

// EnrichAthenaWorkGroup calls GetWorkGroup per workgroup (capped at EnrichmentCap) to
// surface governance and security findings.
//
// Findings:
//   - WorkGroup.Configuration.EnforceWorkGroupConfiguration == false → "~",
//     athenaCodeSettingsNotEnforced.
//   - WorkGroup.Configuration.ResultConfiguration.EncryptionConfiguration == nil → "~",
//     athenaCodeResultsUnencrypted, carrying the result location so the row
//     names where the unencrypted output lands.
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
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
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
		// EnforceWorkGroupConfiguration defaults to true; false means callers can bypass settings.
		// No supporting row: the phrase already says the setting is overridable,
		// and U11 forbids restating it underneath.
		if cfg.EnforceWorkGroupConfiguration != nil && !*cfg.EnforceWorkGroupConfiguration {
			setWave2Finding(&result, key, athenaCodeSettingsNotEnforced,
				"settings can be overridden per query", "~", "athena", nil)

		}
		// Missing encryption on result configuration is a security concern.
		if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.EncryptionConfiguration == nil {
			var rows []domain.DetailRow
			if loc := aws.ToString(resultOutputLocation(cfg)); loc != "" {
				rows = append(rows, domain.DetailRow{Label: "Results written to", Value: loc, Tier: "~"})
			}
			setWave2Finding(&result, key, athenaCodeResultsUnencrypted,
				"query results stored unencrypted", "~", "athena", rows)

		}
	})
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, nil
}

// resultOutputLocation returns the workgroup's configured result location, or
// nil when the workgroup has no result configuration at all.
func resultOutputLocation(cfg *athenatypes.WorkGroupConfiguration) *string {
	if cfg.ResultConfiguration == nil {
		return nil
	}
	return cfg.ResultConfiguration.OutputLocation
}
