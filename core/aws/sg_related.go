// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sg_related.go contains Security Group related-resource checker functions.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSGVPC reads the vpc_id field directly from the SG resource.
func checkSGVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkSGEC2 scans the EC2 cache for instances whose SecurityGroups slice
// contains a GroupId matching the security group's ID.
func checkSGEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return keyMissing("ec2", "sgID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("ec2", err)
	}
	if list == nil {
		return NotRead("ec2")
	}

	var ids []string
	for _, r := range list {
		inst, ok := assertStruct[ec2types.Instance](r.RawStruct)
		if !ok {
			continue
		}
		for _, sg := range inst.SecurityGroups {
			if sg.GroupId != nil && *sg.GroupId == sgID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkSGENI scans the ENI cache for network interfaces whose Groups slice
// contains a GroupId matching the security group's ID.
func checkSGENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return keyMissing("eni", "sgID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if list == nil {
		return NotRead("eni")
	}

	var ids []string
	for _, r := range list {
		eni, ok := assertStruct[ec2types.NetworkInterface](r.RawStruct)
		if !ok {
			continue
		}
		for _, sg := range eni.Groups {
			if sg.GroupId != nil && *sg.GroupId == sgID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkSGELB scans the ELB cache for load balancers whose SecurityGroups slice
// contains a value matching the security group's ID.
func checkSGELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return keyMissing("elb", "sgID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return ReadFailed("elb", err)
	}
	if list == nil {
		return NotRead("elb")
	}

	var ids []string
	for _, r := range list {
		lb, ok := assertStruct[elbv2types.LoadBalancer](r.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(lb.SecurityGroups, sgID) {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncated)
}

// checkSGCFN checks the SG's tags for aws:cloudformation:stack-name.
func checkSGCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.SecurityGroup](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}
	return relatedResultTrunc("cfn", []string{stackName}, false)
}

// checkSGSG counts the security groups this group's own rules name: the
// UserIdGroupPairs[].GroupId of its IpPermissions and IpPermissionsEgress
// (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_UserIdGroupPair.html).
func checkSGSG(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sg, ok := assertStruct[ec2types.SecurityGroup](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	// A pair whose UserId is another account's names a group of that
	// account, which no row of this list is.
	var refs []string
	foreign := false
	for _, p := range append(slices.Clone(sg.IpPermissions), sg.IpPermissionsEgress...) {
		for _, pair := range p.UserIdGroupPairs {
			switch id := aws.ToString(pair.GroupId); {
			case pair.UserId != nil && sg.OwnerId != nil && *pair.UserId != *sg.OwnerId:
				foreign = true
			case id != res.ID:
				refs = append(refs, id)
			}
		}
	}
	return alsoPartial(relatedRefs("sg", refs, refContext(clients, cache, "sg")), foreign)
}

// checkSGLambda scans the Lambda cache for functions whose VpcConfig.SecurityGroupIds
// slice contains this security group's ID. Pattern C — reverse lookup.
func checkSGLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return keyMissing("lambda", "sgID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if list == nil {
		return NotRead("lambda")
	}

	var ids []string
	for _, r := range list {
		fn, ok := assertStruct[lambdatypes.FunctionConfiguration](r.RawStruct)
		if !ok {
			continue
		}
		if fn.VpcConfig == nil {
			continue
		}
		if slices.Contains(fn.VpcConfig.SecurityGroupIds, sgID) {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}
