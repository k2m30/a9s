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
			VpcEndpointId:       new("vpce-abc123"),
			VpcId:               new("vpc-abc123"),
			SubnetIds:           []string{"subnet-1", "subnet-2"},
			Groups:              []ec2types.SecurityGroupIdentifier{{GroupId: new("sg-1")}},
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
			VpcEndpointId: new("vpce-gw123"),
			VpcId:         new("vpc-abc123"),
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

// --- VPC checker (Pattern F — reads vpc_id from Fields) ---

// TestRelated_VPCE_VPC_HasVPCID verifies that Fields["vpc_id"] is returned.
func TestRelated_VPCE_VPC_HasVPCID(t *testing.T) {
	res := vpceSrcInterfaceResource() // vpc_id = "vpc-abc123"
	checker := vpceCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

// TestRelated_VPCE_VPC_EmptyVPCID verifies that an empty vpc_id returns Count=0.
func TestRelated_VPCE_VPC_EmptyVPCID(t *testing.T) {
	res := resource.Resource{
		ID:        "vpce-nofield",
		Fields:    map[string]string{"vpc_id": ""},
		RawStruct: ec2types.VpcEndpoint{},
	}
	checker := vpceCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id)", result.Count)
	}
}

// --- ACM checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_ACM_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_ACM_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_ACM_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_ACM_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (non-empty ID, cannot determine ACM from list API)", result.Count)
	}
}

// --- CF checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_CF_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_CF_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_CF_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_CF_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (CloudFront VPC Origins not in list response)", result.Count)
	}
}

// --- R53 checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_R53_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_R53_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_R53_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_R53_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (private zones via ListHostedZonesByVPC not in list response)", result.Count)
	}
}

// --- S3 checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_S3_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_S3_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_S3_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_S3_NonEmptyID(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (policy interpretation not available from list)", result.Count)
	}
}

// --- TG checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_TG_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_TG_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_TG_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_TG_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (DescribeTargetHealth needed, not in cache)", result.Count)
	}
}

// --- WAF checker (Pattern stub — empty ID → 0, non-empty → -1) ---

// TestRelated_VPCE_WAF_EmptyID verifies Count=0 for empty endpoint ID.
func TestRelated_VPCE_WAF_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_VPCE_WAF_NonEmptyID verifies Count=-1 for a real endpoint ID.
func TestRelated_VPCE_WAF_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (WAF associations resolved from WAF side)", result.Count)
	}
}

// --- Alarm checker (Pattern C — cache scan, VpcEndpointId dimension) ---

// TestRelated_VPCE_Alarm_MatchByDimension verifies that an alarm with
// "VpcEndpointId" dimension matching the endpoint ID is returned.
func TestRelated_VPCE_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "vpce-packets-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("vpce-packets-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("VpcEndpointId"), Value: aws.String("vpce-abc123")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := vpceSrcInterfaceResource() // ID = "vpce-abc123"

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpce-packets-alarm" {
		t.Errorf("ResourceIDs = %v, want [vpce-packets-alarm]", result.ResourceIDs)
	}
}

// TestRelated_VPCE_Alarm_NoMatch verifies that alarms with a different
// VpcEndpointId dimension return Count=0.
func TestRelated_VPCE_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-vpce-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-vpce-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("VpcEndpointId"), Value: aws.String("vpce-different")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_VPCE_Alarm_CacheMissNoClients verifies Count=-1 when the alarm
// cache is empty and no clients are available.
func TestRelated_VPCE_Alarm_CacheMissNoClients(t *testing.T) {
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// TestRelated_VPCE_Alarm_EmptyID verifies Count=0 for an empty endpoint ID.
func TestRelated_VPCE_Alarm_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty endpoint ID)", result.Count)
	}
}

// --- Logs checker (Pattern C — nil clients → -1) ---

// TestRelated_VPCE_Logs_NilClients verifies Count=-1 when clients are nil
// (cannot call DescribeFlowLogs).
func TestRelated_VPCE_Logs_NilClients(t *testing.T) {
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// TestRelated_VPCE_Logs_EmptyID verifies Count=0 for an empty endpoint ID.
func TestRelated_VPCE_Logs_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty endpoint ID)", result.Count)
	}
}
