package unit_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgwCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func tgwCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("tgw") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("tgw related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("tgw related checker for %s not found", target)
	return nil
}

const tgwTestID = "tgw-abc123"

// tgwSrcResource returns a canonical test resource for a TGW.
func tgwSrcResource() resource.Resource {
	return resource.Resource{
		ID:   tgwTestID,
		Name: "test-tgw",
		Fields: map[string]string{
			"tgw_id": tgwTestID,
		},
		RawStruct: ec2types.TransitGateway{
			TransitGatewayId: strPtr(tgwTestID),
		},
	}
}

// --- RTB checker tests (Pattern C — reverse cache lookup) ---

// TestRelated_TGW_RTB_Match verifies that an RTB whose route has a
// TransitGatewayId matching the source TGW is counted.
func TestRelated_TGW_RTB_Match(t *testing.T) {
	res := tgwSrcResource()
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "rtb-match",
				RawStruct: ec2types.RouteTable{
					Routes: []ec2types.Route{
						{TransitGatewayId: strPtr(tgwTestID), DestinationCidrBlock: strPtr("10.1.0.0/16")},
					},
				},
			},
		}},
	}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_TGW_RTB_NoMatch verifies that RTBs whose routes point to a
// different TGW produce Count=0.
func TestRelated_TGW_RTB_NoMatch(t *testing.T) {
	res := tgwSrcResource()
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "rtb-other",
				RawStruct: ec2types.RouteTable{
					Routes: []ec2types.Route{
						{TransitGatewayId: strPtr("tgw-different"), DestinationCidrBlock: strPtr("10.2.0.0/16")},
					},
				},
			},
		}},
	}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_TGW_NilClients verifies that the RTB checker returns Count=-1
// when the cache has no rtb entry (cache miss / nil clients).
func TestRelated_TGW_NilClients(t *testing.T) {
	res := tgwSrcResource()
	emptyCache := resource.ResourceCache{}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, emptyCache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients, empty cache)", result.Count)
	}
}

// --- Stub checker assertions ---

// TestRelated_TGW_VPCStub verifies that the vpc related def is registered but
// has a nil Checker (stub, not yet implemented).
func TestRelated_TGW_VPCStub(t *testing.T) {
	defs := resource.GetRelated("tgw")
	for _, def := range defs {
		if def.TargetType == "vpc" {
			if def.Checker != nil {
				t.Error("tgw vpc: expected nil Checker (stub)")
			}
			return
		}
	}
	t.Error("tgw vpc related def not found")
}

// --- CFN checker tests (tag-based, no cache needed) ---

// TestRelated_TGW_CFN_HasTag verifies that a TGW with the aws:cloudformation:stack-name
// tag produces Count=1 with the stack name in ResourceIDs.
func TestRelated_TGW_CFN_HasTag(t *testing.T) {
	res := resource.Resource{
		ID: tgwTestID,
		RawStruct: ec2types.TransitGateway{
			Tags: []ec2types.Tag{
				{Key: strPtr("aws:cloudformation:stack-name"), Value: strPtr("network-stack")},
			},
		},
	}

	checker := tgwCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (TGW has CFN tag)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "network-stack" {
		t.Errorf("ResourceIDs = %v, want [\"network-stack\"]", result.ResourceIDs)
	}
}

// TestRelated_TGW_CFN_NoTag verifies that a TGW without the aws:cloudformation:stack-name
// tag produces Count=0.
func TestRelated_TGW_CFN_NoTag(t *testing.T) {
	res := resource.Resource{
		ID:        tgwTestID,
		RawStruct: ec2types.TransitGateway{},
	}

	checker := tgwCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (TGW has no CFN tag)", result.Count)
	}
}

// --- Demo checker test ---

// TestRelatedDemo_TGW_Registered verifies the demo checker is registered and
// returns valid results with all expected target types present.
func TestRelatedDemo_TGW_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("tgw")
	if checker == nil {
		t.Fatal("no demo checker registered for tgw")
	}

	// Use the known fixture ID that returns a non-zero count.
	src := resource.Resource{ID: "tgw-0aaa111111111111a"}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"vpc": false, "rtb": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result should have Count > 0 (rtb=1 for this fixture).
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
