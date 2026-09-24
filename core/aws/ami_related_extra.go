// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ami_related_extra.go contains additional AMI related-resource checkers
// required by docs/related-resources.md.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkAMICFN scans AMI tags for aws:cloudformation:stack-name and matches
// the cfn cache. CloudFormation records no stack on an image, and finding the
// stacks that name one means reading every template, so an image without the
// tag was not searched.
func checkAMICFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ""
	if img, ok := assertStruct[ec2types.Image](res.RawStruct); ok {
		for _, t := range img.Tags {
			if t.Key != nil && *t.Key == "aws:cloudformation:stack-name" && t.Value != nil {
				stackName = *t.Value
				break
			}
		}
	}
	if stackName == "" {
		return relatedAnswer("cfn", relatedRead{unread: true})
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
}

// checkAMIKMS counts the keys the AMI's snapshots are encrypted with
// (Snapshot.KmsKeyId). DescribeImages returns no key on a block device: an
// EbsBlockDevice's KmsKeyId "is only supported on BlockDeviceMapping objects
// called by RunInstances, RequestSpotFleet, and RequestSpotInstances"
// (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EbsBlockDevice.html).
func checkAMIKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}
	snaps, read := amiSnapshotRows(ctx, clients, cache, img)
	var refs []string
	for _, s := range snaps {
		if snap, ok := assertStruct[ec2types.Snapshot](s.RawStruct); ok {
			refs = append(refs, aws.ToString(snap.KmsKeyId))
		} else {
			read.partial = true
		}
	}
	return relatedAnswer("kms", joinReads(read, readOf(kmsRelated(ctx, clients, cache, refs))))
}

// checkAMING scans the node-group list for node groups using this AMI:
// Fields["image_id"], which the fetcher reads off the launch template. A
// node group on a template whose read failed has it absent and may run this
// AMI; one without a template runs the EKS-optimised image of its AmiType.
func checkAMING(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return keyMissing("ng", "amiID")
	}
	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return ReadFailed("ng", err)
	}
	if ngList == nil {
		return NotRead("ng")
	}
	var ids []string
	unread := false
	for _, ngRes := range ngList {
		imageID, read := ngRes.Fields["image_id"]
		switch {
		case imageID == amiID:
			ids = append(ids, ngRes.ID)
		case !read:
			ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
			unread = unread || ok && ng.LaunchTemplate != nil && aws.ToString(ng.LaunchTemplate.Id) != ""
		}
	}
	return relatedResultTrunc("ng", ids, truncated || unread)
}
