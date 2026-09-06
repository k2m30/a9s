// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ebs_issue_enrichment.go — Wave 2 issue enrichment for the ebs resource type.
package aws

import (
	"context"
	"strings"

	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ebs canonical FindingCodes.
const (
	ebsCodeVolumeIODegraded domain.FindingCode = "ebs.volume-io-degraded"
)

// EnrichEBSVolumeStatus calls DescribeVolumeStatus (account-wide, paginated) and returns
// a Finding for every volume with non-ok status.
// Severity is "!" (broken/degraded). Walks up to EnrichmentCap pages via NextToken.
func EnrichEBSVolumeStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}

	// Both cache-only joins run before the client guard below: a session with
	// no EC2 client still knows whether a plan selects the volume and whether a
	// snapshot of it exists.
	account := accountIDFromClients(ctx, clients, clients.IdentityStore())
	addBackupCoverage(cache, "ebs", CodeEBSNotInBackupPlan, resources, func(r resource.Resource) (string, map[string]string, bool) {
		return ebsVolumeARN(r, account), ebsVolumeTags(r), true
	}, &result)
	addEBSSnapshotCoverage(cache, resources, &result)

	if clients.EC2 == nil {
		return result, nil
	}
	// Build a set of known resource IDs so we can detect unmatched API returns.
	knownIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			knownIDs[r.ID] = true
		}
	}
	var allVolumeStatuses []ec2types.VolumeStatusItem
	var nextToken *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.EC2.DescribeVolumeStatus(ctx, &ec2svc.DescribeVolumeStatusInput{
			NextToken: nextToken,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: result.TruncatedIDs}, err
		}
		allVolumeStatuses = append(allVolumeStatuses, out.VolumeStatuses...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	for _, v := range allVolumeStatuses {
		if v.VolumeId == nil {
			continue
		}
		volID := *v.VolumeId
		// Track unmatched: API returned a volume not in the input resources slice.
		if len(knownIDs) > 0 && !knownIDs[volID] {
			continue
		}
		if v.VolumeStatus == nil || v.VolumeStatus.Status == ec2types.VolumeStatusInfoStatusOk {
			continue
		}
		ioState := domain.HumanizeStatusPhrase(string(v.VolumeStatus.Status))
		rows := []domain.DetailRow{
			{Label: "I/O State", Value: ioState, Tier: "!"},
		}
		// Most recent event (if any).
		if len(v.Events) > 0 {
			ev := v.Events[0]
			eventVal := ""
			if ev.EventType != nil {
				eventVal = *ev.EventType
			}
			if ev.Description != nil && *ev.Description != "" {
				eventVal = *ev.Description
			}
			if eventVal != "" {
				rows = append(rows, domain.DetailRow{Label: "Event", Value: eventVal, Tier: "~"})
			}
		}
		// Most recent action code (if any).
		if len(v.Actions) > 0 {
			ac := v.Actions[0]
			if ac.Code != nil && *ac.Code != "" {
				rows = append(rows, domain.DetailRow{Label: "Action Code", Value: *ac.Code})
			}
		}
		setWave2Finding(&result, volID, ebsCodeVolumeIODegraded, "volume I/O degraded", "!", "ebs", rows)
	}
	result.Truncated = truncated
	return result, nil
}

// ebsVolumeARN builds the ARN a backup selection matches on. The ebs fetcher
// writes neither the account nor the region, so the account comes from the
// session's caller identity and the region is the volume's availability zone
// without its trailing letter. An unresolved account yields no ARN, which the
// join reads as "cannot tell" rather than "uncovered".
func ebsVolumeARN(r resource.Resource, account string) string {
	volumeID := r.Fields["volume_id"]
	if volumeID == "" {
		volumeID = r.ID
	}
	az := r.Fields["az"]
	if account == "" || volumeID == "" || az == "" {
		return ""
	}
	return "arn:aws:ec2:" + strings.TrimRight(az, "abcdefghijklmnopqrstuvwxyz") + ":" + account + ":volume/" + volumeID
}

// ebsVolumeTags reads the volume's own tags, which a backup selection may
// choose it by. ebs is the only type in this batch whose row carries them.
func ebsVolumeTags(r resource.Resource) map[string]string {
	vol, ok := assertStruct[ec2types.Volume](r.RawStruct)
	if !ok {
		return nil
	}
	tags := make(map[string]string, len(vol.Tags))
	for _, t := range vol.Tags {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}
	return tags
}

// addEBSSnapshotCoverage reports every attached volume the snapshot list holds
// no snapshot of. Like the backup join it answers from the cache alone, so an
// unfetched or cut-short snapshot list reports nothing: the snapshot may be on
// a page nobody read. A volume that is not attached is left alone — there is
// nothing running to lose.
func addEBSSnapshotCoverage(cache resource.ResourceCache, resources []resource.Resource, result *IssueEnricherResult) {
	entry, ok := cache["ebs-snap"]
	if !ok || entry.IsTruncated {
		return
	}
	snapshotted := make(map[string]bool, len(entry.Resources))
	for _, snap := range entry.Resources {
		if v := snap.Fields["volume_id"]; v != "" {
			snapshotted[v] = true
		}
	}
	for _, r := range resources {
		volumeID := r.Fields["volume_id"]
		if volumeID == "" {
			volumeID = r.ID
		}
		if volumeID == "" || r.Fields["state"] != string(ec2types.VolumeStateInUse) || snapshotted[volumeID] {
			continue
		}
		setWave2Finding(result, r.ID, CodeEBSNoSnapshot, "no snapshot exists", "~", "ebs", []domain.DetailRow{{Label: "Snapshots", Value: "0"}})
	}
}
