package unit_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// subnetCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func subnetCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("subnet") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("subnet related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("subnet related checker for %s not found", target)
	return nil
}

// subnetSrcResource returns a canonical test resource for subnet-abc123.
func subnetSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "subnet-abc123",
		Name: "test-subnet",
		Fields: map[string]string{
			"subnet_id":         "subnet-abc123",
			"vpc_id":            "vpc-11111111",
			"cidr_block":        "10.0.1.0/24",
			"availability_zone": "us-east-1a",
			"state":             "available",
			"available_ips":     "251",
		},
	}
}

// --- EC2 Instances checker tests ---

func TestRelated_Subnet_EC2_Match(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "i-abc123",
				Fields: map[string]string{
					"subnet_id": "subnet-abc123",
				},
				RawStruct: ec2types.Instance{SubnetId: new("subnet-abc123")},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_Subnet_EC2_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "i-def456",
				Fields: map[string]string{
					"subnet_id": "subnet-other999",
				},
				RawStruct: ec2types.Instance{SubnetId: new("subnet-other999")},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- ENI checker tests ---

func TestRelated_Subnet_ENI_Match(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "eni-aabbccdd",
				Fields: map[string]string{
					"subnet_id": "subnet-abc123",
				},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- NAT Gateway checker tests ---

func TestRelated_Subnet_NAT_Match(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "nat-00112233",
				Fields: map[string]string{
					"subnet_id": "subnet-abc123",
				},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- ELB checker tests ---

func TestRelated_Subnet_ELB_Match(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "my-alb",
				RawStruct: elbv2types.LoadBalancer{
					AvailabilityZones: []elbv2types.AvailabilityZone{
						{SubnetId: new("subnet-abc123")},
					},
				},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_Subnet_ELB_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-alb",
				RawStruct: elbv2types.LoadBalancer{
					AvailabilityZones: []elbv2types.AvailabilityZone{
						{SubnetId: new("subnet-other999")},
					},
				},
			},
		}},
	}

	checker := subnetCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Nil clients / empty cache tests ---

func TestRelated_Subnet_NilClients(t *testing.T) {
	res := subnetSrcResource()
	emptyCache := resource.ResourceCache{}

	for _, target := range []string{"ec2", "eni", "nat", "elb"} {
		checker := subnetCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients, empty cache)", target, result.Count)
		}
	}
}

// --- RTB checker tests ---

func TestRelated_Subnet_RTB_ExplicitAssoc(t *testing.T) {
	res := subnetSrcResource()
	subnetID := "subnet-abc123"
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "rtb-explicit",
			RawStruct: ec2types.RouteTable{
				Associations: []ec2types.RouteTableAssociation{
					{SubnetId: &subnetID},
				},
			},
		}}},
	}

	checker := subnetCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (explicit association)", result.Count)
	}
}

func TestRelated_Subnet_RTB_MainRTB(t *testing.T) {
	res := subnetSrcResource()
	mainAssoc := true
	vpcID := "vpc-11111111"
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "rtb-main",
			RawStruct: ec2types.RouteTable{
				VpcId: &vpcID,
				Associations: []ec2types.RouteTableAssociation{
					{Main: &mainAssoc},
				},
			},
		}}},
	}

	checker := subnetCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (main RTB fallback)", result.Count)
	}
}

func TestRelated_Subnet_RTB_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	otherSubnet := "subnet-other999"
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "rtb-other",
			RawStruct: ec2types.RouteTable{
				Associations: []ec2types.RouteTableAssociation{
					{SubnetId: &otherSubnet},
				},
			},
		}}},
	}

	checker := subnetCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different subnet)", result.Count)
	}
}

// --- CFN checker tests ---

func TestRelated_Subnet_CFN_HasTag(t *testing.T) {
	res := resource.Resource{
		ID:     "subnet-abc123",
		Fields: map[string]string{"vpc_id": "vpc-11111111"},
		RawStruct: ec2types.Subnet{
			Tags: []ec2types.Tag{
				{Key: new("aws:cloudformation:stack-name"), Value: new("my-stack")},
			},
		},
	}

	checker := subnetCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (has CFN tag)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [\"my-stack\"]", result.ResourceIDs)
	}
}

func TestRelated_Subnet_CFN_NoTag(t *testing.T) {
	res := resource.Resource{
		ID:        "subnet-abc123",
		Fields:    map[string]string{"vpc_id": "vpc-11111111"},
		RawStruct: ec2types.Subnet{},
	}

	checker := subnetCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count)
	}
}

// --- NavigableFields test ---

func TestNavigableFields_Subnet(t *testing.T) {
	fields := resource.GetNavigableFields("subnet")
	found := false
	for _, f := range fields {
		if f.FieldPath == "VpcId" && f.TargetType == "vpc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("subnet NavigableField VpcId→vpc not registered")
	}
}

// --- VPC checker tests ---

// TestRelated_Subnet_VPC_Found: subnet with vpc_id in Fields returns the VPC.
func TestRelated_Subnet_VPC_Found(t *testing.T) {
	res := subnetSrcResource()
	checker := subnetCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (vpc_id present)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-11111111" {
		t.Errorf("ResourceIDs = %v, want [vpc-11111111]", result.ResourceIDs)
	}
}

// TestRelated_Subnet_VPC_NoVPCID: subnet with missing vpc_id returns Count:0.
func TestRelated_Subnet_VPC_NoVPCID(t *testing.T) {
	res := resource.Resource{
		ID:     "subnet-abc123",
		Fields: map[string]string{},
	}
	checker := subnetCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no vpc_id field)", result.Count)
	}
}

