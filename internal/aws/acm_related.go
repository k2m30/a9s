// acm_related.go contains ACM certificate related-resource checker functions.
package aws

import (
	"context"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)


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
