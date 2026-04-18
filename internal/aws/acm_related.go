// acm_related.go contains ACM certificate related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
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
		return resource.ApproximateZero("cf")
	}
	return relatedResult("cf", ids)
}

// acmCertInUseBy returns the ARNs from acm:DescribeCertificate.InUseBy for
// this ACM certificate. Pattern C: one API call.
func acmCertInUseBy(ctx context.Context, clients any, res resource.Resource) ([]string, error) {
	certARN := ""
	raw, ok := assertStruct[acmtypes.CertificateSummary](res.RawStruct)
	if ok && raw.CertificateArn != nil {
		certARN = *raw.CertificateArn
	}
	if certARN == "" {
		return nil, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ACM == nil {
		return nil, errNoR53Client
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*acm.DescribeCertificateOutput, error) {
		return c.ACM.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: &certARN})
	})
	if err != nil {
		return nil, err
	}
	if out.Certificate == nil {
		return nil, nil
	}
	return out.Certificate.InUseBy, nil
}

// checkACMELB reports load balancers using this certificate via
// acm:DescribeCertificate.InUseBy filtered to elbv2:loadbalancer ARNs.
func checkACMELB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	var ids []string
	for _, arn := range arns {
		if !strings.Contains(arn, ":loadbalancer/") {
			continue
		}
		parts := strings.Split(arn, "/")
		// ALB/NLB ARN shape: ":loadbalancer/app/<name>/<id>"  → parts ends [..., "app", name, id]
		// Classic ELB shape: ":loadbalancer/<name>"           → parts ends [..., "loadbalancer", name]
		switch {
		case len(parts) >= 4 && (parts[len(parts)-3] == "app" || parts[len(parts)-3] == "net" || parts[len(parts)-3] == "gateway"):
			ids = append(ids, parts[len(parts)-2])
		case len(parts) >= 2 && strings.HasSuffix(parts[len(parts)-2], ":loadbalancer"):
			ids = append(ids, parts[len(parts)-1])
		}
	}
	return relatedResult("elb", ids)
}

// checkACMAPIGW reports API Gateway custom domains using this certificate
// via acm:DescribeCertificate.InUseBy filtered to apigateway domain ARNs.
func checkACMAPIGW(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1, Err: err}
	}
	var ids []string
	for _, arn := range arns {
		if strings.Contains(arn, "/domainnames/") {
			if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
				ids = append(ids, arn[idx+1:])
			}
		} else if strings.Contains(arn, "/restapis/") {
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

// checkACMR53 reports Route 53 hosted zones containing DNS validation
// records for this ACM certificate. Pattern C: one acm:DescribeCertificate
// call extracts DomainValidationOptions[].ResourceRecord.Name; then we
// determine which hosted zone hosts each validation record by matching the
// record name against cached zones' names (longest suffix match).
func checkACMR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	certARN := ""
	raw, ok := assertStruct[acmtypes.CertificateSummary](res.RawStruct)
	if ok && raw.CertificateArn != nil {
		certARN = *raw.CertificateArn
	}
	if certARN == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ACM == nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*acm.DescribeCertificateOutput, error) {
		return c.ACM.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: &certARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1, Err: err}
	}
	if out.Certificate == nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	var recordNames []string
	for _, dvo := range out.Certificate.DomainValidationOptions {
		if dvo.ResourceRecord != nil && dvo.ResourceRecord.Name != nil {
			recordNames = append(recordNames, strings.TrimSuffix(strings.ToLower(*dvo.ResourceRecord.Name), "."))
		}
	}
	if len(recordNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	zoneList, _, _ := FetchRelatedTarget(ctx, clients, cache, "r53")
	if zoneList == nil {
		// Without zone cache we can only report a "we saw validation records" signal.
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}
	seen := map[string]bool{}
	var ids []string
	for _, recordName := range recordNames {
		// Longest-suffix match against zone names.
		bestZoneID := ""
		bestZoneLen := 0
		for _, zoneRes := range zoneList {
			zn := strings.TrimSuffix(strings.ToLower(zoneRes.Fields["name"]), ".")
			if zn == "" {
				continue
			}
			if strings.HasSuffix(recordName, zn) && len(zn) > bestZoneLen {
				bestZoneID = zoneRes.ID
				bestZoneLen = len(zn)
			}
		}
		if bestZoneID != "" && !seen[bestZoneID] {
			seen[bestZoneID] = true
			ids = append(ids, bestZoneID)
		}
	}
	return relatedResult("r53", ids)
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
