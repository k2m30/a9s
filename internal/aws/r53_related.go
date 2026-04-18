// r53_related.go contains Route 53 hosted-zone related-resource checker functions.
// Each checker makes at most one route53:ListResourceRecordSets call for the
// current zone and extracts the target type from the resulting record set's
// AliasTarget.DNSName / ResourceRecords[] values.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("r53", []resource.RelatedDef{
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkR53ELB, NeedsTargetCache: true},
		{TargetType: "cf", DisplayName: "CloudFront", Checker: checkR53CF, NeedsTargetCache: true},
		{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkR53ACM},
		{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkR53APIGW, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkR53Logs},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkR53S3, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPCs", Checker: checkR53VPC},
	})
}

// errNoR53Client sentinel used to distinguish missing-client from API error.
var errNoR53Client = errors.New("route53 client not initialized")

// r53ListRecordsFirstPage makes a single ListResourceRecordSets call for the
// given hosted zone via RetryOnThrottle. Zone ID may be in the raw form
// ("Z1ABCD") or canonical "/hostedzone/Z1ABCD"; the API accepts both.
func r53ListRecordsFirstPage(ctx context.Context, clients any, zoneID string) ([]r53types.ResourceRecordSet, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return nil, errNoR53Client
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.ListResourceRecordSetsOutput, error) {
		return c.Route53.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{HostedZoneId: &zoneID})
	})
	if err != nil {
		return nil, err
	}
	return out.ResourceRecordSets, nil
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

// canonicalDNS lowercases and strips a single trailing dot.
func canonicalDNS(s string) string {
	s = strings.ToLower(s)
	return strings.TrimSuffix(s, ".")
}

