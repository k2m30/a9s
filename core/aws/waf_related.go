// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// waf_related.go defines the related-resource checkers for WAF Web ACLs.
// The Related slice for "waf" is registered via the catalog struct literal in
// catalog_security.go; this file contains only the per-target checker
// functions referenced from that catalog entry.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkWAFELB calls wafv2:ListResourcesForWebACL with ALB resource type and
// returns the load balancers it names (Pattern A — direct API call). A load
// balancer takes a REGIONAL-scope ACL; a CLOUDFRONT-scope one attaches to
// distributions alone.
func checkWAFELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.UnknownRelated("elb")
	}
	if res.Fields["scope"] == wafScopeCloudFront {
		return resource.ProvenZero("elb", "scope")
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
// publishes them under the Web ACL's name, while the row is keyed by its id.
func checkWAFAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "waf", res)
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
	region := wafRegionOf(res.Fields["scope"])
	api := c.wafIn(res.Fields["scope"])
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetLoggingConfigurationOutput, error) {
		return api.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: &webACLArn})
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
	return inRegion(clients, region, relatedRefs("logs", groups, refContext(c.InRegion(region), cache, "logs")))
}

// checkWAFCF reports CloudFront distributions associated with this Web ACL.
// CloudFront can only bind Web ACLs with Scope=CLOUDFRONT. For REGIONAL
// WAFs the answer is definitively 0. For CLOUDFRONT-scope WAFs: Pattern C
// via cloudfront:ListDistributionsByWebACLId (1 call).
func checkWAFCF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	scope := res.Fields["scope"]
	if scope != wafScopeCloudFront {
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
	if !ok || c == nil {
		return resource.UnknownRelated("cf")
	}
	ids, err := wafDistributionIDs(ctx, c, webACLArn)
	var noList UnusableAnswerErr
	switch {
	case errors.Is(err, errClientMissing), errors.As(err, &noList):
		return resource.UnknownRelated("cf")
	case err != nil:
		return resource.ErrorRelated("cf", err)
	}
	return relatedResultTrunc("cf", ids, false)
}

// checkWAFAPIGW calls wafv2:ListResourcesForWebACL with API Gateway resource type
// and returns matching API IDs (Pattern A — direct API call). A gateway stage
// takes a REGIONAL-scope ACL; a CLOUDFRONT-scope one attaches to distributions
// alone.
func checkWAFAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.UnknownRelated("apigw")
	}
	if res.Fields["scope"] == wafScopeCloudFront {
		return resource.ProvenZero("apigw", "scope")
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
