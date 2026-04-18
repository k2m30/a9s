// cf_related.go contains CloudFront distribution related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

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
		return resource.ApproximateZero("s3")
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
		return resource.ApproximateZero("elb")
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
		return resource.ApproximateZero("waf")
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
		return resource.ApproximateZero("acm")
	}
	return relatedResult("acm", ids)
}

// checkCfR53 reports Route 53 hosted zones whose names plausibly contain this
// CloudFront distribution's aliases (suffix match). A zone z is reported as
// related when any of the distribution's aliases equals z.Name or ends with
// "." + z.Name. This is a cache-only heuristic: we do NOT confirm the zone
// actually has an alias record for the distribution (that would require
// O(N)-per-zone route53:ListResourceRecordSets calls). It IS, however, far
// better than returning "unknown" for every distribution, and matches the
// O(1)-per-zone budget of all other related-panel checkers.
//
// When the r53 cache is truncated and zero matches found, returns
// ApproximateZero (honest "0+").
func checkCfR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}

	// Extract the distribution's aliases. RawStruct is cftypes.DistributionSummary;
	// the field is Aliases.Items ([]string).
	// Fallback: parse Fields["aliases"] (comma-joined in the fetcher).
	aliases := extractCfAliases(res)
	if len(aliases) == 0 {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}

	zoneList, truncated, err := cfRelatedResources(ctx, clients, cache, "r53")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1, Err: err}
	}
	if zoneList == nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}

	var ids []string
	for _, zoneRes := range zoneList {
		zoneName := r53ZoneName(zoneRes)
		if zoneName == "" {
			continue
		}
		// Zone names typically end with "." — normalize.
		zoneName = strings.TrimSuffix(zoneName, ".")
		for _, alias := range aliases {
			if alias == "" {
				continue
			}
			a := strings.TrimSuffix(alias, ".")
			if a == zoneName || strings.HasSuffix(a, "."+zoneName) {
				ids = append(ids, zoneRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("r53")
	}
	return relatedResult("r53", ids)
}

// extractCfAliases returns the distribution's alias domains from either
// RawStruct (preferred) or Fields["aliases"] (fallback, comma-joined).
func extractCfAliases(res resource.Resource) []string {
	// Try RawStruct first. CloudFront DistributionSummary has .Aliases.Items.
	dist, ok := assertStruct[cftypes.DistributionSummary](res.RawStruct)
	if ok && dist.Aliases != nil {
		return dist.Aliases.Items
	}
	// Fallback to Fields.
	raw := res.Fields["aliases"]
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// r53ZoneName returns the zone name for an r53 cache entry. The r53 fetcher
// stores the name in Fields["name"] and also Name on the Resource.
func r53ZoneName(res resource.Resource) string {
	if res.Name != "" {
		return res.Name
	}
	return res.Fields["name"]
}

// checkCfAlarm reports CloudWatch alarms on this CloudFront distribution.
// CloudFront alarms use dimension "DistributionId" (global metrics). Scans
// the alarm cache for that dimension matching this distribution's ID.
func checkCfAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	distID := res.ID
	if distID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := cfRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Name != nil && *d.Name == "DistributionId" && d.Value != nil && *d.Value == distID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkCfLambda reports Lambda@Edge associations on this distribution.
// Pattern C: one cloudfront:GetDistributionConfig call; extract
// LambdaFunctionAssociations across default + ordered cache behaviors.
func checkCfLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	distID := res.ID
	if distID == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudFront == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudfront.GetDistributionConfigOutput, error) {
		return c.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: &distID})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if out.DistributionConfig == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	cfg := out.DistributionConfig

	seen := make(map[string]bool)
	var ids []string
	collect := func(lfa *cftypes.LambdaFunctionAssociations) {
		if lfa == nil {
			return
		}
		for _, item := range lfa.Items {
			if item.LambdaFunctionARN == nil || *item.LambdaFunctionARN == "" {
				continue
			}
			arn := *item.LambdaFunctionARN
			name := arn
			if idx := strings.LastIndex(arn, ":function:"); idx >= 0 {
				rest := arn[idx+len(":function:"):]
				if before, _, ok := strings.Cut(rest, ":"); ok {
					name = before
				} else {
					name = rest
				}
			}
			if seen[name] {
				return
			}
			seen[name] = true
			ids = append(ids, name)
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
	return relatedResult("lambda", ids)
}

// checkCfLogs reports the S3 bucket receiving access logs for this
// distribution (CloudFront writes access logs to S3, not CW Logs).
// Pattern C: one cloudfront:GetDistributionConfig call; read Logging.Bucket.
func checkCfLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	distID := res.ID
	if distID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudFront == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudfront.GetDistributionConfigOutput, error) {
		return c.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: &distID})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if out.DistributionConfig == nil || out.DistributionConfig.Logging == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	lg := out.DistributionConfig.Logging
	if lg.Enabled == nil || !*lg.Enabled || lg.Bucket == nil || *lg.Bucket == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	bucket := *lg.Bucket
	if idx := strings.Index(bucket, ".s3"); idx > 0 {
		bucket = bucket[:idx]
	}
	return relatedResult("logs", []string{bucket})
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
