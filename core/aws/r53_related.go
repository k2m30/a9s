// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// r53_related.go contains Route 53 hosted-zone related-resource checker functions.
// Each checker makes at most one route53:ListResourceRecordSets call for the
// current zone and extracts the target type from the resulting record set's
// AliasTarget.DNSName / ResourceRecords[] values.
package aws

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// errClientMissing is a sentinel returned by related-resource checkers when
// the service client required for the check is not initialized. Callers
// should treat it as "unknown" (RelatedUnknown) rather than a real API error.
var errClientMissing = errors.New("AWS service client not initialized")

// r53ListRecordsFirstPage makes a single ListResourceRecordSets call for the
// given hosted zone via RetryOnThrottle. Zone ID may be in the raw form
// ("Z1ABCD") or canonical "/hostedzone/Z1ABCD"; the API accepts both. AWS
// defaults this call to at most 100 records per page (no MaxItems is set
// here), so truncated reports whether out.IsTruncated came back true — a
// zone with more than 100 records has records this single call never saw.
func r53ListRecordsFirstPage(ctx context.Context, clients any, zoneID string) (sets []r53types.ResourceRecordSet, truncated bool, err error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return nil, false, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.ListResourceRecordSetsOutput, error) {
		return c.Route53.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{HostedZoneId: &zoneID})
	})
	if err != nil {
		return nil, false, err
	}
	return out.ResourceRecordSets, out.IsTruncated, nil
}

// r53AliasDNSNames returns every AliasTarget.DNSName found across the record
// set slice, lowercased, trailing-dot-stripped for uniform matching.
func r53AliasDNSNames(sets []r53types.ResourceRecordSet) []string {
	var out []string
	for _, r := range sets {
		if r.AliasTarget == nil || r.AliasTarget.DNSName == nil {
			continue
		}
		out = append(out, canonicalDNS(*r.AliasTarget.DNSName))
	}
	return out
}

// r53RelatedResult folds the zone's own record scan and the target list's
// pagination into the one truncation flag the row renders. Either being cut
// short means "what you see is what was read", which is what "(N+)" says.
func r53RelatedResult(target string, ids []string, recordsTruncated, targetTruncated bool) resource.RelatedCheckResult {
	return relatedResultTrunc(target, ids, recordsTruncated || targetTruncated)
}

// checkR53ELB reports load balancers referenced by AliasTarget.DNSName in
// this zone's records. Pattern C: one ListResourceRecordSets call per zone,
// then cross-check the DNS names against the ELB cache.
func checkR53ELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("elb", nil, false)
	}
	sets, recordsTruncated, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("elb")
		}
		return resource.ErrorRelated("elb", err)
	}
	aliases := r53AliasDNSNames(sets)
	if len(aliases) == 0 {
		return relatedResultTrunc("elb", nil, recordsTruncated)
	}
	// Only alias records pointing at "*.elb.amazonaws.com" are ELB aliases.
	wanted := make(map[string]struct{})
	for _, d := range aliases {
		if strings.Contains(d, ".elb.amazonaws.com") {
			wanted[d] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return relatedResultTrunc("elb", nil, recordsTruncated)
	}
	elbList, elbTruncated, fetchErr := FetchRelatedTarget(ctx, clients, cache, "elb")
	if elbList == nil {
		if fetchErr != nil {
			return resource.ErrorRelated("elb", fetchErr)
		}
		// Nothing cached and no fetcher: the aliases name something we cannot
		// look up. The alias DNS name is not a elb ID, so reporting it would
		// offer the operator a row that navigates to nothing.
		return resource.UnknownRelated("elb")
	}
	var ids []string
	for _, elbRes := range elbList {
		dns := canonicalDNS(elbRes.Fields["dns_name"])
		// ELB DNS names also may be prefixed with "dualstack." in alias form.
		if dns == "" {
			continue
		}
		if _, found := wanted[dns]; found {
			ids = append(ids, elbRes.ID)
			continue
		}
		if _, found := wanted["dualstack."+dns]; found {
			ids = append(ids, elbRes.ID)
		}
	}
	return r53RelatedResult("elb", ids, recordsTruncated, elbTruncated)
}

