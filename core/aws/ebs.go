// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEBSVolumesPage calls the EC2 DescribeVolumes API and returns a single
// page of volumes. Pass an empty continuationToken for the first page.
func FetchEBSVolumesPage(ctx context.Context, api EC2DescribeVolumesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVolumesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeVolumes(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EBS volumes: %w", err)
	}

	var resources []resource.Resource
	for _, vol := range output.Volumes {
		volumeID := ""
		if vol.VolumeId != nil {
			volumeID = *vol.VolumeId
		}

		name := ""
		for _, tag := range vol.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		state := string(vol.State)

		size := ""
		if vol.Size != nil {
			size = strconv.Itoa(int(*vol.Size))
		}

		volType := string(vol.VolumeType)

		iops := ""
		if vol.Iops != nil {
			iops = strconv.Itoa(int(*vol.Iops))
		}

		encrypted := "false"
		if vol.Encrypted != nil && *vol.Encrypted {
			encrypted = "true"
		}

		// Multi-Attach puts one io1/io2 volume on up to 16 instances at once,
		// and each of them carries the volume in its own block device mappings.
		var attached []string
		for _, att := range vol.Attachments {
			if att.InstanceId != nil && *att.InstanceId != "" {
				attached = append(attached, *att.InstanceId)
			}
		}
		attachedTo := strings.Join(attached, ",")

		az := ""
		if vol.AvailabilityZone != nil {
			az = *vol.AvailabilityZone
		}

		created := ""
		if vol.CreateTime != nil {
			created = vol.CreateTime.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:   volumeID,
			Name: name,
			Fields: map[string]string{
				"volume_id":   volumeID,
				"name":        name,
				"state":       state,
				"size":        size,
				"type":        volType,
				"iops":        iops,
				"encrypted":   encrypted,
				"attached_to": attachedTo,
				"az":          az,
				"created":     created,
			},
			RawStruct: vol,
		}

		r = withWave1(r, ebsFindings(state, attachedTo, created, encrypted)...)

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// FetchEBSSnapshotsPage calls the EC2 DescribeSnapshots API and returns a single
// page of snapshots. Only returns snapshots owned by the caller ("self").
// Pass an empty continuationToken for the first page.
func FetchEBSSnapshotsPage(ctx context.Context, api EC2DescribeSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeSnapshotsInput{
		OwnerIds:   []string{"self"},
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeSnapshots(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EBS snapshots: %w", err)
	}

	var resources []resource.Resource
	for _, snap := range output.Snapshots {
		resources = append(resources, snapshotToResource(snap))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// FetchEBSSnapshotsByIDs fetches specific EBS snapshots by ID, bypassing the
// OwnerIds=self filter the paginated fetcher applies. Used by the related-panel
// lazy-add path so checkers referencing shared or public snapshots still drill
// into a real entry, and by an exact-ID drill. Missing IDs are handled by
// fetchEC2ByIDs.
func FetchEBSSnapshotsByIDs(ctx context.Context, api EC2DescribeSnapshotsAPI, ids []string) ([]resource.Resource, error) {
	return fetchEC2ByIDs(ctx, "ebs-snap FetchByIDs", "InvalidSnapshot.NotFound", "snap-", ids, func(ids []string) ([]resource.Resource, error) {
		out, err := api.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{SnapshotIds: ids})
		if err != nil {
			return nil, err
		}
		resources := make([]resource.Resource, 0, len(out.Snapshots))
		for _, snap := range out.Snapshots {
			resources = append(resources, snapshotToResource(snap))
		}
		return resources, nil
	})
}

// ebsSnapshotUnusable reports a snapshot nothing can be restored from as it
// stands: a failed one, or one in the Recycle Bin. The ebs-snap state table
// classes them, so the list and the volume enricher's snapshot-coverage join
// cannot disagree about which snapshot counts as cover.
func ebsSnapshotUnusable(state string) bool {
	class := LifecycleClassOf("ebs-snap", state)
	return class == catalog.LifecycleFailedLive || class == catalog.LifecyclePendingDeletion
}

// snapshotToResource converts an ec2types.Snapshot to our generic Resource.
// Extracted so FetchEBSSnapshotsPage and FetchEBSSnapshotsByIDs agree on
// shape — the lazy-add path must emit the same field keys that the paginated
// path does, otherwise reverse-scan checkers reading Fields["volume_id"]
// miss lazily-added snapshots.
func snapshotToResource(snap ec2types.Snapshot) resource.Resource {
	snapshotID := ""
	if snap.SnapshotId != nil {
		snapshotID = *snap.SnapshotId
	}
	name := ""
	for _, tag := range snap.Tags {
		if tag.Key != nil && *tag.Key == "Name" {
			if tag.Value != nil {
				name = *tag.Value
			}
			break
		}
	}
	state := string(snap.State)
	volumeID := ""
	if snap.VolumeId != nil {
		volumeID = *snap.VolumeId
	}
	size := ""
	if snap.VolumeSize != nil {
		size = strconv.Itoa(int(*snap.VolumeSize))
	}
	encrypted := strconv.FormatBool(!ebsSnapUnencrypted(snap))
	description := ""
	if snap.Description != nil {
		description = *snap.Description
	}
	started := ""
	if snap.StartTime != nil {
		started = snap.StartTime.Format("2006-01-02 15:04")
	}
	progress := ""
	if snap.Progress != nil {
		progress = *snap.Progress
	}
	r := resource.Resource{
		ID:   snapshotID,
		Name: name,
		Fields: map[string]string{
			"snapshot_id": snapshotID,
			"name":        name,
			"state":       state,
			"volume_id":   volumeID,
			"size":        size,
			"encrypted":   encrypted,
			"description": description,
			"started":     started,
			"progress":    progress,
		},
		RawStruct: snap,
	}

	r = withWave1(r, withState("ebs-snap", state, ebsSnapStructuralFindings(snap))...)
	return r
}

// ebsSnapUnencrypted is the one reading of a snapshot's encryption for both
// the encrypted field and the finding: a snapshot AWS does not state as
// encrypted counts as unencrypted.
func ebsSnapUnencrypted(snap ec2types.Snapshot) bool {
	return snap.Encrypted == nil || !*snap.Encrypted
}

func ebsSnapStructuralFindings(snap ec2types.Snapshot) []domain.Finding {
	if ebsSnapUnencrypted(snap) {
		return []domain.Finding{wave1Finding(CodeEBSSnapUnencrypted)}
	}
	if snap.StartTime != nil {
		if age := time.Since(*snap.StartTime); age > days(ebsSnapAgedAfterDays) {
			desc := ""
			if snap.Description != nil {
				desc = *snap.Description
			}
			isAutomated := strings.HasPrefix(desc, "Created by CreateImage") ||
				strings.Contains(strings.ToLower(desc), "automated")
			if isAutomated {
				days := int(age.Hours() / 24)
				return []domain.Finding{wave1Finding(CodeEBSSnapAgedAutomated, strconv.Itoa(days))}
			}
		}
	}
	return nil
}

// ebsFindings is the one predicate for a volume. The orphan check is
// declared for the normal states only: a volume being created or deleted is
// unattached by its lifecycle.
func ebsFindings(state, attachedTo, created, encrypted string) []domain.Finding {
	var posture []domain.Finding
	if attachedTo == "" {
		if t, err := time.Parse("2006-01-02 15:04", created); err == nil {
			if age := time.Since(t); age > days(ebsOrphanAfterDays) {
				days := int(age.Hours() / 24)
				posture = append(posture, wave1Finding(CodeEBSOrphanUnattached, strconv.Itoa(days)))
			}
		}
	}
	if encrypted == "false" {
		posture = append(posture, wave1Finding(CodeEBSUnencrypted))
	}
	return withState("ebs", state, posture)
}
