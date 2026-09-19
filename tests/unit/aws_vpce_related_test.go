package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

func TestRelated_VPCE_Subnet_HasIDs(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2: %v", len(result.ResourceIDs()), result.ResourceIDs())
	}
}

func TestRelated_VPCE_Subnet_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_VPCE_SG_HasGroups(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sg-1" {
		t.Errorf("ResourceIDs = %v, want [sg-1]", result.ResourceIDs())
	}
}

func TestRelated_VPCE_SG_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_VPCE_RTB_HasIDs(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2: %v", len(result.ResourceIDs()), result.ResourceIDs())
	}
}

func TestRelated_VPCE_RTB_Empty(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_VPCE_ENI_HasIDs(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "eni-1" {
		t.Errorf("ResourceIDs = %v, want [eni-1]", result.ResourceIDs())
	}
}

func TestRelated_VPCE_ENI_Empty(t *testing.T) {
	res := vpceSrcGatewayResource()
	checker := vpceCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

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
		if result.State() == domain.RelatedResolved && result.Count() != 0 {
			t.Errorf("target %q: State=%v Count=%d, want RelatedUnknown or a resolved 0 for bad RawStruct", target, result.State(), result.Count())
		}
	}
}

func TestNavigableFields_VPCE(t *testing.T) {
	nav := resource.IsFieldNavigableForTest("vpce", "VpcId")
	if nav == nil {
		t.Fatal("expected navigable field VpcId not found for vpce")
	}
	if nav.TargetType != "vpc" {
		t.Errorf("VpcId TargetType = %q, want %q", nav.TargetType, "vpc")
	}
}

func TestRelated_VPCE_VPC_HasVPCID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs())
	}
}

func TestRelated_VPCE_VPC_EmptyVPCID(t *testing.T) {
	res := resource.Resource{
		ID:        "vpce-nofield",
		Fields:    map[string]string{"vpc_id": ""},
		RawStruct: ec2types.VpcEndpoint{},
	}
	checker := vpceCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id)", result.Count())
	}
}

func TestRelated_VPCE_R53_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := vpceCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

func TestRelated_VPCE_R53_NonEmptyID(t *testing.T) {
	res := vpceSrcInterfaceResource()
	checker := vpceCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (private zones via ListHostedZonesByVPC not in list response)", result.State())
	}
}

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
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpce-packets-alarm" {
		t.Errorf("ResourceIDs = %v, want [vpce-packets-alarm]", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_VPCE_Alarm_CacheMissNoClients(t *testing.T) {
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count())
	}
}

func TestRelated_VPCE_Alarm_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := vpceCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty endpoint ID)", result.Count())
	}
}

func TestRelated_VPCE_Logs_NilClients(t *testing.T) {
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_VPCE_Logs_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty endpoint ID)", result.Count())
	}
}
