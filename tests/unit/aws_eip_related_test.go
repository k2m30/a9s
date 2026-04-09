package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func eipCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("eip") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("eip related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("eip related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_EIP(t *testing.T) {
	expected := map[string]string{
		"InstanceId":         "ec2",
		"NetworkInterfaceId": "eni",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigable("eip", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for eip", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

// --- EC2 checker (Pattern F — reads InstanceId from RawStruct) ---

func TestRelated_EIP_EC2_Associated(t *testing.T) {
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60001",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60001"),
			InstanceId:   aws.String("i-0a1b2c3d4e5f60001"),
		},
	}
	checker := eipCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "i-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [i-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EIP_EC2_NoInstance(t *testing.T) {
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60002",
		Fields: map[string]string{"instance_id": ""},
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60002"),
			InstanceId:   nil,
		},
	}
	checker := eipCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// --- ENI checker (Pattern F — reads NetworkInterfaceId from RawStruct) ---

func TestRelated_EIP_ENI_Associated(t *testing.T) {
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60003",
		Fields: map[string]string{},
		RawStruct: ec2types.Address{
			AllocationId:       aws.String("eipalloc-0a1b2c3d4e5f60003"),
			NetworkInterfaceId: aws.String("eni-0a1b2c3d4e5f60001"),
		},
	}
	checker := eipCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eni-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [eni-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EIP_ENI_NoENI(t *testing.T) {
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60004",
		Fields: map[string]string{},
		RawStruct: ec2types.Address{
			AllocationId:       aws.String("eipalloc-0a1b2c3d4e5f60004"),
			NetworkInterfaceId: nil,
		},
	}
	checker := eipCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// --- NAT checker (Pattern C — cache-based, matches AllocationId in NatGatewayAddresses) ---

func TestRelated_EIP_NAT_Found(t *testing.T) {
	allocationID := "eipalloc-0a1b2c3d4e5f60005"
	natRes := resource.Resource{
		ID: "nat-0a1b2c3d4e5f60001",
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String("nat-0a1b2c3d4e5f60001"),
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String(allocationID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{natRes}},
	}
	source := resource.Resource{
		ID:     allocationID,
		Fields: map[string]string{"allocation_id": allocationID},
		RawStruct: ec2types.Address{
			AllocationId: aws.String(allocationID),
		},
	}

	checker := eipCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "nat-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [nat-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EIP_NAT_NoMatch(t *testing.T) {
	natRes := resource.Resource{
		ID: "nat-0a1b2c3d4e5f60002",
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String("nat-0a1b2c3d4e5f60002"),
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-different-0000001")},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{natRes}},
	}
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60006",
		Fields: map[string]string{"allocation_id": "eipalloc-0a1b2c3d4e5f60006"},
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60006"),
		},
	}

	checker := eipCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EIP_NAT_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "eipalloc-0a1b2c3d4e5f60007",
		Fields: map[string]string{"allocation_id": "eipalloc-0a1b2c3d4e5f60007"},
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60007"),
		},
	}

	checker := eipCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}
