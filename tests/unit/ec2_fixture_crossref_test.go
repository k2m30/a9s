package unit

// ec2_fixture_crossref_test.go — EC2 fixture cross-reference tests.
//
// Verifies that every ResourceID referenced by EC2 fixtures (via RawStruct fields)
// and by the EC2 related-demo checker (fixtures_related.go) resolves to an actual
// fixture in demo.GetResources(targetType).
//
// FAILS NOW tests document known fixture mismatches so the coder knows exactly what
// to fix. PASSES NOW tests are regression guards.

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"  // trigger aws init() registrations
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// resourceIDSet returns a set of IDs from a slice of resources.
func resourceIDSet(t *testing.T, resourceType string) map[string]bool {
	t.Helper()
	resources, ok := demo.GetResources(resourceType)
	if !ok {
		t.Fatalf("demo.GetResources(%q) returned ok=false; %q fixtures must be registered", resourceType, resourceType)
	}
	ids := make(map[string]bool, len(resources))
	for _, r := range resources {
		ids[r.ID] = true
	}
	return ids
}

// getFirstEC2Instance returns the ec2types.Instance RawStruct for the first EC2
// fixture (i-0a1b2c3d4e5f60001) or fatals the test.
func getFirstEC2Instance(t *testing.T) ec2types.Instance {
	t.Helper()
	const firstID = "i-0a1b2c3d4e5f60001"
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("demo.GetResources(\"ec2\") returned ok=false")
	}
	for _, r := range resources {
		if r.ID == firstID {
			if r.RawStruct == nil {
				t.Fatalf("ec2 resource %s: RawStruct is nil", firstID)
			}
			inst, ok := r.RawStruct.(ec2types.Instance)
			if !ok {
				t.Fatalf("ec2 resource %s: RawStruct is %T, want ec2types.Instance", firstID, r.RawStruct)
			}
			return inst
		}
	}
	t.Fatalf("EC2 fixture with ID %q not found", firstID)
	return ec2types.Instance{}
}

// getEC2RelatedResults calls the EC2 related-demo checker with a stub resource
// and returns the results indexed by TargetType. Fatals if the checker is not
// registered.
func getEC2RelatedResults(t *testing.T) map[string]resource.RelatedCheckResult {
	t.Helper()
	checker := resource.GetRelatedDemo("ec2")
	if checker == nil {
		t.Fatal("resource.GetRelatedDemo(\"ec2\") returned nil; EC2 related-demo checker must be registered")
	}
	stub := resource.Resource{ID: "i-0a1b2c3d4e5f60001", Name: "web-prod-01"}
	results := checker(stub)
	index := make(map[string]resource.RelatedCheckResult, len(results))
	for _, r := range results {
		index[r.TargetType] = r
	}
	return index
}

// ---------------------------------------------------------------------------
// Test 1 — EC2 → AMI cross-reference
// FAILS NOW: makeEC2Instance() does not set ImageId on RawStruct.
// ---------------------------------------------------------------------------

// TestFixture_EC2_ImageId_MatchesAMIFixture verifies that the first EC2 instance's
// RawStruct.ImageId matches an AMI fixture in demo.GetResources("ami").
//
// FAILS NOW: RawStruct.ImageId is nil because makeEC2Instance does not set it.
func TestFixture_EC2_ImageId_MatchesAMIFixture(t *testing.T) {
	inst := getFirstEC2Instance(t)
	if inst.ImageId == nil {
		t.Fatal("RawStruct.ImageId is nil on ec2 fixture i-0a1b2c3d4e5f60001; must be populated to verify AMI cross-reference")
	}
	if *inst.ImageId == "" {
		t.Fatal("RawStruct.ImageId is empty string on ec2 fixture i-0a1b2c3d4e5f60001")
	}

	amiIDs := resourceIDSet(t, "ami")
	if !amiIDs[*inst.ImageId] {
		t.Errorf("ec2 fixture i-0a1b2c3d4e5f60001: RawStruct.ImageId=%q does not match any AMI fixture ID; available AMI IDs: %v",
			*inst.ImageId, amiIDList(amiIDs))
	}
}

