// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkASGSubnets parses VPCZoneIdentifier (comma-separated subnet IDs) from the ASG.
func checkASGSubnets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if asg.VPCZoneIdentifier == nil || *asg.VPCZoneIdentifier == "" {
		return resource.ProvenZero("subnet", "asg.VPCZoneIdentifier")
	}
	parts := strings.Split(*asg.VPCZoneIdentifier, ",")
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	if len(ids) == 0 {
		return resource.ProvenZero("subnet", "ids")
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkASGTG checks the cache for target groups referencing this ASG via TargetGroupARNs.
func checkASGTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("tg")
	}
	if len(asg.TargetGroupARNs) == 0 {
		return resource.ProvenZero("tg", "asg.TargetGroupARNs")
	}

	arnSet := map[string]bool{}
	for _, arn := range asg.TargetGroupARNs {
		if arn != "" {
			arnSet[arn] = true
		}
	}

	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return resource.ErrorRelated("tg", err)
	}
	if tgList == nil {
		return resource.UnknownRelated("tg")
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if ok && raw.TargetGroupArn != nil && arnSet[*raw.TargetGroupArn] {
			ids = append(ids, tgRes.ID)
		}
	}
	return relatedResultTrunc("tg", ids, truncated)
}

// checkASGSG resolves security groups associated with this ASG's launch configuration or template.
// LaunchConfig.SecurityGroups[] or LaunchTemplate.SecurityGroupIds[] / NetworkInterfaces[].Groups[].
func checkASGSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("sg")
	}

	var ids []string

	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		lcs, err := launchConfigurations(ctx, c.AutoScaling, *asg.LaunchConfigurationName)
		if err != nil {
			return resource.ErrorRelated("sg", err)
		}
		if len(lcs) > 0 {
			ids = append(ids, lcs[0].SecurityGroups...)
		}
		return relatedResultTrunc("sg", ids, false)
	}

	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		return resource.ProvenZero("sg", "ltSpec.LaunchTemplateId")
	}

	versions, err := launchTemplateVersions(ctx, c.EC2, ltSpec.LaunchTemplateId, ltSpec.Version)
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}
	for _, v := range versions {
		if v.LaunchTemplateData == nil {
			continue
		}
		ids = append(ids, v.LaunchTemplateData.SecurityGroupIds...)
		for _, ni := range v.LaunchTemplateData.NetworkInterfaces {
			ids = append(ids, ni.Groups...)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkASGSNS resolves SNS topics associated with this ASG via notification and lifecycle hook configurations.
// autoscaling:DescribeNotificationConfigurations.TopicARN
// autoscaling:DescribeLifecycleHooks.NotificationTargetARN (if it's an SNS ARN)
func checkASGSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sns")
	}
	asgName := ""
	if asg.AutoScalingGroupName != nil {
		asgName = *asg.AutoScalingGroupName
	}
	if asgName == "" {
		asgName = res.ID
	}
	if asgName == "" {
		return resource.ProvenZero("sns", "asgName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("sns")
	}

	var ids []string

	notifs, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]asgtypes.NotificationConfiguration, *string, error) {
		out, err := c.AutoScaling.DescribeNotificationConfigurations(ctx, &autoscaling.DescribeNotificationConfigurationsInput{
			AutoScalingGroupNames: []string{asgName},
			NextToken:             token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.NotificationConfigurations, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	for _, n := range notifs {
		if n.TopicARN != nil && *n.TopicARN != "" {
			ids = append(ids, *n.TopicARN)
		}
	}

	hookOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLifecycleHooksOutput, error) {
		return c.AutoScaling.DescribeLifecycleHooks(ctx, &autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: aws.String(asgName),
		})
	})
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	for _, h := range hookOut.LifecycleHooks {
		if h.NotificationTargetARN == nil {
			continue
		}
		if _, isTopic := ARNForService(*h.NotificationTargetARN, "sns"); isTopic {
			ids = append(ids, *h.NotificationTargetARN)
		}
	}

	return relatedResultTrunc("sns", ids, !complete)
}

// checkASGVPC resolves VPCs associated with this ASG via VPCZoneIdentifier.
// VPCZoneIdentifier is a comma-separated list of subnet IDs; each subnet belongs to one VPC.
// ec2:DescribeSubnets(SubnetIds=[...]) → deduplicated VpcId values.
func checkASGVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if asg.VPCZoneIdentifier == nil || *asg.VPCZoneIdentifier == "" {
		return resource.ProvenZero("vpc", "asg.VPCZoneIdentifier")
	}
	var subnetIDs []string
	for s := range strings.SplitSeq(*asg.VPCZoneIdentifier, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			subnetIDs = append(subnetIDs, s)
		}
	}
	if len(subnetIDs) == 0 {
		return resource.ProvenZero("vpc", "subnetIDs")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("vpc")
	}

	subnets, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.Subnet, *string, error) {
		out, err := c.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: subnetIDs, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Subnets, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("vpc", err)
	}
	var vpcIDs []string
	for _, sn := range subnets {
		if sn.VpcId != nil && *sn.VpcId != "" {
			vpcIDs = append(vpcIDs, *sn.VpcId)
		}
	}
	return relatedResultTrunc("vpc", vpcIDs, !complete)
}
