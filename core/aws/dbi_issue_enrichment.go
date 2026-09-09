// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_issue_enrichment.go — Wave 2 enrichment for dbi: DescribePendingMaintenanceActions.
package aws

import (
	"context"
	"errors"
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

// EnrichDBIMaintenance calls DescribePendingMaintenanceActions (account-wide, paginated)
// and emits one Finding per dbi instance with pending maintenance. Severity "~"
// (Wave 2 ~ does not bump the S1 menu badge). The merged
// S4 status phrase (e.g. "maintenance scheduled" alone, or "stopped (+1)" stacked
// over a Wave-1 finding) is computed at render time from r.Findings via
// domain.StatusPhrase; this enricher only emits Findings.
func EnrichDBIMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}

	// Backup coverage runs before the client guard below: a type whose own API
	// client is missing is still either selected by a plan or not, and the tag
	// read it may need is skipped along with everything else in that case.
	var tagRead backupTagReader
	if clients != nil && clients.RDS != nil {
		if api, ok := clients.RDS.(RDSListTagsForResourceAPI); ok {
			tagRead = func(ctx context.Context, arn string) (map[string]string, error) {
				return rdsTagsForARN(ctx, api, arn)
			}
		}
	}
	arnAndTags, tagErr := backupTagsAccessor(ctx, cache, resources, tagRead, &result, "ListTagsForResource")
	addBackupCoverage(cache, "dbi", CodeDBINotInBackupPlan, resources, arnAndTags, &result)

	if clients == nil || clients.RDS == nil {
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

	// instanceKeyOf names the row one pending-maintenance entry answers for,
	// or "" for a cluster, another RDS resource, or an instance this list does
	// not show. The page walk and the finding loop below must agree on that,
	// so they read the same function.
	instanceKeyOf := func(action rdstypes.ResourcePendingMaintenanceActions) string {
		if action.ResourceIdentifier == nil || !isInstanceARN(*action.ResourceIdentifier) {
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

	// A page that fails ends the walk but keeps the pages already read: the
	// instances they name have real pending maintenance whatever happened
	// afterwards. A failed page does lower-bound the walk, unlike the cap,
	// which bounds only informational coverage for this "~"-only pass.
	allActions, _, _, walkErr := walkAccountPages(&result, resources, oneItemPerRow, instanceKeyOf,
		func(token *string) ([]rdstypes.ResourcePendingMaintenanceActions, *string, error) {
			out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{Marker: token})
			if err != nil {
				return nil, nil, err
			}
			return out.PendingMaintenanceActions, out.Marker, nil
		})
	SetTruncated(&result, walkErr != nil)

	for _, action := range allActions {
		key := instanceKeyOf(action)
		if key == "" {
			continue
		}

		// Summary is the short S5 phrase; every concrete fact (Action,
		// Description, Earliest Target, Apply Method) lives only in Rows so
		// the Attention section does not render duplicated content.
		var rows []domain.DetailRow
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil && *pa.Action != "" {
				rows = append(rows, domain.DetailRow{Label: "Action", Value: *pa.Action, Tier: "~"})
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
			}
		}

		setWave2Finding(&result, key, dbiCodePendingMaintenance, rows)
	}

	engineErr := enrichDBIEngineVersions(ctx, clients, resources, &result)

	// Pending maintenance is "~"-only: EnrichmentCap bounds informational
	// coverage, never the issue count. The engine-deprecated pass below is
	// "!", and sets Truncated itself when its walk is cut short.
	return result, errors.Join(tagErr, walkErr, engineErr)
}

// enrichDBIEngineVersions calls DescribeDBEngineVersions once per distinct
// (engine, version) pair across the instances — the pairs repeat heavily in
// a real fleet, so the per-run cache turns an N-instance walk into a handful
// of calls — and emits the deprecated-engine finding for every instance on a
// version AWS no longer lists as available.
func enrichDBIEngineVersions(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	resources = capAtEnrichmentCap(result, resources, resourceIDsOf)

	type enginePair struct{ engine, version string }
	deprecatedByPair := map[enginePair]bool{}
	var failures []Failure

	for i := range resources {
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
				MarkSkipped(result, r.ID, &failures, err)
				continue
			}
			deprecated = isDeprecatedEngineVersion(out.DBEngineVersions)
			deprecatedByPair[pair] = deprecated
		}
		if !deprecated {
			continue
		}
		setWave2Finding(result, r.ID, dbiCodeEngineDeprecated, []domain.DetailRow{{Label: "Engine", Value: pair.engine + " " + pair.version, Tier: tierOf(dbiCodeEngineDeprecated)}})

	}

	SetTruncated(result, len(failures) > 0)
	return AggregateFailures("DescribeDBEngineVersions", failures, len(resources))
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

// rdsTagsForARN reads one RDS resource's tags. The call answers for instances
// and clusters alike, and returns the whole set in one response.
func rdsTagsForARN(ctx context.Context, api RDSListTagsForResourceAPI, arn string) (map[string]string, error) {
	out, err := api.ListTagsForResource(ctx, &rds.ListTagsForResourceInput{ResourceName: aws.String(arn)})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, len(out.TagList))
	for _, t := range out.TagList {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}
