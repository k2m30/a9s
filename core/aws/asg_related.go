// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkASGEC2 reads Instances[] from the ASG RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkASGEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	var ids []string
	for _, inst := range asg.Instances {
		if inst.InstanceId != nil && *inst.InstanceId != "" {
			ids = append(ids, *inst.InstanceId)
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("ec2", nil, false)
	}
	return relatedResult("ec2", ids)
}

// checkASGAlarm searches the alarm cache for alarms with an "AutoScalingGroupName" dimension
// matching this ASG's name.
// Pattern D — dimension-based lookup.
func checkASGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "AutoScalingGroupName", res.ID)
}

// checkASGNG searches the node group cache for EKS node groups whose AutoScalingGroups
// include this ASG by name.
// Pattern C — reverse cache lookup.
func checkASGNG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asgName := res.ID
	if asgName == "" {
		return resource.KnownRelated("ng", nil, false)
	}

	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return resource.ErrorRelated("ng", err)
	}
	if ngList == nil {
		return resource.UnknownRelated("ng")
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
	return relatedResultTrunc("ng", ids, truncated)
}

// checkASGAMI resolves the AMI used by the ASG's launch configuration or launch template.
// For launch configs: autoscaling:DescribeLaunchConfigurations.ImageId.
// For launch templates: ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId.
func checkASGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ami")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("ami")
	}

	// LaunchConfigurationName path
	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			return c.AutoScaling.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
				LaunchConfigurationNames: []string{*asg.LaunchConfigurationName},
			})
		})
		if err != nil {
			return resource.ErrorRelated("ami", err)
		}
		if len(out.LaunchConfigurations) > 0 && out.LaunchConfigurations[0].ImageId != nil {
			imageID := *out.LaunchConfigurations[0].ImageId
			if imageID != "" {
				return relatedResult("ami", []string{imageID})
			}
		}
		return resource.KnownRelated("ami", nil, false)
	}

	// LaunchTemplate path (direct or via MixedInstancesPolicy)
	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		return resource.KnownRelated("ami", nil, false)
	}

	version := aws.String("$Latest")
	if ltSpec.Version != nil && *ltSpec.Version != "" {
		version = ltSpec.Version
	}

	ltOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
		return c.EC2.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: ltSpec.LaunchTemplateId,
			Versions:         []string{*version},
		})
	})
	if err != nil {
		return resource.ErrorRelated("ami", err)
	}
	for _, v := range ltOut.LaunchTemplateVersions {
		if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
			return relatedResult("ami", []string{*v.LaunchTemplateData.ImageId})
		}
	}
	return resource.KnownRelated("ami", nil, false)
}

// checkASGELB resolves load balancers associated with this ASG.
// Classic ELB names come directly from parent.LoadBalancerNames.
// ALB/NLB ARNs are resolved from parent.TargetGroupARNs via elbv2:DescribeTargetGroups.LoadBalancerArns.
func checkASGELB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("elb")
	}
	if len(asg.LoadBalancerNames) == 0 && len(asg.TargetGroupARNs) == 0 {
		return resource.KnownRelated("elb", nil, false)
	}

	var ids []string
	// Classic ELB names are direct IDs
	ids = append(ids, asg.LoadBalancerNames...)

	// Resolve ALB/NLB from TG ARNs. Bailing out here still reports the classic
	// ELB names already collected, but as a truncated lower bound — the
	// ALB/NLB side of TargetGroupARNs was never resolved, so ids is not the
	// exhaustive answer.
	if len(asg.TargetGroupARNs) > 0 {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			if len(ids) > 0 {
				return relatedResultTrunc("elb", ids, true)
			}
			return resource.UnknownRelated("elb")
		}
		tgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetGroupsOutput, error) {
			return c.ELBv2.DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{
				TargetGroupArns: asg.TargetGroupARNs,
			})
		})
		if err != nil {
			if len(ids) > 0 {
				return relatedResultTrunc("elb", ids, true)
			}
			return resource.ErrorRelated("elb", err)
		}
		for _, tg := range tgOut.TargetGroups {
			ids = append(ids, tg.LoadBalancerArns...)
		}
	}
	return relatedResult("elb", ids)
}

