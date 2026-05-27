package unit_test

// aws_cf_r53_related_test.go — Tests for the CloudFront → Route 53 zone-suffix
// related checker (checkCfR53). The checker is unexported; we retrieve it via
// resource.GetRelated("cf") and the "r53" TargetType entry, matching the pattern
// used by other related-checker tests (see aws_sg_related_test.go).
//
// Invariants tested:
//   - Exact zone-name match: alias == zone.Name → Count=1
//   - Subdomain suffix match: alias ends with "."+zone.Name → Count=1
//   - Multiple aliases across multiple zones → Count=len(matched zones)
//   - No-match: different domain → Count=0
//   - Truncated cache with no matches → Approximate=true
//   - No aliases → Count=0
//   - Trailing dot normalisation: zone "example.com." matches alias "example.com"

import (
	"context"
	"testing"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	_ "github.com/k2m30/a9s/v3/internal/aws" // trigger init() registrations
	"github.com/k2m30/a9s/v3/internal/resource"
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

// makeCFResource builds a resource.Resource representing a CloudFront distribution
// with the given aliases. The RawStruct is set to a cftypes.DistributionSummary so
// that extractCfAliases uses the preferred RawStruct path.
func makeCFResource(id string, aliases []string) resource.Resource {
	items := make([]string, len(aliases))
	copy(items, aliases)
	qty := int32(len(items))

	dist := cftypes.DistributionSummary{
		Id: &id,
		Aliases: &cftypes.Aliases{
			Quantity: &qty,
			Items:    items,
		},
	}
	return resource.Resource{
		ID:        id,
		Name:      id,
		RawStruct: dist,
	}
}

// makeR53Resource builds a resource.Resource representing a Route 53 hosted zone.
// The zone name is stored in resource.Name (the r53ZoneName helper reads Name first).
func makeR53Resource(id, zoneName string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   zoneName,
		Fields: map[string]string{"name": zoneName, "status": "active"},
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

// ---------------------------------------------------------------------------
// TestCheckCfR53_MatchesExactZoneName
// ---------------------------------------------------------------------------

// TestCheckCfR53_MatchesExactZoneName: alias "example.com" matches zone "example.com".
func TestCheckCfR53_MatchesExactZoneName(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E1ABC", []string{"example.com"})
	zone := makeR53Resource("/hostedzone/Z001", "example.com")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (exact zone name match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs, zone.ID)
	}
	if result.Approximate {
		t.Error("Approximate must be false when cache is not truncated")
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_MatchesSubdomainOfZone
// ---------------------------------------------------------------------------

// TestCheckCfR53_MatchesSubdomainOfZone: alias "www.example.com" matches zone "example.com".
func TestCheckCfR53_MatchesSubdomainOfZone(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E2DEF", []string{"www.example.com"})
	zone := makeR53Resource("/hostedzone/Z002", "example.com")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (subdomain suffix match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs, zone.ID)
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_MultipleAliasesAcrossMultipleZones
// ---------------------------------------------------------------------------

// TestCheckCfR53_MultipleAliasesAcrossMultipleZones: two aliases matching two
// different zones each → Count=2.
func TestCheckCfR53_MultipleAliasesAcrossMultipleZones(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E3GHI", []string{"www.example.com", "api.other.com"})
	zone1 := makeR53Resource("/hostedzone/Z003", "example.com")
	zone2 := makeR53Resource("/hostedzone/Z004", "other.com")
	// An unrelated zone must NOT be counted.
	zone3 := makeR53Resource("/hostedzone/Z005", "unrelated.io")
	cache := r53Cache(false, zone1, zone2, zone3)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (one zone per alias domain)", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries ([%q, %q])", result.ResourceIDs, zone1.ID, zone2.ID)
	}
	// Verify both zone IDs are present (order may vary).
	idSet := make(map[string]bool)
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	if !idSet[zone1.ID] {
		t.Errorf("ResourceIDs missing zone1 %q; got %v", zone1.ID, result.ResourceIDs)
	}
	if !idSet[zone2.ID] {
		t.Errorf("ResourceIDs missing zone2 %q; got %v", zone2.ID, result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_NoMatchDifferentDomain
// ---------------------------------------------------------------------------

// TestCheckCfR53_NoMatchDifferentDomain: alias "example.com" does NOT match zone "other.com".
func TestCheckCfR53_NoMatchDifferentDomain(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E4JKL", []string{"example.com"})
	zone := makeR53Resource("/hostedzone/Z006", "other.com")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching zone)", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ResourceIDs = %v, want empty (no match)", result.ResourceIDs)
	}
	if result.Approximate {
		t.Error("Approximate must be false when cache is complete and no match found")
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_TruncatedEmptyCacheReturnsApproximate
// ---------------------------------------------------------------------------

// TestCheckCfR53_TruncatedEmptyCacheReturnsApproximate: when r53 cache is
// truncated and contains no matching zones, the result must be Approximate=true
// (not a hard zero — more zones may exist beyond the cache window).
func TestCheckCfR53_TruncatedEmptyCacheReturnsApproximate(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E5MNO", []string{"example.com"})
	// Cache is truncated and the zone "other.com" does not match.
	zone := makeR53Resource("/hostedzone/Z007", "other.com")
	cache := r53Cache(true, zone) // IsTruncated=true

	result := checker(context.Background(), nil, res, cache)

	// Must be ApproximateZero — not confirmed-zero, because unscanned pages
	// might contain a matching zone.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for truncated-cache miss", result.Count)
	}
	if !result.Approximate {
		t.Error("Approximate must be true when cache is truncated and no match found — more zones may exist")
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_NoAliasesReturnsZero
// ---------------------------------------------------------------------------

// TestCheckCfR53_NoAliasesReturnsZero: distribution with no aliases → Count=0.
func TestCheckCfR53_NoAliasesReturnsZero(t *testing.T) {
	checker := cfR53Checker(t)

	// Build distribution with empty Aliases.Items.
	zero := int32(0)
	dist := cftypes.DistributionSummary{
		Aliases: &cftypes.Aliases{
			Quantity: &zero,
			Items:    nil,
		},
	}
	distID := "E6PQR"
	dist.Id = &distID
	res := resource.Resource{
		ID:        distID,
		RawStruct: dist,
	}

	zone := makeR53Resource("/hostedzone/Z008", "example.com")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for distribution with no aliases", result.Count)
	}
}

// ---------------------------------------------------------------------------
// TestCheckCfR53_TrailingDotNormalized
// ---------------------------------------------------------------------------

// TestCheckCfR53_TrailingDotNormalized: zone name "example.com." (with trailing
// dot, as Route 53 stores it) matches alias "example.com" after normalisation.
func TestCheckCfR53_TrailingDotNormalized(t *testing.T) {
	checker := cfR53Checker(t)

	res := makeCFResource("E7STU", []string{"example.com"})
	// Zone name has trailing dot — standard Route 53 format.
	zone := makeR53Resource("/hostedzone/Z009", "example.com.")
	cache := r53Cache(false, zone)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 — trailing dot must be normalised before matching;"+
			" zone.Name=%q, alias=%q", result.Count, zone.Name, "example.com")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs, zone.ID)
	}
}
