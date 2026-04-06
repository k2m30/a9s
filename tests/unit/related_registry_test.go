package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ═══════════════════════════════════════════════════════════════════════════
// Related registry unit tests
// ═══════════════════════════════════════════════════════════════════════════

var testRelatedDefs = []resource.RelatedDef{
	{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
}

var testNavigableFields = []resource.NavigableField{
	{FieldPath: "VpcId", TargetType: "vpc"},
	{FieldPath: "SubnetId", TargetType: "subnet"},
}

// TestRegisterRelated_StoresAndRetrieves verifies that RelatedDef entries are
// stored by RegisterRelated and returned by GetRelated, and that GetRelated
// returns nil for an unknown short name.
func TestRegisterRelated_StoresAndRetrieves(t *testing.T) {
	resource.RegisterRelated("test_reg", testRelatedDefs)
	defer resource.UnregisterRelated("test_reg")

	got := resource.GetRelated("test_reg")
	if got == nil {
		t.Fatal("GetRelated(\"test_reg\") returned nil, want 2 entries")
	}
	if len(got) != len(testRelatedDefs) {
		t.Fatalf("GetRelated(\"test_reg\") returned %d entries, want %d", len(got), len(testRelatedDefs))
	}
	for i, def := range got {
		if def.TargetType != testRelatedDefs[i].TargetType {
			t.Errorf("entry[%d].TargetType = %q, want %q", i, def.TargetType, testRelatedDefs[i].TargetType)
		}
		if def.DisplayName != testRelatedDefs[i].DisplayName {
			t.Errorf("entry[%d].DisplayName = %q, want %q", i, def.DisplayName, testRelatedDefs[i].DisplayName)
		}
	}

	unknown := resource.GetRelated("unknown")
	if unknown != nil {
		t.Errorf("GetRelated(\"unknown\") = %v, want nil", unknown)
	}
}

// TestRegisterRelated_ReplacesExisting verifies that registering the same
// short name twice replaces the previous definitions.
func TestRegisterRelated_ReplacesExisting(t *testing.T) {
	first := []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	}
	second := []resource.RelatedDef{
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: nil},
	}

	resource.RegisterRelated("test_reg", first)
	defer resource.UnregisterRelated("test_reg")

	resource.RegisterRelated("test_reg", second)

	got := resource.GetRelated("test_reg")
	if got == nil {
		t.Fatal("GetRelated(\"test_reg\") returned nil after re-registration, want 2 entries")
	}
	if len(got) != len(second) {
		t.Fatalf("GetRelated(\"test_reg\") returned %d entries after re-registration, want %d", len(got), len(second))
	}
	if got[0].TargetType != "asg" {
		t.Errorf("got[0].TargetType = %q, want \"asg\"", got[0].TargetType)
	}
	if got[1].TargetType != "elb" {
		t.Errorf("got[1].TargetType = %q, want \"elb\"", got[1].TargetType)
	}
}

// TestUnregisterRelated_RemovesEntry verifies that UnregisterRelated causes
// GetRelated to return nil for the removed short name.
func TestUnregisterRelated_RemovesEntry(t *testing.T) {
	resource.RegisterRelated("test_reg", testRelatedDefs)
	resource.UnregisterRelated("test_reg")

	got := resource.GetRelated("test_reg")
	if got != nil {
		t.Errorf("GetRelated(\"test_reg\") = %v after unregister, want nil", got)
	}
}

