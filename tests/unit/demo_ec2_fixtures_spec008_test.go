package unit

// demo_ec2_fixtures_spec008_test.go — Spec-008: EC2 fixture field completeness.
//
// BUG: makeEC2Instance() in internal/demo/fixtures_compute.go does NOT set:
//   - ImageId *string            — RawStruct.ImageId is nil
//   - KeyName *string            — RawStruct.KeyName is nil
//   - Architecture               — RawStruct.Architecture is empty string
//   - Placement *ec2types.Placement — RawStruct.Placement is nil (no AvailabilityZone)
//   - SecurityGroups []ec2types.GroupIdentifier — RawStruct.SecurityGroups is nil/empty
//
// Tests marked "FAILS NOW" will fail against the current fixtures.
// Tests marked "PASSES NOW" are cross-reference regression guards.
//
// All tests COMPILE NOW because they rely only on existing symbols
// (demo.GetResources, ec2types.Instance).

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws" // trigger aws init() registrations
	demo "github.com/k2m30/a9s/v3/internal/demo"
)

// getEC2Fixtures returns EC2 demo fixtures or fatals the test.
func getEC2Fixtures(t *testing.T) []ec2types.Instance {
	t.Helper()
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false; EC2 fixtures must be registered")
	}
	if len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ec2\") returned empty slice; expected at least 10 fixtures")
	}
	instances := make([]ec2types.Instance, 0, len(resources))
	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("ec2 resource[%d] (ID=%s): RawStruct is nil; all EC2 fixtures must have RawStruct", i, r.ID)
			continue
		}
		inst, ok := r.RawStruct.(ec2types.Instance)
		if !ok {
			t.Errorf("ec2 resource[%d] (ID=%s): RawStruct is %T, want ec2types.Instance", i, r.ID, r.RawStruct)
			continue
		}
		instances = append(instances, inst)
	}
	return instances
}

// ---------------------------------------------------------------------------
// FAILS NOW: fields not populated in makeEC2Instance
// ---------------------------------------------------------------------------

// TestDemo_008_EC2_RawStructHasImageId verifies that every EC2 fixture has a
// non-empty ImageId in its RawStruct.
//
// FAILS NOW: makeEC2Instance() does not set ImageId.
func TestDemo_008_EC2_RawStructHasImageId(t *testing.T) {
	instances := getEC2Fixtures(t)
	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if inst.ImageId == nil {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.ImageId is nil; must be populated (e.g. \"ami-0123456789abcdef0\")", i, id)
			continue
		}
		if *inst.ImageId == "" {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.ImageId is empty string", i, id)
		}
	}
}

// TestDemo_008_EC2_RawStructHasKeyName verifies that every EC2 fixture has a
// non-empty KeyName in its RawStruct.
//
// FAILS NOW: makeEC2Instance() does not set KeyName.
func TestDemo_008_EC2_RawStructHasKeyName(t *testing.T) {
	instances := getEC2Fixtures(t)
	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if inst.KeyName == nil {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.KeyName is nil; must be populated (e.g. \"acme-prod-key\")", i, id)
			continue
		}
		if *inst.KeyName == "" {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.KeyName is empty string", i, id)
		}
	}
}

// TestDemo_008_EC2_RawStructHasArchitecture verifies that every EC2 fixture has
// a non-empty Architecture value in its RawStruct.
//
// FAILS NOW: makeEC2Instance() does not set Architecture.
func TestDemo_008_EC2_RawStructHasArchitecture(t *testing.T) {
	instances := getEC2Fixtures(t)
	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if inst.Architecture == "" {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.Architecture is empty; must be set (e.g. \"x86_64\" or \"arm64\")", i, id)
		}
	}
}

// TestDemo_008_EC2_RawStructHasPlacement verifies that every EC2 fixture has a
// non-nil Placement with a non-empty AvailabilityZone in its RawStruct.
//
// FAILS NOW: makeEC2Instance() does not set Placement.
func TestDemo_008_EC2_RawStructHasPlacement(t *testing.T) {
	instances := getEC2Fixtures(t)
	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if inst.Placement == nil {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.Placement is nil; must be set with AvailabilityZone", i, id)
			continue
		}
		if inst.Placement.AvailabilityZone == nil || *inst.Placement.AvailabilityZone == "" {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.Placement.AvailabilityZone is empty; must be set (e.g. \"us-east-1a\")", i, id)
		}
	}
}

