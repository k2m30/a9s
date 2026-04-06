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

func igwCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("igw") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("igw related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("igw related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_IGW_Registered(t *testing.T) {
	expected := map[string]string{
		"Attachments.VpcId": "vpc",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("igw", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for igw", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

func TestNavigableFields_IGW_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("igw")
	if !ok || len(resources) == 0 {
		t.Fatal("no igw demo fixtures available")
	}

	// The first fixture should be an attached IGW with a non-empty VpcId in Attachments.
	raw, ok := resources[0].RawStruct.(ec2types.InternetGateway)
	if !ok {
		t.Fatalf("RawStruct is not ec2types.InternetGateway, got %T", resources[0].RawStruct)
	}
	if len(raw.Attachments) == 0 {
		t.Error("fixture RawStruct.Attachments is empty — Attachments.VpcId field path cannot resolve")
	}
	if raw.Attachments[0].VpcId == nil || *raw.Attachments[0].VpcId == "" {
		t.Error("fixture RawStruct.Attachments[0].VpcId is nil or empty — navigable field path cannot resolve")
	}
}

// --- VPC checker (Pattern F — reads VpcId from RawStruct Attachments) ---

func TestRelated_IGW_VPC_Found(t *testing.T) {
	const igwID = "igw-0aaa111111111111a"
	const vpcID = "vpc-0abc123def456789a"

	source := resource.Resource{
		ID:   igwID,
		Name: "prod-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": vpcID,
			"state":  "attached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String(vpcID), State: ec2types.AttachmentStatusAttached},
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

	checker := igwCheckerByTarget(t, "vpc")
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

func TestRelated_IGW_VPC_NoAttachments(t *testing.T) {
	const igwID = "igw-0ccc333333333333c"

	source := resource.Resource{
		ID:   igwID,
		Name: "detached-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": "",
			"state":  "detached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments:       []ec2types.InternetGatewayAttachment{},
		},
	}

	vpcRes := resource.Resource{
		ID:   "vpc-0abc123def456789a",
		Name: "prod-vpc",
	}
	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{Resources: []resource.Resource{vpcRes}},
	}

	checker := igwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for IGW with no attachments", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_IGW_VPC_CacheMissNoClients(t *testing.T) {
	const igwID = "igw-0aaa111111111111a"
	const vpcID = "vpc-0abc123def456789a"

	source := resource.Resource{
		ID:   igwID,
		Name: "prod-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": vpcID,
			"state":  "attached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String(vpcID), State: ec2types.AttachmentStatusAttached},
			},
		},
	}

	checker := igwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Route Tables checker (Pattern C — cache, Routes.GatewayId matches IGW ID) ---

func TestRelated_IGW_RTB_Found(t *testing.T) {
	const igwID = "igw-0aaa111111111111a"

	rtbRes := resource.Resource{
		ID:   "rtb-0bbb222222222222b",
		Name: "prod-public",
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0bbb222222222222b"),
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("10.0.0.0/16"),
					GatewayId:            aws.String("local"),
				},
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					GatewayId:            aws.String(igwID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{rtbRes}},
	}

	source := resource.Resource{
		ID:   igwID,
		Name: "prod-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": "vpc-0abc123def456789a",
			"state":  "attached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-0abc123def456789a"), State: ec2types.AttachmentStatusAttached},
			},
		},
	}

	checker := igwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "rtb-0bbb222222222222b" {
		t.Errorf("ResourceIDs = %v, want [rtb-0bbb222222222222b]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_IGW_RTB_NotFound(t *testing.T) {
	const igwID = "igw-0aaa111111111111a"
	const otherIGWID = "igw-0bbb222222222222b"

	// RTB routes point to a different gateway, not our IGW.
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
					GatewayId:            aws.String(otherIGWID),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{rtbRes}},
	}

	source := resource.Resource{
		ID:   igwID,
		Name: "prod-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": "vpc-0abc123def456789a",
			"state":  "attached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-0abc123def456789a"), State: ec2types.AttachmentStatusAttached},
			},
		},
	}

	checker := igwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_IGW_RTB_CacheMissNoClients(t *testing.T) {
	const igwID = "igw-0aaa111111111111a"

	source := resource.Resource{
		ID:   igwID,
		Name: "prod-igw",
		Fields: map[string]string{
			"igw_id": igwID,
			"vpc_id": "vpc-0abc123def456789a",
			"state":  "attached",
		},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-0abc123def456789a"), State: ec2types.AttachmentStatusAttached},
			},
		},
	}

	checker := igwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Demo Checker ---

func TestRelatedDemo_IGW_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("igw")
	if checker == nil {
		t.Fatal("no demo checker registered for igw")
	}

	results := checker(resource.Resource{ID: "igw-0aaa111111111111a"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify both expected target types are present.
	wantTargets := map[string]bool{"vpc": false, "rtb": false}
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

	// At least one result must have Count > 0.
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