// TestRegisterNavigableFields_StoresAndRetrieves verifies that NavigableField
// entries are stored by RegisterNavigableFields and returned by
// GetNavigableFields, and that GetNavigableFields returns nil for an unknown
// short name.
func TestRegisterNavigableFields_StoresAndRetrieves(t *testing.T) {
	resource.RegisterNavigableFields("test_reg", testNavigableFields)
	defer resource.UnregisterNavigableFields("test_reg")

	got := resource.GetNavigableFields("test_reg")
	if got == nil {
		t.Fatal("GetNavigableFields(\"test_reg\") returned nil, want 2 entries")
	}
	if len(got) != len(testNavigableFields) {
		t.Fatalf("GetNavigableFields(\"test_reg\") returned %d entries, want %d", len(got), len(testNavigableFields))
	}
	for i, nf := range got {
		if nf.FieldPath != testNavigableFields[i].FieldPath {
			t.Errorf("entry[%d].FieldPath = %q, want %q", i, nf.FieldPath, testNavigableFields[i].FieldPath)
		}
		if nf.TargetType != testNavigableFields[i].TargetType {
			t.Errorf("entry[%d].TargetType = %q, want %q", i, nf.TargetType, testNavigableFields[i].TargetType)
		}
	}

	unknown := resource.GetNavigableFields("unknown")
	if unknown != nil {
		t.Errorf("GetNavigableFields(\"unknown\") = %v, want nil", unknown)
	}
}

// TestUnregisterNavigableFields_RemovesEntry verifies that
// UnregisterNavigableFields causes GetNavigableFields to return nil for the
// removed short name.
func TestUnregisterNavigableFields_RemovesEntry(t *testing.T) {
	resource.RegisterNavigableFields("test_reg", testNavigableFields)
	resource.UnregisterNavigableFields("test_reg")

	got := resource.GetNavigableFields("test_reg")
	if got != nil {
		t.Errorf("GetNavigableFields(\"test_reg\") = %v after unregister, want nil", got)
	}
}

// TestRegisterRelatedDemo_StoresAndRetrieves verifies that a demo checker is
// stored by RegisterRelatedDemo and returned by GetRelatedDemo, and that
// GetRelatedDemo returns nil for an unknown short name.
func TestRegisterRelatedDemo_StoresAndRetrieves(t *testing.T) {
	checker := func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 2, ResourceIDs: []string{"tg-aaa", "tg-bbb"}, Err: nil},
		}
	}

	// Use a unique key so this test does not collide with other tests that
	// register demo checkers for "ec2". Overwrite with nil on cleanup to reset
	// the entry (nil func is a valid map value and GetRelatedDemo will return nil).
	resource.RegisterRelatedDemo("ec2_demo_test", checker)
	defer resource.RegisterRelatedDemo("ec2_demo_test", nil)

	got := resource.GetRelatedDemo("ec2_demo_test")
	if got == nil {
		t.Fatal("GetRelatedDemo(\"ec2_demo_test\") returned nil, want non-nil checker")
	}

	results := got(resource.Resource{ID: "i-test", Name: "test-instance"})
	if len(results) != 1 {
		t.Fatalf("demo checker returned %d results, want 1", len(results))
	}
	if results[0].TargetType != "tg" {
		t.Errorf("results[0].TargetType = %q, want \"tg\"", results[0].TargetType)
	}
	if results[0].Count != 2 {
		t.Errorf("results[0].Count = %d, want 2", results[0].Count)
	}

	unknown := resource.GetRelatedDemo("unknown")
	if unknown != nil {
		t.Errorf("GetRelatedDemo(\"unknown\") = %v, want nil", unknown)
	}
}

// TestRelatedDef_NilChecker verifies that a RelatedDef with a nil Checker can
// be stored and retrieved without panicking.
func TestRelatedDef_NilChecker(t *testing.T) {
	defs := []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPCs", Checker: nil},
	}

	resource.RegisterRelated("ec2_nil_checker", defs)
	defer resource.UnregisterRelated("ec2_nil_checker")

	got := resource.GetRelated("ec2_nil_checker")
	if got == nil {
		t.Fatal("GetRelated(\"ec2_nil_checker\") returned nil, want 1 entry")
	}
	if len(got) != 1 {
		t.Fatalf("GetRelated(\"ec2_nil_checker\") returned %d entries, want 1", len(got))
	}
	if got[0].Checker != nil {
		t.Error("expected Checker to be nil, got non-nil")
	}

	// Verify that calling the nil checker does not panic (it must not be invoked
	// directly here — the test only verifies storage/retrieval roundtrip).
	if got[0].TargetType != "vpc" {
		t.Errorf("TargetType = %q, want \"vpc\"", got[0].TargetType)
	}
}

