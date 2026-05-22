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

// EnrichEFSMountTargets calls DescribeMountTargets per file system (cap EnrichmentCap, per-FS
// pagination up to PerParentPageCap pages) and emits one EnrichmentFinding per file system
// with any mount target whose LifeCycleState is not "available".
//
// Finding contract (spec §4, U11):
//   - Summary  = "mount target down"  (exact §4 phrase; ≤ 40 chars; no Row values embedded)
//   - Rows     = [{Mount Target, <mtID>, "!"}, {AZ, <az>}, {State, <state>, "!"}, {Degraded, "N/M"}]
//   - Severity = "!"
//
// AS-140: the enricher no longer writes FieldUpdates["status"]. The merged
// S4 phrase ("mount target down" alone, or stacked with Wave-1 findings) is
// computed at render time from r.Findings via phraseFromFindings.
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
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
	}
	return IssueEnricherResult{
		IssueCount:   len(findings),
		Truncated:    truncated,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: make(map[string]map[string]string),
	}, AggregateFailures("efs-enrich: DescribeMountTargets", failures, total)
}
