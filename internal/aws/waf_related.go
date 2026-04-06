// waf_related.go registers related-resource definitions for WAF Web ACLs.
// All checkers are nil stubs — WAF associations require wafv2:ListResourcesForWebACL
// which is not fetched by the current cache layer.
package aws

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
	resource.RegisterRelated("waf", []resource.RelatedDef{
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: nil, NeedsTargetCache: true},
		{TargetType: "apigw", DisplayName: "API Gateways", Checker: nil, NeedsTargetCache: true},
		{TargetType: "cf", DisplayName: "CloudFront", Checker: nil, NeedsTargetCache: true},
	})
}
