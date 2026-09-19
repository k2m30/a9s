package unit_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_VPC_NoMatch(t *testing.T) {
	const otherVPC = "vpc-zzzzzz"
	res := vpcSrcResource()

	singleEntry := func(_, id string) resource.ResourceCacheEntry {
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
		if result.Count() != 0 {
			t.Errorf("target %q: Count = %d, want 0 (no match)", target, result.Count())
		}
	}
}

func TestRelated_VPC_NilClients(t *testing.T) {
	res := vpcSrcResource()
	emptyCache := resource.ResourceCache{}

	targets := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce"}
	for _, target := range targets {
		checker := vpcCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.State() != domain.RelatedUnknown {
			t.Errorf("target %q: Count = %d, want -1 (nil clients, empty cache)", target, result.Count())
		}
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (VPC has CFN tag)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-stack" {
		t.Errorf("ResourceIDs = %v, want [\"vpc-stack\"]", result.ResourceIDs())
	}
}

func TestRelated_VPC_CFN_NoTag(t *testing.T) {
	res := resource.Resource{
		ID:        vpcTestID,
		Fields:    map[string]string{"vpc_id": vpcTestID},
		RawStruct: ec2types.Vpc{},
	}

	checker := vpcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (VPC has no CFN tag)", result.Count())
	}
}
