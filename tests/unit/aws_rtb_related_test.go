package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// --- CloudFormation checker (tag-based: aws:cloudformation:stack-name → cfn cache) ---

func TestRelated_RTB_CFN_Found(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-abc123",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-abc123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
				{Key: aws.String("Name"), Value: aws.String("my-rtb")},
			},
		},
	}
	cfnRes := resource.Resource{
		ID:     "my-stack",
		Name:   "my-stack",
		Fields: map[string]string{"stack_name": "my-stack"},
	}
	otherCfn := resource.Resource{
		ID:     "other-stack",
		Name:   "other-stack",
		Fields: map[string]string{"stack_name": "other-stack"},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes, otherCfn}},
	}

	checker := rtbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RTB_CFN_NotFound(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-abc123",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-abc123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}
	otherCfn := resource.Resource{
		ID:     "different-stack",
		Name:   "different-stack",
		Fields: map[string]string{"stack_name": "different-stack"},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{otherCfn}},
	}

	checker := rtbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_RTB_CFN_NoTag(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-abc123",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-abc123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("my-rtb")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "any-stack", Name: "any-stack"},
		}},
	}

	checker := rtbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cfn tag)", result.Count)
	}
}

func TestRelated_RTB_CFN_CacheMiss(t *testing.T) {
	source := resource.Resource{
		ID: "rtb-abc123",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-abc123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}

	checker := rtbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}
