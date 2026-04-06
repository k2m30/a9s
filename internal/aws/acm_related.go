// acm_related.go contains ACM certificate related-resource checker functions.
package aws

import (
	"context"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkACMELB returns Count: 0 because ELBv2 DescribeLoadBalancers does not
// include listener/certificate data — the relationship cannot be determined from cache.
func checkACMELB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
}

// checkACMCF searches the CloudFront cache for distributions whose viewer
// certificate ARN matches this ACM certificate's ARN.
// Pattern C — cache lookup via ViewerCertificate.ACMCertificateArn.
func checkACMCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	certARN := res.ID
	if certARN == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}

	cfList, truncated, err := acmRelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok {
			continue
		}
		if dist.ViewerCertificate == nil || dist.ViewerCertificate.ACMCertificateArn == nil {
			continue
		}
		if *dist.ViewerCertificate.ACMCertificateArn == certARN {
			ids = append(ids, cfRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	return relatedResult("cf", ids)
}

// checkACMAPIGW returns Count: 0 because API Gateway custom domain certificate
// data is not available in the list API — the relationship cannot be determined
// from cache alone.
func checkACMAPIGW(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
}

// checkACMR53 returns Count: 0 because ACM validation DNS records are not
// available in the hosted zone list API — the relationship cannot be determined
// from cache alone.
func checkACMR53(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
}

// acmRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func acmRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
