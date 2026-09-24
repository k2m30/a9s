// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEbELB resolves the load balancers of this EB environment,
// elasticbeanstalk:DescribeEnvironmentResources.EnvironmentResources.LoadBalancers[].Name,
// against the elb list. That list holds ELBv2 load balancers only, so an
// environment on a Classic Load Balancer counts none of its rows.
func checkEbELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return NotRead("elb")
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return foundNone("elb", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("elb")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return ReadFailed("elb", err)
	}
	if out.EnvironmentResources == nil {
		return foundNone("elb", "out.EnvironmentResources")
	}
	var refs []string
	for _, lb := range out.EnvironmentResources.LoadBalancers {
		refs = append(refs, aws.ToString(lb.Name))
	}
	return listedRelated(ctx, clients, cache, "elb", refs, false)
}

// checkEbTG resolves target groups for this EB environment.
// elasticbeanstalk:DescribeEnvironmentResources returns LoadBalancers[].Name
// (not ARN), resolved to an ARN with elbv2:DescribeLoadBalancers(Names=[name]).
// A target group names every load balancer that forwards to it, through a
// listener's default action or its rules, in TargetGroup.LoadBalancerArns.
func checkEbTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return NotRead("tg")
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return foundNone("tg", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("tg")
	}

	resOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return ReadFailed("tg", err)
	}
	if resOut.EnvironmentResources == nil || len(resOut.EnvironmentResources.LoadBalancers) == 0 {
		return foundNone("tg", "resOut.EnvironmentResources.LoadBalancers")
	}

	var lbARNs []string
	var reads rowReads
	for _, lb := range resOut.EnvironmentResources.LoadBalancers {
		if lb.Name == nil || *lb.Name == "" {
			reads.missed()
			continue
		}
		lbName, isV2 := resource.ResolveRef("elb", *lb.Name, refContext(clients, cache, "elb"))
		if !isV2 {
			continue
		}

		lbs, _, lbErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.LoadBalancer, *string, error) {
			out, callErr := c.ELBv2.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
				Names:  []string{lbName},
				Marker: marker,
			})
			if callErr != nil {
				return nil, nil, callErr
			}
			return out.LoadBalancers, out.NextMarker, nil
		})
		// A Classic Load Balancer is no ELBv2 load balancer: the call answers
		// LoadBalancerNotFound, and a Classic one has no target groups.
		if ErrCodeIs(lbErr, "LoadBalancerNotFound") {
			reads.read++
			continue
		}
		if lbErr != nil {
			reads.fail(lbName, lbErr)
			continue
		}
		reads.read++
		for _, found := range lbs {
			if aws.ToString(found.LoadBalancerName) == lbName {
				lbARNs = append(lbARNs, aws.ToString(found.LoadBalancerArn))
			}
		}
	}
	if len(lbARNs) == 0 {
		return reads.answer("tg", "eb-related: DescribeLoadBalancers", nil, false)
	}

	tgList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "tg")
	if err != nil {
		return ReadFailed("tg", err)
	}
	if tgList == nil {
		return NotRead("tg")
	}
	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if ok && slices.ContainsFunc(raw.LoadBalancerArns, func(arn string) bool { return slices.Contains(lbARNs, arn) }) {
			ids = append(ids, tgRes.ID)
		}
	}
	return reads.answer("tg", "eb-related: DescribeLoadBalancers", ids, truncated)
}