func TestRelated_ACM_Registered(t *testing.T) {
	defs := resource.GetRelated("acm")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for acm")
	}

	expected := []string{"elb", "cf", "apigw", "r53"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_Alarm_Registered(t *testing.T) {
	defs := resource.GetRelated("alarm")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for alarm")
	}

	expected := []string{"sns", "asg"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}

	// Verify sns has a non-nil checker, asg is a stub
	for _, def := range defs {
		switch def.TargetType {
		case "sns":
			if def.Checker == nil {
				t.Error("alarm sns: Checker should not be nil")
			}
		case "asg":
			if def.Checker != nil {
				t.Error("alarm asg: Checker should be nil (stub)")
			}
		}
	}
}

func TestRelated_AMI_Registered(t *testing.T) {
	defs := resource.GetRelated("ami")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ami")
	}

	expected := []string{"ec2", "ebs-snap", "asg"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}

	// ec2 and ebs-snap should have non-nil checkers, asg is a stub
	for _, def := range defs {
		switch def.TargetType {
		case "ec2", "ebs-snap":
			if def.Checker == nil {
				t.Errorf("ami %s: Checker should not be nil", def.TargetType)
			}
		case "asg":
			if def.Checker != nil {
				t.Error("ami asg: Checker should be nil (stub)")
			}
		}
	}
}

func TestRelated_APIGW_Registered(t *testing.T) {
	defs := resource.GetRelated("apigw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for apigw")
	}

	expected := []string{"lambda", "logs", "waf"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_Athena_Registered(t *testing.T) {
	defs := resource.GetRelated("athena")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for athena")
	}

	expected := []string{"s3", "kms"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_Backup_Registered(t *testing.T) {
	defs := resource.GetRelated("backup")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for backup")
	}

	expected := []string{"role"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_ASG_Registered(t *testing.T) {
	defs := resource.GetRelated("asg")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for asg")
	}

	expected := []string{"ec2", "tg", "subnet", "alarm", "ng"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_CB_Registered(t *testing.T) {
	defs := resource.GetRelated("cb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cb")
	}

	expected := []string{"logs", "role", "pipeline"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_CF_Registered(t *testing.T) {
	defs := resource.GetRelated("cf")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cf")
	}

	expected := []string{"s3", "elb", "waf", "acm", "r53"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_CFN_Registered(t *testing.T) {
	defs := resource.GetRelated("cfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cfn")
	}

	expected := []string{"role"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_Codeartifact_Registered(t *testing.T) {
	defs := resource.GetRelated("codeartifact")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for codeartifact")
	}

	expected := []string{"cb"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_CtEvents_Registered(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ct-events")
	}

	expected := []string{"role", "iam-user"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_DBC_Registered(t *testing.T) {
	defs := resource.GetRelated("dbc")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbc")
	}

	expected := []string{"sg", "alarm", "secrets", "logs"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", exp)
		}
	}
}

func TestRelated_DBI_Registered(t *testing.T) {
	defs := resource.GetRelated("dbi")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbi")
	}
	if len(defs) < 6 {
		t.Errorf("expected at least 6 related defs for dbi, got %d", len(defs))
	}
}

func TestRelated_DDB_Registered(t *testing.T) {
	defs := resource.GetRelated("ddb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ddb")
	}
	if len(defs) < 3 {
		t.Errorf("expected at least 3 related defs for ddb, got %d", len(defs))
	}
}

func TestRelated_DocdbSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("docdb-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for docdb-snap")
	}
	if len(defs) < 2 {
		t.Errorf("expected at least 2 related defs for docdb-snap, got %d", len(defs))
	}
}

func TestRelated_EbRule_Registered(t *testing.T) {
	defs := resource.GetRelated("eb-rule")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eb-rule")
	}
}

func TestRelated_EBS_Registered(t *testing.T) {
	defs := resource.GetRelated("ebs")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ebs")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ec2":      {"EC2 Instance", true},
		"ebs-snap": {"EBS Snapshots", true},
		"kms":      {"KMS Key", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ebs %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ebs %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ebs %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_EBSSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("ebs-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ebs-snap")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ami": {"AMIs", true},
		"ebs": {"EBS Volume", true},
		"ec2": {"EC2 Instance", true},
		"kms": {"KMS Key", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ebs-snap %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ebs-snap %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ebs-snap %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ebs-snap", target)
		}
	}
}

func TestRelated_EC2_Registered(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ec2")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"tg":        {"Target Groups", true},
		"asg":       {"Auto Scaling Groups", true},
		"alarm":     {"CloudWatch Alarms", true},
		"ng":        {"EKS Node Groups", true},
		"cfn":       {"CloudFormation Stacks", true},
		"eip":       {"Elastic IPs", true},
		"ebs":       {"EBS Volumes", true},
		"ebs-snap":  {"EBS Snapshots", true},
		"ct-events": {"CloudTrail Events", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ec2 %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ec2 %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ec2 %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ec2", target)
		}
	}
}

func TestRelated_ECR_Registered(t *testing.T) {
	defs := resource.GetRelated("ecr")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ecr")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"lambda": {"Lambda Functions", true},
		"cb":     {"CodeBuild Projects", true},
		"cfn":    {"CloudFormation Stacks", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ecr %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ecr %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ecr %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ecr", target)
		}
	}
}

