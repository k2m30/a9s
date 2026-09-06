// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// efs_issue_enrichment.go — Wave 2 issue enrichment for the efs resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// efs canonical FindingCodes.
const (
	efsCodeMountTargetDown domain.FindingCode = "efs.mount-target-down"
	efsCodePublicPolicy    domain.FindingCode = "efs.public-policy"
	efsCodeNoBackupPolicy  domain.FindingCode = "efs.no-backup-policy"
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
// computed at render time from r.Findings via domain.StatusPhrase.
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.EFS == nil {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	truncated := len(resources) > EnrichmentCap
	var failures []string
	var policyFailures []string
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

		// The policy checks are independent of the mount-target walk below —
		// they run first so a mount-target pagination failure cannot swallow
		// them.
		if !resourceIsTearingDown(r.RawStruct) {
			enrichEFSPolicies(ctx, clients, fsID, ownAccount, &result, &policyFailures, &mu)
		}

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
		state := efsMountTargetState(firstBad.LifeCycleState)

		mu.Lock()
		defer mu.Unlock()
		// Summary must NOT embed any Row value (U11 contract).
		setWave2Finding(&result, fsID, efsCodeMountTargetDown, "mount target down", "!", "efs", []domain.DetailRow{
			{Label: "Mount Target", Value: mtID, Tier: "!"},
			{Label: "AZ", Value: az},
			{Label: "State", Value: state, Tier: "!"},
			{Label: "Degraded", Value: fmt.Sprintf("%d/%d", unavailableCount, totalMT)},
		})

	})
	sort.Strings(failures)
	sort.Strings(policyFailures)
	result.Truncated = truncated
	result.FieldUpdates = make(map[string]map[string]string)
	// The two passes are counted separately: each names how many of the same
	// N file systems it could not answer for, and folding them into one
	// tally would report more failures than there were file systems.
	return result, errors.Join(
		AggregateFailures("efs-enrich: DescribeMountTargets", failures, total),
		AggregateFailures("efs-enrich: file system and backup policy", policyFailures, total),
	)
}

// enrichEFSPolicies reads the file system's resource policy and its AWS
// Backup policy. Both calls answer "not configured" with an error AWS treats
// as normal (PolicyNotFound), which is a finding for the backup policy and a
// clean bill of health for the resource policy.
func enrichEFSPolicies(
	ctx context.Context,
	clients *ServiceClients,
	fsID string,
	ownAccount string,
	result *IssueEnricherResult,
	failures *[]string,
	mu *sync.Mutex,
) {
	policyOut, policyErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeFileSystemPolicyOutput, error) {
		return clients.EFS.DescribeFileSystemPolicy(ctx, &efs.DescribeFileSystemPolicyInput{FileSystemId: aws.String(fsID)})
	})
	backupOut, backupErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeBackupPolicyOutput, error) {
		return clients.EFS.DescribeBackupPolicy(ctx, &efs.DescribeBackupPolicyInput{FileSystemId: aws.String(fsID)})
	})

	mu.Lock()
	defer mu.Unlock()

	switch {
	case isEFSPolicyNotFound(policyErr):
		// No file system policy at all: nothing grants public access.
	case policyErr != nil:
		MarkSkipped(result, fsID, failures, "DescribeFileSystemPolicy", policyErr)
	default:
		doc, parseErr := iampolicy.Parse(aws.ToString(policyOut.Policy))
		if parseErr != nil {
			result.TruncatedIDs[fsID] = true
			break
		}
		ex := iampolicy.Evaluate(doc, ownAccount)
		if ex.Public {
			setWave2Finding(result, fsID, efsCodePublicPolicy, "file system policy open to anyone", "!", "efs",
				[]domain.DetailRow{
					{Label: "Principal", Value: "*", Tier: "!"},
					{Label: "Actions", Value: strings.Join(ex.PublicActions, ", ")},
				})

		}
	}

	// A file system with no backup policy at all is as unbacked-up as one
	// whose policy is disabled, so both answer the same way.
	backedUp := false
	switch {
	case isEFSPolicyNotFound(backupErr):
	case backupErr != nil:
		MarkSkipped(result, fsID, failures, "DescribeBackupPolicy", backupErr)
		return
	case backupOut.BackupPolicy != nil:
		backedUp = backupOut.BackupPolicy.Status == efstypes.StatusEnabled
	}
	if backedUp {
		return
	}
	setWave2Finding(result, fsID, efsCodeNoBackupPolicy, "automatic backups off", "~", "efs", nil)
}

// isEFSPolicyNotFound reports whether err is EFS's "no policy is set" answer
// for either the file system policy or the backup policy. Matched on the
// error code as well as the modeled type so a response the SDK could not bind
// to *PolicyNotFound still classifies correctly.
func isEFSPolicyNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFound *efstypes.PolicyNotFound
	if errors.As(err, &notFound) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "PolicyNotFound"
}

// efsMountTargetState words a mount target's lifecycle state for a detail
// row. AWS reports a failed mount target as the bare state "error", which
// says nothing the row's colour has not already said; the others describe
// themselves.
func efsMountTargetState(s efstypes.LifeCycleState) string {
	if s == efstypes.LifeCycleStateError {
		return "mount failed"
	}
	return string(s)
}
