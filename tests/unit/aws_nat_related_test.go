package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func natCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("nat") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("nat related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("nat related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_NAT_Registered(t *testing.T) {
	expected := map[string]string{
		"VpcId":                           "vpc",
		"SubnetId":                        "subnet",
		"NatGatewayAddresses.AllocationId": "eip",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("nat", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for nat", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- VPC checker (Pattern F — reads VpcId from RawStruct, then cache lookup) ---

func TestRelated_NAT_VPC_Found(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const vpcID = "vpc-0abc123def456789a"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         vpcID,
			"subnet_id":      "subnet-0aaa111111111111a",
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String(vpcID),
			SubnetId:     aws.String("subnet-0aaa111111111111a"),
			State:        ec2types.NatGatewayStateAvailable,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0aaa111111111111a")},
			},
		},
	}

	vpcRes := resource.Resource{
		ID:   vpcID,
		Name: "prod-vpc",
		RawStruct: ec2types.Vpc{
			VpcId: aws.String(vpcID),
		},
	}
	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: []resource.Resource{vpcRes}},
	}

	checker := natCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != vpcID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, vpcID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_VPC_NotFound(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const vpcID = "vpc-0abc123def456789a"
	const otherVPCID = "vpc-0999999999999999z"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         vpcID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String(vpcID),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	// Cache contains a different VPC — not the one our NAT belongs to.
	otherVPC := resource.Resource{
		ID:   otherVPCID,
		Name: "other-vpc",
	}
	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: []resource.Resource{otherVPC}},
	}

	checker := natCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_VPC_CacheMissNoClients(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const vpcID = "vpc-0abc123def456789a"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         vpcID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String(vpcID),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	checker := natCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Subnet checker (Pattern F — reads SubnetId from RawStruct, then cache lookup) ---

func TestRelated_NAT_Subnet_Found(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const subnetID = "subnet-0aaa111111111111a"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         "vpc-0abc123def456789a",
			"subnet_id":      subnetID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String("vpc-0abc123def456789a"),
			SubnetId:     aws.String(subnetID),
			State:        ec2types.NatGatewayStateAvailable,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0aaa111111111111a")},
			},
		},
	}

	subnetRes := resource.Resource{
		ID:   subnetID,
		Name: "prod-public-subnet-1a",
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{subnetRes}},
	}

	checker := natCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != subnetID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, subnetID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_Subnet_NotFound(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const subnetID = "subnet-0aaa111111111111a"
	const otherSubnetID = "subnet-0999999999999999z"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"subnet_id":      subnetID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			SubnetId:     aws.String(subnetID),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	// Cache contains a different subnet.
	otherSubnet := resource.Resource{
		ID:   otherSubnetID,
		Name: "other-subnet",
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{otherSubnet}},
	}

	checker := natCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_Subnet_CacheMissNoClients(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const subnetID = "subnet-0aaa111111111111a"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"subnet_id":      subnetID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			SubnetId:     aws.String(subnetID),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	checker := natCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Route Tables checker (Pattern C — cache, Routes.NatGatewayId matches NAT ID) ---

func TestRelated_NAT_RTB_Found(t *testing.T) {
	const natID = "nat-0aaa111111111111a"

	rtbRes := resource.Resource{
		ID:   "rtb-0aaa111111111111a",
		Name: "prod-main",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0aaa111111111111a"),
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("10.0.0.0/16"),
					GatewayId:            aws.String("local"),
				},
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					NatGatewayId:         aws.String(natID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{rtbRes}},
	}

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         "vpc-0abc123def456789a",
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String("vpc-0abc123def456789a"),
			SubnetId:     aws.String("subnet-0aaa111111111111a"),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	checker := natCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "rtb-0aaa111111111111a" {
		t.Errorf("ResourceIDs = %v, want [rtb-0aaa111111111111a]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_RTB_NotFound(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const otherNATID = "nat-0bbb222222222222b"

	// RTB routes point to a different NAT, not our NAT.
	rtbRes := resource.Resource{
		ID:   "rtb-0ddd444444444444d",
		Name: "staging-main",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0ddd444444444444d"),
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("10.1.0.0/16"),
					GatewayId:            aws.String("local"),
				},
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					NatGatewayId:         aws.String(otherNATID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{rtbRes}},
	}

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         "vpc-0abc123def456789a",
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String("vpc-0abc123def456789a"),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	checker := natCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_RTB_CacheMissNoClients(t *testing.T) {
	const natID = "nat-0aaa111111111111a"

	source := resource.Resource{
		ID:   natID,
		Name: "prod-nat-1a",
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         "vpc-0abc123def456789a",
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String("vpc-0abc123def456789a"),
			State:        ec2types.NatGatewayStateAvailable,
		},
	}

	checker := natCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}
