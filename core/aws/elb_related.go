// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_related.go contains ELB related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	"github.com/k2m30/a9s/v3/core/resource"
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
		return resource.KnownRelated("tg", nil, false)
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
		if !ok {
			continue
		}
		if slices.Contains(raw.LoadBalancerArns, elbARN) {
			ids = append(ids, tgRes.ID)
		}
	}
	return relatedResultTrunc("tg", ids, truncated)
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

	// Compute the ARN suffix: everything after "loadbalancer/"
	const prefix = "loadbalancer/"
	arnSuffix := elbARN
	if _, after, found := strings.Cut(elbARN, prefix); found {
		arnSuffix = after
	}

	return alarmIDsByDimension(ctx, clients, cache, "", "LoadBalancer", arnSuffix)
}

// checkELBSG extracts security group IDs from the ELBv2 LoadBalancer's
// SecurityGroups slice (ALBs only; NLBs and GLBs return an empty list).
// Pattern F — no cache needed.
func checkELBSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
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
		return resource.KnownRelated("vpc", nil, false)
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
		return resource.KnownRelated("cfn", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("cfn")
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{elbARN}})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	for _, td := range out.TagDescriptions {
		for _, tag := range td.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil && *tag.Value != "" {
				return relatedResult("cfn", []string{*tag.Value})
			}
		}
	}
	return resource.KnownRelated("cfn", nil, false)
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
		return resource.KnownRelated("acm", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("acm")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeListenersOutput, error) {
		return c.ELBv2.DescribeListeners(ctx, &elbv2.DescribeListenersInput{LoadBalancerArn: &elbARN})
	})
	if err != nil {
		return resource.ErrorRelated("acm", err)
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
		return resource.KnownRelated("cf", nil, false)
	}

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return resource.ErrorRelated("cf", err)
	}
	if cfList == nil {
		return resource.UnknownRelated("cf")
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
	return relatedResultTrunc("cf", ids, truncated)
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
		return resource.KnownRelated("eni", nil, false)
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("eni")
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
	return relatedResultTrunc("eni", ids, truncated)
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
		return resource.KnownRelated("s3", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("s3")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
		return c.ELBv2.DescribeLoadBalancerAttributes(ctx, &elbv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: &elbARN})
	})
	if err != nil {
		return resource.ErrorRelated("s3", err)
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
		return resource.UnknownRelated("subnet")
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
//
// AWS WAFv2 only supports CloudFront, ALB, API Gateway, AppSync, Cognito,
// Verified Access, and App Runner — NLBs and GWLBs are unsupported and
// calling GetWebACLForResource with a non-ALB ELB returns
// WAFInvalidParameterException. Short-circuit when the ELB type is a known
// unsupported value (network or gateway) to keep the related-panel result a
// definitive zero instead of a fetch failure. Fall through to the API call
// for "application" and unknown types so ALBs with missing Fields["type"]
// (e.g. cache rehydration paths) still get checked via RawStruct fallback.
func checkELBWAF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.KnownRelated("waf", nil, false)
	}
	lbType := res.Fields["type"]
	if lbType == "" {
		if raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct); ok {
			lbType = string(raw.Type)
		}
	}
	if lbType == "network" || lbType == "gateway" {
		return resource.KnownRelated("waf", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.UnknownRelated("waf")
	}
	api, ok := c.WAFv2.(WAFv2GetWebACLForResourceAPI)
	if !ok {
		return resource.UnknownRelated("waf")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetWebACLForResourceOutput, error) {
		return api.GetWebACLForResource(ctx, &wafv2.GetWebACLForResourceInput{ResourceArn: &elbARN})
	})
	if err != nil {
		return resource.ErrorRelated("waf", err)
	}
	if out.WebACL == nil {
		return resource.KnownRelated("waf", nil, false)
	}
	id := ""
	if out.WebACL.Id != nil {
		id = *out.WebACL.Id
	}
	if id == "" && out.WebACL.ARN != nil {
		id = *out.WebACL.ARN
	}
	if id == "" {
		return resource.KnownRelated("waf", nil, false)
	}
	return relatedResult("waf", []string{id})
}
