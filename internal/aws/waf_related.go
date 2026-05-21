// waf_related.go defines the related-resource checkers for WAF Web ACLs.
// The Related slice for "waf" is registered via the catalog struct literal in
// catalog_security.go (AS-814); this file now contains only the per-target
// checker functions referenced from that catalog entry.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkWAFELB calls wafv2:ListResourcesForWebACL with ALB resource type and
// returns matching load balancer names (Pattern A — direct API call).
func checkWAFELB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	out, err := c.WAFv2.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
		WebACLArn:    &webACLArn,
		ResourceType: wafv2types.ResourceTypeApplicationLoadBalancer,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	var ids []string
	for _, arn := range out.ResourceArns {
		// Extract LB name from ARN: arn:aws:elasticloadbalancing:...:loadbalancer/app/NAME/hash
		if parts := strings.Split(arn, "/"); len(parts) >= 3 {
			ids = append(ids, parts[len(parts)-2])
		}
	}
	return relatedResult("elb", ids)
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
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "alarm")
	if err != nil {
		if _, sok := clients.(*ServiceClients); !sok {
			alarmList, truncated, err = nil, false, nil
		}
	}
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
			if d.Name != nil && *d.Name == "WebACL" && d.Value != nil && *d.Value == name {
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

// checkWAFLogs reports log destinations (CloudWatch Logs group or Firehose
// stream) configured for this Web ACL. Pattern C: one wafv2:GetLoggingConfiguration
// call returning LogDestinationConfigs (ARNs of the log destinations).
func checkWAFLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetLoggingConfigurationOutput, error) {
		return c.WAFv2.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: &webACLArn})
	})
	if err != nil {
		// WAFNonexistentItemException = no logging configured → real 0.
		var notFound *wafv2types.WAFNonexistentItemException
		if errors.As(err, &notFound) {
			return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if out.LoggingConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	var ids []string
	for _, d := range out.LoggingConfiguration.LogDestinationConfigs {
		if d == "" {
			continue
		}
		// Extract log-group name from CW Logs ARN:
		//   arn:aws:logs:REGION:ACCT:log-group:NAME:*
		if strings.Contains(d, ":log-group:") {
			parts := strings.Split(d, ":log-group:")
			if len(parts) == 2 {
				name := parts[1]
				if colon := strings.Index(name, ":"); colon >= 0 {
					name = name[:colon]
				}
				if name != "" {
					ids = append(ids, name)
					continue
				}
			}
		}
		// Firehose / S3 destinations: pass through full ARN so the finding
		// carries the destination identity.
		ids = append(ids, d)
	}
	return relatedResult("logs", ids)
}

// checkWAFCF reports CloudFront distributions associated with this Web ACL.
// CloudFront can only bind Web ACLs with Scope=CLOUDFRONT. For REGIONAL
// WAFs the answer is definitively 0. For CLOUDFRONT-scope WAFs: Pattern C
// via cloudfront:ListDistributionsByWebACLId (1 call).
func checkWAFCF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	scope := res.Fields["scope"]
	if scope != string(wafv2types.ScopeCloudfront) {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	webACLID := res.ID
	if webACLID == "" {
		webACLID = res.Fields["id"]
	}
	if webACLID == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudFront == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	api, ok := c.CloudFront.(CloudFrontListDistributionsByWebACLIdAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
		return api.ListDistributionsByWebACLId(ctx, &cloudfront.ListDistributionsByWebACLIdInput{WebACLId: &webACLID})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if out.DistributionList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	var ids []string
	for _, d := range out.DistributionList.Items {
		if d.Id != nil && *d.Id != "" {
			ids = append(ids, *d.Id)
		}
	}
	return relatedResult("cf", ids)
}

// checkWAFAPIGW calls wafv2:ListResourcesForWebACL with API Gateway resource type
// and returns matching API IDs (Pattern A — direct API call).
func checkWAFAPIGW(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	webACLArn := res.Fields["arn"]
	if webACLArn == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
	}
	out, err := c.WAFv2.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
		WebACLArn:    &webACLArn,
		ResourceType: wafv2types.ResourceTypeApiGateway,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1, Err: err}
	}
	var ids []string
	for _, arn := range out.ResourceArns {
		// Extract API ID from stage ARN: arn:aws:apigateway:REGION::/restapis/ID/stages/STAGE
		if strings.Contains(arn, "/restapis/") {
			parts := strings.Split(arn, "/")
			for i, p := range parts {
				if p == "restapis" && i+1 < len(parts) {
					ids = append(ids, parts[i+1])
					break
				}
			}
		}
	}
	return relatedResult("apigw", ids)
}