func TestRelated_ECS_Registered(t *testing.T) {
	defs := resource.GetRelated("ecs")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ecs")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ecs-svc": {"ECS Services", true},
		"alarm":   {"CloudWatch Alarms", true},
		"cfn":     {"CloudFormation Stacks", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ecs %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ecs %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ecs %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ecs", target)
		}
	}
}

func TestRelated_ECSSvc_Registered(t *testing.T) {
	defs := resource.GetRelated("ecs-svc")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ecs-svc")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ecs":   {"ECS Clusters", true},
		"tg":    {"Target Groups", true},
		"alarm": {"CloudWatch Alarms", true},
		"cfn":   {"CloudFormation Stacks", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ecs-svc %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ecs-svc %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ecs-svc %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ecs-svc", target)
		}
	}
}

func TestRelated_ECSTask_Registered(t *testing.T) {
	defs := resource.GetRelated("ecs-task")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ecs-task")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ecs-svc": {"ECS Services", true},
		"ecs":     {"ECS Clusters", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ecs-task %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ecs-task %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ecs-task %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ecs-task", target)
		}
	}
}

func TestRelated_EFS_Registered(t *testing.T) {
	defs := resource.GetRelated("efs")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for efs")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"kms":    {"KMS Keys", true},
		"cfn":    {"CloudFormation Stacks", true},
		"lambda": {"Lambda Functions", false},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("efs %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("efs %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("efs %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for efs", target)
		}
	}
}

func TestRelated_EIP_Registered(t *testing.T) {
	defs := resource.GetRelated("eip")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eip")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ec2": {"EC2 Instances", true},
		"eni": {"Network Interfaces", true},
		"nat": {"NAT Gateways", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("eip %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("eip %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("eip %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for eip", target)
		}
	}
}

func TestRelated_EKS_Registered(t *testing.T) {
	defs := resource.GetRelated("eks")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eks")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"ng":    {"Node Groups", true},
		"alarm": {"CloudWatch Alarms", true},
		"cfn":   {"CloudFormation Stacks", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("eks %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("eks %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("eks %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for eks", target)
		}
	}
}

