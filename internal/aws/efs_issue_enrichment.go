// efs_issue_enrichment.go — Wave 2 issue enrichment for the efs resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("efs", EnrichEFSMountTargets, 100)
}

// EnrichEFSMountTargets calls DescribeMountTargets per file system (cap EnrichmentCap, per-FS
// pagination up to PerParentPageCap pages) and emits one EnrichmentFinding per file system
// with any mount target whose LifeCycleState is not "available".
//
// Finding contract (spec §4, U11):
//   - Summary  = "mount target down"  (exact §4 phrase; ≤ 40 chars; no Row values embedded)
//   - Rows     = [{Mount Target, <mtID>, "!"}, {AZ, <az>}, {State, <state>, "!"}, {Degraded, "N/M"}]
//   - Severity = "!"
//   - FieldUpdates[fsID]["status"]:
//     — existing status == "" → "mount target down"
//     — otherwise             → "mount target down (+N)" where N = hidden+1 from existing suffix
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.EFS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		fsID := r.ID
		if fsID == "" {
			continue
		}
		total++
		// Paginate mount targets per file system using Marker/NextMarker.
		var allMountTargets []efstypes.MountTargetDescription
		var mtMarker *string
		mtPages := 0
		mtTruncated := false
		pageFailed := false
		for {
			if mtPages >= PerParentPageCap {
				mtTruncated = true
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeMountTargetsOutput, error) {
				return clients.EFS.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{
					FileSystemId: aws.String(fsID),
					Marker:       mtMarker,
				})
			})
			mtPages++
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
				truncated = true
				truncatedIDs[r.ID] = true
				pageFailed = true
				break
			}
			allMountTargets = append(allMountTargets, out.MountTargets...)
			if out.NextMarker == nil {
				break
			}
			mtMarker = out.NextMarker
		}
		if mtTruncated || pageFailed {
			continue
		}

		// Count unavailable mount targets (N) and total (M).
		totalMT := len(allMountTargets)
		var firstBad *efstypes.MountTargetDescription
		unavailableCount := 0
		for j := range allMountTargets {
			mt := &allMountTargets[j]
			if mt.LifeCycleState != efstypes.LifeCycleStateAvailable {
				unavailableCount++
				if firstBad == nil {
					firstBad = mt
				}
			}
		}

		if firstBad == nil {
			// All mount targets healthy — no finding.
			continue
		}

		mtID := ""
		if firstBad.MountTargetId != nil {
			mtID = *firstBad.MountTargetId
		}
		az := ""
		if firstBad.AvailabilityZoneName != nil {
			az = *firstBad.AvailabilityZoneName
		}
		state := string(firstBad.LifeCycleState)

		// Summary must NOT embed any Row value (U11 contract).
		finding := resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "mount target down",
			Rows: []resource.FindingRow{
				{Label: "Mount Target", Value: mtID, Tier: "!"},
				{Label: "AZ", Value: az},
				{Label: "State", Value: state, Tier: "!"},
				{Label: "Degraded", Value: fmt.Sprintf("%d/%d", unavailableCount, totalMT)},
			},
		}
		findings[fsID] = finding

		// FieldUpdates: the Wave-2 phrase becomes the top, the Wave-1 phrases
		// carried in r.Issues become the hidden count N. Deriving N from
		// len(r.Issues) keeps the enricher idempotent — re-running against
		// already-merged FieldUpdates["status"] never double-bumps the suffix,
		// because Issues is fetcher-owned and stable across enrichment runs.
		var newStatus string
		if len(r.Issues) == 0 {
			newStatus = "mount target down"
		} else {
			newStatus = fmt.Sprintf("mount target down (+%d)", len(r.Issues))
		}
		fu := map[string]string{"status": newStatus}
		fieldUpdates[fsID] = fu
	}
	return IssueEnricherResult{
		IssueCount:   len(findings),
		Truncated:    truncated,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
	}, AggregateFailures("efs-enrich: DescribeMountTargets", failures, total)
}
