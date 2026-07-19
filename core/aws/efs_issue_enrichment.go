// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// efs_issue_enrichment.go — Wave 2 issue enrichment for the efs resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// efs canonical FindingCodes.
const (
	efsCodeMountTargetDown domain.FindingCode = "efs.mount-target-down"
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
// The enricher no longer writes FieldUpdates["status"]. The merged
// S4 phrase ("mount target down" alone, or stacked with Wave-1 findings) is
// computed at render time from r.Findings via phraseFromFindings.
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.EFS == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		fsID := r.ID
		if fsID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		// Paginate mount targets per file system using Marker/NextMarker.
		var allMountTargets []efstypes.MountTargetDescription
		var mtMarker *string
		mtPages := 0
		mtTruncated := false
		pageFailed := false
		var pageErr error
		for {
			if mtPages >= PerParentPageCap {
				mtTruncated = true
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
				pageFailed = true
				pageErr = err
				break
			}
			allMountTargets = append(allMountTargets, out.MountTargets...)
			if out.NextMarker == nil {
				break
			}
			mtMarker = out.NextMarker
		}

		if mtTruncated || pageFailed {
			mu.Lock()
			defer mu.Unlock()
			truncated = true
			result.TruncatedIDs[r.ID] = true
			if pageFailed {
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, pageErr))
			}
			return
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
			return
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

		mu.Lock()
		defer mu.Unlock()
		// Summary must NOT embed any Row value (U11 contract).
		setWave2Finding(&result, fsID, efsCodeMountTargetDown, "mount target down", "!", "efs", []domain.DetailRow{
			{Label: "Mount Target", Value: mtID, Tier: "!"},
			{Label: "AZ", Value: az},
			{Label: "State", Value: state, Tier: "!"},
			{Label: "Degraded", Value: fmt.Sprintf("%d/%d", unavailableCount, totalMT)},
		}, "")
	})
	sort.Strings(failures)
	result.Truncated = truncated
	result.FieldUpdates = make(map[string]map[string]string)
	return result, AggregateFailures("efs-enrich: DescribeMountTargets", failures, total)
}
