// elb_related.go contains ELB related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkELBTargetGroups checks the cache for target groups whose LoadBalancerArns
// contains this ELB's ARN.
func checkELBTargetGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	tgList, truncated, err := elbRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(raw.LoadBalancerArns, elbARN) {
			ids = append(ids, tgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("tg")
	}
	return relatedResult("tg", ids)
}

// checkELBAlarms checks the cache for CloudWatch alarms with a "LoadBalancer"
// dimension matching the ARN suffix of this ELB (everything after "loadbalancer/").
func checkELBAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	// Compute the ARN suffix: everything after "loadbalancer/"
	const prefix = "loadbalancer/"
	arnSuffix := elbARN
	if _, after, found := strings.Cut(elbARN, prefix); found {
		arnSuffix = after
	}

	alarmList, truncated, err := elbRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Name != nil && *d.Name == "LoadBalancer" && d.Value != nil && *d.Value == arnSuffix {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkELBSG extracts security group IDs from the ELBv2 LoadBalancer's
// SecurityGroups slice (ALBs only; NLBs and GLBs return an empty list).
// Pattern F — no cache needed.
func checkELBSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sgID := range raw.SecurityGroups {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkELBVPC returns the VPC this load balancer runs in (Pattern F).
// Reads vpc_id from Fields which is populated by the ELB fetcher.
func checkELBVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkELBCFN reports the CloudFormation stack owning this ELB via the
// aws:cloudformation:stack-name tag. Pattern C: one elbv2:DescribeTags call
// keyed by the LoadBalancer ARN.
func checkELBCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{elbARN}})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	for _, td := range out.TagDescriptions {
		for _, tag := range td.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil && *tag.Value != "" {
				return relatedResult("cfn", []string{*tag.Value})
			}
		}
	}
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkELBR53 reports Route 53 records (alias targets) pointing at this load
// balancer's DNS name. Resource record sets live on route53:ListResourceRecordSets
// (per-zone) and are not cached at the hosted-zone cache level, so identifying
// the records that alias this ELB requires O(N) record-set queries across all
// zones — outside the 1-call budget for related-panel checkers.
// Returns Count: -1 (unknown) to signal the data is not available.
func checkELBR53(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["dns_name"] == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
}

// checkELBACM reports ACM certificates attached to this ELB's HTTPS/TLS
// listeners. Pattern C: one elbv2:DescribeListeners call per ELB; extract
// Certificates[].CertificateArn from each listener.
func checkELBACM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeListenersOutput, error) {
		return c.ELBv2.DescribeListeners(ctx, &elbv2.DescribeListenersInput{LoadBalancerArn: &elbARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1, Err: err}
	}
	var ids []string
	seen := make(map[string]bool)
	for _, ls := range out.Listeners {
		for _, cert := range ls.Certificates {
			if cert.CertificateArn == nil || *cert.CertificateArn == "" {
				continue
			}
			arn := *cert.CertificateArn
			if seen[arn] {
				continue
			}
			seen[arn] = true
			ids = append(ids, arn)
		}
	}
	return relatedResult("acm", ids)
}

// checkELBCF reports CloudFront distributions using this ELB as an origin.
// CloudFront Origins' DomainName may reference this ELB's DNS name.
func checkELBCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dnsName := res.Fields["dns_name"]
	if dnsName == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}

	cfList, truncated, err := elbRelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok || dist.Origins == nil {
			continue
		}
		for _, origin := range dist.Origins.Items {
			if origin.DomainName != nil && *origin.DomainName == dnsName {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cf")
	}
	return relatedResult("cf", ids)
}

// checkELBENI reports ENIs owned by this ELB. ELB-owned ENIs have
// RequesterId "amazon-elb" and Description "ELB app/<name>/<hash>".
// Scans the eni cache for matching descriptions.
func checkELBENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	lbName := res.Fields["name"]
	if lbName == "" {
		lbName = res.Name
	}
	if lbName == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	eniList, truncated, err := elbRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}

	var ids []string
	for _, eniRes := range eniList {
		raw, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if raw.RequesterId == nil || *raw.RequesterId != "amazon-elb" {
			continue
		}
		if raw.Description == nil {
			continue
		}
		desc := *raw.Description
		if !strings.HasPrefix(desc, "ELB ") {
			continue
		}
		parts := strings.Split(desc[4:], "/")
		if len(parts) >= 2 && parts[1] == lbName {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eni")
	}
	return relatedResult("eni", ids)
}

// checkELBS3 reports the S3 bucket receiving ELB access logs.
// Pattern C: one elbv2:DescribeLoadBalancerAttributes call; read the
// "access_logs.s3.bucket" attribute.
func checkELBS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
		return c.ELBv2.DescribeLoadBalancerAttributes(ctx, &elbv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: &elbARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	var ids []string
	for _, a := range out.Attributes {
		if a.Key != nil && *a.Key == "access_logs.s3.bucket" && a.Value != nil && *a.Value != "" {
			ids = append(ids, *a.Value)
		}
	}
	return relatedResult("s3", ids)
}

// checkELBSubnet extracts subnet IDs from the LB's AvailabilityZones slice.
func checkELBSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	var ids []string
	seen := make(map[string]bool)
	for _, az := range raw.AvailabilityZones {
		if az.SubnetId == nil || *az.SubnetId == "" {
			continue
		}
		if seen[*az.SubnetId] {
			continue
		}
		seen[*az.SubnetId] = true
		ids = append(ids, *az.SubnetId)
	}
	return relatedResult("subnet", ids)
}

// checkELBWAF reports the WAF Web ACL attached to this ELB.
// Pattern C: one wafv2:GetWebACLForResource call with the ELB ARN.
func checkELBWAF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	api, ok := c.WAFv2.(WAFv2GetWebACLForResourceAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetWebACLForResourceOutput, error) {
		return api.GetWebACLForResource(ctx, &wafv2.GetWebACLForResourceInput{ResourceArn: &elbARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1, Err: err}
	}
	if out.WebACL == nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	id := ""
	if out.WebACL.Id != nil {
		id = *out.WebACL.Id
	}
	if id == "" && out.WebACL.ARN != nil {
		id = *out.WebACL.ARN
	}
	if id == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	return relatedResult("waf", []string{id})
}

// elbRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func elbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
