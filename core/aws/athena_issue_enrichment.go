// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_issue_enrichment.go — Wave 2 issue enrichment for the athena resource type.
package aws

import (
	"context"
	"errors"
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
//   - WorkGroup.Configuration.ResultConfiguration.EncryptionConfiguration == nil,
//     on a workgroup not using Athena owned storage → "~",
//     athenaCodeResultsUnencrypted, carrying the result location so the row
//     names where the unencrypted output lands.
//
// Per-WG errors mark Truncated=true and are skipped.
// Skip when clients.Athena == nil.
func EnrichAthenaWorkGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients.Athena == nil {
		return result, nil
	}
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
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
			MarkSkipped(&result, r.ID, &failures, err)
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
		if cfg.EnforceWorkGroupConfiguration != nil && !*cfg.EnforceWorkGroupConfiguration {
			setWave2Finding(&result, key, athenaCodeSettingsNotEnforced, nil)

		}
		// Results in Athena owned storage are encrypted whatever the
		// workgroup asks for — an AWS owned key when it names none — so the
		// S3-side encryption setting decides nothing for such a workgroup.
		managedResults := cfg.ManagedQueryResultsConfiguration != nil && cfg.ManagedQueryResultsConfiguration.Enabled
		if !managedResults && (cfg.ResultConfiguration == nil || cfg.ResultConfiguration.EncryptionConfiguration == nil) {
			var rows []domain.DetailRow
			if loc := aws.ToString(resultOutputLocation(cfg)); loc != "" {
				rows = append(rows, domain.DetailRow{Label: "Results written to", Value: loc, Tier: "~"})
			}
			setWave2Finding(&result, key, athenaCodeResultsUnencrypted, rows)

		}
	})
	MarkInformationalOnly(&result)
	return result, errors.Join(loopErr, AggregateFailures("GetWorkGroup", failures, n))
}

// resultOutputLocation returns the workgroup's configured result location, or
// nil when the workgroup has no result configuration at all.
func resultOutputLocation(cfg *athenatypes.WorkGroupConfiguration) *string {
	if cfg.ResultConfiguration == nil {
		return nil
	}
	return cfg.ResultConfiguration.OutputLocation
}