// checkR53ELB reports load balancers referenced by AliasTarget.DNSName in
// this zone's records. Pattern C: one ListResourceRecordSets call per zone,
// then cross-check the DNS names against the ELB cache.
func checkR53ELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	sets, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	aliases := r53AliasDNSNames(sets)
	if len(aliases) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	// Only alias records pointing at "*.elb.amazonaws.com" are ELB aliases.
	wanted := make(map[string]struct{})
	for _, d := range aliases {
		if strings.Contains(d, ".elb.amazonaws.com") {
			wanted[d] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	elbList, _, _ := FetchRelatedTarget(ctx, clients, cache, "elb")
	if elbList == nil {
		// Fallback: return the alias DNS names as Ids when cache is unavailable.
		ids := make([]string, 0, len(wanted))
		for d := range wanted {
			ids = append(ids, d)
		}
		return relatedResult("elb", ids)
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
	return relatedResult("elb", ids)
}

// checkR53CF reports CloudFront distributions referenced by AliasTarget.DNSName
// in this zone's records. Pattern C: one ListResourceRecordSets call, match
// against CF domain names in the cf cache.
func checkR53CF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	sets, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	aliases := r53AliasDNSNames(sets)
	if len(aliases) == 0 {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	wanted := make(map[string]struct{})
	for _, d := range aliases {
		if strings.Contains(d, ".cloudfront.net") {
			wanted[d] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	cfList, _, _ := FetchRelatedTarget(ctx, clients, cache, "cf")
	if cfList == nil {
		ids := make([]string, 0, len(wanted))
		for d := range wanted {
			ids = append(ids, d)
		}
		return relatedResult("cf", ids)
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
	return relatedResult("cf", ids)
}

// checkR53APIGW reports API Gateways fronted by AliasTarget.DNSName in this
// zone's records. Pattern C: one ListResourceRecordSets call, look for alias
// DNS names of form "<api-id>.execute-api.<region>.amazonaws.com".
func checkR53APIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	sets, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1, Err: err}
	}
	aliases := r53AliasDNSNames(sets)
	// Extract API IDs from execute-api hostnames.
	wantedIDs := make(map[string]struct{})
	for _, d := range aliases {
		if !strings.Contains(d, ".execute-api.") {
			continue
		}
		if idx := strings.Index(d, ".execute-api."); idx > 0 {
			wantedIDs[d[:idx]] = struct{}{}
		}
	}
	if len(wantedIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	apigwList, _, _ := FetchRelatedTarget(ctx, clients, cache, "apigw")
	if apigwList == nil {
		ids := make([]string, 0, len(wantedIDs))
		for id := range wantedIDs {
			ids = append(ids, id)
		}
		return relatedResult("apigw", ids)
	}
	var ids []string
	for _, apigwRes := range apigwList {
		if _, found := wantedIDs[apigwRes.ID]; found {
			ids = append(ids, apigwRes.ID)
		}
	}
	return relatedResult("apigw", ids)
}

// checkR53S3 reports S3 buckets referenced by AliasTarget.DNSName (S3 website
// endpoints) in this zone's records.
func checkR53S3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	sets, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	aliases := r53AliasDNSNames(sets)
	wantedBuckets := make(map[string]struct{})
	for _, d := range aliases {
		// S3 website alias: <bucket>.s3-website-<region>.amazonaws.com
		// or <bucket>.s3-website.<region>.amazonaws.com
		if !strings.Contains(d, ".s3-website") {
			continue
		}
		if idx := strings.Index(d, ".s3-website"); idx > 0 {
			wantedBuckets[d[:idx]] = struct{}{}
		}
	}
	if len(wantedBuckets) == 0 {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	s3List, _, _ := FetchRelatedTarget(ctx, clients, cache, "s3")
	if s3List == nil {
		ids := make([]string, 0, len(wantedBuckets))
		for b := range wantedBuckets {
			ids = append(ids, b)
		}
		return relatedResult("s3", ids)
	}
	var ids []string
	for _, s3Res := range s3List {
		if _, found := wantedBuckets[s3Res.ID]; found {
			ids = append(ids, s3Res.ID)
		}
	}
	return relatedResult("s3", ids)
}

// checkR53ACM reports ACM certificates whose DNS validation CNAME records
// (pattern "_<hex>.<domain>") live in this zone. Pattern C: one
// ListResourceRecordSets call; count CNAME records whose name starts with
// "_" and whose value ends with ".acm-validations.aws." — the ACM validation
// record contract.
func checkR53ACM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	sets, err := r53ListRecordsFirstPage(ctx, clients, zoneID)
	if err != nil {
		if errors.Is(err, errNoR53Client) {
			return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "acm", Count: -1, Err: err}
	}
	var ids []string
	for _, r := range sets {
		if r.Type != r53types.RRTypeCname {
			continue
		}
		name := ""
		if r.Name != nil {
			name = *r.Name
		}
		if !strings.HasPrefix(name, "_") {
			continue
		}
		for _, rr := range r.ResourceRecords {
			if rr.Value == nil {
				continue
			}
			v := canonicalDNS(*rr.Value)
			if strings.HasSuffix(v, ".acm-validations.aws") {
				// The validation record name itself identifies the cert domain
				// being validated; we treat each unique validation record as
				// one cert.
				ids = append(ids, strings.TrimSuffix(name, "."))
				break
			}
		}
	}
	return relatedResult("acm", ids)
}

// checkR53Logs reports CloudWatch log groups receiving query-log traffic for
// this zone. Query-log configuration is on route53:ListQueryLoggingConfigs
// (not on ListHostedZones). That API is not in Route53API yet; returns
// Count:-1 until wired. No second-call workaround exists at 1-call budget.
func checkR53Logs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
}

// checkR53VPC reports VPCs associated with a private hosted zone.
// Pattern C: one route53:GetHostedZone call returns HostedZone + VPCs list
// for private zones.
func checkR53VPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["private_zone"] != "true" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	zoneID := res.ID
	if zoneID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Route53 == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.GetHostedZoneOutput, error) {
		return c.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: &zoneID})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
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
