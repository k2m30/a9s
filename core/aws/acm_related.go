// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// acm_related.go contains ACM certificate related-resource checker functions.
package aws

import (
	"context"
	"errors"
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
		return foundNone("cf", "certARN")
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
// acm:DescribeCertificate.InUseBy filtered to elbv2:loadbalancer ARNs.
func checkACMELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return NotRead("elb")
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("elb")
		}
		return ReadFailed("elb", err)
	}
	var refs []string
	for _, arn := range arns {
		if a, ok := ARNForService(arn, "elasticloadbalancing"); ok && strings.HasPrefix(a.Resource, "loadbalancer/") {
			refs = append(refs, arn)
		}
	}
	return unreadZero(res, relatedRefs("elb", refs, refContext(clients, cache, "elb")))
}

// checkACMAPIGW reports API Gateway custom domains using this certificate
// via acm:DescribeCertificate.InUseBy filtered to apigateway domain ARNs.
func checkACMAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" && res.Name == "" {
		return NotRead("apigw")
	}
	arns, err := acmCertInUseBy(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("apigw")
		}
		return ReadFailed("apigw", err)
	}
	var refs []string
	for _, arn := range arns {
		if _, ok := ARNForService(arn, "apigateway"); ok {
			refs = append(refs, arn)
		}
	}
	return unreadZero(res, relatedRefs("apigw", refs, refContext(clients, cache, "apigw")))
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
		// The zone that holds the record is the innermost one containing it:
		// a name may sit in a zone of its own and in every parent zone, and
		// the deepest is where a record of that name is written. ACM
		// validates against public DNS, so a private zone never holds it.
		bestZoneID := ""
		bestZoneLen := 0
		for _, zoneRes := range zoneList {
			zn := canonicalDNS(zoneRes.Fields["name"])
			if zoneRes.Fields["private_zone"] == "true" || !dnsZoneHosts(zn, recordName) {
				continue
			}
			if len(zn) > bestZoneLen {
				bestZoneID = zoneRes.ID
				bestZoneLen = len(zn)
			}
		}
		if bestZoneID != "" && !seen[bestZoneID] {
			seen[bestZoneID] = true
			ids = append(ids, bestZoneID)
		}
	}
	return unreadZero(res, relatedResultTrunc("r53", ids, truncated))
}
