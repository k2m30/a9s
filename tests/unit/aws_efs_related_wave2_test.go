// aws_efs_related_wave2_test.go — coverage wave 2 for efs_related.go checkers.
// Covers: checkEFSSG (0%), checkEFSSubnet (0%).
// Both checkers scan the ENI cache and filter by Description containing fsID.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func efsENIWithSGAndSubnet(fsID, eniID, sgID, subnetID string) resource.Resource {
	return resource.Resource{
		ID:   eniID,
		Name: eniID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
			// Description for EFS mount-target ENI contains the filesystem ID.
			Description: aws.String("EFS mount target for " + fsID),
			SubnetId:    aws.String(subnetID),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(sgID)},
			},
		},
	}
}

func efsSourceResourceWithRaw(fsID string) resource.Resource {
	return resource.Resource{
		ID:   fsID,
		Name: fsID,
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String(fsID),
		},
	}
}

// ---------------------------------------------------------------------------
// checkEFSSG — Pattern C: ENI cache scan, Description contains fsID → sg IDs
// ---------------------------------------------------------------------------

func TestRelated_EFS_SG_MatchesSGsFromMountTargetENI(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const sgID = "sg-0aaa111111111111a"

	eniRes := efsENIWithSGAndSubnet(fsID, "eni-0a1b2c3d4e5f60001", sgID, "subnet-0aaa111111111111a")
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	checker := efsCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != sgID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, sgID)
	}
}

func TestRelated_EFS_SG_NoMatchWhenENIBelongsToDifferentFS(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const otherFSID = "fs-0zzzzzzzzzzzzzzzz"

	// ENI description references a different FS.
	eniRes := efsENIWithSGAndSubnet(otherFSID, "eni-0zzz", "sg-0zzz", "subnet-0zzz")
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	checker := efsCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ENI for different FS)", result.Count)
	}
}

// Edge: two mount-target ENIs for same FS, same SG — SG deduplicated in result.
func TestRelated_EFS_SG_DeduplicatesSGsAcrossMultipleENIs(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const sharedSGID = "sg-0shared111111111"

	eniRes1 := efsENIWithSGAndSubnet(fsID, "eni-0az1", sharedSGID, "subnet-0az1")
	eniRes2 := efsENIWithSGAndSubnet(fsID, "eni-0az2", sharedSGID, "subnet-0az2")

	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{
			Resources: []resource.Resource{eniRes1, eniRes2},
		},
	}

	checker := efsCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (shared SG deduplicated)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != sharedSGID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, sharedSGID)
	}
}

// ---------------------------------------------------------------------------
// checkEFSSubnet — Pattern C: ENI cache scan, Description contains fsID → subnet IDs
// ---------------------------------------------------------------------------

func TestRelated_EFS_Subnet_MatchesSubnetsFromMountTargetENI(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60002"
	const subnetID = "subnet-0aaa111111111111a"

	eniRes := efsENIWithSGAndSubnet(fsID, "eni-0a1b2c3d4e5f60002", "sg-0aaa", subnetID)
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	checker := efsCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != subnetID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, subnetID)
	}
}

func TestRelated_EFS_Subnet_NoMatchWhenENIBelongsToDifferentFS(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60002"
	const otherFSID = "fs-0zzzzzzzzzzzzzzzz"

	eniRes := efsENIWithSGAndSubnet(otherFSID, "eni-0zzz", "sg-0zzz", "subnet-0zzz")
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}

	checker := efsCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ENI for different FS)", result.Count)
	}
}

// Edge: multiple ENIs in different subnets for the same FS — all subnets returned.
func TestRelated_EFS_Subnet_MultipleSubnetsFromDifferentAZENIs(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60002"
	const subnet1 = "subnet-0az1111111111a"
	const subnet2 = "subnet-0az2222222222b"

	eniRes1 := efsENIWithSGAndSubnet(fsID, "eni-0az1", "sg-0shared", subnet1)
	eniRes2 := efsENIWithSGAndSubnet(fsID, "eni-0az2", "sg-0shared", subnet2)

	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{
			Resources: []resource.Resource{eniRes1, eniRes2},
		},
	}

	checker := efsCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, efsSourceResourceWithRaw(fsID), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2; ResourceIDs: %v", result.Count, result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{subnet1, subnet2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}
