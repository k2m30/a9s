package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkASGSubnets parses VPCZoneIdentifier (comma-separated subnet IDs) from the ASG.
// Pattern F — no cache needed.
func checkASGSubnets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if asg.VPCZoneIdentifier == nil || *asg.VPCZoneIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}

// checkASGTG checks the cache for target groups referencing this ASG via TargetGroupARNs.
// Pattern C: ASG RawStruct has TargetGroupARNs; match against tg cache by ARN from RawStruct.
func checkASGTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	if len(asg.TargetGroupARNs) == 0 {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	arnSet := map[string]bool{}
	for _, arn := range asg.TargetGroupARNs {
		if arn != "" {
			arnSet[arn] = true
		}
	}

	tgList, truncated, err := asgRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if ok && raw.TargetGroupArn != nil && arnSet[*raw.TargetGroupArn] {
			ids = append(ids, tgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("tg")
	}
	return relatedResult("tg", ids)
}

// checkASGSG resolves security groups associated with this ASG's launch configuration or template.
// LaunchConfig.SecurityGroups[] or LaunchTemplate.SecurityGroupIds[] / NetworkInterfaces[].Groups[].
func checkASGSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	var ids []string

	// Launch configuration path
	if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			return c.AutoScaling.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
				LaunchConfigurationNames: []string{*asg.LaunchConfigurationName},
			})
		})
		if err != nil {
			return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
		}
		if len(out.LaunchConfigurations) > 0 {
			ids = append(ids, out.LaunchConfigurations[0].SecurityGroups...)
		}
		return relatedResult("sg", ids)
	}

	// Launch template path
	ltSpec := asg.LaunchTemplate
	if ltSpec == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltSpec = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if ltSpec == nil || ltSpec.LaunchTemplateId == nil || *ltSpec.LaunchTemplateId == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	for _, v := range ltOut.LaunchTemplateVersions {
		if v.LaunchTemplateData == nil {
			continue
		}
		ids = append(ids, v.LaunchTemplateData.SecurityGroupIds...)
		for _, ni := range v.LaunchTemplateData.NetworkInterfaces {
			ids = append(ids, ni.Groups...)
		}
	}
	return relatedResult("sg", ids)
}

// checkASGSNS resolves SNS topics associated with this ASG via notification and lifecycle hook configurations.
// autoscaling:DescribeNotificationConfigurations.TopicARN
// autoscaling:DescribeLifecycleHooks.NotificationTargetARN (if it's an SNS ARN)
func checkASGSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	asgName := ""
	if asg.AutoScalingGroupName != nil {
		asgName = *asg.AutoScalingGroupName
	}
	if asgName == "" {
		asgName = res.ID
	}
	if asgName == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	var ids []string

	// Notification configurations
	notifOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
		return c.AutoScaling.DescribeNotificationConfigurations(ctx, &autoscaling.DescribeNotificationConfigurationsInput{
			AutoScalingGroupNames: []string{asgName},
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	for _, n := range notifOut.NotificationConfigurations {
		if n.TopicARN != nil && *n.TopicARN != "" {
			ids = append(ids, *n.TopicARN)
		}
	}

	// Lifecycle hooks
	hookOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLifecycleHooksOutput, error) {
		return c.AutoScaling.DescribeLifecycleHooks(ctx, &autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: aws.String(asgName),
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	for _, h := range hookOut.LifecycleHooks {
		if h.NotificationTargetARN != nil && strings.HasPrefix(*h.NotificationTargetARN, "arn:aws:sns:") {
			ids = append(ids, *h.NotificationTargetARN)
		}
	}

	return relatedResult("sns", ids)
}

// checkASGVPC resolves VPCs associated with this ASG via VPCZoneIdentifier.
// VPCZoneIdentifier is a comma-separated list of subnet IDs; each subnet belongs to one VPC.
// ec2:DescribeSubnets(SubnetIds=[...]) → deduplicated VpcId values.
func checkASGVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if asg.VPCZoneIdentifier == nil || *asg.VPCZoneIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	var subnetIDs []string
	for s := range strings.SplitSeq(*asg.VPCZoneIdentifier, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			subnetIDs = append(subnetIDs, s)
		}
	}
	if len(subnetIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeSubnetsOutput, error) {
		return c.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: subnetIDs})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	var vpcIDs []string
	for _, sn := range out.Subnets {
		if sn.VpcId != nil && *sn.VpcId != "" {
			vpcIDs = append(vpcIDs, *sn.VpcId)
		}
	}
	return relatedResult("vpc", vpcIDs)
}
