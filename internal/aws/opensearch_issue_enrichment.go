// opensearch_issue_enrichment.go — Wave 2 issue enrichment for opensearch.
// No AWS API call — reads signal flags populated by the fetcher.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("opensearch", EnrichOpenSearchDomains, 100)
}

// EnrichOpenSearchDomains emits EnrichmentFindings for background-check signals
// read from resource Fields populated by FetchOpenSearchDomains:
//
//   - service_software_update_available == "true"  → Severity "!", Summary "software update forced soon"
//   - encryption_at_rest_enabled == "false"        → Severity "~", Summary "encryption at rest off"
//
// When both signals are active, the "!" branch is emitted (! beats ~) with an
// additional row {Label:"Additional", Value:"encryption at rest off", Tier:"~"}.
//
// IssueCount counts resources with update_available ("!" severity); "~"-only
// instances never bump.
//
// No FieldUpdates — the fetcher is authoritative for Status on opensearch.
// clients may be nil; no API calls are made.
func EnrichOpenSearchDomains(_ context.Context, _ *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	bangCount := 0

	for _, r := range resources {
		if r.ID == "" {
			continue
		}

		// Deleted domains are being torn down — no actionable background findings.
		// Without this guard a Deleted+UpdateAvailable domain would still emit a
		// `!` finding, which unifiedIssueCount counts into the main-menu badge
		// even though the row itself is Dim. Suppresses the badge contamination
		// without affecting the Dim row's S4 "deleting: removal in progress" phrase
		// (that comes from the fetcher and is independent of this enricher).
		if r.Fields["deleted"] == "true" {
			continue
		}

		updateAvailable := r.Fields["service_software_update_available"] == "true"
		encOff := r.Fields["encryption_at_rest_enabled"] == "false"

		if !updateAvailable && !encOff {
			continue
		}

		var finding resource.EnrichmentFinding

		if updateAvailable {
			// "!" branch — update forced soon. Include enc-off as additional row
			// when both signals are active so the "~" finding is not lost.
			var rows []resource.FindingRow
			if updateDate := r.Fields["automated_update_date"]; updateDate != "" {
				rows = append(rows, resource.FindingRow{Label: "Automated Update", Value: updateDate, Tier: "!"})
			}
			if cv := r.Fields["current_version"]; cv != "" {
				rows = append(rows, resource.FindingRow{Label: "Current Version", Value: cv})
			}
			if nv := r.Fields["new_version"]; nv != "" {
				rows = append(rows, resource.FindingRow{Label: "New Version", Value: nv})
			}
			if encOff {
				// Surface the hidden ~ finding as an additional row (U11 contract:
				// its value must not appear in Summary).
				rows = append(rows, resource.FindingRow{Label: "Additional", Value: "encryption at rest off", Tier: "~"})
			}
			finding = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "software update forced soon",
				Rows:     rows,
			}
			bangCount++
		} else {
			// Only enc-off — "~" finding, no rows needed.
			finding = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "encryption at rest off",
			}
			// "~" never bumps bangCount.
		}

		findings[r.ID] = finding
	}

	return IssueEnricherResult{
		IssueCount:   bangCount,
		TruncatedIDs: map[string]bool{},
		Findings:     findings,
		// FieldUpdates intentionally nil — fetcher is authoritative for Status.
	}, nil
}