// TestDemo_008_EC2_RawStructHasSecurityGroups verifies that every EC2 fixture
// has at least one SecurityGroup with a non-empty GroupId in its RawStruct.
//
// FAILS NOW: makeEC2Instance() does not set SecurityGroups.
func TestDemo_008_EC2_RawStructHasSecurityGroups(t *testing.T) {
	instances := getEC2Fixtures(t)
	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if len(inst.SecurityGroups) == 0 {
			t.Errorf("ec2 instance[%d] (ID=%s): RawStruct.SecurityGroups is empty; must have at least one entry", i, id)
			continue
		}
		for j, sg := range inst.SecurityGroups {
			if sg.GroupId == nil || *sg.GroupId == "" {
				t.Errorf("ec2 instance[%d] (ID=%s): SecurityGroups[%d].GroupId is nil/empty", i, id, j)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// FAILS NOW: SecurityGroups not set, so cross-ref cannot be verified
// ---------------------------------------------------------------------------

// TestDemo_008_EC2_SecurityGroup_CrossRef_SGExists verifies that the GroupId
// for every EC2 fixture's SecurityGroup matches an SG fixture in the demo data.
//
// FAILS NOW: SecurityGroups is nil/empty in EC2 fixtures.
func TestDemo_008_EC2_SecurityGroup_CrossRef_SGExists(t *testing.T) {
	instances := getEC2Fixtures(t)

	sgResources, ok := demo.GetResources("sg")
	if !ok {
		t.Fatal("demo.GetResources(\"sg\") returned ok=false; SG fixtures must be registered")
	}
	sgIDs := make(map[string]bool, len(sgResources))
	for _, sg := range sgResources {
		sgIDs[sg.ID] = true
	}

	for i, inst := range instances {
		id := "<unknown>"
		if inst.InstanceId != nil {
			id = *inst.InstanceId
		}
		if len(inst.SecurityGroups) == 0 {
			// This sub-failure is also caught by TestDemo_008_EC2_RawStructHasSecurityGroups
			t.Errorf("ec2 instance[%d] (ID=%s): SecurityGroups is empty; cannot verify cross-reference", i, id)
			continue
		}
		for j, sg := range inst.SecurityGroups {
			if sg.GroupId == nil {
				continue
			}
			if !sgIDs[*sg.GroupId] {
				t.Errorf("ec2 instance[%d] (ID=%s): SecurityGroups[%d].GroupId=%q does not match any SG fixture ID", i, id, j, *sg.GroupId)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// PASSES NOW: VpcId and SubnetId are already populated in makeEC2Instance
// These are regression guards — they must continue passing after the fix.
// ---------------------------------------------------------------------------

// TestDemo_008_EC2_VpcId_CrossRef_VpcExists verifies that every EC2 fixture's
// VpcId matches a VPC fixture in the demo data.
//
// PASSES NOW: makeEC2Instance sets VpcId = prodVPCID which exists in vpc fixtures.
func TestDemo_008_EC2_VpcId_CrossRef_VpcExists(t *testing.T) {
	ec2Resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false")
	}
	vpcResources, ok := demo.GetResources("vpc")
	if !ok {
		t.Fatal("demo.GetResources(\"vpc\") returned ok=false; VPC fixtures must be registered")
	}
	vpcIDs := make(map[string]bool, len(vpcResources))
	for _, vpc := range vpcResources {
		vpcIDs[vpc.ID] = true
	}

	for i, r := range ec2Resources {
		vpcID, ok := r.Fields["vpc_id"]
		if !ok || vpcID == "" {
			// Some terminated instances may have no VPC — skip rather than fail
			continue
		}
		if !vpcIDs[vpcID] {
			t.Errorf("ec2 resource[%d] (ID=%s): Fields[\"vpc_id\"]=%q does not match any VPC fixture ID", i, r.ID, vpcID)
		}
	}
}

// TestDemo_008_EC2_SubnetId_CrossRef_SubnetExists verifies that every EC2
// fixture's SubnetId matches a subnet fixture in the demo data.
//
// PASSES NOW: makeEC2Instance sets SubnetId to one of the prodPublicSubnet* constants
// which exist in subnet fixtures.
func TestDemo_008_EC2_SubnetId_CrossRef_SubnetExists(t *testing.T) {
	ec2Resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false")
	}
	subnetResources, ok := demo.GetResources("subnet")
	if !ok {
		t.Fatal("demo.GetResources(\"subnet\") returned ok=false; subnet fixtures must be registered")
	}
	subnetIDs := make(map[string]bool, len(subnetResources))
	for _, sn := range subnetResources {
		subnetIDs[sn.ID] = true
	}

	for i, r := range ec2Resources {
		subnetID, ok := r.Fields["subnet_id"]
		if !ok || subnetID == "" {
			// Some instances (e.g. terminated) may have empty subnet — skip
			continue
		}
		if !subnetIDs[subnetID] {
			t.Errorf("ec2 resource[%d] (ID=%s): Fields[\"subnet_id\"]=%q does not match any subnet fixture ID", i, r.ID, subnetID)
		}
	}
}

// TestDemo_008_EC2_AllFixturesHaveRawStruct verifies that all EC2 fixtures have
// a non-nil RawStruct. This is a baseline requirement for YAML and detail views.
//
// PASSES NOW — regression guard.
func TestDemo_008_EC2_AllFixturesHaveRawStruct(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false")
	}
	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("ec2 resource[%d] (ID=%s): RawStruct is nil; all EC2 fixtures must have RawStruct", i, r.ID)
		}
		_, isInstance := r.RawStruct.(ec2types.Instance)
		if r.RawStruct != nil && !isInstance {
			t.Errorf("ec2 resource[%d] (ID=%s): RawStruct is %T, want ec2types.Instance", i, r.ID, r.RawStruct)
		}
	}
}

// TestDemo_008_EC2_MinimumFixtureCount verifies there are at least 10 EC2
// fixtures available (the spec calls for 25).
//
// PASSES NOW — regression guard.
func TestDemo_008_EC2_MinimumFixtureCount(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false")
	}
	if len(resources) < 10 {
		t.Errorf("expected at least 10 EC2 fixtures, got %d", len(resources))
	}
}
