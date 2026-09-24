// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lt_related.go contains Launch Template related-resource checker functions.
//
// Four checkers (ami, kms, sg, subnet) read the RawStruct: the fetcher's
// DescribeLaunchTemplateVersions("$Default") pass already carries every
// field they read, via assertStruct[LTRaw] — zero extra AWS calls, as in
// transfer_related.go. A degraded row's zero DefaultVersion reads as empty
// fields (never a panic), reported as unknown per checker.
//
// Three checkers (asg, ng, ec2) are the reverse direction — "who references
// this template" — and read the SIBLING resource cache directly rather than
// RawStruct, via the shared cachedTypedRows tri-state helper
// (related_common.go): cache absent → unknown ("?"), cache present with rows
// that don't assert to the expected type (disk-seeded, no RawStruct) →
// unknown, cache present and typed → scan. Never a live AWS call, never a
// fake zero. See docs/resources/lt.md.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkLTAMI reads DefaultVersion.LaunchTemplateData.ImageId directly;
// pivots only when the value matches "ami-" — a "resolve:ssm:" reference
// is a display fact, not a pivot (docs/resources/lt.md).
func checkLTAMI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[LTRaw](res.RawStruct)
	if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
		return NotRead("ami")
	}
	imageID := aws.ToString(raw.DefaultVersion.LaunchTemplateData.ImageId)
	if !strings.HasPrefix(imageID, "ami-") {
		return foundNone("ami", "imageID")
	}
	return relatedResultTrunc("ami", []string{imageID}, false)
}

// checkLTKMS reads DefaultVersion.LaunchTemplateData.BlockDeviceMappings[].Ebs.KmsKeyId
// directly; key-id/ARN forms only — alias forms are detail-only
// (docs/resources/lt.md), so an "alias/..." or ":alias/..."
// reference is skipped rather than extracted.
func checkLTKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[LTRaw](res.RawStruct)
	if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
		return NotRead("kms")
	}
	var ids []string
	for _, bdm := range raw.DefaultVersion.LaunchTemplateData.BlockDeviceMappings {
		if bdm.Ebs == nil || bdm.Ebs.KmsKeyId == nil || *bdm.Ebs.KmsKeyId == "" {
			continue
		}
		keyRef := *bdm.Ebs.KmsKeyId
		if strings.HasPrefix(keyRef, "alias/") || strings.Contains(keyRef, ":alias/") {
			continue
		}
		ids = append(ids, kmsRefFromField(keyRef, "lt"))
	}
	return kmsRelated(ctx, clients, cache, ids)
}

// checkLTSG unions SecurityGroupIds and NetworkInterfaces[].Groups —
// mutually exclusive by API design (docs/resources/lt.md); SecurityGroups
// (EC2-Classic names) are detail-only.
func checkLTSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[LTRaw](res.RawStruct)
	if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
		return NotRead("sg")
	}
	data := raw.DefaultVersion.LaunchTemplateData
	ids := make([]string, 0, len(data.SecurityGroupIds))
	ids = append(ids, data.SecurityGroupIds...)
	for _, ni := range data.NetworkInterfaces {
		ids = append(ids, ni.Groups...)
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkLTSubnet reads DefaultVersion.LaunchTemplateData.NetworkInterfaces[].SubnetId
// directly; usually empty by design (docs/resources/lt.md — the subnet
// normally comes from the ASG/NG side).
func checkLTSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[LTRaw](res.RawStruct)
	if !ok || raw.DefaultVersion.LaunchTemplateData == nil {
		return NotRead("subnet")
	}
	var ids []string
	for _, ni := range raw.DefaultVersion.LaunchTemplateData.NetworkInterfaces {
		if ni.SubnetId != nil && *ni.SubnetId != "" {
			ids = append(ids, *ni.SubnetId)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkLTASG scans the already-loaded "asg" cache for groups referencing
// this template, via LaunchTemplate, the
// MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification, or any
// per-Overrides[] LaunchTemplateSpecification (docs/resources/lt.md).
// Zero extra API calls.
func checkLTASG(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asgList, truncated, ok := cachedTypedRows[asgtypes.AutoScalingGroup](cache, "asg")
	if !ok {
		return NotRead("asg")
	}
	var ids []string
	for _, row := range asgList {
		if ltReferencedByASG(row.Raw, res.ID, res.Name) {
			ids = append(ids, row.ID)
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// ltReferencedByASG reports whether asg's plain LaunchTemplate, its
// MixedInstancesPolicy launch template, or any of its per-instance-type
// Overrides[] name the template with ltID / ltName.
func ltReferencedByASG(asg asgtypes.AutoScalingGroup, ltID, ltName string) bool {
	names := func(spec *asgtypes.LaunchTemplateSpecification) bool {
		return spec != nil && namesLaunchTemplate(aws.ToString(spec.LaunchTemplateId), aws.ToString(spec.LaunchTemplateName), ltID, ltName)
	}
	if names(asg.LaunchTemplate) {
		return true
	}
	if asg.MixedInstancesPolicy == nil || asg.MixedInstancesPolicy.LaunchTemplate == nil {
		return false
	}
	mip := asg.MixedInstancesPolicy.LaunchTemplate
	if names(mip.LaunchTemplateSpecification) {
		return true
	}
	return slices.ContainsFunc(mip.Overrides, func(o asgtypes.LaunchTemplateOverrides) bool {
		return names(o.LaunchTemplateSpecification)
	})
}

// checkLTNG scans the already-loaded "ng" cache for node groups pinning this
// template via Nodegroup.LaunchTemplate.Id or .Name
// (docs/resources/lt.md). Zero extra API calls.
func checkLTNG(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ngList, truncated, ok := cachedTypedRows[ekstypes.Nodegroup](cache, "ng")
	if !ok {
		return NotRead("ng")
	}
	var ids []string
	for _, row := range ngList {
		if row.Raw.LaunchTemplate == nil {
			continue
		}
		if namesLaunchTemplate(aws.ToString(row.Raw.LaunchTemplate.Id), aws.ToString(row.Raw.LaunchTemplate.Name), res.ID, res.Name) {
			ids = append(ids, row.ID)
		}
	}
	return relatedResultTrunc("ng", ids, truncated)
}

// checkLTEC2 scans the already-loaded "ec2" cache for instances tagged with
// this template's id via the AWS auto-tag "aws:ec2launchtemplate:id" —
// catches direct, ASG-launched, and NG-launched instances alike
// (docs/resources/lt.md). Zero extra API calls.
func checkLTEC2(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ec2List, truncated, ok := cachedTypedRows[ec2types.Instance](cache, "ec2")
	if !ok {
		return NotRead("ec2")
	}
	var ids []string
	for _, row := range ec2List {
		if tagValue(row.Raw.Tags, "aws:ec2launchtemplate:id") == res.ID {
			ids = append(ids, row.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}
