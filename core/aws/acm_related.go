// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// acm_related.go contains ACM certificate related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkACMCF searches the CloudFront cache for distributions whose viewer
// certificate ARN matches this ACM certificate's ARN.
// Cache lookup via ViewerCertificate.ACMCertificateArn.
func checkACMCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// CloudFront's ACMCertificateArn is the full ARN, so matching uses the
	// ARN, not the bare domain.
	certARN := res.Fields["certificate_arn"]
	if certARN == "" {
		// Fall back to RawStruct for callers that pass sparse resources.
		if raw, ok := assertStruct[acmtypes.CertificateSummary](res.RawStruct); ok && raw.CertificateArn != nil {
			certARN = *raw.CertificateArn
		}
	}
	if certARN == "" {
		return keyMissing("cf", "certARN")
	}

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return ReadFailed("cf", err)
	}
	if cfList == nil {
		return NotRead("cf")
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
	return relatedResultTrunc("cf", ids, truncated)
}

// acmCertInUseBy returns the ARNs from acm:DescribeCertificate.InUseBy for
// this ACM certificate.
func acmCertInUseBy(ctx context.Context, clients any, res resource.Resource) ([]string, error) {
	certARN := ""
	raw, ok := assertStruct[acmtypes.CertificateSummary](res.RawStruct)
	if ok && raw.CertificateArn != nil {
		certARN = *raw.CertificateArn
	}
	if certARN == "" {
		return []string{}, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ACM == nil {
		return nil, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*acm.DescribeCertificateOutput, error) {
		return c.ACM.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: &certARN})
	})
	if err != nil {
		return nil, err
	}
	if out.Certificate == nil {
		return []string{}, nil
	}
	return out.Certificate.InUseBy, nil
}

// checkACMELB reports load balancers using this certificate via
// acm:DescribeCertificate.InUseBy, kept to the ELBv2 load balancers the elb
// list holds: an ELBv2 ARN names its type, name and id,
// "loadbalancer/app/my-load-balancer/50dc6c495c0c9188"
// (https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html),
// where a Classic Load Balancer's is "loadbalancer/<name>".
func checkACMELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return NotRead("elb")
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		return ReadFailed("elb", err)
	}
	var refs []string
	for _, arn := range arns {
		if a, ok := ARNForService(arn, "elasticloadbalancing"); ok && strings.HasPrefix(a.Resource, "loadbalancer/") && strings.Count(a.Resource, "/") == 3 {
			refs = append(refs, arn)
		}
	}
	return unreadZero(res, relatedRefs("elb", refs, refContext(clients, cache, "elb")))
}

// checkACMAPIGW reports the APIs served with this certificate. InUseBy names
// an API Gateway custom domain ("/domainnames/<name>"), and the APIs are the
// ones that domain's API mappings name.
func checkACMAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return NotRead("apigw")
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		return ReadFailed("apigw", err)
	}
	var refs []string
	var mapped relatedRead
	for _, arn := range arns {
		if domain, ok := apigwDomainRefToID(arn); ok {
			mapped = joinReads(mapped, apigwDomainAPIs(ctx, clients, domain, ""))
		} else if _, ok := ARNForService(arn, "apigateway"); ok {
			refs = append(refs, arn)
		}
	}
	ids, dropped := resolveRefs("apigw", append(refs, mapped.ids...), refContext(clients, cache, "apigw"))
	mapped.ids, mapped.partial = ids, mapped.partial || dropped
	return unreadZero(res, relatedAnswer("apigw", mapped))
}

// checkACMR53 reports Route 53 hosted zones containing DNS validation
// records for this ACM certificate. One acm:DescribeCertificate
// call extracts DomainValidationOptions[].ResourceRecord.Name; then we
// determine which hosted zone hosts each validation record by matching the
// record name against cached zones' names (longest suffix match).
func checkACMR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return NotRead("r53")
	}
	certARN := ""
	raw, ok := assertStruct[acmtypes.CertificateSummary](res.RawStruct)
	if ok && raw.CertificateArn != nil {
		certARN = *raw.CertificateArn
	}
	if certARN == "" {
		return unreadZero(res, foundNone("r53", "certARN"))
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ACM == nil {
		return NotRead("r53")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*acm.DescribeCertificateOutput, error) {
		return c.ACM.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: &certARN})
	})
	if err != nil {
		return ReadFailed("r53", err)
	}
	if out.Certificate == nil {
		return unreadZero(res, foundNone("r53", "out.Certificate"))
	}
	var recordNames []string
	for _, dvo := range out.Certificate.DomainValidationOptions {
		if dvo.ResourceRecord != nil && dvo.ResourceRecord.Name != nil {
			recordNames = append(recordNames, canonicalDNS(*dvo.ResourceRecord.Name))
		}
	}
	if len(recordNames) == 0 {
		return unreadZero(res, foundNone("r53", "recordNames"))
	}
	zoneList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "r53")
	if err != nil {
		return ReadFailed("r53", err)
	}
	if zoneList == nil {
		// Without zone cache we can only report a "we saw validation records" signal.
		return NotRead("r53")
	}
	seen := map[string]bool{}
	var ids []string
	for _, recordName := range recordNames {
		for _, id := range publicZoneHolding(zoneList, recordName) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return unreadZero(res, relatedResultTrunc("r53", ids, truncated))
}
