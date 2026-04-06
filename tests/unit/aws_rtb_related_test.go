package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func rtbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("rtb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("rtb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("rtb related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_RTB_Registered(t *testing.T) {
	expected := map[string]string{
		"VpcId":                 "vpc",
		"Associations.SubnetId": "subnet",
		"Routes.NatGatewayId":   "nat",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("rtb", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for rtb", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

func TestNavigableFields_RTB_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("rtb")
	if !ok || len(resources) == 0 {
		t.Fatal("no rtb demo fixtures available")
	}

	// First fixture: rtb-0aaa111111111111a (main RTB — has NatGatewayId, no SubnetId in assoc)
	raw, ok := resources[0].RawStruct.(ec2types.RouteTable)
	if !ok {
		t.Fatalf("RawStruct is not ec2types.RouteTable, got %T", resources[0].RawStruct)
	}
	if raw.VpcId == nil || *raw.VpcId == "" {
		t.Error("fixture[0] RawStruct.VpcId is nil or empty — VpcId field path cannot resolve")
	}
	if len(raw.Routes) == 0 {
		t.Error("fixture[0] RawStruct.Routes is empty — Routes.NatGatewayId field path cannot resolve")
	}

	hasNatGW := false
	for _, r := range raw.Routes {
		if r.NatGatewayId != nil && *r.NatGatewayId != "" {
			hasNatGW = true
			break
		}
	}
	if !hasNatGW {
		t.Error("fixture[0] RawStruct.Routes has no route with non-nil NatGatewayId")
	}

	// Second fixture: rtb-0bbb222222222222b (public RTB — has SubnetId in associations)
	if len(resources) < 2 {
		t.Fatal("expected at least 2 rtb demo fixtures")
	}
	raw2, ok := resources[1].RawStruct.(ec2types.RouteTable)
	if !ok {
		t.Fatalf("fixture[1] RawStruct is not ec2types.RouteTable, got %T", resources[1].RawStruct)
	}

	hasSubnetID := false
	for _, assoc := range raw2.Associations {
		if assoc.SubnetId != nil && *assoc.SubnetId != "" {
			hasSubnetID = true
			break
		}
	}
	if !hasSubnetID {
		t.Error("fixture[1] RawStruct.Associations has no association with non-nil SubnetId")
	}
}

// --- checkRTBSubnet (forward: Associations SubnetId → subnet cache) ---

func TestRelated_RTB_Subnet_Found(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			VpcId:        aws.String("vpc-test"),
			Associations: []ec2types.RouteTableAssociation{
				{SubnetId: aws.String("subnet-aaa"), Main: aws.Bool(false)},
				{SubnetId: aws.String("subnet-bbb"), Main: aws.Bool(false)},
			},
		},
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "subnet-aaa", Name: "subnet-a"},
			{ID: "subnet-bbb", Name: "subnet-b"},
		}},
	}

	checker := rtbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_Subnet_NotFound(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			VpcId:        aws.String("vpc-test"),
			Associations: []ec2types.RouteTableAssociation{
				{SubnetId: aws.String("subnet-zzz"), Main: aws.Bool(false)},
			},
		},
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "subnet-aaa", Name: "subnet-a"},
			{ID: "subnet-bbb", Name: "subnet-b"},
		}},
	}

	checker := rtbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_Subnet_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			VpcId:        aws.String("vpc-test"),
			Associations: []ec2types.RouteTableAssociation{
				{SubnetId: aws.String("subnet-aaa"), Main: aws.Bool(false)},
			},
		},
	}

	checker := rtbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

func TestRelated_RTB_Subnet_NoAssociations(t *testing.T) {
	// Main RTB typically has Main=true association with no SubnetId.
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			VpcId:        aws.String("vpc-test"),
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(true)},
			},
		},
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "subnet-aaa", Name: "subnet-a"},
		}},
	}

	checker := rtbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (main assoc has no SubnetId)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// --- checkRTBNAT (forward: Routes NatGatewayId → nat cache) ---

func TestRelated_RTB_NAT_Found(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-12345")},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "nat-12345", Name: "prod-nat"},
		}},
	}

	checker := rtbCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "nat-12345" {
		t.Errorf("ResourceIDs = %v, want [nat-12345]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_NAT_NotFound(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-99999")},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "nat-12345", Name: "prod-nat"},
		}},
	}

	checker := rtbCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_NAT_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-12345")},
			},
		},
	}

	checker := rtbCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- checkRTBIGW (forward: Routes GatewayId with "igw-" prefix → igw cache) ---

func TestRelated_RTB_IGW_Found(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-12345")},
			},
		},
	}
	cache := resource.ResourceCache{
		"igw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "igw-12345", Name: "prod-igw"},
		}},
	}

	checker := rtbCheckerByTarget(t, "igw")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "igw-12345" {
		t.Errorf("ResourceIDs = %v, want [igw-12345]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_IGW_NotFound(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-99999")},
			},
		},
	}
	cache := resource.ResourceCache{
		"igw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "igw-12345", Name: "prod-igw"},
		}},
	}

	checker := rtbCheckerByTarget(t, "igw")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_IGW_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-12345")},
			},
		},
	}

	checker := rtbCheckerByTarget(t, "igw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

func TestRelated_RTB_IGW_LocalGateway(t *testing.T) {
	// Routes that have GatewayId="local" should be filtered out — local is not an IGW.
	source := resource.Resource{
		ID: "rtb-test",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-test"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
			},
		},
	}
	cache := resource.ResourceCache{
		"igw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "igw-12345", Name: "prod-igw"},
		}},
	}

	checker := rtbCheckerByTarget(t, "igw")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (local gateway must be filtered out)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// --- CloudFormation stub ---

func TestRelated_RTB_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("rtb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for rtb")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("rtb cfn Checker should be nil (stub) — ec2types.RouteTable has no Tags field accessible without fetching")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for rtb")
}

// --- Demo Checker ---

func TestRelatedDemo_RTB_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("rtb")
	if checker == nil {
		t.Fatal("no demo checker registered for rtb")
	}

	// Use the first fixture: rtb-0aaa111111111111a (main RTB with NAT gateway route)
	results := checker(resource.Resource{ID: "rtb-0aaa111111111111a"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"subnet": false, "nat": false, "igw": false, "cfn": false}
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

	// At least one result must have Count > 0. The first fixture has a NAT gateway route.
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

	// Specifically verify nat has Count=1 for rtb-0aaa111111111111a.
	for _, r := range results {
		if r.TargetType == "nat" {
			if r.Count != 1 {
				t.Errorf("nat Count = %d, want 1 for rtb-0aaa111111111111a", r.Count)
			}
			break
		}
	}
}
