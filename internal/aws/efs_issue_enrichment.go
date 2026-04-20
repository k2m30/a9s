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
// pagination up to PerParentPageCap pages) and returns a Finding for any file system with a
// mount target whose LifeCycleState is not "available". Severity is "!" (broken/degraded).
// Summary: "mount target unavailable: <mountTargetID> in <az>".
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.EFS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		fsID := r.ID
		if fsID == "" {
			continue
		}
		// Paginate mount targets per file system using Marker/NextMarker.
		var allMountTargets []efstypes.MountTargetDescription
		var mtMarker *string
		mtPages := 0
		mtTruncated := false
		for {
			if mtPages >= PerParentPageCap {
				mtTruncated = true
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			out, err := clients.EFS.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{
				FileSystemId: aws.String(fsID),
				Marker:       mtMarker,
			})
			mtPages++
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			allMountTargets = append(allMountTargets, out.MountTargets...)
			if out.NextMarker == nil {
				break
			}
			mtMarker = out.NextMarker
		}
		if mtTruncated {
			continue
		}
		for _, mt := range allMountTargets {
			if mt.LifeCycleState == "available" {
				continue
			}
			mtID := ""
			if mt.MountTargetId != nil {
				mtID = *mt.MountTargetId
			}
			az := ""
			if mt.AvailabilityZoneName != nil {
				az = *mt.AvailabilityZoneName
			}
			findings[fsID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("mount target unavailable: %s in %s", mtID, az),
				Rows: []resource.FindingRow{
					{Label: "Mount Target", Value: mtID, Tier: "!"},
					{Label: "AZ", Value: az},
					{Label: "State", Value: string(mt.LifeCycleState), Tier: "!"},
				},
			}
			break // first finding per FS is sufficient
		}
	}
	return IssueEnricherResult{IssueCount: len(findings), Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