// amiIDList returns AMI IDs as a sorted slice for readable error output.
func amiIDList(ids map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out
}

// ---------------------------------------------------------------------------
// Test 2 — Related TG ResourceIDs → TG fixtures
// FAILS NOW: related fixture uses ARN format; TG fixtures use name-based IDs.
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedTG_IDsMatchFixtures verifies that every ResourceID in the
// EC2 related-demo checker for TargetType "tg" matches a TG fixture.
//
// FAILS NOW: fixtures_related.go returns
//   "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/demo-web-tg/abc123"
// but TG fixtures use IDs like "acme-web-tg".
func TestFixture_EC2_RelatedTG_IDsMatchFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	result, ok := related["tg"]
	if !ok {
		t.Fatal("EC2 related checker has no entry for TargetType \"tg\"; must register at least one TG result")
	}
	if len(result.ResourceIDs) == 0 {
		t.Skip("EC2 related checker returned no ResourceIDs for \"tg\"; skipping cross-reference check")
	}

	tgIDs := resourceIDSet(t, "tg")
	for _, id := range result.ResourceIDs {
		if !tgIDs[id] {
			t.Errorf("EC2 related TG ResourceID=%q does not match any TG fixture ID; fix fixtures_related.go to use fixture IDs (e.g. \"acme-web-tg\")", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3 — Related ASG ResourceIDs → ASG fixtures
// FAILS NOW: related fixture uses "demo-web-asg"; ASG fixtures use "acme-web-prod-asg".
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedASG_IDsMatchFixtures verifies that every ResourceID in the
// EC2 related-demo checker for TargetType "asg" matches an ASG fixture.
//
// FAILS NOW: fixtures_related.go returns "demo-web-asg" but ASG fixtures use
// "acme-web-prod-asg".
func TestFixture_EC2_RelatedASG_IDsMatchFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	result, ok := related["asg"]
	if !ok {
		t.Fatal("EC2 related checker has no entry for TargetType \"asg\"; must register at least one ASG result")
	}
	if len(result.ResourceIDs) == 0 {
		t.Skip("EC2 related checker returned no ResourceIDs for \"asg\"; skipping cross-reference check")
	}

	asgIDs := resourceIDSet(t, "asg")
	for _, id := range result.ResourceIDs {
		if !asgIDs[id] {
			t.Errorf("EC2 related ASG ResourceID=%q does not match any ASG fixture ID; fix fixtures_related.go to use fixture IDs (e.g. \"acme-web-prod-asg\")", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4 — Related Alarm ResourceIDs → Alarm fixtures
// FAILS NOW: related uses "demo-ec2-cpu-high", "demo-ec2-status-check"; alarm
// fixtures use "api-high-error-rate", "rds-cpu-utilization", etc.
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedAlarm_IDsMatchFixtures verifies that every ResourceID in
// the EC2 related-demo checker for TargetType "alarm" matches an alarm fixture.
//
// FAILS NOW: fixtures_related.go returns "demo-ec2-cpu-high" and
// "demo-ec2-status-check" which do not exist in cloudwatch alarm fixtures.
func TestFixture_EC2_RelatedAlarm_IDsMatchFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	result, ok := related["alarm"]
	if !ok {
		t.Fatal("EC2 related checker has no entry for TargetType \"alarm\"; must register at least one alarm result")
	}
	if len(result.ResourceIDs) == 0 {
		t.Skip("EC2 related checker returned no ResourceIDs for \"alarm\"; skipping cross-reference check")
	}

	alarmIDs := resourceIDSet(t, "alarm")
	for _, id := range result.ResourceIDs {
		if !alarmIDs[id] {
			t.Errorf("EC2 related alarm ResourceID=%q does not match any alarm fixture ID; fix fixtures_related.go to use fixture IDs (e.g. \"api-high-error-rate\")", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5 — EC2 related checker registers EIP
// FAILS NOW: fixtures_related.go only registers tg, asg, alarm, cfn — no eip.
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedEIP_Registered asserts that the EC2 related-demo checker
// returns a result with TargetType "eip".
//
// FAILS NOW: fixtures_related.go does not register "eip" in the EC2 related results.
func TestFixture_EC2_RelatedEIP_Registered(t *testing.T) {
	related := getEC2RelatedResults(t)
	if _, ok := related["eip"]; !ok {
		t.Error("EC2 related checker has no entry for TargetType \"eip\"; must add EIP result to fixtures_related.go for EC2 navigation stories")
	}
}

// ---------------------------------------------------------------------------
// Test 6 — Related EIP ResourceIDs → EIP fixtures (if registered)
// FAILS NOW: EIP not registered (blocked by Test 5).
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedEIP_IDsMatchFixtures verifies that every ResourceID in the
// EC2 related-demo checker for TargetType "eip" matches an EIP fixture.
//
// FAILS NOW: "eip" is not registered in fixtures_related.go.
// After Test 5 is fixed and EIP is registered, this test verifies the IDs match.
func TestFixture_EC2_RelatedEIP_IDsMatchFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	result, ok := related["eip"]
	if !ok {
		t.Skip("EC2 related checker has no entry for TargetType \"eip\"; skipping ID cross-reference (see TestFixture_EC2_RelatedEIP_Registered)")
	}
	if len(result.ResourceIDs) == 0 {
		t.Skip("EC2 related checker returned no ResourceIDs for \"eip\"; skipping cross-reference check")
	}

	eipIDs := resourceIDSet(t, "eip")
	for _, id := range result.ResourceIDs {
		if !eipIDs[id] {
			t.Errorf("EC2 related EIP ResourceID=%q does not match any EIP fixture ID; use allocation IDs like \"eipalloc-0aaa111111111111a\"", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 7 — EC2 related checker registers EBS Snapshots
// FAILS NOW: fixtures_related.go does not register "ebs-snap".
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedEBSSnap_Registered asserts that the EC2 related-demo
// checker returns a result with TargetType "ebs-snap".
//
// FAILS NOW: fixtures_related.go does not register "ebs-snap" in the EC2 results.
func TestFixture_EC2_RelatedEBSSnap_Registered(t *testing.T) {
	related := getEC2RelatedResults(t)
	if _, ok := related["ebs-snap"]; !ok {
		t.Error("EC2 related checker has no entry for TargetType \"ebs-snap\"; must add EBS Snapshot result to fixtures_related.go for EC2 navigation stories")
	}
}

// ---------------------------------------------------------------------------
// Test 8 — Related EBS Snap ResourceIDs → Snapshot fixtures (if registered)
// FAILS NOW: ebs-snap not registered (blocked by Test 7).
// ---------------------------------------------------------------------------

// TestFixture_EC2_RelatedEBSSnap_IDsMatchFixtures verifies that every ResourceID in
// the EC2 related-demo checker for TargetType "ebs-snap" matches a snapshot fixture.
//
// FAILS NOW: "ebs-snap" is not registered in fixtures_related.go.
func TestFixture_EC2_RelatedEBSSnap_IDsMatchFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	result, ok := related["ebs-snap"]
	if !ok {
		t.Skip("EC2 related checker has no entry for TargetType \"ebs-snap\"; skipping ID cross-reference (see TestFixture_EC2_RelatedEBSSnap_Registered)")
	}
	if len(result.ResourceIDs) == 0 {
		t.Skip("EC2 related checker returned no ResourceIDs for \"ebs-snap\"; skipping cross-reference check")
	}

	snapIDs := resourceIDSet(t, "ebs-snap")
	for _, id := range result.ResourceIDs {
		if !snapIDs[id] {
			t.Errorf("EC2 related EBS Snap ResourceID=%q does not match any ebs-snap fixture ID; use snapshot IDs like \"snap-0a1b2c3d4e5f60001\"", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 9 — ALL EC2 related ResourceIDs resolve (comprehensive catch-all)
// FAILS NOW: tg, asg, alarm IDs do not match their fixtures.
// ---------------------------------------------------------------------------

// TestFixture_EC2_AllRelatedResourceIDs_ExistAsFixtures is a comprehensive check:
// for every TargetType returned by the EC2 related-demo checker, every ResourceID
// must resolve to a fixture in demo.GetResources(targetType).
//
// FAILS NOW: "tg", "asg", and "alarm" have ResourceIDs that do not exist in fixtures.
func TestFixture_EC2_AllRelatedResourceIDs_ExistAsFixtures(t *testing.T) {
	related := getEC2RelatedResults(t)
	if len(related) == 0 {
		t.Fatal("EC2 related checker returned zero results; expected at least tg, asg, alarm, cfn")
	}

	for targetType, result := range related {
		if len(result.ResourceIDs) == 0 {
			continue // Count=0 entries (e.g. cfn) have no IDs to check
		}
		targetIDs := resourceIDSet(t, targetType)
		for _, id := range result.ResourceIDs {
			if !targetIDs[id] {
				t.Errorf("EC2 related %q ResourceID=%q does not exist in demo.GetResources(%q); fixtures_related.go must use IDs that match fixture data",
					targetType, id, targetType)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test 10 — EC2 NavigableFields target types have at least one fixture
// PASSES NOW: vpc, subnet, ami fixtures all exist.
// (Regression guard — keeps passing after related-view infrastructure changes.)
// ---------------------------------------------------------------------------

// TestFixture_EC2_NavigableFieldTargets_HaveFixtures verifies that for every
// NavigableField registered for "ec2", at least one fixture exists for the target type.
//
// PASSES NOW: VpcId→"vpc", SubnetId→"subnet", ImageId→"ami" all have fixtures.
func TestFixture_EC2_NavigableFieldTargets_HaveFixtures(t *testing.T) {
	fields := resource.GetNavigableFields("ec2")
	if len(fields) == 0 {
		t.Fatal("resource.GetNavigableFields(\"ec2\") returned empty; navigable fields must be registered via aws init()")
	}

	for _, nf := range fields {
		resources, ok := demo.GetResources(nf.TargetType)
		if !ok {
			t.Errorf("NavigableField FieldPath=%q TargetType=%q: demo.GetResources(%q) returned ok=false; target type has no fixtures",
				nf.FieldPath, nf.TargetType, nf.TargetType)
			continue
		}
		if len(resources) == 0 {
			t.Errorf("NavigableField FieldPath=%q TargetType=%q: demo.GetResources(%q) returned empty slice; at least one fixture required",
				nf.FieldPath, nf.TargetType, nf.TargetType)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 11 — First EC2 instance SecurityGroups cross-reference SG fixtures
// FAILS NOW: makeEC2Instance() does not set SecurityGroups on RawStruct.
// ---------------------------------------------------------------------------

// TestFixture_EC2_FirstInstance_SecurityGroups_ResolveSGs verifies that the first
// EC2 fixture (i-0a1b2c3d4e5f60001) has SecurityGroups populated in RawStruct and
// that each GroupId matches an SG fixture.
//
// FAILS NOW: makeEC2Instance() does not set SecurityGroups; RawStruct.SecurityGroups
// is nil/empty.
func TestFixture_EC2_FirstInstance_SecurityGroups_ResolveSGs(t *testing.T) {
	inst := getFirstEC2Instance(t)

	if len(inst.SecurityGroups) == 0 {
		t.Fatal("ec2 fixture i-0a1b2c3d4e5f60001: RawStruct.SecurityGroups is empty; must have at least one SecurityGroup for cross-reference")
	}

	sgIDs := resourceIDSet(t, "sg")
	for j, sg := range inst.SecurityGroups {
		if sg.GroupId == nil || *sg.GroupId == "" {
			t.Errorf("ec2 fixture i-0a1b2c3d4e5f60001: SecurityGroups[%d].GroupId is nil/empty", j)
			continue
		}
		if !sgIDs[*sg.GroupId] {
			t.Errorf("ec2 fixture i-0a1b2c3d4e5f60001: SecurityGroups[%d].GroupId=%q does not match any SG fixture ID",
				j, *sg.GroupId)
		}
	}
}
