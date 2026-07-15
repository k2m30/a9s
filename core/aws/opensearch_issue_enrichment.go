// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// opensearch_issue_enrichment.go — Wave 2 issue enrichment for opensearch.
// No AWS API call — reads signal flags populated by the fetcher.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// opensearch canonical FindingCodes.
const (
	opensearchCodeUpdateForced  domain.FindingCode = "opensearch.update-forced"
	opensearchCodeEncryptionOff domain.FindingCode = "opensearch.encryption-off"
)

// opensearchUpdateForcedDetail and opensearchEncryptionOffDetail are the S5
// "concrete operator sentence" texts stamped onto Finding.Detail. Each names
// its own condition only — neither references the other — so the two
// problems read as independent facts even in the both-active case, where
// both survive as their own Finding in IssueEnricherResult.Findings[id]
// (map[string][]domain.Finding) rather than one condition being crammed into
// the other's AttentionDetail row.
const (
	opensearchUpdateForcedDetail  = "AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window."
	opensearchEncryptionOffDetail = "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place."
)

// EnrichOpenSearchDomains emits Findings for background-check signals
// read from resource Fields populated by FetchOpenSearchDomains:
//
//   - service_software_update_available == "true"  → Severity "!", Summary "software update forced soon"
//   - encryption_at_rest_enabled == "false"        → Severity "~", Summary "encryption at rest off"
//
// Each condition names its own FindingCode and Detail sentence and is
// evaluated independently of the other via its own setWave2Finding call. When
// both are active on the same resource, both Findings survive in
// IssueEnricherResult.Findings[id] — update-forced ("!") and encryption-off
// ("~") each carry their own Code/Phrase/Detail. The two conditions never
// share a Detail sentence or a Phrase — each keeps its own wording
// (opensearchUpdateForcedDetail vs opensearchEncryptionOffDetail) so an
// operator reading either surface sees two distinct problems, not one
// problem with an appendix.
//
// IssueCount counts resources with update_available ("!" severity); "~"-only
// instances never bump.
//
// No FieldUpdates — the fetcher is authoritative for Status on opensearch.
// clients may be nil; no API calls are made.
func EnrichOpenSearchDomains(_ context.Context, _ *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
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

		if updateAvailable {
			// "!" condition — update forced soon, evaluated independently of encOff.
			var rows []domain.DetailRow
			if updateDate := r.Fields["automated_update_date"]; updateDate != "" {
				rows = append(rows, domain.DetailRow{Label: "Automated Update", Value: updateDate, Tier: "!"})
			}
			if cv := r.Fields["current_version"]; cv != "" {
				rows = append(rows, domain.DetailRow{Label: "Current Version", Value: cv})
			}
			if nv := r.Fields["new_version"]; nv != "" {
				rows = append(rows, domain.DetailRow{Label: "New Version", Value: nv})
			}
			setWave2Finding(&result, r.ID, opensearchCodeUpdateForced, "software update forced soon", "!", "opensearch", rows, opensearchUpdateForcedDetail)
			bangCount++
		}
		if encOff {
			// "~" condition — encryption at rest off, evaluated independently of
			// updateAvailable. When updateAvailable also fired above, this
			// survives as its own Finding alongside it (setWave2Finding's
			// append-style same-resourceID handling), not folded into it. "~"
			// never bumps bangCount.
			setWave2Finding(&result, r.ID, opensearchCodeEncryptionOff, "encryption at rest off", "~", "opensearch", nil, opensearchEncryptionOffDetail)
		}
	}

	result.IssueCount = bangCount
	// FieldUpdates intentionally nil — fetcher is authoritative for Status.
	return result, nil
}
