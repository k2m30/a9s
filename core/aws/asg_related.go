// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"

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
	var ids []string
	for _, inst := range asg.Instances {
		if inst.InstanceId != nil && *inst.InstanceId != "" {
			ids = append(ids, *inst.InstanceId)
		}
	}
	if len(ids) == 0 {
		return foundNone("ec2", "ids")
	}
	return relatedResultTrunc("ec2", ids, false)
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
		return foundNone("ng", "asgName")
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

// checkASGAMI resolves the AMI used by the ASG's launch configuration or launch template.
// For launch configs: autoscaling:DescribeLaunchConfigurations.ImageId.
// For launch templates: ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId.
func checkASGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("ami")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("ami")
	}

	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		lcs, err := launchConfigurations(ctx, c.AutoScaling, *asg.LaunchConfigurationName)
		if err != nil {
			return ReadFailed("ami", err)
		}
		if len(lcs) > 0 && aws.ToString(lcs[0].ImageId) != "" {
			return relatedRefs("ami", []string{*lcs[0].ImageId}, refContext(clients, nil, "ami"))
		}
		return foundNone("ami", "LaunchConfiguration.ImageId")
	}

	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		return foundNone("ami", "ltSpec.LaunchTemplateId")
	}

	versions, err := launchTemplateVersions(ctx, c.EC2, ltSpec.LaunchTemplateId, ltSpec.Version)
	if err != nil {
		return ReadFailed("ami", err)
	}
	for _, v := range versions {
		if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
			return relatedRefs("ami", []string{*v.LaunchTemplateData.ImageId}, refContext(clients, nil, "ami"))
		}
	}
	return foundNone("ami", "LaunchTemplateData.ImageId")
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

// checkASGRole resolves IAM roles associated with this ASG.
// Sources: ServiceLinkedRoleARN (direct), LaunchConfig.IamInstanceProfile, LaunchTemplate.IamInstanceProfile.
// Instance profile names are resolved to role ARNs via iam:GetInstanceProfile.
func checkASGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return NotRead("role")
	}

	var refs []string
	if asg.ServiceLinkedRoleARN != nil && *asg.ServiceLinkedRoleARN != "" {
		refs = append(refs, *asg.ServiceLinkedRoleARN)
	}
	rc := refContext(clients, cache, "role")

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		// Without clients, the instance-profile role path was never checked —
		// ids (if any) is a lower bound, not the exhaustive answer.
		if ids, _ := resolveRefs("role", refs, rc); len(ids) > 0 {
			return relatedResultTrunc("role", ids, true)
		}
		return NotRead("role")
	}

	// Resolve instance profile from launch config or launch template. A call
	// that did not answer leaves the same gap the missing-clients branch
	// above describes, and gets the same answer: what was found is a lower
	// bound, and nothing found is unknown rather than none.
	profileNameOrARN, checked := asgResolveInstanceProfile(ctx, c, asg)
	if checked && profileNameOrARN != "" {
		roleARNs, resolved := asgInstanceProfileToRoles(ctx, c, profileNameOrARN)
		refs = append(refs, roleARNs...)
		checked = resolved
	}
	ids, dropped := resolveRefs("role", refs, rc)
	if !checked {
		if len(ids) > 0 {
			return relatedResultTrunc("role", ids, true)
		}
		return NotRead("role")
	}

	return relatedResultTrunc("role", ids, dropped)
}

// asgResolveInstanceProfile reads the IamInstanceProfile from the ASG's launch config or launch template.
// Returns the profile name or ARN, or empty string if none is found.
// checked is false when a call did not answer, which is a different fact from
// an ASG that names no instance profile: the first leaves the role pivot's
// count a lower bound, the second makes it exact.
func asgResolveInstanceProfile(ctx context.Context, c *ServiceClients, asg asgtypes.AutoScalingGroup) (profile string, checked bool) {
	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		lcs, err := launchConfigurations(ctx, c.AutoScaling, *asg.LaunchConfigurationName)
		// The caller turns false into a lower-bound count or UnknownRelated,
		// so the role pivot never claims an exact number it did not read.
		// no finding: false is the record.
		if err != nil || len(lcs) == 0 {
			return "", false
		}
		if lcs[0].IamInstanceProfile != nil {
			return *lcs[0].IamInstanceProfile, true
		}
		// no finding: the launch configuration was read and names no instance
		// profile, which is an answer.
		return "", true
	}

	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		// no finding: the ASG names no launch template, so there is no
		// instance profile to read and nothing went unanswered.
		return "", true
	}

	versions, err := launchTemplateVersions(ctx, c.EC2, ltSpec.LaunchTemplateId, ltSpec.Version)
	// As in the launch-configuration arm above, the caller renders the pivot
	// as a lower bound or as unknown.
	// no finding: false is the record.
	if err != nil || len(versions) == 0 {
		return "", false
	}
	ltData := versions[0].LaunchTemplateData
	if ltData == nil || ltData.IamInstanceProfile == nil {
		// no finding: the template version was read and declares no instance
		// profile, which is an answer.
		return "", true
	}
	if ltData.IamInstanceProfile.Arn != nil && *ltData.IamInstanceProfile.Arn != "" {
		return *ltData.IamInstanceProfile.Arn, true
	}
	if ltData.IamInstanceProfile.Name != nil && *ltData.IamInstanceProfile.Name != "" {
		return *ltData.IamInstanceProfile.Name, true
	}
	// no finding: the profile block is present and names neither an ARN nor a
	// name, which the template read answered for.
	return "", true
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
