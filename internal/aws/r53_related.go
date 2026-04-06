// r53_related.go contains Route 53 hosted zone related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkR53ELB returns Count: 0 because Route 53 alias record targets (ELBs) are
// not available in the hosted zone list API — the relationship cannot be
// determined from cache alone.
func checkR53ELB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
}

// checkR53CF returns Count: 0 because Route 53 alias record targets (CloudFront)
// are not available in the hosted zone list API — the relationship cannot be
// determined from cache alone.
func checkR53CF(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
}

// checkR53ACM returns Count: 0 because ACM validation DNS records are not
// available in the hosted zone list API — the relationship cannot be determined
// from cache alone.
func checkR53ACM(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
}