// checkEbSG resolves security groups configured for this EB environment via configuration settings.
// elasticbeanstalk:DescribeConfigurationSettings OptionSettings:
//   - aws:autoscaling:launchconfiguration / SecurityGroups (comma-separated IDs)
//   - aws:elbv2:loadbalancer / SecurityGroups (comma-separated IDs)
func checkEbSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}

	appName := ""
	if eb.ApplicationName != nil {
		appName = *eb.ApplicationName
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if appName == "" || envName == "" {
		return foundNone("sg", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("sg")
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return ReadFailed("sg", err)
	}

	var ids []string
	for _, cfg := range cfgOut.ConfigurationSettings {
		for _, opt := range cfg.OptionSettings {
			ns := ""
			if opt.Namespace != nil {
				ns = *opt.Namespace
			}
			name := ""
			if opt.OptionName != nil {
				name = *opt.OptionName
			}
			val := ""
			if opt.Value != nil {
				val = *opt.Value
			}
			if val == "" {
				continue
			}
			isSGField := (ns == "aws:autoscaling:launchconfiguration" && name == "SecurityGroups") ||
				(ns == "aws:elbv2:loadbalancer" && name == "SecurityGroups")
			if isSGField {
				for sg := range strings.SplitSeq(val, ",") {
					sg = strings.TrimSpace(sg)
					if sg != "" {
						ids = append(ids, sg)
					}
				}
			}
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkEbRole resolves IAM roles for this EB environment via configuration settings.
// elasticbeanstalk:DescribeConfigurationSettings OptionSettings:
//   - aws:autoscaling:launchconfiguration / IamInstanceProfile → iam:GetInstanceProfile → roles
//   - aws:elasticbeanstalk:environment / ServiceRole → direct role ARN or name
func checkEbRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return NotRead("role")
	}

	appName := ""
	if eb.ApplicationName != nil {
		appName = *eb.ApplicationName
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if appName == "" || envName == "" {
		return foundNone("role", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("role")
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return ReadFailed("role", err)
	}

	var refs []string
	// resolved stays true until a profile lookup does not answer; the same
	// rule the ASG role pivot follows, because it is the same call.
	resolved := true
	for _, cfg := range cfgOut.ConfigurationSettings {
		for _, opt := range cfg.OptionSettings {
			ns := ""
			if opt.Namespace != nil {
				ns = *opt.Namespace
			}
			name := ""
			if opt.OptionName != nil {
				name = *opt.OptionName
			}
			val := ""
			if opt.Value != nil {
				val = *opt.Value
			}
			if val == "" {
				continue
			}
			switch {
			case ns == "aws:autoscaling:launchconfiguration" && name == "IamInstanceProfile":
				roleARNs, answered := asgInstanceProfileToRoles(ctx, c, val)
				refs = append(refs, roleARNs...)
				resolved = resolved && answered
			case ns == "aws:elasticbeanstalk:environment" && name == "ServiceRole":
				refs = append(refs, val)
			}
		}
	}
	ids, dropped := resolveRefs("role", refs, refContext(clients, cache, "role"))
	if !resolved {
		if len(ids) > 0 {
			return relatedResultTrunc("role", ids, true)
		}
		return NotRead("role")
	}
	return relatedResultTrunc("role", ids, dropped)
}

// checkEbS3 resolves S3 buckets referenced by application versions for this EB environment.
// elasticbeanstalk:DescribeApplicationVersions(ApplicationName) → ApplicationVersions[].SourceBundle.S3Bucket.
func checkEbS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return NotRead("s3")
	}

	appName := ""
	if eb.ApplicationName != nil {
		appName = *eb.ApplicationName
	}
	if appName == "" {
		return foundNone("s3", "appName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("s3")
	}

	versions, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ebtypes.ApplicationVersionDescription, *string, error) {
		out, err := c.ElasticBeanstalk.DescribeApplicationVersions(ctx, &elasticbeanstalk.DescribeApplicationVersionsInput{
			ApplicationName: &appName,
			NextToken:       token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.ApplicationVersions, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("s3", err)
	}

	var buckets []string
	for _, av := range versions {
		if av.SourceBundle != nil && av.SourceBundle.S3Bucket != nil && *av.SourceBundle.S3Bucket != "" {
			buckets = append(buckets, *av.SourceBundle.S3Bucket)
		}
	}
	return relatedResultTrunc("s3", buckets, !complete)
}
