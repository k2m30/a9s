package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func amiCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ami") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ami related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ami related checker for %s not found", target)
	return nil
}

// --- EC2 Checker Tests (Pattern C) ---

func TestRelated_AMI_EC2_Found(t *testing.T) {
	amiID := "ami-0abc123"
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-0instance1", Fields: map[string]string{"image_id": amiID}},
			{ID: "i-0instance2", Fields: map[string]string{"image_id": "ami-other"}},
		}},
	}
	res := resource.Resource{ID: amiID}

	checker := amiCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "i-0instance1" {
		t.Errorf("ResourceIDs = %v, want [i-0instance1]", result.ResourceIDs)
	}
}

func TestRelated_AMI_EC2_NotFound(t *testing.T) {
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-0other", Fields: map[string]string{"image_id": "ami-other"}},
		}},
	}
	res := resource.Resource{ID: "ami-0abc123"}

	checker := amiCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_AMI_EC2_CacheMissNoClients(t *testing.T) {
	res := resource.Resource{ID: "ami-0abc123"}
	checker := amiCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_AMI_EC2_EmptyAMIID(t *testing.T) {
	res := resource.Resource{ID: ""}
	checker := amiCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty AMI ID", result.Count)
	}
}

// --- EBS Snapshot Checker Tests (Pattern F) ---

func TestRelated_AMI_EBSSnaps_Found(t *testing.T) {
	snapID := "snap-0abc123"
	img := ec2types.Image{
		ImageId: new("ami-0abc123"),
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{
			{Ebs: &ec2types.EbsBlockDevice{SnapshotId: &snapID}},
		},
	}
	res := resource.Resource{ID: "ami-0abc123", RawStruct: img}

	checker := amiCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != snapID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, snapID)
	}
}

func TestRelated_AMI_EBSSnaps_NoSnapshots(t *testing.T) {
	img := ec2types.Image{
		ImageId:             new("ami-0abc123"),
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{},
	}
	res := resource.Resource{ID: "ami-0abc123", RawStruct: img}

	checker := amiCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_AMI_EBSSnaps_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "ami-0abc123", RawStruct: "not-an-image"}

	checker := amiCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- ami→asg: traverses asg → asg.Instances[] → ec2 cache → image_id ---

// TestRelated_AMI_ASG_MatchViaRunningInstances verifies that an AMI matches an
// ASG when one of that ASG's running instances was launched from the AMI.
func TestRelated_AMI_ASG_MatchViaRunningInstances(t *testing.T) {
	const amiID = "ami-0abc1234def56789"
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:   "web-asg",
				Name: "web-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("web-asg"),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-web-1")},
						{InstanceId: aws.String("i-web-2")},
					},
				},
			},
			{
				ID:   "other-asg",
				Name: "other-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("other-asg"),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-other-1")},
					},
				},
			},
		}},
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-web-1", Fields: map[string]string{"image_id": amiID}},
			{ID: "i-web-2", Fields: map[string]string{"image_id": "ami-unrelated"}},
			{ID: "i-other-1", Fields: map[string]string{"image_id": "ami-unrelated"}},
		}},
	}
	source := resource.Resource{ID: amiID}

	checker := amiCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "web-asg" {
		t.Errorf("ResourceIDs = %v, want [web-asg]", result.ResourceIDs)
	}
	if result.TargetType != "asg" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "asg")
	}
}

// TestRelated_AMI_ASG_NoMatch verifies that an AMI not used by any running
// ASG instance returns Count=0.
func TestRelated_AMI_ASG_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "web-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("web-asg"),
					Instances:            []asgtypes.Instance{{InstanceId: aws.String("i-web-1")}},
				},
			},
		}},
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-web-1", Fields: map[string]string{"image_id": "ami-different"}},
		}},
	}
	source := resource.Resource{ID: "ami-0abc1234def56789"}
	checker := amiCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_AMI_ASG_EmptyAMIID returns Count=0 without any cache access.
func TestRelated_AMI_ASG_EmptyAMIID(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := amiCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty AMI ID", result.Count)
	}
}

// TestRelated_AMI_ASG_CacheMissNoClients returns Count=-1 when neither cache
// nor a live client can provide ASG/EC2 data.
func TestRelated_AMI_ASG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "ami-0abc1234def56789"}
	checker := amiCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}
