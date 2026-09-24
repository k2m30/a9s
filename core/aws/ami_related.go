// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkAMIEC2 checks the cache for EC2 instances launched from this AMI.
// Matches ec2.Fields["image_id"] against the AMI's ID.
func checkAMIEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return keyMissing("ec2", "amiID")
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("ec2", err)
	}
	if ec2List == nil {
		return NotRead("ec2")
	}

	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.Fields["image_id"] == amiID {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkAMIEBSSnaps reads the backing snapshot IDs from the AMI's block device mappings.
// The data is in RawStruct.
func checkAMIEBSSnaps(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return NotRead("ebs-snap")
	}
	return relatedResultTrunc("ebs-snap", amiSnapshotIDs(img), false)
}

// amiSnapshotIDs is the snapshots the AMI's EBS block devices are created
// from, Image.BlockDeviceMappings[].Ebs.SnapshotId
// (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EbsBlockDevice.html).
func amiSnapshotIDs(img ec2types.Image) []string {
	var ids []string
	for _, bdm := range img.BlockDeviceMappings {
		if bdm.Ebs != nil && aws.ToString(bdm.Ebs.SnapshotId) != "" {
			ids = append(ids, *bdm.Ebs.SnapshotId)
		}
	}
	return ids
}

// amiSnapshotRows reads the snapshot rows the AMI is registered from out of
// the ebs-snap list. A snapshot the list does not hold makes the read partial:
// the list is the account's own snapshots, and the one missing may be on a
// page not read, or shared from another account.
func amiSnapshotRows(ctx context.Context, clients any, cache resource.ResourceCache, img ec2types.Image) ([]resource.Resource, relatedRead) {
	ids := amiSnapshotIDs(img)
	if len(ids) == 0 {
		return nil, relatedRead{}
	}
	snaps, truncated, err := FetchRelatedTarget(ctx, clients, cache, "ebs-snap")
	if snaps == nil {
		return nil, unreadBy(err)
	}
	var rows []resource.Resource
	for _, s := range snaps {
		if slices.Contains(ids, s.ID) {
			rows = append(rows, s)
		}
	}
	return rows, relatedRead{partial: truncated || len(rows) < len(ids), failure: err}
}

// checkAMIASG reports the groups that launch from this AMI: the groups whose
// launch sources name it (readASGLaunch), running instances or not, since a
// group at desired capacity 0 launches from it the moment it scales out; and
// the groups running an instance of it, which an older template version may
// have launched.
func checkAMIASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return keyMissing("asg", "amiID")
	}
	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return ReadFailed("asg", err)
	}
	if asgList == nil {
		return NotRead("asg")
	}
	// The instance list is the second place the relation lives: one not read
	// leaves the launch-source matches a lower bound, not the answer.
	ec2Image := map[string]string{}
	var instances relatedRead
	if len(asgList) > 0 {
		ec2List, ec2Truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
		instances = pagedRead(!ec2Truncated, err)
		for _, ec2Res := range ec2List {
			ec2Image[ec2Res.ID] = ec2Res.Fields["image_id"]
		}
	}
	asgList, capped := fanOut(asgList)
	truncated = truncated || capped
	var ids []string
	var reads rowReads
	for _, asgRes := range asgList {
		asg, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			reads.missed()
			continue
		}
		if slices.ContainsFunc(asgMembers(asg), func(id string) bool { return ec2Image[id] == amiID }) {
			reads.read++
			ids = append(ids, asgRes.ID)
			continue
		}
		launch, read := readASGLaunch(ctx, clients, asg)
		switch {
		case slices.Contains(launch.images, amiID):
			reads.read++
			ids = append(ids, asgRes.ID)
		case read.failed:
			reads.fail(asgRes.ID, read.failure)
		case read.unread:
			reads.missed()
		default:
			reads.read++
		}
		truncated = truncated || read.partial
	}
	return alsoRead(reads.answer("asg", "ami-related: launch source", ids, truncated), instances)
}
