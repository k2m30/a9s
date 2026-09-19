package unit_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpc-peer is a single-call type with no degraded rows, so any error fails
// the fetch.
func vpcPeerResourceByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	result, err := awsclient.FetchVpcPeeringConnectionsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchVpcPeeringConnectionsPage returned error: %v", err)
	}
	for _, r := range result.Resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// vpcPeerRTBCache builds an "rtb" cache entry from the shipped demo route
// tables: two routes into ProdPeerSharedID, one blackholed route into
// WarnPeerBlackholeID, none into WarnPeerNoRouteID.
func vpcPeerRTBCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	rows, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchRouteTablesPage(context.Background(), fakes.NewEC2(), token)
	})
	if err != nil {
		t.Fatalf("FetchRouteTablesPage: %v", err)
	}
	return resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: rows},
	}
}

// vpcPeerVPCCache builds a "vpc" cache entry from the demo VPCs:
// fixtProdVPCID is a member; the cross-account remote VPC is not.
func vpcPeerVPCCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	rows, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchVPCsPage(context.Background(), fakes.NewEC2(), token)
	})
	if err != nil {
		t.Fatalf("FetchVPCsPage: %v", err)
	}
	return resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: rows},
	}
}

func TestRelated_VpcPeer_Registered(t *testing.T) {
	defs := resource.GetRelated("vpc-peer")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for vpc-peer")
	}

	expectedTargets := []string{"rtb", "vpc", "ct-events"}
	if len(defs) != len(expectedTargets) {
		t.Errorf("vpc-peer: len(GetRelated) = %d, want exactly %d (sg must not sneak in as an extra registration — spec §2 explicitly excludes it)", len(defs), len(expectedTargets))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		for _, want := range expectedTargets {
			if def.TargetType != want {
				continue
			}
			seen[want] = true
			if def.Checker == nil {
				t.Errorf("vpc-peer %q: Checker should not be nil", def.TargetType)
			}
			if def.DisplayName == "" {
				t.Errorf("vpc-peer %q: DisplayName should not be empty", def.TargetType)
			}
		}
	}
	for _, target := range expectedTargets {
		if !seen[target] {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_VpcPeer_SGNotRegistered(t *testing.T) {
	defs := resource.GetRelated("vpc-peer")
	for _, def := range defs {
		if def.TargetType == "sg" {
			t.Error("vpc-peer: target \"sg\" must not be registered (spec §2 explicit exclusion — no declared link, cross-region peers cannot reference SGs at all)")
		}
	}
}

func TestRelated_VpcPeer_GraphRootRTBCount(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	cache := vpcPeerRTBCache(t)

	checker := checkerByTarget(t, "vpc-peer", "rtb")
	result := checker(context.Background(), nil, res, cache)
	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want RelatedResolved", result.State())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	want := []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"}
	sort.Strings(want)
	if !reflect.DeepEqual(result.ResourceIDs(), want) {
		t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs(), want)
	}
}

// A blackholed route reference still counts as "who routes into this
// tunnel" for the related PANEL — the panel answers a structural question
// (does a route reference this pcx), distinct from the enrichment layer's
// health judgment (is that route usable). WarnPeerBlackholeID's one
// referencing route (rtb-0ddd444444444444d) must still be counted.
func TestRelated_VpcPeer_RTB_BlackholedRouteStillCounted(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.WarnPeerBlackholeID)
	cache := vpcPeerRTBCache(t)

	checker := checkerByTarget(t, "vpc-peer", "rtb")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (a blackholed route is still a route reference)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "rtb-0ddd444444444444d" {
		t.Errorf("ResourceIDs = %v, want [rtb-0ddd444444444444d]", result.ResourceIDs())
	}
}

func TestRelated_VpcPeer_RTB_NoRouteZero(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.WarnPeerNoRouteID)
	cache := vpcPeerRTBCache(t)

	checker := checkerByTarget(t, "vpc-peer", "rtb")
	result := checker(context.Background(), nil, res, cache)
	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want RelatedResolved (rtb cache present and typed, just zero matches)", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// Cache absent entirely (this session never loaded rtb) is the only Unknown