// checkASGRole resolves IAM roles associated with this ASG.
// Sources: ServiceLinkedRoleARN (direct), LaunchConfig.IamInstanceProfile, LaunchTemplate.IamInstanceProfile.
// Instance profile names are resolved to role ARNs via iam:GetInstanceProfile.
func checkASGRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}

	var ids []string

	// ServiceLinkedRoleARN is directly a role ARN — strip to the bare
	// RoleName so the role cache's FetchByIDs (keyed on RoleName) resolves it,
	// mirroring the ec2_related.go precedent for role-ARN-to-name reduction.
	if asg.ServiceLinkedRoleARN != nil && *asg.ServiceLinkedRoleARN != "" {
		ids = append(ids, arnRoleName(*asg.ServiceLinkedRoleARN))
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		// Without clients, the instance-profile role path was never checked —
		// ids (if any) is a lower bound, not the exhaustive answer.
		if len(ids) > 0 {
			return relatedResultTrunc("role", ids, true)
		}
		return resource.UnknownRelated("role")
	}

	// Resolve instance profile from launch config or launch template
	profileNameOrARN := asgResolveInstanceProfile(ctx, c, asg)
	if profileNameOrARN != "" {
		roleARNs := asgInstanceProfileToRoles(ctx, c, profileNameOrARN)
		ids = append(ids, roleARNs...)
	}

	return relatedResult("role", ids)
}

// asgResolveInstanceProfile reads the IamInstanceProfile from the ASG's launch config or launch template.
// Returns the profile name or ARN, or empty string if none is found.
func asgResolveInstanceProfile(ctx context.Context, c *ServiceClients, asg asgtypes.AutoScalingGroup) string {
	// Launch configuration path
	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			return c.AutoScaling.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
				LaunchConfigurationNames: []string{*asg.LaunchConfigurationName},
			})
		})
		if err == nil && len(out.LaunchConfigurations) > 0 && out.LaunchConfigurations[0].IamInstanceProfile != nil {
			return *out.LaunchConfigurations[0].IamInstanceProfile
		}
		return ""
	}

	// Launch template path
	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		return ""
	}

	version := aws.String("$Latest")
	if ltSpec.Version != nil && *ltSpec.Version != "" {
		version = ltSpec.Version
	}
	ltOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
		return c.EC2.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: ltSpec.LaunchTemplateId,
			Versions:         []string{*version},
		})
	})
	if err != nil || len(ltOut.LaunchTemplateVersions) == 0 {
		return ""
	}
	ltData := ltOut.LaunchTemplateVersions[0].LaunchTemplateData
	if ltData == nil || ltData.IamInstanceProfile == nil {
		return ""
	}
	if ltData.IamInstanceProfile.Arn != nil && *ltData.IamInstanceProfile.Arn != "" {
		return *ltData.IamInstanceProfile.Arn
	}
	if ltData.IamInstanceProfile.Name != nil && *ltData.IamInstanceProfile.Name != "" {
		return *ltData.IamInstanceProfile.Name
	}
	return ""
}

// asgInstanceProfileToRoles resolves a profile name or ARN to role ARNs via iam:GetInstanceProfile.
func asgInstanceProfileToRoles(ctx context.Context, c *ServiceClients, profileNameOrARN string) []string {
	// Extract name from ARN if needed (arn:aws:iam::<acct>:instance-profile/<name>)
	profileName := profileNameOrARN
	if strings.Contains(profileNameOrARN, ":instance-profile/") {
		parts := strings.SplitN(profileNameOrARN, ":instance-profile/", 2)
		if len(parts) == 2 {
			profileName = parts[1]
		}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetInstanceProfileOutput, error) {
		return c.IAM.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
	})
	if err != nil || out.InstanceProfile == nil {
		return nil
	}
	var roleARNs []string
	for _, r := range out.InstanceProfile.Roles {
		if r.Arn != nil && *r.Arn != "" {
			roleARNs = append(roleARNs, arnRoleName(*r.Arn))
		}
	}
	return roleARNs
}

// asgRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func asgRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	return relatedResourcesFor(ctx, clients, cache, target)
}