func TestRelated_ELB_Registered(t *testing.T) {
	defs := resource.GetRelated("elb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for elb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"tg":    {"Target Groups", true},
		"alarm": {"CW Alarms", true},
		"cfn":   {"CloudFormation", false},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("elb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("elb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("elb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for elb", target)
		}
	}
}

func TestRelated_ENI_Registered(t *testing.T) {
	defs := resource.GetRelated("eni")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eni")
	}

	expected := []string{"ec2", "sg", "eip"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				if def.Checker == nil {
					t.Errorf("eni %q: Checker should not be nil", exp)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for eni", exp)
		}
	}
}

func TestRelated_Glue_Registered(t *testing.T) {
	defs := resource.GetRelated("glue")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for glue")
	}

	expected := []string{"role", "alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for glue", exp)
		}
	}
}

func TestRelated_IAMGroup_Registered(t *testing.T) {
	defs := resource.GetRelated("iam-group")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-group")
	}

	expected := []string{"iam-user", "policy"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for iam-group", exp)
		}
	}
}

func TestRelated_IAMUser_Registered(t *testing.T) {
	defs := resource.GetRelated("iam-user")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-user")
	}

	expected := []string{"iam-group", "policy"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for iam-user", exp)
		}
	}
}

func TestRelated_IGW_Registered(t *testing.T) {
	defs := resource.GetRelated("igw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for igw")
	}

	expected := []string{"vpc", "rtb"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for igw", exp)
		}
	}
}

func TestRelated_Kinesis_Registered(t *testing.T) {
	defs := resource.GetRelated("kinesis")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for kinesis")
	}

	expected := []string{"lambda", "alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for kinesis", exp)
		}
	}
}

func TestRelated_KMS_Registered(t *testing.T) {
	defs := resource.GetRelated("kms")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for kms")
	}

	expected := []string{"ebs", "dbi", "secrets"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for kms", exp)
		}
	}

	// All three checkers must be non-nil (no stubs).
	for _, def := range defs {
		switch def.TargetType {
		case "ebs", "dbi", "secrets":
			if def.Checker == nil {
				t.Errorf("kms %q: Checker should not be nil", def.TargetType)
			}
		}
	}
}

func TestRelated_Lambda_Registered(t *testing.T) {
	defs := resource.GetRelated("lambda")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for lambda")
	}

	expected := []string{"role", "alarm", "sqs", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for lambda", exp)
		}
	}

	// role and alarm must have non-nil checkers; sqs and cfn are stubs (nil).
	for _, def := range defs {
		switch def.TargetType {
		case "role", "alarm":
			if def.Checker == nil {
				t.Errorf("lambda %q: Checker should not be nil", def.TargetType)
			}
		case "sqs", "cfn":
			if def.Checker != nil {
				t.Errorf("lambda %q: Checker should be nil (stub)", def.TargetType)
			}
		}
	}
}

func TestRelated_Logs_Registered(t *testing.T) {
	defs := resource.GetRelated("logs")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for logs")
	}

	expected := []string{"lambda", "alarm"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for logs", exp)
		}
	}

	// lambda and alarm must have non-nil checkers.
	for _, def := range defs {
		switch def.TargetType {
		case "lambda", "alarm":
			if def.Checker == nil {
				t.Errorf("logs %q: Checker should not be nil", def.TargetType)
			}
		}
	}
}

func TestRelated_MSK_Registered(t *testing.T) {
	defs := resource.GetRelated("msk")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for msk")
	}

	expected := []string{"lambda", "alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for msk", exp)
		}
	}
}

func TestRelated_NAT_Registered(t *testing.T) {
	defs := resource.GetRelated("nat")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for nat")
	}

	expected := []string{"vpc", "subnet", "rtb"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for nat", exp)
		}
	}
}

// ─── compile-time reference to context so the import is used ────────────────
// RelatedChecker requires context.Context; verify the type is usable.
var _ resource.RelatedChecker = func(
	_ context.Context,
	_ any,
	_ resource.Resource,
	_ resource.ResourceCache,
) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{}
}
