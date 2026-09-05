// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_issue_enrichment.go — Wave 2 enrichment for dbi: DescribePendingMaintenanceActions.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// dbi canonical FindingCodes.
const (
	dbiCodePendingMaintenance domain.FindingCode = "dbi.pending-maintenance"
	dbiCodeEngineDeprecated   domain.FindingCode = "dbi.engine-deprecated"
)

// dbiEngineDeprecatedDetail is the S5 operator sentence for an instance
// running an engine version AWS no longer supports.
const dbiEngineDeprecatedDetail = "AWS no longer supports this engine version, so it stops receiving security patches and will be force-upgraded on AWS's schedule. Upgrade to a supported version during a maintenance window of your choosing."

// EnrichDBIMaintenance calls DescribePendingMaintenanceActions (account-wide, paginated)
// and emits one Finding per dbi instance with pending maintenance. Severity "~"
// (Wave 2 ~ does not bump the S1 menu badge). The merged
// S4 status phrase (e.g. "maintenance scheduled" alone, or "stopped (+1)" stacked
// over a Wave-1 finding) is computed at render time from r.Findings via
// domain.StatusPhrase; this enricher only emits Findings.
func EnrichDBIMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}

	if clients == nil || clients.RDS == nil {
		return result, nil
	}

	// Paginate with a cap.
	var allActions []rdstypes.ResourcePendingMaintenanceActions
	var marker *string
	pages := 0
	for pages < EnrichmentCap {
		out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{Marker: marker})
		pages++
		if err != nil {
			return result, err
		}
		allActions = append(allActions, out.PendingMaintenanceActions...)
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
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

	for _, action := range allActions {
		if action.ResourceIdentifier == nil {
			continue
		}
		arn := *action.ResourceIdentifier
		if !isInstanceARN(arn) {
			continue // dbc / other RDS resources — not dbi
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

		// Summary is the short S5 phrase; every concrete fact (Action,
		// Description, Earliest Target, Apply Method) lives only in Rows so
		// the Attention section does not render duplicated content.
		var rows []domain.DetailRow
		var firstAction, firstDescription string
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil && *pa.Action != "" {
				rows = append(rows, domain.DetailRow{Label: "Action", Value: *pa.Action, Tier: "~"})
				if firstAction == "" {
					firstAction = *pa.Action
				}
			}
			if pa.OptInStatus != nil && *pa.OptInStatus != "" {
				rows = append(rows, domain.DetailRow{Label: "Apply Method", Value: *pa.OptInStatus})
			}
			if pa.AutoAppliedAfterDate != nil {
				rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.AutoAppliedAfterDate), Tier: "~"})
			} else if pa.ForcedApplyDate != nil {
				rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.ForcedApplyDate), Tier: "~"})
			}
			if pa.Description != nil && *pa.Description != "" {
				rows = append(rows, domain.DetailRow{Label: "Description", Value: *pa.Description})
				if firstDescription == "" {
					firstDescription = *pa.Description
				}
			}
		}

		// Detail (S5): docs/resources/dbi.md §4 row "Pending maintenance overdue".
		detail := ""
		if firstAction != "" && firstDescription != "" {
			detail = fmt.Sprintf("Pending maintenance action overdue: %s (%s).", firstAction, firstDescription)
		}

		setWave2Finding(&result, key, dbiCodePendingMaintenance, "maintenance scheduled", "~", "dbi", rows, detail)
	}

	enrichDBIEngineVersions(ctx, clients, resources, &result)

	// Pending maintenance is "~"-only: EnrichmentCap bounds informational
	// coverage, never the issue count. The engine-deprecated pass below is
	// "!", and sets Truncated itself when its walk is cut short.
	return result, nil
}

// enrichDBIEngineVersions calls DescribeDBEngineVersions once per distinct
// (engine, version) pair across the instances — the pairs repeat heavily in
// a real fleet, so the per-run cache turns an N-instance walk into a handful
// of calls — and emits the deprecated-engine finding for every instance on a
// version AWS no longer lists as available.
func enrichDBIEngineVersions(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) {
	n := min(len(resources), EnrichmentCap)
	if n < len(resources) {
		result.Truncated = true
	}

	type enginePair struct{ engine, version string }
	deprecatedByPair := map[enginePair]bool{}
	var failures []string

	for i := range n {
		r := resources[i]
		db, ok := assertStruct[rdstypes.DBInstance](r.RawStruct)
		if !ok || resourceIsTearingDown(r.RawStruct) {
			continue
		}
		pair := enginePair{engine: aws.ToString(db.Engine), version: aws.ToString(db.EngineVersion)}
		if pair.engine == "" || pair.version == "" {
			continue
		}
		deprecated, known := deprecatedByPair[pair]
		if !known {
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBEngineVersionsOutput, error) {
				return clients.RDS.DescribeDBEngineVersions(ctx, &rds.DescribeDBEngineVersionsInput{
					Engine:        aws.String(pair.engine),
					EngineVersion: aws.String(pair.version),
					// Without IncludeAll a deprecated version is simply absent
					// from the response, which reads the same as a typo. With
					// it, AWS returns the row and its Status says which it is.
					IncludeAll: aws.Bool(true),
				})
			})
			if err != nil {
				MarkSkipped(result, r.ID, &failures, "DescribeDBEngineVersions", err)
				continue
			}
			deprecated = isDeprecatedEngineVersion(out.DBEngineVersions)
			deprecatedByPair[pair] = deprecated
		}
		if !deprecated {
			continue
		}
		setWave2Finding(result, r.ID, dbiCodeEngineDeprecated, "engine version deprecated", "!", "dbi",
			[]domain.DetailRow{{Label: "Engine", Value: pair.engine + " " + pair.version, Tier: "!"}},
			dbiEngineDeprecatedDetail)
	}

	if len(failures) > 0 {
		result.Truncated = true
	}
}

// isDeprecatedEngineVersion reads the DescribeDBEngineVersions answer for one
// (engine, version) pair. An empty answer means AWS does not list the version
// at all, which for a version an instance is demonstrably running means it
// has been retired.
func isDeprecatedEngineVersion(versions []rdstypes.DBEngineVersion) bool {
	if len(versions) == 0 {
		return true
	}
	for _, v := range versions {
		if strings.EqualFold(aws.ToString(v.Status), "available") {
			return false
		}
	}
	return true
}
