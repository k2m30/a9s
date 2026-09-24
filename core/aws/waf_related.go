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
		return NotRead("elb")
	}
	if res.Fields["scope"] == wafScopeCloudFront {
		return foundNone("elb", "scope")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return NotRead("elb")
	}
	arns, err := wafResourcesOfType(ctx, c.WAFv2, webACLArn, wafv2types.ResourceTypeApplicationLoadBalancer)
	if err != nil {
		return ReadFailed("elb", err)
	}
	return relatedRefs("elb", arns, refContext(clients, cache, "elb"))
}

// checkWAFAlarm reports CloudWatch alarms on this Web ACL's metrics. WAF
// publishes them under the Web ACL's name, while the row is keyed by its id.
func checkWAFAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "waf", res)
}

// checkWAFLogs reports the CloudWatch log groups among the log destinations
// configured for this Web ACL (a Firehose stream or S3 bucket destination is
// no log group), in every LogScope: a configuration is owned by the customer,
// by Security Lake or by a CloudWatch telemetry rule, and GetLoggingConfiguration
// answers for one scope at a time
// (https://docs.aws.amazon.com/waf/latest/APIReference/API_GetLoggingConfiguration.html).
func checkWAFLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return keyMissing("logs", "webACLArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return NotRead("logs")
	}
	api := c.wafIn(res.Fields["scope"])
	var groups []string
	var failures []Failure
	scopes := wafv2types.LogScope("").Values()
	for _, scope := range scopes {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetLoggingConfigurationOutput, error) {
			return api.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: &webACLArn, LogScope: scope})
		})
		if _, none := errors.AsType[*wafv2types.WAFNonexistentItemException](err); none {
			continue
		}
		if err != nil {
			failures = append(failures, FailedCall(string(scope), err))
			continue
		}
		if out.LoggingConfiguration == nil {
			continue
		}
		for _, d := range out.LoggingConfiguration.LogDestinationConfigs {
			if _, ok := ARNForService(d, "logs"); ok {
				groups = append(groups, d)
			}
		}
	}
	reads := refReadsIn(clients, cache, "logs", refsByRegion(groups))
	reads[""] = joinReads(reads[""], relatedRead{
		partial: len(failures) > 0,
		unread:  len(failures) == len(scopes),
		failure: AggregateFailures("waf-related: GetLoggingConfiguration", failures, len(scopes)),
	})
	return regionalAnswer(clients, "logs", reads)
}

// checkWAFCF reports CloudFront distributions associated with this Web ACL.
// CloudFront can only bind Web ACLs with Scope=CLOUDFRONT. For REGIONAL
// WAFs the answer is definitively 0. For CLOUDFRONT-scope WAFs: Pattern C
// via cloudfront:ListDistributionsByWebACLId (1 call).
func checkWAFCF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	scope := res.Fields["scope"]
	if scope != wafScopeCloudFront {
		return foundNone("cf", "scope")
	}
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		webACLArn = res.Fields["id"]
	}
	if webACLArn == "" {
		webACLArn = res.ID
	}
	if webACLArn == "" {
		return keyMissing("cf", "webACLArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("cf")
	}
	ids, complete, err := wafDistributionIDs(ctx, c, webACLArn)
	var noList UnusableAnswerErr
	switch {
	case errors.Is(err, errClientMissing), errors.As(err, &noList):
		return NotRead("cf")
	case err != nil:
		return ReadFailed("cf", err)
	}
	return relatedResultTrunc("cf", ids, !complete)
}

// checkWAFAPIGW calls wafv2:ListResourcesForWebACL with API Gateway resource type
// and returns matching API IDs (Pattern A — direct API call). A gateway stage
// takes a REGIONAL-scope ACL; a CLOUDFRONT-scope one attaches to distributions
// alone.
func checkWAFAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return NotRead("apigw")
	}
	if res.Fields["scope"] == wafScopeCloudFront {
		return foundNone("apigw", "scope")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return NotRead("apigw")
	}
	arns, err := wafResourcesOfType(ctx, c.WAFv2, webACLArn, wafv2types.ResourceTypeApiGateway)
	if err != nil {
		return ReadFailed("apigw", err)
	}
	return relatedRefs("apigw", arns, refContext(clients, cache, "apigw"))
}
