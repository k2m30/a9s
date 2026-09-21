// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cf_related.go contains CloudFront distribution related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCfS3 reports the S3 buckets this distribution reads from and logs to:
// the buckets its origins address, and its standard-logging bucket
// (DistributionConfig.Logging.Bucket, one cloudfront:GetDistributionConfig
// call). Both are bucket endpoint hosts, read by S3OriginBucket. A config
// that could not be read leaves the count a lower bound.
func checkCfS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	var hosts []string
	if dist.Origins != nil {
		for _, origin := range dist.Origins.Items {
			hosts = append(hosts, aws.ToString(origin.DomainName))
		}
	}
	cfg, cfgErr := cfDistributionConfig(ctx, clients, res.ID)
	if cfg != nil && cfg.Logging != nil && aws.ToBool(cfg.Logging.Enabled) {
		hosts = append(hosts, aws.ToString(cfg.Logging.Bucket))
	}
	var buckets []string
	for _, host := range hosts {
		if bucket, ok := S3OriginBucket(host); ok {
			buckets = append(buckets, bucket)
		}
	}
	if len(buckets) == 0 {
		return relatedResultTrunc("s3", nil, cfgErr != nil)
	}

	s3List, _, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return resource.ErrorRelated("s3", err)
	}
	if s3List == nil {
		return resource.UnknownRelated("s3")
	}
	ids, lowerBound := listedRefs("s3", buckets, refContext(clients, cache, "s3"), s3List)
	return relatedResultTrunc("s3", ids, lowerBound || cfgErr != nil)
}

// cfDistributionConfig reads the distribution's full config, which the list
// summary leaves out (Lambda@Edge associations, standard logging).
func cfDistributionConfig(ctx context.Context, clients any, distID string) (*cftypes.DistributionConfig, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudFront == nil {
		return nil, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudfront.GetDistributionConfigOutput, error) {
		return c.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: &distID})
	})
	if err != nil {
		return nil, err
	}
	return out.DistributionConfig, nil
}

// checkCfELB searches the ELB cache for load balancers whose DNS name is
// referenced as an origin in this CloudFront distribution.
func checkCfELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("elb")
	}
	if dist.Origins == nil {
		return resource.ProvenZero("elb", "dist.Origins")
	}

	var origins []string
	for _, origin := range dist.Origins.Items {
		if name := aws.ToString(origin.DomainName); maybeELBDNS(name) {
			origins = append(origins, name)
		}
	}
	if len(origins) == 0 {
		return resource.ProvenZero("elb", "the distribution's origins")
	}

	elbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if elbList == nil {
		return resource.UnknownRelated("elb")
	}

	var ids []string
	for _, elbRes := range elbList {
		if slices.ContainsFunc(origins, func(o string) bool { return dnsAliasNames(o, elbRes.Fields["dns_name"]) }) {
			ids = append(ids, elbRes.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncated)
}

// checkCfWAF searches the WAF cache for the Web ACL associated with this
// CloudFront distribution. The WebACLId field holds a full ARN.
func checkCfWAF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("waf")
	}
	if dist.WebACLId == nil || *dist.WebACLId == "" {
		return resource.ProvenZero("waf", "dist.WebACLId")
	}
	webACLID := *dist.WebACLId

	wafList, truncated, err := relatedResourcesFor(ctx, clients, cache, "waf")
	if err != nil {
		return resource.ErrorRelated("waf", err)
	}
	if wafList == nil {
		return resource.UnknownRelated("waf")
	}

	var ids []string
	for _, wafRes := range wafList {
		if wafRes.Fields["arn"] == webACLID || wafRes.ID == webACLID {
			ids = append(ids, wafRes.ID)
		}
	}
	return relatedResultTrunc("waf", ids, truncated)
}

