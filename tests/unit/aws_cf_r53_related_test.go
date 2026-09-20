package unit_test

// aws_cf_r53_related_test.go — Tests for the CloudFront → Route 53 related
// checker (checkCfR53). The checker is unexported; we retrieve it via
// resource.GetRelated("cf") and the "r53" TargetType entry, matching the pattern
// used by other related-checker tests (see aws_sg_related_test.go).

import (
	"context"
	"strings"
	"testing"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	_ "github.com/k2m30/a9s/v3/core/aws" // trigger init() registrations
	"github.com/k2m30/a9s/v3/core/resource"
)

// cfR53Checker retrieves the checkCfR53 function from the "cf" related registry.
// Fails the test if the checker is not registered.
func cfR53Checker(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("cf") {
		if def.TargetType == "r53" {
			if def.Checker == nil {
				t.Fatal("cf related checker for r53 is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("cf related checker for r53 not found — checkCfR53 must be registered in cf.go")
	return nil
}

// makeCFResource builds a resource.Resource representing a CloudFront
// distribution. A distribution answers under its own
// "<id>.cloudfront.net" domain name, which is what a Route 53 alias record
// pointing at it carries.
func makeCFResource(id, domainName string) resource.Resource {
	dist := cftypes.DistributionSummary{
		Id:         &id,
		DomainName: &domainName,
	}
	return resource.Resource{
		ID:        id,
		Name:      id,
		Fields:    map[string]string{"domain_name": domainName},
		RawStruct: dist,
	}
}

// makeR53Resource builds a resource.Resource representing a Route 53 hosted
// zone whose records alias the given targets. The r53 fetcher joins every
// AliasTarget.DNSName it enumerated into Fields["alias_targets"].
func makeR53Resource(id, zoneName string, aliasTargets ...string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: zoneName,
		Fields: map[string]string{
			"name": zoneName, "status": "active",
			"alias_targets": strings.Join(aliasTargets, ","),
		},
	}
}

// r53Cache builds a ResourceCache with the given r53 resources.
func r53Cache(truncated bool, zones ...resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{
		"r53": {
			Resources:   zones,
			IsTruncated: truncated,
		},
	}
}

// TestCheckCfR53_MatchesZoneWithAliasRecord: a zone with an alias record
// pointing at this distribution is the zone that serves it.
func TestCheckCfR53_MatchesZoneWithAliasRecord(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E1ABC", "d111111abcdef8.cloudfront.net")
	zone := makeR53Resource("/hostedzone/Z001", "example.com", "d111111abcdef8.cloudfront.net")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (the zone aliases this distribution)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs(), zone.ID)
	}
	if result.Truncated() {
		t.Error("Truncated must be false when cache is not truncated")
	}
}

// TestCheckCfR53_MatchesAlongsideOtherAliasTargets: a zone aliases several
// things; the distribution's own domain among them is what counts.
func TestCheckCfR53_MatchesAlongsideOtherAliasTargets(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E2DEF", "d222222abcdef8.cloudfront.net")
	zone := makeR53Resource("/hostedzone/Z002", "example.com",
		"acme-web-alb-1234567890.us-east-1.elb.amazonaws.com", "d222222abcdef8.cloudfront.net")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs(), zone.ID)
	}
}

// TestCheckCfR53_MultipleZonesAliasOneDistribution: one distribution can be
// aliased from several zones → Count=2.
func TestCheckCfR53_MultipleZonesAliasOneDistribution(t *testing.T) {
	checker := cfR53Checker(t)

	const domainName = "d333333abcdef8.cloudfront.net"
	res := makeCFResource("E3GHI", domainName)
	zone1 := makeR53Resource("/hostedzone/Z003", "example.com", domainName)
	zone2 := makeR53Resource("/hostedzone/Z004", "other.com", domainName)
	zone3 := makeR53Resource("/hostedzone/Z005", "unrelated.io", "d999999abcdef8.cloudfront.net")
	cache := r53Cache(false, zone1, zone2, zone3)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (one per zone that aliases it)", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries ([%q, %q])", result.ResourceIDs(), zone1.ID, zone2.ID)
	}
	idSet := make(map[string]bool)
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	if !idSet[zone1.ID] {
		t.Errorf("ResourceIDs missing zone1 %q; got %v", zone1.ID, result.ResourceIDs())
	}
	if !idSet[zone2.ID] {
		t.Errorf("ResourceIDs missing zone2 %q; got %v", zone2.ID, result.ResourceIDs())
	}
}

// TestCheckCfR53_NoMatchDifferentDistribution: a zone that aliases another
// distribution is not this one's.
func TestCheckCfR53_NoMatchDifferentDistribution(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E4JKL", "d444444abcdef8.cloudfront.net")
	zone := makeR53Resource("/hostedzone/Z006", "other.com", "d555555abcdef8.cloudfront.net")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no zone aliases this distribution)", result.Count())
	}
	if len(result.ResourceIDs()) != 0 {
		t.Errorf("ResourceIDs = %v, want empty (no match)", result.ResourceIDs())
	}
	if result.Truncated() {
		t.Error("Truncated must be false when cache is complete and no match found")
	}
}

// TestCheckCfR53_TruncatedEmptyCacheReturnsTruncated: when r53 cache is
// truncated and contains no matching zones, the result must be Truncated=true
// (not a hard zero — more zones may exist beyond the cache window).
func TestCheckCfR53_TruncatedEmptyCacheReturnsTruncated(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E5MNO", "d555555abcdef8.cloudfront.net")
	zone := makeR53Resource("/hostedzone/Z007", "other.com", "d666666abcdef8.cloudfront.net")
	cache := r53Cache(true, zone)

	result := checker(context.Background(), nil, res, cache)

	// Must be TruncatedResult — not confirmed-zero, because unscanned pages
	// might contain a matching zone.
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for truncated-cache miss", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated must be true when cache is truncated and no match found — more zones may exist")
	}
}

// TestCheckCfR53_NoDomainNameReturnsZero: a row carrying no domain name names
// nothing an alias record could point at → Count=0.
func TestCheckCfR53_NoDomainNameReturnsZero(t *testing.T) {
	checker := cfR53Checker(t)

	distID := "E6PQR"
	res := resource.Resource{
		ID:        distID,
		RawStruct: cftypes.DistributionSummary{Id: &distID},
	}

	zone := makeR53Resource("/hostedzone/Z008", "example.com", "d777777abcdef8.cloudfront.net")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for a distribution with no domain name", result.Count())
	}
}

// TestCheckCfR53_TrailingDotNormalized: Route 53 stores an alias target fully
// qualified, with a trailing dot.
func TestCheckCfR53_TrailingDotNormalized(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E7STU", "d888888abcdef8.cloudfront.net")
	zone := makeR53Resource("/hostedzone/Z009", "example.com.", "d888888abcdef8.cloudfront.net.")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 — a trailing dot must be normalised before matching", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs(), zone.ID)
	}
}
