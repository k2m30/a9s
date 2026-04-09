package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sgCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func sgCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sg") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sg related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sg related checker for %s not found", target)
	return nil
}

// ─── Navigable Field Registration ────────────────────────────────────────────

// TestNavigableFields_SG_Registered verifies that VpcId -> vpc is registered
// as a navigable field for the sg resource type.
func TestNavigableFields_SG_Registered(t *testing.T) {
	nav := resource.IsFieldNavigable("sg", "VpcId")
	if nav == nil {
		t.Fatal("expected navigable field \"VpcId\" not found for sg")
	}
	if nav.TargetType != "vpc" {
		t.Errorf("VpcId TargetType = %q, want \"vpc\"", nav.TargetType)
	}
}

// ─── VPC checker (Pattern F — forward field from res.Fields["vpc_id"]) ───────

// TestRelated_SG_VPC_Found verifies that a security group with a vpc_id field
// returns Count:1 and the VPC ID in ResourceIDs.
func TestRelated_SG_VPC_Found(t *testing.T) {
	source := resource.Resource{
		ID: "sg-test",
		Fields: map[string]string{
			"vpc_id": "vpc-123",
		},
	}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-123" {
		t.Errorf("ResourceIDs = %v, want [\"vpc-123\"]", result.ResourceIDs)
	}
}

