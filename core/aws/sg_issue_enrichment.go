// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sg_issue_enrichment.go — Wave 2 issue enrichment for the sg resource type.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sgCodeUnused is the canonical FindingCode for a security group no network
// interface references.
const sgCodeUnused domain.FindingCode = "sg.unused"

// sgUnusedPhrase is the S4 status phrase for sgCodeUnused.
const sgUnusedPhrase = "not attached to anything"

// EnrichSGUsage cross-references each security group against the already
// loaded "eni" cache: a group no network interface references is dead
// configuration. Zero AWS API calls — the ENI list is the only place that
// records group membership, and it is already fetched for its own type.
//
// Absence is only provable from a complete list: when the "eni" list has not
// been loaded this session, or was truncated at the first page, the enricher
// is a no-op rather than reporting every group as unused. Default groups are
// exempt — AWS creates one per VPC and it cannot be deleted, so "unattached"
// is its normal state, not a finding (sgCodeDefaultWithRules is the signal
// that applies to those).
func EnrichSGUsage(_ context.Context, _ *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]bool),
	}

	eniEntry, eniLoaded := cache["eni"]
	if !eniLoaded || eniEntry.IsTruncated {
		return result, nil
	}

	referenced := make(map[string]bool)
	for _, eni := range eniEntry.Resources {
		for groupID := range strings.SplitSeq(eni.Fields["security_groups"], ",") {
			groupID = strings.TrimSpace(groupID)
			if groupID != "" {
				referenced[groupID] = true
			}
		}
	}

	for _, r := range resources {
		if r.ID == "" || referenced[r.ID] || r.Fields["group_name"] == "default" {
			continue
		}
		setWave2Finding(&result, r.ID, sgCodeUnused, []domain.DetailRow{{Label: "Network interfaces referencing", Value: "0", Tier: tierOf(sgCodeUnused)}})

	}

	return result, nil
}