// checkCfACM searches the ACM cache for the certificate associated with this
// CloudFront distribution via ViewerCertificate.ACMCertificateArn.
func checkCfACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("acm")
	}
	if dist.ViewerCertificate == nil || dist.ViewerCertificate.ACMCertificateArn == nil || *dist.ViewerCertificate.ACMCertificateArn == "" {
		return resource.ProvenZero("acm", "dist.ViewerCertificate.ACMCertificateArn")
	}
	certARN := *dist.ViewerCertificate.ACMCertificateArn

	// CloudFront serves a custom viewer certificate from us-east-1 alone, so
	// the certificate is never on the session region's own list.
	certRegion := arnRegionOf(certARN, "acm")
	acmList, _, truncated, err := relatedListIn(ctx, clients, cache, "acm", certRegion)
	if err != nil {
		return resource.ErrorRelated("acm", err)
	}
	if acmList == nil {
		return resource.UnknownRelated("acm")
	}

	var ids []string
	for _, acmRes := range acmList {
		if acmRes.Fields["certificate_arn"] == certARN || acmRes.ID == certARN {
			ids = append(ids, acmRes.ID)
		}
	}
	return inRegion(clients, certRegion, relatedResultTrunc("acm", ids, truncated))
}

// checkCfR53 reports the Route 53 hosted zones with an alias record
// pointing at this distribution. The r53 fetcher already carries every
// zone's AliasTarget.DNSName values in Fields["alias_targets"], so the join
// costs no call of its own.
//
// When the r53 cache is truncated and zero matches found, returns
// a truncated "(0+)" result.
func checkCfR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domainName := res.Fields["domain_name"]
	if dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct); ok && dist.DomainName != nil {
		domainName = *dist.DomainName
	}
	if domainName == "" {
		return resource.ProvenZero("r53", "domain_name")
	}

	zoneList, truncated, err := relatedResourcesFor(ctx, clients, cache, "r53")
	if err != nil {
		return resource.ErrorRelated("r53", err)
	}
	if zoneList == nil {
		return resource.UnknownRelated("r53")
	}

	var ids []string
	for _, zoneRes := range zoneList {
		// The zone fetcher reads one page of record sets. When more remain,
		// this distribution's alias record may be on one of them, so the
		// answer is a lower bound — which is what r53 → cf already renders
		// for the same zone, and the two directions of one relationship have
		// to agree.
		if zoneRes.Fields["records_truncated"] == "true" {
			truncated = true
		}
		targets := strings.Split(zoneRes.Fields["alias_targets"], ",")
		if slices.ContainsFunc(targets, func(t string) bool { return dnsAliasNames(t, domainName) }) {
			ids = append(ids, zoneRes.ID)
		}
	}
	return relatedResultTrunc("r53", ids, truncated)
}

// checkCfAlarm reports CloudWatch alarms on this CloudFront distribution.
// CloudFront alarms use dimension "DistributionId" (global metrics). Scans
// the alarm cache for that dimension matching this distribution's ID.
func checkCfAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "cf", res)
}

// checkCfLambda reports Lambda@Edge associations on this distribution.
// One cloudfront:GetDistributionConfig call; extract
// LambdaFunctionAssociations across default + ordered cache behaviors.
func checkCfLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.KnownRelated("lambda", nil, false)
	}
	cfg, err := cfDistributionConfig(ctx, clients, res.ID)
	if errors.Is(err, errClientMissing) {
		return resource.UnknownRelated("lambda")
	}
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	if cfg == nil {
		return resource.ProvenZero("lambda", "cfg")
	}
	var arns []string
	collect := func(lfa *cftypes.LambdaFunctionAssociations) {
		if lfa == nil {
			return
		}
		for _, item := range lfa.Items {
			arns = append(arns, aws.ToString(item.LambdaFunctionARN))
		}
	}
	if cfg.DefaultCacheBehavior != nil {
		collect(cfg.DefaultCacheBehavior.LambdaFunctionAssociations)
	}
	if cfg.CacheBehaviors != nil {
		for _, cb := range cfg.CacheBehaviors.Items {
			collect(cb.LambdaFunctionAssociations)
		}
	}
	return relatedRefs("lambda", arns, refContext(clients, cache, "lambda"))
}