// checkR53CF reports CloudFront distributions referenced by AliasTarget.DNSName
// in this zone's records. Pattern C: one ListResourceRecordSets call, match
// against CF domain names in the cf cache.
func checkR53CF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("cf", nil, false)
	}
	sets, recordsTruncated, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("cf")
		}
		return resource.ErrorRelated("cf", err)
	}
	aliases := r53AliasDNSNames(sets)
	if len(aliases) == 0 {
		return relatedResultTrunc("cf", nil, recordsTruncated)
	}
	wanted := make(map[string]struct{})
	for _, d := range aliases {
		if strings.Contains(d, ".cloudfront.net") {
			wanted[d] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return relatedResultTrunc("cf", nil, recordsTruncated)
	}
	cfList, cfTruncated, fetchErr := FetchRelatedTarget(ctx, clients, cache, "cf")
	if cfList == nil {
		if fetchErr != nil {
			return resource.ErrorRelated("cf", fetchErr)
		}
		// Nothing cached and no fetcher: the aliases name something we cannot
		// look up. The alias DNS name is not a cf ID, so reporting it would
		// offer the operator a row that navigates to nothing.
		return resource.UnknownRelated("cf")
	}
	var ids []string
	for _, cfRes := range cfList {
		dn := canonicalDNS(cfRes.Fields["domain_name"])
		if dn == "" {
			continue
		}
		if _, found := wanted[dn]; found {
			ids = append(ids, cfRes.ID)
		}
	}
	return r53RelatedResult("cf", ids, recordsTruncated, cfTruncated)
}

// checkR53APIGW reports API Gateways fronted by AliasTarget.DNSName in this
// zone's records. Pattern C: one ListResourceRecordSets call, look for alias
// DNS names of form "<api-id>.execute-api.<region>.amazonaws.com".
func checkR53APIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("apigw", nil, false)
	}
	sets, recordsTruncated, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("apigw")
		}
		return resource.ErrorRelated("apigw", err)
	}
	var apiIDs []string
	for _, d := range r53AliasDNSNames(sets) {
		if id := executeAPIHostID(d); id != "" {
			apiIDs = append(apiIDs, id)
		}
	}
	if len(apiIDs) == 0 {
		return relatedResultTrunc("apigw", nil, recordsTruncated)
	}
	apigwList, apigwTruncated, fetchErr := FetchRelatedTarget(ctx, clients, cache, "apigw")
	if apigwList == nil {
		if fetchErr != nil {
			return resource.ErrorRelated("apigw", fetchErr)
		}
		// Nothing cached and no fetcher: the aliases name something we cannot
		// look up. The alias DNS name is not a apigw ID, so reporting it would
		// offer the operator a row that navigates to nothing.
		return resource.UnknownRelated("apigw")
	}
	ids, dropped := listedRefs("apigw", apiIDs, refContext(clients, cache, "apigw"), apigwList)
	return r53RelatedResult("apigw", ids, recordsTruncated || dropped, apigwTruncated)
}

// checkR53S3 reports the S3 buckets this zone's S3-website alias records
// address. The bucket is the record's own name — see r53S3WebsiteBucketNames,
// which the zone fetcher joins on too.
func checkR53S3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	sets, recordsTruncated, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("s3")
		}
		return resource.ErrorRelated("s3", err)
	}
	wantedBuckets := make(map[string]struct{})
	for _, name := range r53S3WebsiteBucketNames(sets) {
		wantedBuckets[name] = struct{}{}
	}
	if len(wantedBuckets) == 0 {
		return relatedResultTrunc("s3", nil, recordsTruncated)
	}
	s3List, s3Truncated, fetchErr := FetchRelatedTarget(ctx, clients, cache, "s3")
	if s3List == nil {
		if fetchErr != nil {
			return resource.ErrorRelated("s3", fetchErr)
		}
		// Nothing cached and no fetcher: the aliases name something we cannot
		// look up. The alias DNS name is not a s3 ID, so reporting it would
		// offer the operator a row that navigates to nothing.
		return resource.UnknownRelated("s3")
	}
	var ids []string
	for _, s3Res := range s3List {
		if _, found := wantedBuckets[s3Res.ID]; found {
			ids = append(ids, s3Res.ID)
		}
	}
	return r53RelatedResult("s3", ids, recordsTruncated, s3Truncated)
}