// TestRelated_SG_VPC_EmptyVpcID verifies that a security group with an empty
// vpc_id field returns Count:0.
func TestRelated_SG_VPC_EmptyVpcID(t *testing.T) {
	source := resource.Resource{
		ID:     "sg-no-vpc",
		Fields: map[string]string{"vpc_id": ""},
	}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ─── EC2 checker (Pattern C — cache: match RawStruct SecurityGroups[].GroupId) ─

// TestRelated_SG_EC2_Found verifies that an EC2 instance whose SecurityGroups
// slice contains the source SG ID is counted.
func TestRelated_SG_EC2_Found(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"ec2": {Resources: []resource.Resource{
			{
				ID: "i-match",
				RawStruct: ec2types.Instance{
					InstanceId: aws.String("i-match"),
					SecurityGroups: []ec2types.GroupIdentifier{
						{GroupId: aws.String("sg-test"), GroupName: aws.String("test-sg")},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 {
		t.Error("ResourceIDs is empty, want at least one entry")
	}
}

// TestRelated_SG_EC2_NotFound verifies that an EC2 instance with a different
// SG ID yields Count:0.
func TestRelated_SG_EC2_NotFound(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"ec2": {Resources: []resource.Resource{
			{
				ID: "i-nomatch",
				RawStruct: ec2types.Instance{
					InstanceId: aws.String("i-nomatch"),
					SecurityGroups: []ec2types.GroupIdentifier{
						{GroupId: aws.String("sg-other"), GroupName: aws.String("other-sg")},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_SG_EC2_CacheMissNoClients verifies that an empty cache with nil
// clients returns Count:-1 (unknown).
func TestRelated_SG_EC2_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss with nil clients)", result.Count)
	}
}

// TestRelated_SG_EC2_EmptySourceID verifies that a source with an empty ID
// returns Count:0.
func TestRelated_SG_EC2_EmptySourceID(t *testing.T) {
	source := resource.Resource{ID: ""}
	cache := resource.ResourceCache{
		"ec2": {Resources: []resource.Resource{
			{
				ID: "i-any",
				RawStruct: ec2types.Instance{
					InstanceId: aws.String("i-any"),
					SecurityGroups: []ec2types.GroupIdentifier{
						{GroupId: aws.String("sg-test"), GroupName: aws.String("test-sg")},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty source ID", result.Count)
	}
}

// ─── ENI checker (Pattern C — cache: match RawStruct Groups[].GroupId) ───────

// TestRelated_SG_ENI_Found verifies that an ENI whose Groups slice contains the
// source SG ID is counted.
func TestRelated_SG_ENI_Found(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"eni": {Resources: []resource.Resource{
			{
				ID: "eni-match",
				RawStruct: ec2types.NetworkInterface{
					NetworkInterfaceId: aws.String("eni-match"),
					Groups: []ec2types.GroupIdentifier{
						{GroupId: aws.String("sg-test"), GroupName: aws.String("test-sg")},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 {
		t.Error("ResourceIDs is empty, want at least one entry")
	}
}

// TestRelated_SG_ENI_NotFound verifies that an ENI with a different SG ID
// yields Count:0.
func TestRelated_SG_ENI_NotFound(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"eni": {Resources: []resource.Resource{
			{
				ID: "eni-nomatch",
				RawStruct: ec2types.NetworkInterface{
					NetworkInterfaceId: aws.String("eni-nomatch"),
					Groups: []ec2types.GroupIdentifier{
						{GroupId: aws.String("sg-other"), GroupName: aws.String("other-sg")},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_SG_ENI_CacheMissNoClients verifies that an empty cache with nil
// clients returns Count:-1 (unknown).
func TestRelated_SG_ENI_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss with nil clients)", result.Count)
	}
}

// ─── ELB checker (Pattern C — cache: match RawStruct SecurityGroups[] string slice) ─

// TestRelated_SG_ELB_Found verifies that a load balancer whose SecurityGroups
// slice contains the source SG ID is counted.
func TestRelated_SG_ELB_Found(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"elb": {Resources: []resource.Resource{
			{
				ID: "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod/abc",
				RawStruct: elbv2types.LoadBalancer{
					LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod/abc"),
					SecurityGroups:  []string{"sg-test"},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 {
		t.Error("ResourceIDs is empty, want at least one entry")
	}
}

// TestRelated_SG_ELB_NotFound verifies that an ELB with a different SG ID
// yields Count:0.
func TestRelated_SG_ELB_NotFound(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{
		"elb": {Resources: []resource.Resource{
			{
				ID: "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other/xyz",
				RawStruct: elbv2types.LoadBalancer{
					LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other/xyz"),
					SecurityGroups:  []string{"sg-other"},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_SG_ELB_CacheMissNoClients verifies that an empty cache with nil
// clients returns Count:-1 (unknown).
func TestRelated_SG_ELB_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "sg-test"}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss with nil clients)", result.Count)
	}
}

// ─── CFN checker (Pattern F — tag: "aws:cloudformation:stack-name") ─────────

// TestRelated_SG_CFN_Found verifies that a security group with the
// aws:cloudformation:stack-name tag returns Count:1 and the stack name as ID.
func TestRelated_SG_CFN_Found(t *testing.T) {
	source := resource.Resource{
		ID: "sg-tagged",
		RawStruct: ec2types.SecurityGroup{
			GroupId: aws.String("sg-tagged"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [\"my-stack\"]", result.ResourceIDs)
	}
}

// TestRelated_SG_CFN_NoTag verifies that a security group with no CFN tag
// returns Count:0.
func TestRelated_SG_CFN_NoTag(t *testing.T) {
	source := resource.Resource{
		ID: "sg-untagged",
		RawStruct: ec2types.SecurityGroup{
			GroupId: aws.String("sg-untagged"),
			Tags:    []ec2types.Tag{},
		},
	}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ─── SG → SG (Referencing SGs) ───────────────────────────────────────────────

// TestRelated_SG_SG_Found verifies that an SG in the cache whose IpPermissions
// contains a UserIdGroupPair referencing the source SG is counted.
func TestRelated_SG_SG_Found(t *testing.T) {
	source := resource.Resource{ID: "sg-source"}
	cache := resource.ResourceCache{
		"sg": {Resources: []resource.Resource{
			{
				ID: "sg-other",
				RawStruct: ec2types.SecurityGroup{
					GroupId: aws.String("sg-other"),
					IpPermissions: []ec2types.IpPermission{
						{
							IpProtocol: aws.String("tcp"),
							FromPort:   aws.Int32(443),
							ToPort:     aws.Int32(443),
							UserIdGroupPairs: []ec2types.UserIdGroupPair{
								{GroupId: aws.String("sg-source")},
							},
						},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_SG_SG_FoundInEgress verifies that an SG in the cache whose
// IpPermissionsEgress contains a UserIdGroupPair referencing the source SG is
// counted.
func TestRelated_SG_SG_FoundInEgress(t *testing.T) {
	source := resource.Resource{ID: "sg-source"}
	cache := resource.ResourceCache{
		"sg": {Resources: []resource.Resource{
			{
				ID: "sg-egress",
				RawStruct: ec2types.SecurityGroup{
					GroupId: aws.String("sg-egress"),
					IpPermissionsEgress: []ec2types.IpPermission{
						{
							IpProtocol: aws.String("-1"),
							UserIdGroupPairs: []ec2types.UserIdGroupPair{
								{GroupId: aws.String("sg-source")},
							},
						},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_SG_SG_SkipsSelf verifies that an SG whose GroupId equals the
// source SG's ID (self-referencing rule) is excluded from results.
func TestRelated_SG_SG_SkipsSelf(t *testing.T) {
	source := resource.Resource{ID: "sg-source"}
	cache := resource.ResourceCache{
		"sg": {Resources: []resource.Resource{
			{
				ID: "sg-source",
				RawStruct: ec2types.SecurityGroup{
					GroupId: aws.String("sg-source"),
					IpPermissions: []ec2types.IpPermission{
						{
							UserIdGroupPairs: []ec2types.UserIdGroupPair{
								{GroupId: aws.String("sg-source")},
							},
						},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (self-reference must be excluded)", result.Count)
	}
}

// TestRelated_SG_SG_NotFound verifies that SGs in the cache which do not
// reference the source SG return Count:0.
func TestRelated_SG_SG_NotFound(t *testing.T) {
	source := resource.Resource{ID: "sg-source"}
	cache := resource.ResourceCache{
		"sg": {Resources: []resource.Resource{
			{
				ID: "sg-unrelated",
				RawStruct: ec2types.SecurityGroup{
					GroupId: aws.String("sg-unrelated"),
					IpPermissions: []ec2types.IpPermission{
						{
							UserIdGroupPairs: []ec2types.UserIdGroupPair{
								{GroupId: aws.String("sg-other-entirely")},
							},
						},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_SG_SG_CacheMissNoClients verifies that an empty cache with nil
// clients returns Count:-1 (cache miss).
func TestRelated_SG_SG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "sg-source"}
	cache := resource.ResourceCache{}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss)", result.Count)
	}
}

// TestRelated_SG_SG_EmptySourceID verifies that a source with an empty ID
// returns Count:0.
func TestRelated_SG_SG_EmptySourceID(t *testing.T) {
	source := resource.Resource{ID: ""}
	cache := resource.ResourceCache{
		"sg": {Resources: []resource.Resource{
			{
				ID: "sg-other",
				RawStruct: ec2types.SecurityGroup{
					GroupId: aws.String("sg-other"),
					IpPermissions: []ec2types.IpPermission{
						{
							UserIdGroupPairs: []ec2types.UserIdGroupPair{
								{GroupId: aws.String("")},
							},
						},
					},
				},
			},
		}},
	}

	checker := sgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty source ID)", result.Count)
	}
}
