// aws_nat_related_wave2_test.go — coverage wave 2 for nat_related.go checkers.
// Covers: checkNATEIP (0%), checkNATENI (0%), checkNATAlarm (0%).
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func natGatewaySource(natID, vpcID, allocID, eniID string) resource.Resource {
	return resource.Resource{
		ID:   natID,
		Name: natID,
		Fields: map[string]string{
			"nat_gateway_id": natID,
			"vpc_id":         vpcID,
			"state":          "available",
		},
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String(natID),
			VpcId:        aws.String(vpcID),
			State:        ec2types.NatGatewayStateAvailable,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{
					AllocationId:       aws.String(allocID),
					NetworkInterfaceId: aws.String(eniID),
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// checkNATEIP — Pattern C: NatGatewayAddresses.AllocationId vs eip cache
// ---------------------------------------------------------------------------

func TestRelated_NAT_EIP_MatchByAllocationID(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const allocID = "eipalloc-0aaa111111111111a"

	eipRes := resource.Resource{
		ID:   allocID,
		Name: allocID,
		RawStruct: ec2types.Address{
			AllocationId: aws.String(allocID),
			PublicIp:     aws.String("54.200.1.100"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{eipRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", allocID, "eni-0aaa111111111111a")

	checker := natCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != allocID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, allocID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_EIP_NoMatchWhenAllocIDNotInCache(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const allocID = "eipalloc-0aaa111111111111a"
	const otherAllocID = "eipalloc-0zzzzzzzzzzzzzzz"

	eipRes := resource.Resource{
		ID: otherAllocID,
		RawStruct: ec2types.Address{
			AllocationId: aws.String(otherAllocID),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{eipRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", allocID, "eni-0aaa111111111111a")

	checker := natCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: EIP resource matched via RawStruct.AllocationId (not resource.ID).
func TestRelated_NAT_EIP_MatchByRawStructAllocationID(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const allocID = "eipalloc-0aaa111111111111a"

	// resource.ID is the public IP, not alloc ID; checker falls back to RawStruct.AllocationId.
	eipRes := resource.Resource{
		ID:   "54.200.1.100",
		Name: "54.200.1.100",
		RawStruct: ec2types.Address{
			AllocationId: aws.String(allocID),
			PublicIp:     aws.String("54.200.1.100"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{eipRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", allocID, "eni-0aaa111111111111a")

	checker := natCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (matched via RawStruct.AllocationId)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "54.200.1.100" {
		t.Errorf("ResourceIDs = %v, want [54.200.1.100]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkNATENI — Pattern C: NatGatewayAddresses.NetworkInterfaceId vs eni cache
// ---------------------------------------------------------------------------

func TestRelated_NAT_ENI_MatchByNetworkInterfaceID(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const eniID = "eni-0aaa111111111111a"

	eniRes := resource.Resource{
		ID:   eniID,
		Name: eniID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", "eipalloc-0aaa111111111111a", eniID)

	checker := natCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != eniID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, eniID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_ENI_NoMatchWhenENINotInCache(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const eniID = "eni-0aaa111111111111a"
	const otherENIID = "eni-0zzzzzzzzzzzzzzz"

	eniRes := resource.Resource{
		ID: otherENIID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(otherENIID),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", "eipalloc-0aaa111111111111a", eniID)

	checker := natCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: NAT Gateway with no addresses → Count=0 (not -1).
func TestRelated_NAT_ENI_NoAddressesReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:   "nat-0aaa111111111111a",
		Name: "prod-nat-1a",
		RawStruct: ec2types.NatGateway{
			NatGatewayId:        aws.String("nat-0aaa111111111111a"),
			VpcId:               aws.String("vpc-0abc123def456789a"),
			State:               ec2types.NatGatewayStateAvailable,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{},
		},
	}

	checker := natCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no addresses)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkNATAlarm — Pattern D: alarm dimension "NatGatewayId"
// ---------------------------------------------------------------------------

func TestRelated_NAT_Alarm_MatchByNatGatewayIDDimension(t *testing.T) {
	const natID = "nat-0aaa111111111111a"
	const alarmName = "nat-active-connections"

	alarmRes := resource.Resource{
		ID:   alarmName,
		Name: alarmName,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String(alarmName),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("NatGatewayId"),
					Value: aws.String(natID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", "eipalloc-0aaa", "eni-0aaa")

	checker := natCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != alarmName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, alarmName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_NAT_Alarm_NoMatchWhenDimensionValueDiffers(t *testing.T) {
	const natID = "nat-0aaa111111111111a"

	alarmRes := resource.Resource{
		ID:   "some-other-alarm",
		Name: "some-other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("some-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("NatGatewayId"),
					Value: aws.String("nat-0bbb222222222222b"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", "eipalloc-0aaa", "eni-0aaa")

	checker := natCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: alarm has wrong dimension name (not NatGatewayId) — not matched.
func TestRelated_NAT_Alarm_NoMatchWhenDimensionNameDiffers(t *testing.T) {
	const natID = "nat-0aaa111111111111a"

	alarmRes := resource.Resource{
		ID:   "ec2-alarm-with-same-value",
		Name: "ec2-alarm-with-same-value",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ec2-alarm-with-same-value"),
			Dimensions: []cwtypes.Dimension{
				{
					// Wrong dimension name
					Name:  aws.String("InstanceId"),
					Value: aws.String(natID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := natGatewaySource(natID, "vpc-0abc123def456789a", "eipalloc-0aaa", "eni-0aaa")

	checker := natCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong dimension name)", result.Count)
	}
}
