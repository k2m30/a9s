// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// waf_related.go defines the related-resource checkers for WAF Web ACLs.
// The Related slice for "waf" is registered via the catalog struct literal in
// catalog_security.go; this file contains only the per-target checker
// functions referenced from that catalog entry.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkWAFELB calls wafv2:ListResourcesForWebACL with ALB resource type and
// returns the load balancers it names (Pattern A — direct API call).
func checkWAFELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.UnknownRelated("elb")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.UnknownRelated("elb")
	}
	out, err := c.WAFv2.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
		WebACLArn:    &webACLArn,
		ResourceType: wafv2types.ResourceTypeApplicationLoadBalancer,
	})
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	return relatedRefs("elb", out.ResourceArns, refContext(clients, cache, "elb"))
}

// checkWAFAlarm reports CloudWatch alarms on this Web ACL's metrics. WAF
// publishes per-WebACL metrics (e.g. CountedRequests) using dimensions
// "WebACL" and "Region" (REGIONAL) — matching on dimension value requires
// the Web ACL name, not ID. Since alarm cache is the source and checker must
// inspect dimensions, we scan it with the name.
func checkWAFAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.Fields["name"]
	if name == "" {
		name = res.Name
	}
	return alarmIDsByDimension(ctx, clients, cache, "", "WebACL", name)
}

// checkWAFLogs reports the CloudWatch log groups among the log destinations
// configured for this Web ACL (a Firehose stream or S3 bucket destination is
// no log group). Pattern C: one wafv2:GetLoggingConfiguration call returning
// LogDestinationConfigs (ARNs of the log destinations).
func checkWAFLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.ProvenZero("logs", "webACLArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.UnknownRelated("logs")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetLoggingConfigurationOutput, error) {
		return c.WAFv2.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: &webACLArn})
	})
	if err != nil {
		// WAFNonexistentItemException = no logging configured → real 0.
		if _, ok := errors.AsType[*wafv2types.WAFNonexistentItemException](err); ok {
			return resource.ProvenZero("logs", "the API answered that none is configured")
		}
		return resource.ErrorRelated("logs", err)
	}
	if out.LoggingConfiguration == nil {
		return resource.ProvenZero("logs", "out.LoggingConfiguration")
	}
	var groups []string
	for _, d := range out.LoggingConfiguration.LogDestinationConfigs {
		if _, ok := ARNForService(d, "logs"); ok {
			groups = append(groups, d)
		}
	}
	return relatedRefs("logs", groups, refContext(clients, cache, "logs"))
}

// checkWAFCF reports CloudFront distributions associated with this Web ACL.
// CloudFront can only bind Web ACLs with Scope=CLOUDFRONT. For REGIONAL
// WAFs the answer is definitively 0. For CLOUDFRONT-scope WAFs: Pattern C
// via cloudfront:ListDistributionsByWebACLId (1 call).
func checkWAFCF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	scope := res.Fields["scope"]
	if scope != string(wafv2types.ScopeCloudfront) {
		return resource.ProvenZero("cf", "scope")
	}
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		webACLArn = res.Fields["id"]
	}
	if webACLArn == "" {
		webACLArn = res.ID
	}
	if webACLArn == "" {
		return resource.ProvenZero("cf", "webACLArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudFront == nil {
		return resource.UnknownRelated("cf")
	}
	api, ok := c.CloudFront.(CloudFrontListDistributionsByWebACLIdAPI)
	if !ok {
		return resource.UnknownRelated("cf")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
		return api.ListDistributionsByWebACLId(ctx, &cloudfront.ListDistributionsByWebACLIdInput{WebACLId: &webACLArn})
	})
	if err != nil {
		return resource.ErrorRelated("cf", err)
	}
	if out.DistributionList == nil {
		return resource.UnknownRelated("cf")
	}
	var ids []string
	for _, d := range out.DistributionList.Items {
		if d.Id != nil && *d.Id != "" {
			ids = append(ids, *d.Id)
		}
	}
	return relatedResultTrunc("cf", ids, false)
}

// checkWAFAPIGW calls wafv2:ListResourcesForWebACL with API Gateway resource type
// and returns matching API IDs (Pattern A — direct API call).
func checkWAFAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.UnknownRelated("apigw")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.UnknownRelated("apigw")
	}
	out, err := c.WAFv2.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
		WebACLArn:    &webACLArn,
		ResourceType: wafv2types.ResourceTypeApiGateway,
	})
	if err != nil {
		return resource.ErrorRelated("apigw", err)
	}
	return relatedRefs("apigw", out.ResourceArns, refContext(clients, cache, "apigw"))
}
