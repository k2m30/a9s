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
	opensearchCodeUpdateForced   domain.FindingCode = "opensearch.update-forced"
	opensearchCodeEncryptionOff  domain.FindingCode = "opensearch.encryption-off"
	opensearchCodePublic         domain.FindingCode = "opensearch.public"
	opensearchCodeHTTPSNotForced domain.FindingCode = "opensearch.https-not-enforced"
	opensearchCodeN2NOff         domain.FindingCode = "opensearch.node-to-node-tls-off"
)

// S5 operator sentences for the network-posture findings.
const (
	opensearchPublicDetail         = "The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals."
	opensearchHTTPSNotForcedDetail = "The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options."
	opensearchN2NOffDetail         = "Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive."
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

// EnrichOpenSearchDomains emits the network-posture Findings from resource
// Fields populated by FetchOpenSearchDomains. Each condition names its own
// FindingCode and Detail sentence and is evaluated independently of the
// others, so a domain that trips several keeps one Finding per problem.
//
// No FieldUpdates — the Status cell is domain.StatusPhrase over the row's
// findings. clients may be nil; no API calls are made.
func EnrichOpenSearchDomains(_ context.Context, _ *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}

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
		if resourceIsTearingDown(r.RawStruct) {
			continue
		}

		// Reachable outside a VPC only counts when the access policy also
		// lets anyone in: a public endpoint fronted by a scoped policy is a
		// deliberate, defended design.
		if r.Fields["vpc_enabled"] == "false" && r.Fields["access_policy_public"] == "true" {
			setWave2Finding(&result, r.ID, opensearchCodePublic, "reachable outside a VPC", "!", "opensearch", []domain.DetailRow{
				{Label: "Endpoint", Value: "public", Tier: "!"},
				{Label: "Access policy", Value: "open", Tier: "!"},
			}, opensearchPublicDetail)
		}
		if r.Fields["enforce_https"] == "false" {
			setWave2Finding(&result, r.ID, opensearchCodeHTTPSNotForced, "HTTPS not enforced", "~", "opensearch", nil, opensearchHTTPSNotForcedDetail)
		}
		if r.Fields["node_to_node_encryption_enabled"] == "false" {
			setWave2Finding(&result, r.ID, opensearchCodeN2NOff, "node-to-node encryption off", "~", "opensearch", nil, opensearchN2NOffDetail)
		}
	}

	// FieldUpdates intentionally nil — fetcher is authoritative for Status.
	return result, nil
}
