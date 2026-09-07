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
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

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
	addBackupCoverage(cache, "dbc", CodeDBCNotInBackupPlan, resources, arnAndTags, &result)

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

	now := nowFunc()
	var failures []Failure

	// clusterKeyOf names the row one pending-maintenance entry answers for, or
	// "" when it belongs to an instance or to a cluster this list does not
	// show. The page walk and the finding loop below must agree on that, so
	// they read the same function.
	clusterKeyOf := func(action docdbtypes.ResourcePendingMaintenanceActions) string {
		if action.ResourceIdentifier == nil || !isClusterARN(*action.ResourceIdentifier) {
			return ""
		}
		// Find the longest matching probeID (specificity wins over prefix).
		key := ""
		for _, id := range probeIDs {
			if strings.HasSuffix(*action.ResourceIdentifier, ":"+id) && len(id) > len(key) {
				key = id
			}
		}
		return key
	}

	allActions, pages, cut, walkErr := walkAccountPages(&result, resources, clusterKeyOf,
		func(token *string) ([]docdbtypes.ResourcePendingMaintenanceActions, *string, error) {
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*docdb.DescribePendingMaintenanceActionsOutput, error) {
				return clients.DocDB.DescribePendingMaintenanceActions(ctx, &docdb.DescribePendingMaintenanceActionsInput{Marker: token})
			})
			if err != nil {
				return nil, nil, err
			}
			return out.PendingMaintenanceActions, out.Marker, nil
		})
	if walkErr != nil {
		failures = append(failures, FailedCall(fmt.Sprintf("page %d", pages), walkErr))
	}

	for _, action := range allActions {
		key := clusterKeyOf(action)
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

		setWave2Finding(&result, key, dbcCodeMaintenanceOverdue, "maintenance overdue", "!", "dbc", rows)
	}

	SetTruncated(&result, cut)
	return result, errors.Join(tagErr, AggregateFailures("DescribePendingMaintenanceActions", failures, pages))
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
