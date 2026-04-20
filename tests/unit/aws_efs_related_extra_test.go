package unit_test

// aws_efs_related_extra_test.go — additional coverage for efs_related_extra.go.
// Covers: checkEFSAlarm, checkEFSEC2, checkEFSENI, checkEFSVPC.
// efsCheckerByTarget is defined in aws_efs_related_test.go (same package).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- checkEFSAlarm (Pattern D — FileSystemId dimension) ---

func TestRelated_EFS_Alarm_Found(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	alarmRes := resource.Resource{
		ID: "efs-throughput-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("efs-throughput-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FileSystemId"), Value: aws.String(fsID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	checker := efsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "efs-throughput-alarm" {
		t.Errorf("ResourceIDs[0] = %q, want efs-throughput-alarm", result.ResourceIDs[0])
	}
}

func TestRelated_EFS_Alarm_NoMatch(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	alarmRes := resource.Resource{
		ID: "efs-other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("efs-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FileSystemId"), Value: aws.String("fs-0000000000000000")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	checker := efsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching dimension)", result.Count)
	}
}

func TestRelated_EFS_Alarm_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "efs-alarm", RawStruct: cwtypes.MetricAlarm{AlarmName: aws.String("efs-alarm")}},
		}},
	}
	checker := efsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_EFS_Alarm_CacheMissNilClients(t *testing.T) {
	source := resource.Resource{ID: "fs-0a1b2c3d4e5f60001"}
	checker := efsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkEFSEC2 (cross-reference via ENI description containing fsID) ---

func TestRelated_EFS_EC2_Found(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	eniRes := resource.Resource{
		ID: "eni-0abc1234567890def",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890def"),
			Description:        aws.String("Amazon EFS " + fsID),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				InstanceId: aws.String("i-0instance1234567890"),
			},
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := efsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "i-0instance1234567890" {
		t.Errorf("ResourceIDs[0] = %q, want i-0instance1234567890", result.ResourceIDs[0])
	}
}

func TestRelated_EFS_EC2_ENINotAttachedToInstance(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	// ENI description contains fsID, but not attached to any instance
	eniRes := resource.Resource{
		ID: "eni-mount-target",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-mount-target"),
			Description:        aws.String("Amazon EFS " + fsID),
			Attachment:         nil,
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := efsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ENI not attached to instance)", result.Count)
	}
}

func TestRelated_EFS_EC2_DeduplicatesInstances(t *testing.T) {
	// Multiple ENIs on the same instance (e.g., two AZs) should count as one EC2 instance.
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	makeENI := func(id, instanceID string) resource.Resource {
		return resource.Resource{
			ID: id,
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String(id),
				Description:        aws.String("Amazon EFS " + fsID),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					InstanceId: aws.String(instanceID),
				},
			},
		}
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			makeENI("eni-az1", "i-0instance1234567890"),
			makeENI("eni-az2", "i-0instance1234567890"),
		}},
	}
	checker := efsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count)
	}
}

func TestRelated_EFS_EC2_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := efsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// --- checkEFSENI (scans eni cache for ENIs with fsID in description) ---

func TestRelated_EFS_ENI_Found(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	eniRes1 := resource.Resource{
		ID: "eni-mount-az1",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-mount-az1"),
			Description:        aws.String("Amazon EFS " + fsID),
		},
	}
	eniRes2 := resource.Resource{
		ID: "eni-mount-az2",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-mount-az2"),
			Description:        aws.String("Amazon EFS " + fsID),
		},
	}
	eniOther := resource.Resource{
		ID: "eni-unrelated",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-unrelated"),
			Description:        aws.String("Amazon EFS fs-0000000000000000"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes1, eniRes2, eniOther}},
	}
	checker := efsCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_EFS_ENI_NoMatch(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	eniRes := resource.Resource{
		ID: "eni-other",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-other"),
			Description:        aws.String("Amazon EFS fs-9999999999999999"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := efsCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ENI matching this fsID)", result.Count)
	}
}

func TestRelated_EFS_ENI_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := efsCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// --- checkEFSVPC (ENI → VpcId) ---

func TestRelated_EFS_VPC_Found(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	eniRes := resource.Resource{
		ID: "eni-mount-az1",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-mount-az1"),
			Description:        aws.String("Amazon EFS " + fsID),
			VpcId:              aws.String("vpc-efs001"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := efsCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpc-efs001" {
		t.Errorf("ResourceIDs[0] = %q, want vpc-efs001", result.ResourceIDs[0])
	}
}

func TestRelated_EFS_VPC_DeduplicatesVPCs(t *testing.T) {
	// Two mount-target ENIs in the same VPC should report a single VPC.
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	makeENI := func(id, vpcID string) resource.Resource {
		return resource.Resource{
			ID: id,
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String(id),
				Description:        aws.String("Amazon EFS " + fsID),
				VpcId:              aws.String(vpcID),
			},
		}
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			makeENI("eni-az1", "vpc-efs001"),
			makeENI("eni-az2", "vpc-efs001"),
		}},
	}
	checker := efsCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count)
	}
}

func TestRelated_EFS_VPC_NoENIMatch(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	source := resource.Resource{ID: fsID}
	eniRes := resource.Resource{
		ID: "eni-other",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-other"),
			Description:        aws.String("Amazon EFS fs-9999999999999999"),
			VpcId:              aws.String("vpc-other"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := efsCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ENI for this fsID)", result.Count)
	}
}

func TestRelated_EFS_VPC_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := efsCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}
