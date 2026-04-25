package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEbELB resolves classic load balancer names for this EB environment.
// elasticbeanstalk:DescribeEnvironmentResources.EnvironmentResources.LoadBalancers[].Name.
func checkEbELB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if out.EnvironmentResources == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	var ids []string
	for _, lb := range out.EnvironmentResources.LoadBalancers {
		if lb.Name != nil && *lb.Name != "" {
			ids = append(ids, *lb.Name)
		}
	}
	return relatedResult("elb", ids)
}

// checkEbTG resolves target groups for this EB environment.
// elasticbeanstalk:DescribeEnvironmentResources returns LoadBalancers[].Name (not ARN).
// elbv2:DescribeListeners requires an ARN, so we first resolve name→ARN via
// elbv2:DescribeLoadBalancers(Names=[name]), then call DescribeListeners with the ARN.
func checkEbTG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}

	resOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
		return c.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if resOut.EnvironmentResources == nil || len(resOut.EnvironmentResources.LoadBalancers) == 0 {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	// Collect LB names, resolve each to ARN via DescribeLoadBalancers.
	var tgARNs []string
	var failures []string
	totalLBs := len(resOut.EnvironmentResources.LoadBalancers)
	for _, lb := range resOut.EnvironmentResources.LoadBalancers {
		if lb.Name == nil || *lb.Name == "" {
			continue
		}
		lbName := *lb.Name

		// Resolve name → ARN.
		lbOut, lbErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeLoadBalancersOutput, error) {
			return c.ELBv2.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
				Names: []string{lbName},
			})
		})
		if lbErr != nil {
			failures = append(failures, fmt.Sprintf("%s: DescribeLoadBalancers: %v", lbName, lbErr))
			continue
		}
		if len(lbOut.LoadBalancers) == 0 {
			continue
		}
		lbARNPtr := lbOut.LoadBalancers[0].LoadBalancerArn
		if lbARNPtr == nil || *lbARNPtr == "" {
			continue
		}
		lbARN := *lbARNPtr

		lsnOut, lsnErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeListenersOutput, error) {
			return c.ELBv2.DescribeListeners(ctx, &elbv2.DescribeListenersInput{
				LoadBalancerArn: &lbARN,
			})
		})
		if lsnErr != nil {
			failures = append(failures, fmt.Sprintf("%s: DescribeListeners: %v", lbName, lsnErr))
			continue
		}
		for _, l := range lsnOut.Listeners {
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
	result := relatedResult("tg", tgARNs)
	result.Err = AggregateFailures("eb-related: LB/Listener lookup", failures, totalLBs)
	return result
}

// checkEbSG resolves security groups configured for this EB environment via configuration settings.
// elasticbeanstalk:DescribeConfigurationSettings OptionSettings:
//   - aws:autoscaling:launchconfiguration / SecurityGroups (comma-separated IDs)
//   - aws:elbv2:loadbalancer / SecurityGroups (comma-separated IDs)
func checkEbSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
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
	return relatedResult("sg", ids)
}

// checkEbRole resolves IAM roles for this EB environment via configuration settings.
// elasticbeanstalk:DescribeConfigurationSettings OptionSettings:
//   - aws:autoscaling:launchconfiguration / IamInstanceProfile → iam:GetInstanceProfile → roles
//   - aws:elasticbeanstalk:environment / ServiceRole → direct role ARN or name
func checkEbRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
		return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
			ApplicationName: &appName,
			EnvironmentName: &envName,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
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
			switch {
			case ns == "aws:autoscaling:launchconfiguration" && name == "IamInstanceProfile":
				// Resolve instance profile to role ARNs
				roleARNs := asgInstanceProfileToRoles(ctx, c, val)
				ids = append(ids, roleARNs...)
			case ns == "aws:elasticbeanstalk:environment" && name == "ServiceRole":
				// ServiceRole may be a role ARN or a role name
				ids = append(ids, val)
			}
		}
	}
	return relatedResult("role", ids)
}

// checkEbS3 resolves S3 buckets referenced by application versions for this EB environment.
// elasticbeanstalk:DescribeApplicationVersions(ApplicationName) → ApplicationVersions[].SourceBundle.S3Bucket.
func checkEbS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	appName := ""
	if eb.ApplicationName != nil {
		appName = *eb.ApplicationName
	}
	if appName == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
		return c.ElasticBeanstalk.DescribeApplicationVersions(ctx, &elasticbeanstalk.DescribeApplicationVersionsInput{
			ApplicationName: &appName,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}

	var buckets []string
	for _, av := range out.ApplicationVersions {
		if av.SourceBundle != nil && av.SourceBundle.S3Bucket != nil && *av.SourceBundle.S3Bucket != "" {
			buckets = append(buckets, *av.SourceBundle.S3Bucket)
		}
	}
	return relatedResult("s3", buckets)
}

// ebRelatedResources returns the resource list for target from cache or fetches it.
func ebRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
