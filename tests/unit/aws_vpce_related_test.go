package unit_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// vpceCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func vpceCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("vpce") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("vpce related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("vpce related checker for %s not found", target)
	return nil
}

// vpceSrcInterfaceResource returns a canonical interface-type VPC endpoint test
// resource with subnets, security groups, ENIs, and no route tables.
func vpceSrcInterfaceResource() resource.Resource {
	return resource.Resource{
		ID: "vpce-abc123",
		Fields: map[string]string{
			"vpc_id": "vpc-abc123",
			"type":   "Interface",
		},
		RawStruct: ec2types.VpcEndpoint{
			VpcEndpointId:       strPtr("vpce-abc123"),
			VpcId:               strPtr("vpc-abc123"),
			SubnetIds:           []string{"subnet-1", "subnet-2"},
			Groups:              []ec2types.SecurityGroupIdentifier{{GroupId: strPtr("sg-1")}},
			NetworkInterfaceIds: []string{"eni-1"},
			RouteTableIds:       []string{},
		},
	}
}

// vpceSrcGatewayResource returns a canonical gateway-type VPC endpoint test
// resource with route tables and no subnets, SGs, or ENIs.
func vpceSrcGatewayResource() resource.Resource {
	return resource.Resource{
		ID: "vpce-gw123",
		Fields: map[string]string{
			"vpc_id": "vpc-abc123",
			"type":   "Gateway",
		},
		RawStruct: ec2types.VpcEndpoint{
			VpcEndpointId: strPtr("vpce-gw123"),
			VpcId:         strPtr("vpc-abc123"),
			RouteTableIds: []string{"rtb-1", "rtb-2"},
		},
	}
}

// --- Subnet checker (Pattern F — reads SubnetIds from RawStruct) ---

// TestRelated_VPCE_Subnet_HasIDs verifies that SubnetIds are counted correctly.
func TestRelated_VPCE_Subnet_HasIDs(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
}

// TestRelated_VPCE_Subnet_Empty verifies that an empty SubnetIds slice returns
// Count=0.
func TestRelated_VPCE_Subnet_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Security Group checker (Pattern F — reads Groups from RawStruct) ---

// TestRelated_VPCE_SG_HasGroups verifies that Groups entries are counted
// correctly.
func TestRelated_VPCE_SG_HasGroups(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sg-1" {
		t.Errorf("ResourceIDs = %v, want [sg-1]", result.ResourceIDs)
	}
}

// TestRelated_VPCE_SG_Empty verifies that an empty Groups slice returns
// Count=0.
func TestRelated_VPCE_SG_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Route Table checker (Pattern F — reads RouteTableIds from RawStruct) ---

// TestRelated_VPCE_RTB_HasIDs verifies that RouteTableIds are counted
// correctly.
func TestRelated_VPCE_RTB_HasIDs(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
}

// TestRelated_VPCE_RTB_Empty verifies that an empty RouteTableIds slice
// returns Count=0.
func TestRelated_VPCE_RTB_Empty(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Network Interface checker (Pattern F — reads NetworkInterfaceIds from
// RawStruct) ---

// TestRelated_VPCE_ENI_HasIDs verifies that NetworkInterfaceIds are counted
// correctly.
func TestRelated_VPCE_ENI_HasIDs(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eni-1" {
		t.Errorf("ResourceIDs = %v, want [eni-1]", result.ResourceIDs)
	}
}

// TestRelated_VPCE_ENI_Empty verifies that an empty NetworkInterfaceIds slice
// returns Count=0.
func TestRelated_VPCE_ENI_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Bad RawStruct ---

// TestRelated_VPCE_BadRawStruct verifies that a wrong RawStruct type causes
// all checkers to return Count=-1 or Count=0 rather than panicking.
func TestRelated_VPCE_BadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "vpce-bad",
		Fields:    map[string]string{"vpc_id": "vpc-abc123"},
		RawStruct: "not-a-vpc-endpoint",
	}

	targets := []string{"subnet", "sg", "rtb", "eni"}
	for _, target := range targets {
		checker := vpceCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count != -1 && result.Count != 0 {
			t.Errorf("target %q: Count = %d, want -1 or 0 for bad RawStruct", target, result.Count)
		}
	}
}

// --- Navigable Field Registration ---

// TestNavigableFields_VPCE verifies that VpcId→vpc is registered as a
// navigable field.
func TestNavigableFields_VPCE(t *testing.T) {
	nav := resource.IsFieldNavigable("vpce", "VpcId")
	if nav == nil {
		t.Fatal("expected navigable field VpcId not found for vpce")
	}
	if nav.TargetType != "vpc" {
		t.Errorf("VpcId TargetType = %q, want %q", nav.TargetType, "vpc")
	}
}

