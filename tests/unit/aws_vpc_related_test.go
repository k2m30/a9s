package unit_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// vpcCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func vpcCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("vpc") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("vpc related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("vpc related checker for %s not found", target)
	return nil
}

const vpcTestID = "vpc-abc123"

// vpcSrcResource returns a canonical test VPC resource.
func vpcSrcResource() resource.Resource {
	return resource.Resource{
		ID:   vpcTestID,
		Name: "prod-vpc",
		Fields: map[string]string{
			"vpc_id": vpcTestID,
		},
		RawStruct: ec2types.Vpc{
			VpcId: new(vpcTestID),
		},
	}
}

// --- Subnet checker (Pattern C — reverse cache lookup by vpc_id field) ---

// TestRelated_VPC_Subnet_Match verifies that a subnet whose vpc_id matches the
// source VPC ID is counted.
func TestRelated_VPC_Subnet_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "subnet-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- Security Group checker ---

// TestRelated_VPC_SG_Match verifies that a security group whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_SG_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"sg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "sg-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- EC2 checker ---

// TestRelated_VPC_EC2_Match verifies that an EC2 instance whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_EC2_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "i-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- ELB checker ---

// TestRelated_VPC_ELB_Match verifies that a load balancer whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_ELB_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod/abc",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- NAT checker ---

// TestRelated_VPC_NAT_Match verifies that a NAT gateway whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_NAT_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "nat-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- IGW checker ---

// TestRelated_VPC_IGW_Match verifies that an internet gateway whose vpc_id
// matches the source VPC ID is counted.
func TestRelated_VPC_IGW_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"igw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "igw-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "igw")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- Route Table checker ---

// TestRelated_VPC_RTB_Match verifies that a route table whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_RTB_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "rtb-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- VPC Endpoint checker ---

// TestRelated_VPC_VPCE_Match verifies that a VPC endpoint whose vpc_id matches
// the source VPC ID is counted.
func TestRelated_VPC_VPCE_Match(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "vpce-1",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}

	checker := vpcCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// --- NoMatch: all caches contain resources in a different VPC ---

// TestRelated_VPC_NoMatch verifies that resources belonging to a different VPC
// produce Count=0 across all real checkers.
func TestRelated_VPC_NoMatch(t *testing.T) {
	const otherVPC = "vpc-zzzzzz"
	res := vpcSrcResource()

	singleEntry := func(target, id string) resource.ResourceCacheEntry {
		return resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: id, Fields: map[string]string{"vpc_id": otherVPC}},
		}}
	}

	cache := resource.ResourceCache{
		"subnet": singleEntry("subnet", "subnet-other"),
		"sg":     singleEntry("sg", "sg-other"),
		"ec2":    singleEntry("ec2", "i-other"),
		"elb":    singleEntry("elb", "arn:elb:other"),
		"nat":    singleEntry("nat", "nat-other"),
		"igw":    singleEntry("igw", "igw-other"),
		"rtb":    singleEntry("rtb", "rtb-other"),
		"vpce":   singleEntry("vpce", "vpce-other"),
	}

	targets := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce"}
	for _, target := range targets {
		checker := vpcCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, cache)
		if result.Count != 0 {
			t.Errorf("target %q: Count = %d, want 0 (no match)", target, result.Count)
		}
	}
}

// --- NilClients: empty cache → Count=-1 ---

// TestRelated_VPC_NilClients verifies that all real checkers return Count=-1
// when the cache is empty and no clients are provided.
func TestRelated_VPC_NilClients(t *testing.T) {
	res := vpcSrcResource()
	emptyCache := resource.ResourceCache{}

	targets := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce"}
	for _, target := range targets {
		checker := vpcCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target %q: Count = %d, want -1 (nil clients, empty cache)", target, result.Count)
		}
	}
}

// --- CFN checker tests (tag-based, no cache needed) ---

// TestRelated_VPC_CFN_HasTag verifies that a VPC with the aws:cloudformation:stack-name
// tag produces Count=1 with the stack name in ResourceIDs.
func TestRelated_VPC_CFN_HasTag(t *testing.T) {
	res := resource.Resource{
		ID:     vpcTestID,
		Fields: map[string]string{"vpc_id": vpcTestID},
		RawStruct: ec2types.Vpc{
			Tags: []ec2types.Tag{
				{Key: new("aws:cloudformation:stack-name"), Value: new("vpc-stack")},
			},
		},
	}

	checker := vpcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (VPC has CFN tag)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-stack" {
		t.Errorf("ResourceIDs = %v, want [\"vpc-stack\"]", result.ResourceIDs)
	}
}

// TestRelated_VPC_CFN_NoTag verifies that a VPC without the aws:cloudformation:stack-name
// tag produces Count=0.
func TestRelated_VPC_CFN_NoTag(t *testing.T) {
	res := resource.Resource{
		ID:        vpcTestID,
		Fields:    map[string]string{"vpc_id": vpcTestID},
		RawStruct: ec2types.Vpc{},
	}

	checker := vpcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (VPC has no CFN tag)", result.Count)
	}
}