// --- ASG checker tests ---

// TestRelated_Subnet_ASG_Match: an ASG whose vpc_zone_identifier contains this
// subnet ID (comma-separated) produces Count:1.
func TestRelated_Subnet_ASG_Match(t *testing.T) {
	res := subnetSrcResource()
	// vpc_zone_identifier contains our subnet plus a second subnet.
	asgRes := resource.Resource{
		ID:   "my-asg",
		Name: "my-asg",
		Fields: map[string]string{
			"vpc_zone_identifier": "subnet-abc123,subnet-other999",
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	checker := subnetCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-asg" {
		t.Errorf("ResourceIDs = %v, want [my-asg]", result.ResourceIDs)
	}
}

// TestRelated_Subnet_ASG_NoMatch: an ASG whose vpc_zone_identifier does not
// include this subnet produces Count:0.
func TestRelated_Subnet_ASG_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	asgRes := resource.Resource{
		ID: "other-asg",
		Fields: map[string]string{
			"vpc_zone_identifier": "subnet-other999,subnet-another888",
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	checker := subnetCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (subnet not in vpc_zone_identifier)", result.Count)
	}
}

// TestRelated_Subnet_ASG_SubnetsFieldFallback: ASG with no vpc_zone_identifier
// but a "subnets" field containing the subnet ID matches.
func TestRelated_Subnet_ASG_SubnetsFieldFallback(t *testing.T) {
	res := subnetSrcResource()
	asgRes := resource.Resource{
		ID: "fallback-asg",
		Fields: map[string]string{
			"subnets": "subnet-abc123",
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	checker := subnetCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (subnets fallback field matched)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "fallback-asg" {
		t.Errorf("ResourceIDs = %v, want [fallback-asg]", result.ResourceIDs)
	}
}

// --- EKS checker tests ---
// checkSubnetEFS is a genuine stub (unconditionally Count:-1 for non-empty ID);
// it is intentionally not tested.

// TestRelated_Subnet_EKS_Match: an EKS cluster whose "subnets" field contains
// this subnet ID produces Count:1.
func TestRelated_Subnet_EKS_Match(t *testing.T) {
	res := subnetSrcResource()
	eksRes := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		Fields: map[string]string{
			"subnets": "subnet-abc123,subnet-other999",
		},
	}
	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{eksRes}},
	}

	checker := subnetCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (EKS subnets field matches)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-cluster" {
		t.Errorf("ResourceIDs = %v, want [my-cluster]", result.ResourceIDs)
	}
}

// TestRelated_Subnet_EKS_SubnetIDsFieldFallback: EKS cluster with no "subnets"
// but a "subnet_ids" field containing the subnet ID also matches.
func TestRelated_Subnet_EKS_SubnetIDsFieldFallback(t *testing.T) {
	res := subnetSrcResource()
	eksRes := resource.Resource{
		ID: "other-cluster",
		Fields: map[string]string{
			"subnet_ids": "subnet-abc123",
		},
	}
	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{eksRes}},
	}

	checker := subnetCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (subnet_ids fallback field matched)", result.Count)
	}
}

// TestRelated_Subnet_EKS_NoMatch: an EKS cluster whose subnets field does not
// contain this subnet produces Count:0.
func TestRelated_Subnet_EKS_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	eksRes := resource.Resource{
		ID: "unrelated-cluster",
		Fields: map[string]string{
			"subnets": "subnet-other999,subnet-another888",
		},
	}
	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{eksRes}},
	}

	checker := subnetCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (subnet not in EKS subnets)", result.Count)
	}
}

// --- VPCE checker tests ---

// TestRelated_Subnet_VPCE_Match: a VPC endpoint whose SubnetIds includes this
// subnet produces Count:1.
func TestRelated_Subnet_VPCE_Match(t *testing.T) {
	res := subnetSrcResource()
	vpceRes := resource.Resource{
		ID:   "vpce-0a1b2c3d4e5f60001",
		Name: "com.amazonaws.us-east-1.s3",
		RawStruct: ec2types.VpcEndpoint{
			VpcEndpointId: new("vpce-0a1b2c3d4e5f60001"),
			SubnetIds:     []string{"subnet-abc123", "subnet-other999"},
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}

	checker := subnetCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (subnet in VPCE SubnetIds)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpce-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [vpce-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
}

// TestRelated_Subnet_VPCE_NoMatch: a VPC endpoint whose SubnetIds does not
// include this subnet produces Count:0.
func TestRelated_Subnet_VPCE_NoMatch(t *testing.T) {
	res := subnetSrcResource()
	vpceRes := resource.Resource{
		ID: "vpce-other",
		RawStruct: ec2types.VpcEndpoint{
			SubnetIds: []string{"subnet-other999"},
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}

	checker := subnetCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (subnet not in VPCE SubnetIds)", result.Count)
	}
}

// TestRelated_Subnet_VPCE_WrongRawStruct: a VPCE resource with no
// RawStruct of the expected type is skipped (no panic, Count:0).
func TestRelated_Subnet_VPCE_WrongRawStruct(t *testing.T) {
	res := subnetSrcResource()
	vpceRes := resource.Resource{
		ID:        "vpce-wrongtype",
		RawStruct: "not-a-VpcEndpoint",
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}

	checker := subnetCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type skipped)", result.Count)
	}
}

// --- Demo checker test ---