// case for this checker.
func TestRelated_VpcPeer_RTB_AbsentCacheUnknown(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	checker := checkerByTarget(t, "vpc-peer", "rtb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (no rtb cache entry at all)", result.State())
	}
}

// A truncated but typed rtb cache is scanned: State stays RelatedResolved,
// Count and ResourceIDs report the partial scan, and Truncated carries through
// so the row renders "N+".
func TestRelated_VpcPeer_RTB_TruncatedCacheResolvedNotUnknown(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	cache := vpcPeerRTBCache(t)
	entry := cache["rtb"]
	entry.IsTruncated = true
	cache["rtb"] = entry

	checker := checkerByTarget(t, "vpc-peer", "rtb")
	result := checker(context.Background(), nil, res, cache)
	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want RelatedResolved (a present, typed rtb cache is trusted even when truncated)", result.State())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (the two known routes are still visible in the truncated cache)", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (cache IsTruncated must carry through so the row renders \"N+\")")
	}
}

// The vpc pivot is a membership gate: only a side whose VpcId is in the local
// vpc cache becomes an entry, so a cross-account accepter never does.

func TestRelated_VpcPeer_GraphRootVPCGate(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	cache := vpcPeerVPCCache(t)

	checker := checkerByTarget(t, "vpc-peer", "vpc")
	result := checker(context.Background(), nil, res, cache)
	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want RelatedResolved", result.State())
	}
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (only the local requester side is a cache member; the cross-account accepter side is never in the local vpc cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-0abc123def456789a" {
		t.Errorf("ResourceIDs = %v, want [vpc-0abc123def456789a] (fixtProdVPCID)", result.ResourceIDs())
	}
}

// The gate is symmetric: it resolves whichever side is in the cache and never
// assumes the requester is the local side.
func TestRelated_VpcPeer_VPCGate_SymmetricNotRequesterHardcoded(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	accepterVpcID := aws.ToString(res.RawStruct.(*ec2types.VpcPeeringConnection).AccepterVpcInfo.VpcId)

	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: accepterVpcID, RawStruct: ec2types.Vpc{VpcId: aws.String(accepterVpcID)}},
		}},
	}

	checker := checkerByTarget(t, "vpc-peer", "vpc")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (the accepter side alone must resolve when it's the one present in the cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != accepterVpcID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), accepterVpcID)
	}
}

// Both sides present in the cache (a same-account/region peering scenario)
// must resolve both as pivots.
func TestRelated_VpcPeer_VPCGate_BothSidesResolve(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	raw := res.RawStruct.(*ec2types.VpcPeeringConnection)
	requesterVpcID := aws.ToString(raw.RequesterVpcInfo.VpcId)
	accepterVpcID := aws.ToString(raw.AccepterVpcInfo.VpcId)

	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: requesterVpcID, RawStruct: ec2types.Vpc{VpcId: aws.String(requesterVpcID)}},
			{ID: accepterVpcID, RawStruct: ec2types.Vpc{VpcId: aws.String(accepterVpcID)}},
		}},
	}

	checker := checkerByTarget(t, "vpc-peer", "vpc")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (both sides are cache members)", result.Count())
	}
}

func TestRelated_VpcPeer_VPC_AbsentCacheUnknown(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	checker := checkerByTarget(t, "vpc-peer", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (no vpc cache entry at all)", result.State())
	}
}

func TestRelated_VpcPeer_CtEvents_Drillable(t *testing.T) {
	res := vpcPeerResourceByID(t, fixtures.ProdPeerSharedID)
	checker := checkerByTarget(t, "vpc-peer", "ct-events")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedDeferred {
		t.Errorf("State = %v, want RelatedDeferred (ct-events is a universal server-side pivot, drillable by resource id)", result.State())
	}
	if len(result.FetchFilter()) == 0 {
		t.Error("FetchFilter is empty, want a CloudTrail LookupEvents filter keyed on the vpc peering connection id")
	}
}
