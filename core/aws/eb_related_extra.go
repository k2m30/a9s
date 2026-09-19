// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEbELB resolves classic load balancer names for this EB environment.
// elasticbeanstalk:DescribeEnvironmentResources.EnvironmentResources.LoadBalancers[].Name.
func checkEbELB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("elb")
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.ProvenZero("elb", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("elb")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if out.EnvironmentResources == nil {
		return resource.ProvenZero("elb", "out.EnvironmentResources")
	}
	var ids []string
	for _, lb := range out.EnvironmentResources.LoadBalancers {
		if lb.Name != nil && *lb.Name != "" {
			ids = append(ids, *lb.Name)
		}
	}
	return relatedResultTrunc("elb", ids, false)
}

// checkEbTG resolves target groups for this EB environment.
// elasticbeanstalk:DescribeEnvironmentResources returns LoadBalancers[].Name (not ARN).
// elbv2:DescribeListeners requires an ARN, so we first resolve name→ARN via
// elbv2:DescribeLoadBalancers(Names=[name]), then call DescribeListeners with the ARN.
func checkEbTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("tg")
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.ProvenZero("tg", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("tg")
	}

	resOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.ErrorRelated("tg", err)
	}
	if resOut.EnvironmentResources == nil || len(resOut.EnvironmentResources.LoadBalancers) == 0 {
		return resource.ProvenZero("tg", "resOut.EnvironmentResources.LoadBalancers")
	}

	var tgARNs []string
	var failures []Failure
	complete := true
	for _, lb := range resOut.EnvironmentResources.LoadBalancers {
		if lb.Name == nil || *lb.Name == "" {
			continue
		}
		lbName := *lb.Name

		lbs, _, lbErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.LoadBalancer, *string, error) {
			out, err := c.ELBv2.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
				Names:  []string{lbName},
				Marker: marker,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.LoadBalancers, out.NextMarker, nil
		})
		if lbErr != nil {
			failures = append(failures, FailedCall(lbName, lbErr))
			continue
		}
		if len(lbs) == 0 {
			continue
		}
		lbARNPtr := lbs[0].LoadBalancerArn
		if lbARNPtr == nil || *lbARNPtr == "" {
			continue
		}
		lbARN := *lbARNPtr

		listeners, listenersComplete, lsnErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.Listener, *string, error) {
			out, err := c.ELBv2.DescribeListeners(ctx, &elbv2.DescribeListenersInput{
				LoadBalancerArn: &lbARN,
				Marker:          marker,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.Listeners, out.NextMarker, nil
		})
		if lsnErr != nil {
			failures = append(failures, FailedCall(lbName, lsnErr))
			continue
		}
		complete = complete && listenersComplete
		for _, l := range listeners {
			for _, act := range l.DefaultActions {
				if act.TargetGroupArn != nil && *act.TargetGroupArn != "" {
					tgARNs = append(tgARNs, *act.TargetGroupArn)
				}
				if act.ForwardConfig != nil {
					for _, tgTuple := range act.ForwardConfig.TargetGroups {
						if tgTuple.TargetGroupArn != nil && *tgTuple.TargetGroupArn != "" {
							tgARNs = append(tgARNs, *tgTuple.TargetGroupArn)
						}
					}
				}
			}
		}
	}
	if len(tgARNs) == 0 {
		// Nothing was confirmed: any failures are a plain fetch failure, not
		// a truncation signal (there is no larger population left unseen).
		if aggErr := AggregateFailures("eb-related: LB/Listener lookup", failures, len(resOut.EnvironmentResources.LoadBalancers)); aggErr != nil {
			return resource.ErrorRelated("tg", aggErr)
		}
	}
	// Some DescribeListeners calls may have failed: tgARNs is a proven subset,
	// not necessarily exhaustive. Truncated (not Errored) keeps the row
	// actionable rather than discarding confirmed matches as a dead end.
	ids, dropped := resolveRefs("tg", tgARNs, refContext(clients, cache, "tg"))
	return relatedResultTrunc("tg", ids, dropped || len(failures) > 0 || !complete)
}

// checkEbSG resolves security groups configured for this EB environment via configuration settings.
// elasticbeanstalk:DescribeConfigurationSettings OptionSettings:
//   - aws:autoscaling:launchconfiguration / SecurityGroups (comma-separated IDs)
//   - aws:elbv2:loadbalancer / SecurityGroups (comma-separated IDs)
func checkEbSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
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
		return resource.ProvenZero("sg", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("sg")
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.ErrorRelated("sg", err)
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
		return resource.UnknownRelated("role")
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
		return resource.ProvenZero("role", "envName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("role")
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.ErrorRelated("role", err)
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
		return resource.UnknownRelated("role")
	}
	return relatedResultTrunc("role", ids, dropped)
}

// checkEbS3 resolves S3 buckets referenced by application versions for this EB environment.
// elasticbeanstalk:DescribeApplicationVersions(ApplicationName) → ApplicationVersions[].SourceBundle.S3Bucket.
func checkEbS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}

	appName := ""
	if eb.ApplicationName != nil {
		appName = *eb.ApplicationName
	}
	if appName == "" {
		return resource.ProvenZero("s3", "appName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("s3")
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
		return resource.ErrorRelated("s3", err)
	}

	var buckets []string
	for _, av := range versions {
		if av.SourceBundle != nil && av.SourceBundle.S3Bucket != nil && *av.SourceBundle.S3Bucket != "" {
			buckets = append(buckets, *av.SourceBundle.S3Bucket)
		}
	}
	return relatedResultTrunc("s3", buckets, !complete)
}
