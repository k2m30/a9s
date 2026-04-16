// cf_related.go contains CloudFront distribution related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCfS3 searches the S3 cache for buckets whose names are referenced as
// origins in this CloudFront distribution. S3 origin domain name formats:
//   - {bucket}.s3.amazonaws.com
//   - {bucket}.s3.{region}.amazonaws.com
//   - {bucket}.s3-{region}.amazonaws.com
func checkCfS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	if dist.Origins == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	s3List, truncated, err := cfRelatedResources(ctx, clients, cache, "s3")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if s3List == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	// Collect bucket names from S3 origin domain names.
	bucketNames := make(map[string]struct{})
	for _, origin := range dist.Origins.Items {
		if origin.DomainName == nil {
			continue
		}
		domain := *origin.DomainName
		// Extract bucket name: the part before ".s3"
		idx := strings.Index(domain, ".s3")
		if idx > 0 {
			bucketNames[domain[:idx]] = struct{}{}
		}
	}
	if len(bucketNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	var ids []string
	for _, s3Res := range s3List {
		if _, found := bucketNames[s3Res.ID]; found {
			ids = append(ids, s3Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	return relatedResult("s3", ids)
}

// checkCfELB searches the ELB cache for load balancers whose DNS name is
// referenced as an origin in this CloudFront distribution.
// ELB origin domain names follow the pattern: {name}-{id}.{region}.elb.amazonaws.com
func checkCfELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	if dist.Origins == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Collect ELB domain names from origins.
	elbDomains := make(map[string]struct{})
	for _, origin := range dist.Origins.Items {
		if origin.DomainName == nil {
			continue
		}
		if strings.Contains(*origin.DomainName, ".elb.amazonaws.com") {
			elbDomains[*origin.DomainName] = struct{}{}
		}
	}
	if len(elbDomains) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	elbList, truncated, err := cfRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if elbList == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}

	var ids []string
	for _, elbRes := range elbList {
		dnsName := elbRes.Fields["dns_name"]
		if _, found := elbDomains[dnsName]; found {
			ids = append(ids, elbRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	return relatedResult("elb", ids)
}

// checkCfWAF searches the WAF cache for the Web ACL associated with this
// CloudFront distribution. The WebACLId field holds a full ARN.
func checkCfWAF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	if dist.WebACLId == nil || *dist.WebACLId == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	webACLID := *dist.WebACLId

	wafList, truncated, err := cfRelatedResources(ctx, clients, cache, "waf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1, Err: err}
	}
	if wafList == nil {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}

	var ids []string
	for _, wafRes := range wafList {
		if wafRes.Fields["arn"] == webACLID || wafRes.ID == webACLID {
			ids = append(ids, wafRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	return relatedResult("waf", ids)
}

// checkCfACM searches the ACM cache for the certificate associated with this
// CloudFront distribution via ViewerCertificate.ACMCertificateArn.
func checkCfACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}
	if dist.ViewerCertificate == nil || dist.ViewerCertificate.ACMCertificateArn == nil || *dist.ViewerCertificate.ACMCertificateArn == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	certARN := *dist.ViewerCertificate.ACMCertificateArn

	acmList, truncated, err := cfRelatedResources(ctx, clients, cache, "acm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1, Err: err}
	}
	if acmList == nil {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}

	var ids []string
	for _, acmRes := range acmList {
		if acmRes.Fields["certificate_arn"] == certARN || acmRes.ID == certARN {
			ids = append(ids, acmRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
	}
	return relatedResult("acm", ids)
}





// cfRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func cfRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
