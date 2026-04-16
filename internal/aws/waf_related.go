// waf_related.go registers related-resource definitions for WAF Web ACLs.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("waf", []resource.RelatedDef{
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkWAFELB, NeedsTargetCache: false},
		{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkWAFAPIGW, NeedsTargetCache: false},
	})

	// wafv2types.WebACLSummary: no cross-ref fields — Name, Id, ARN, Description, LockToken only.
	// Associations (ELB/APIGW/CF) are resolved via checkWAF* related checkers at runtime.
}

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




