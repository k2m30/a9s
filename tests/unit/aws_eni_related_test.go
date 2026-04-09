package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func eniCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("eni") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("eni related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("eni related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_ENI_Registered(t *testing.T) {
	expected := map[string]string{
		"VpcId":                    "vpc",
		"SubnetId":                 "subnet",
		"Groups.GroupId":           "sg",
		"Attachment.InstanceId":    "ec2",
		"Association.AllocationId": "eip",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("eni", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for eni", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- EC2 checker (Pattern C — cache-based, matches Attachment.InstanceId) ---

func TestRelated_ENI_EC2_Found(t *testing.T) {
	ec2Res := resource.Resource{
		ID:   "i-test",
		Name: "test-instance",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-test"),
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{
		ID: "eni-test-ec2",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-ec2"),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				InstanceId: aws.String("i-test"),
			},
		},
	}

	checker := eniCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "i-test" {
		t.Errorf("ResourceIDs = %v, want [i-test]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ENI_EC2_NotFound(t *testing.T) {
	ec2Res := resource.Resource{
		ID:   "i-other",
		Name: "other-instance",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-other"),
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{
		ID: "eni-test-ec2-notfound",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-ec2-notfound"),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				InstanceId: aws.String("i-test"),
			},
		},
	}

	checker := eniCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ENI_EC2_NoAttachment(t *testing.T) {
	ec2Res := resource.Resource{
		ID: "i-test",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-test"),
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{
		ID: "eni-test-no-attach",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-no-attach"),
			Attachment:         nil,
		},
	}

	checker := eniCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Attachment)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ENI_EC2_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "eni-test-ec2-cache-miss",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-ec2-cache-miss"),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				InstanceId: aws.String("i-test"),
			},
		},
	}

	checker := eniCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Security Groups checker (Pattern C — cache-based, matches Groups[].GroupId) ---

func TestRelated_ENI_SG_Found(t *testing.T) {
	sgRes1 := resource.Resource{
		ID: "sg-test1",
		RawStruct: ec2types.SecurityGroup{
			GroupId: aws.String("sg-test1"),
		},
	}
	sgRes2 := resource.Resource{
		ID: "sg-test2",
		RawStruct: ec2types.SecurityGroup{
			GroupId: aws.String("sg-test2"),
		},
	}
	cache := resource.ResourceCache{
		"sg": resource.ResourceCacheEntry{Resources: []resource.Resource{sgRes1, sgRes2}},
	}
	source := resource.Resource{
		ID: "eni-test-sg",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-sg"),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-test1"), GroupName: aws.String("test-sg-1")},
				{GroupId: aws.String("sg-test2"), GroupName: aws.String("test-sg-2")},
			},
		},
	}

	checker := eniCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ENI_SG_NotFound(t *testing.T) {
	sgRes := resource.Resource{
		ID: "sg-other",
		RawStruct: ec2types.SecurityGroup{
			GroupId: aws.String("sg-other"),
		},
	}
	cache := resource.ResourceCache{
		"sg": resource.ResourceCacheEntry{Resources: []resource.Resource{sgRes}},
	}
	source := resource.Resource{
		ID: "eni-test-sg-notfound",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-sg-notfound"),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-test1"), GroupName: aws.String("test-sg-1")},
			},
		},
	}

	checker := eniCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ENI_SG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "eni-test-sg-cache-miss",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-sg-cache-miss"),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-test1"), GroupName: aws.String("test-sg-1")},
			},
		},
	}

	checker := eniCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Elastic IPs checker (Pattern C — cache-based, matches Association.AllocationId) ---

func TestRelated_ENI_EIP_Found(t *testing.T) {
	eipRes := resource.Resource{
		ID: "eipalloc-test",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-test"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{eipRes}},
	}
	source := resource.Resource{
		ID: "eni-test-eip",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-eip"),
			Association: &ec2types.NetworkInterfaceAssociation{
				AllocationId: aws.String("eipalloc-test"),
			},
		},
	}

	checker := eniCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eipalloc-test" {
		t.Errorf("ResourceIDs = %v, want [eipalloc-test]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ENI_EIP_NoAssociation(t *testing.T) {
	eipRes := resource.Resource{
		ID: "eipalloc-test",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-test"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{eipRes}},
	}
	source := resource.Resource{
		ID: "eni-test-eip-no-assoc",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-eip-no-assoc"),
			Association:        nil,
		},
	}

	checker := eniCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Association)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ENI_EIP_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "eni-test-eip-cache-miss",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-test-eip-cache-miss"),
			Association: &ec2types.NetworkInterfaceAssociation{
				AllocationId: aws.String("eipalloc-test"),
			},
		},
	}

	checker := eniCheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}
