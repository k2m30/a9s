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

// --- Demo checker test ---
