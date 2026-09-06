// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbc_issue_enrichment.go — Wave 2 issue enrichment for the dbc resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// nowFunc is the time source for overdue-date checks. Tests override it to a
// fixed past/future anchor via package-level replacement.
var nowFunc = time.Now

// dbc canonical FindingCodes.
const (
	dbcCodeMaintenanceOverdue domain.FindingCode = "dbc.maintenance-overdue"
)

// EnrichDBCMaintenance calls DescribePendingMaintenanceActions (account-wide,
// paginated) and emits one Finding per dbc cluster with overdue maintenance.
// Severity "!" (Wave 2 "!" bumps the S1 menu badge). A finding is "overdue" when either:
//   - AutoAppliedAfterDate is non-nil AND in the past, OR
//   - ForcedApplyDate is non-nil AND in the past.
//
// The merged S4 status phrase (e.g. "maintenance overdue" alone, or
// "stopped (+1)" stacked over a Wave-1 finding) is computed at render time
// from r.Findings via domain.StatusPhrase; this enricher only emits Findings.
func EnrichDBCMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}

	// Backup coverage runs before the client guard below: a type whose own API
	// client is missing is still either selected by a plan or not, and the tag
	// read it may need is skipped along with everything else in that case.
	var tagRead backupTagReader
	if clients != nil && clients.DocDB != nil {
		if api, ok := clients.DocDB.(DocDBListTagsForResourceAPI); ok {
			tagRead = func(ctx context.Context, arn string) (map[string]string, error) {
				return docdbTagsForARN(ctx, api, arn)
			}
		}
	}
	arnAndTags, tagErr := backupTagsAccessor(ctx, cache, resources, tagRead, &result, "ListTagsForResource")
	addBackupCoverage(cache, "dbc", resources, arnAndTags, &result)

	if clients == nil || clients.DocDB == nil {
		return result, tagErr
	}

	// Deterministic ARN-suffix matching via ordered probeIDs. There is no
	// parallel statusByID map: the merged S4 phrase (single-finding or
	// Wave-1+Wave-2 stacked) is computed at render time from r.Findings, so
	// the enricher does not read the fetcher's status overlay here.
	probeIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs = append(probeIDs, r.ID)
		}
	}

	var marker *string
	truncated := false
	pages := 0
	now := nowFunc()
	var failures []string

	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*docdb.DescribePendingMaintenanceActionsOutput, error) {
			return clients.DocDB.DescribePendingMaintenanceActions(ctx, &docdb.DescribePendingMaintenanceActionsInput{Marker: marker})
		})
		pages++
		if err != nil {
			truncated = true
			failures = append(failures, fmt.Sprintf("page %d: %v", pages, err))
			break
		}

		for _, action := range out.PendingMaintenanceActions {
			if action.ResourceIdentifier == nil {
				continue
			}
			arn := *action.ResourceIdentifier
			if !isClusterARN(arn) {
				continue // instance ARN or other resource — not dbc
			}

			// Find the longest matching probeID (specificity wins over prefix).
			key := ""
			for _, id := range probeIDs {
				if strings.HasSuffix(arn, ":"+id) && len(id) > len(key) {
					key = id
				}
			}
			if key == "" {
				continue
			}

			// Check overdue: emit a finding ONLY when any action detail has a past date.
			overdue := false
			for _, pa := range action.PendingMaintenanceActionDetails {
				if pa.ForcedApplyDate != nil && pa.ForcedApplyDate.Before(now) {
					overdue = true
					break
				}
				if pa.AutoAppliedAfterDate != nil && pa.AutoAppliedAfterDate.Before(now) {
					overdue = true
					break
				}
			}
			if !overdue {
				continue
			}

			// Build rows. Summary is the short S5 phrase; every concrete fact
			// (Action, Description, Earliest Target, Apply Method) lives only in
			// Rows so the Attention section does not render duplicated content (U11).
			var rows []domain.DetailRow
			for _, pa := range action.PendingMaintenanceActionDetails {
				if pa.Action != nil && *pa.Action != "" {
					rows = append(rows, domain.DetailRow{Label: "Action", Value: *pa.Action, Tier: "!"})
				}
				if pa.OptInStatus != nil && *pa.OptInStatus != "" {
					rows = append(rows, domain.DetailRow{Label: "Apply Method", Value: *pa.OptInStatus})
				}
				if pa.AutoAppliedAfterDate != nil {
					rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.AutoAppliedAfterDate), Tier: "!"})
				} else if pa.ForcedApplyDate != nil {
					rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.ForcedApplyDate), Tier: "!"})
				}
				if pa.Description != nil && *pa.Description != "" {
					rows = append(rows, domain.DetailRow{Label: "Description", Value: *pa.Description})
				}
			}

			setWave2Finding(&result, key, dbcCodeMaintenanceOverdue, "maintenance overdue", "!", "dbc", rows, "")
		}

		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	result.Truncated = truncated
	return result, errors.Join(tagErr, AggregateFailures("dbc-enrich: DescribePendingMaintenanceActions", failures, pages))
}

// isClusterARN returns true when the ARN's resource-type segment is "cluster".
// Format: arn:aws:rds:region:account:cluster:id  (DocDB clusters use the RDS service prefix in ARNs)
func isClusterARN(arn string) bool {
	parts := strings.Split(arn, ":")
	return len(parts) >= 7 && parts[5] == "cluster"
}

// docdbTagsForARN reads one cluster's tags. DocumentDB and Aurora clusters
// both answer this call, and it returns the whole set in one response.
func docdbTagsForARN(ctx context.Context, api DocDBListTagsForResourceAPI, arn string) (map[string]string, error) {
	out, err := api.ListTagsForResource(ctx, &docdb.ListTagsForResourceInput{ResourceName: aws.String(arn)})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, len(out.TagList))
	for _, t := range out.TagList {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}