// checkR53ACM reports ACM certificates whose DNS validation CNAME records
// (pattern "_<token>.<domain>") live in this zone. Pattern C: one
// ListResourceRecordSets call; a CNAME whose name starts with "_" and whose
// value ends with ".acm-validations.aws." — the ACM validation record
// contract — validates <domain>, and a certificate in the acm list whose
// domain or alternative name is <domain> (or its wildcard) is the one it
// validates.
func checkR53ACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("acm", nil, false)
	}
	sets, recordsTruncated, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("acm")
		}
		return resource.ErrorRelated("acm", err)
	}
	validated := map[string]bool{}
	for _, r := range sets {
		domain, ok := acmValidationDomain(aws.ToString(r.Name))
		if r.Type != r53types.RRTypeCname || !ok {
			continue
		}
		for _, rr := range r.ResourceRecords {
			if rr.Value != nil && strings.HasSuffix(canonicalDNS(*rr.Value), ".acm-validations.aws") {
				validated[domain] = true
				break
			}
		}
	}
	if len(validated) == 0 {
		return relatedResultTrunc("acm", nil, recordsTruncated)
	}
	certs, certsTruncated, err := relatedResourcesFor(ctx, clients, cache, "acm")
	if err != nil {
		return resource.ErrorRelated("acm", err)
	}
	if certs == nil {
		return resource.UnknownRelated("acm")
	}
	var ids []string
	for _, cert := range certs {
		sum, ok := assertStruct[acmtypes.CertificateSummary](cert.RawStruct)
		if !ok {
			continue
		}
		names := append([]string{aws.ToString(sum.DomainName)}, sum.SubjectAlternativeNameSummaries...)
		if slices.ContainsFunc(names, func(n string) bool { return validated[certDomain(n)] }) {
			ids = append(ids, cert.ID)
		}
	}
	return relatedResultTrunc("acm", ids, recordsTruncated || certsTruncated)
}

// checkR53Logs reports CloudWatch log groups receiving query-log traffic for
// this zone. Pattern C: one route53:ListQueryLoggingConfigs call filtered by
// HostedZoneId; match CloudWatchLogsLogGroupArn against the logs cache.
func checkR53Logs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("logs", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return resource.UnknownRelated("logs")
	}
	api, ok := c.Route53.(Route53ListQueryLoggingConfigsAPI)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.ListQueryLoggingConfigsOutput, error) {
		return api.ListQueryLoggingConfigs(ctx, &route53.ListQueryLoggingConfigsInput{HostedZoneId: &zoneID})
	})
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if out == nil || len(out.QueryLoggingConfigs) == 0 {
		return resource.KnownRelated("logs", nil, false)
	}

	logList, logsTruncated, fetchErr := FetchRelatedTarget(ctx, clients, cache, "logs")
	if logList == nil {
		if fetchErr != nil {
			return resource.ErrorRelated("logs", fetchErr)
		}
		// Nothing cached and no fetcher: the aliases name something we cannot
		// look up. The alias DNS name is not a logs ID, so reporting it would
		// offer the operator a row that navigates to nothing.
		return resource.UnknownRelated("logs")
	}

	var arns []string
	for _, cfg := range out.QueryLoggingConfigs {
		if cfg.CloudWatchLogsLogGroupArn != nil {
			arns = append(arns, *cfg.CloudWatchLogsLogGroupArn)
		}
	}
	ids, dropped := listedRefs("logs", arns, refContext(clients, cache, "logs"), logList)
	return r53RelatedResult("logs", ids, dropped, logsTruncated)
}

// checkR53VPC reports VPCs associated with a private hosted zone.
// Pattern C: one route53:GetHostedZone call returns HostedZone + VPCs list
// for private zones.
func checkR53VPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["private_zone"] != "true" {
		return resource.KnownRelated("vpc", nil, false)
	}
	zoneID := res.ID
	if zoneID == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return resource.UnknownRelated("vpc")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.GetHostedZoneOutput, error) {
		return c.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: &zoneID})
	})
	if err != nil {
		return resource.ErrorRelated("vpc", err)
	}
	var ids []string
	seen := make(map[string]bool)
	for _, v := range out.VPCs {
		if v.VPCId == nil || *v.VPCId == "" || seen[*v.VPCId] {
			continue
		}
		seen[*v.VPCId] = true
		ids = append(ids, *v.VPCId)
	}
	return relatedResult("vpc", ids)
}
