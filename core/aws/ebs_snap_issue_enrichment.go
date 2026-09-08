// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ebs_snap_issue_enrichment.go — Wave 1 cross-ref enricher for ebs-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the ebs-snap parent (ec2 Volume) extractors. The enricher detects one
// signal from docs/attention-signals.md's ebs-snap row:
//
//   - orphan: source VolumeId NOT found in the loaded ebs cache.
//     Phrase: "orphan: source volume deleted"
//
// ec2.Volume carries no retention-period concept, so RetentionEnabled is
// false — only the orphan rule fires; GetCreatedAt/GetSnapshotType/
// GetParentRetention are left nil per SnapshotCrossRefConfig's contract.
//
// Wave classification: zero AWS API calls. The enricher scans the in-memory
// ebs ResourceCache only.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ebsSnapCodePublic — the snapshot's createVolumePermission includes the
// "all" group, so every AWS account can restore a volume from it.
const ebsSnapCodePublic domain.FindingCode = "ebs-snap.public"

// enrichEBSSnapCrossRef is the IssueEnricherFunc registered for ebs-snap:
// the cache-only orphan scan, plus the one account-wide DescribeSnapshots
// call that names the snapshots restorable by every AWS account.
func enrichEBSSnapCrossRef(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result, err := ebsSnapOrphanCrossRef(ctx, clients, resources, cache)
	if result.Findings == nil {
		result.Findings = make(map[string][]domain.Finding)
	}
	if result.TruncatedIDs == nil {
		result.TruncatedIDs = make(map[string]bool)
	}
	publicErr := ebsSnapPublicShares(ctx, clients, resources, &result)
	if err != nil {
		return result, err
	}
	return result, publicErr
}

// ebsSnapPublicShares asks EC2 once, account-wide, which of this account's
// own snapshots are restorable by everyone (RestorableByUserIds=["all"]) —
// one call for the whole page rather than a
// DescribeSnapshotAttribute per snapshot. Only snapshots present in the
// input page get a finding.
func ebsSnapPublicShares(ctx context.Context, clients *ServiceClients, resources []resource.Resource, result *IssueEnricherResult) error {
	if clients.EC2 == nil || len(resources) == 0 {
		return nil
	}
	known := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			known[r.ID] = true
		}
	}

	const op = "DescribeSnapshots(RestorableByUserIds=all)"
	// The one account-wide walker owns the other half of the rule: a snapshot
	// the walked pages never named is uninspected once the walk is cut, not
	// private. Membership of the all-restorable set is decided by a single
	// sighting, so a named snapshot stays answered.
	public, pages, cut, walkErr := walkAccountPages(result, resources, oneItemPerRow,
		func(snap ec2types.Snapshot) string { return aws.ToString(snap.SnapshotId) },
		func(token *string) ([]ec2types.Snapshot, *string, error) {
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2svc.DescribeSnapshotsOutput, error) {
				return clients.EC2.DescribeSnapshots(ctx, &ec2svc.DescribeSnapshotsInput{
					OwnerIds:            []string{"self"},
					RestorableByUserIds: []string{"all"},
					NextToken:           token,
				})
			})
			if err != nil {
				return nil, nil, err
			}
			return out.Snapshots, out.NextToken, nil
		})
	for _, snap := range public {
		id := aws.ToString(snap.SnapshotId)
		if !known[id] {
			continue
		}
		setWave2Finding(result, id, ebsSnapCodePublic, []domain.DetailRow{{Label: "Public", Value: "yes", Tier: tierOf(ebsSnapCodePublic)}})
	}
	if cut {
		SetTruncated(result, true)
	}
	if walkErr != nil {
		return AggregateFailures(op, []Failure{FailedOnPage(pages, walkErr)}, len(resources))
	}
	return nil
}

// ebsSnapOrphanCrossRef is the SnapshotCrossRef helper instantiated with
// ec2types.Snapshot / ec2types.Volume extractors.
var ebsSnapOrphanCrossRef = EnrichSnapshotCrossRef(SnapshotCrossRefConfig{
	ParentShortName: "ebs",
	GetParentID: func(raw any) (string, bool) {
		snap, ok := assertStruct[ec2types.Snapshot](raw)
		if !ok || snap.VolumeId == nil || *snap.VolumeId == "" {
			return "", false
		}
		return *snap.VolumeId, true
	},
	ParentRowLabel:   "Source Volume",
	RetentionEnabled: false,
	OrphanCode:       CodeEBSSnapOrphan,
})
