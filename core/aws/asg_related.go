// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkASGEC2 reads Instances[] from the ASG RawStruct and returns their IDs.
func checkASGEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("ec2")
	}
	return relatedResultTrunc("ec2", asgMembers(asg), false)
}

// checkASGAlarm searches the alarm cache for alarms with an "AutoScalingGroupName" dimension
// matching this ASG's name.
func checkASGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "asg", res)
}

// checkASGNG searches the node group cache for EKS node groups whose AutoScalingGroups
// include this ASG by name.
func checkASGNG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asgName := res.ID
	if asgName == "" {
		return keyMissing("ng", "asgName")
	}

	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return ReadFailed("ng", err)
	}
	if ngList == nil {
		return NotRead("ng")
	}

	var ids []string
	for _, ngRes := range ngList {
		ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		if !ok {
			continue
		}
		if ng.Resources == nil {
			continue
		}
		for _, asgItem := range ng.Resources.AutoScalingGroups {
			if asgItem.Name != nil && *asgItem.Name == asgName {
				ids = append(ids, ngRes.ID)
				break
			}
		}
	}
	return relatedAnswer("ng", relatedRead{ids: ids, partial: truncated, atMostOne: true})
}

// checkASGAMI reports the images the group's launch sources launch from.
func checkASGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("ami")
	}
	launch, read := readASGLaunch(ctx, clients, asg)
	ids, dropped := resolveRefs("ami", launch.images, refContext(clients, nil, "ami"))
	return relatedAnswer("ami", joinReads(read, relatedRead{ids: ids, partial: dropped}))
}

// checkASGELB resolves the ALB/NLB behind this ASG's TargetGroupARNs via
// elbv2:DescribeTargetGroups.LoadBalancerArns. Classic LoadBalancerNames name
// no row: the elb type holds ELBv2 load balancers only.
func checkASGELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("elb")
	}
	if len(asg.TargetGroupARNs) == 0 {
		return foundNone("elb", "asg.TargetGroupARNs")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("elb")
	}
	tgs, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.TargetGroup, *string, error) {
		out, err := c.ELBv2.DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{
			TargetGroupArns: asg.TargetGroupARNs,
			Marker:          marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.TargetGroups, out.NextMarker, nil
	})
	if err != nil {
		return ReadFailed("elb", err)
	}
	var refs []string
	for _, tg := range tgs {
		refs = append(refs, tg.LoadBalancerArns...)
	}
	ids, dropped := resolveRefs("elb", refs, refContext(clients, cache, "elb"))
	return relatedResultTrunc("elb", ids, dropped || !complete)
}

// checkASGRole reports the group's roles: its service-linked role, and the
// roles of the instance profiles its launch sources name, read with
// iam:GetInstanceProfile.
func checkASGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	refs := []string{aws.ToString(asg.ServiceLinkedRoleARN)}
	launch, read := readASGLaunch(ctx, clients, asg)
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		for _, profile := range slices.Compact(slices.Sorted(slices.Values(launch.profiles))) {
			if profile == "" {
				continue
			}
			roles, resolved := asgInstanceProfileToRoles(ctx, c, profile)
			refs = append(refs, roles...)
			read.partial = read.partial || !resolved
		}
	}
	ids, dropped := resolveRefs("role", refs, refContext(clients, cache, "role"))
	return relatedAnswer("role", joinReads(read, relatedRead{ids: ids, partial: dropped}))
}

// asgInstanceProfileToRoles resolves a profile name or ARN to role ARNs via
// iam:GetInstanceProfile. resolved is false when the call did not answer, so
// the caller can say its role count is a lower bound rather than exact.
func asgInstanceProfileToRoles(ctx context.Context, c *ServiceClients, profileNameOrARN string) (roles []string, resolved bool) {
	profileName := instanceProfileName(profileNameOrARN)
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetInstanceProfileOutput, error) {
		return c.IAM.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
	})
	// The caller marks its role count a lower bound rather than reporting the
	// roles it did read as all of them.
	// no finding: false is the record.
	if err != nil || out.InstanceProfile == nil {
		return nil, false
	}
	var roleARNs []string
	for _, r := range out.InstanceProfile.Roles {
		if r.Arn != nil && *r.Arn != "" {
			roleARNs = append(roleARNs, *r.Arn)
		}
	}
	return roleARNs, true
}

// launchConfigurations reads the named launch configuration.
func launchConfigurations(ctx context.Context, api ASGDescribeLaunchConfigurationsAPI, name string) ([]asgtypes.LaunchConfiguration, error) {
	lcs, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]asgtypes.LaunchConfiguration, *string, error) {
		out, err := api.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
			LaunchConfigurationNames: []string{name},
			NextToken:                token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.LaunchConfigurations, out.NextToken, nil
	})
	return lcs, err
}

// launchTemplateVersions reads the version of a launch template a group
// launches from. A LaunchTemplateSpecification that names no version takes
// AWS's default for the field, "$Default" — the template's default version,
// which is not necessarily its latest.
func launchTemplateVersions(ctx context.Context, api EC2DescribeLaunchTemplateVersionsAPI, id, version *string) ([]ec2types.LaunchTemplateVersion, error) {
	v := cmp.Or(aws.ToString(version), "$Default")
	versions, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.LaunchTemplateVersion, *string, error) {
		out, err := api.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: id,
			Versions:         []string{v},
			NextToken:        token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.LaunchTemplateVersions, out.NextToken, nil
	})
	return versions, err
}
