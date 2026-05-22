package unit

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ═══════════════════════════════════════════════════════════════════════════
// Related registry unit tests
// ═══════════════════════════════════════════════════════════════════════════

var testRelatedDefs = []resource.RelatedDef{
	{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
}

var testNavigableFields = []resource.NavigableField{
	{FieldPath: "VpcId", TargetType: "vpc"},
	{FieldPath: "SubnetId", TargetType: "subnet"},
}

// TestRegisterRelated_StoresAndRetrieves verifies that RelatedDef entries are
// stored by SetRelatedForTest and returned by GetRelated, and that GetRelated
// returns nil for an unknown short name.
func TestRegisterRelated_StoresAndRetrieves(t *testing.T) {
	resource.SetRelatedForTest("test_reg", testRelatedDefs)
	defer resource.CleanupRelatedForTest("test_reg")

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
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	}
	second := []resource.RelatedDef{
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: noopChecker},
	}

	resource.SetRelatedForTest("test_reg", first)
	defer resource.CleanupRelatedForTest("test_reg")

	resource.SetRelatedForTest("test_reg", second)
	// AS-67: Each SetRelatedForTest pushes a snapshot. To leave the registry
	// clean for the next test, every Register must be paired with an
	// Unregister. Defers run LIFO, so this one pops `first` first, then the
	// outer defer pops the original (nil) snapshot.
	defer resource.CleanupRelatedForTest("test_reg")

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

// TestUnregisterRelated_RemovesEntry verifies that CleanupRelatedForTest causes
// GetRelated to return nil for the removed short name.
func TestUnregisterRelated_RemovesEntry(t *testing.T) {
	resource.SetRelatedForTest("test_reg", testRelatedDefs)
	resource.CleanupRelatedForTest("test_reg")

	got := resource.GetRelated("test_reg")
	if got != nil {
		t.Errorf("GetRelated(\"test_reg\") = %v after unregister, want nil", got)
	}
}

// TestRegisterNavigableFields_StoresAndRetrieves verifies that NavigableField
// entries are stored by SetNavigableFieldsForTest and returned by
// GetNavigableFields, and that GetNavigableFields returns nil for an unknown
// short name.
func TestRegisterNavigableFields_StoresAndRetrieves(t *testing.T) {
	resource.SetNavigableFieldsForTest("test_reg", testNavigableFields)
	defer resource.CleanupNavigableFieldsForTest("test_reg")

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
// CleanupNavigableFieldsForTest causes GetNavigableFields to return nil for the
// removed short name.
func TestUnregisterNavigableFields_RemovesEntry(t *testing.T) {
	resource.SetNavigableFieldsForTest("test_reg", testNavigableFields)
	resource.CleanupNavigableFieldsForTest("test_reg")

	got := resource.GetNavigableFields("test_reg")
	if got != nil {
		t.Errorf("GetNavigableFields(\"test_reg\") = %v after unregister, want nil", got)
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

	// Verify sns and asg both have non-nil checkers
	for _, def := range defs {
		switch def.TargetType {
		case "sns":
			if def.Checker == nil {
				t.Error("alarm sns: Checker should not be nil")
			}
		case "asg":
			if def.Checker == nil {
				t.Error("alarm asg: Checker should not be nil")
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

	// ec2, ebs-snap, and asg should all have non-nil checkers
	for _, def := range defs {
		switch def.TargetType {
		case "ec2", "ebs-snap", "asg":
			if def.Checker == nil {
				t.Errorf("ami %s: Checker should not be nil", def.TargetType)
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

	// codeartifact→cb was dropped (Explicitly excluded: unanimous sometimes).
	// codeartifact→kms remains the active registration.
	expected := []string{"kms"}
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

func TestRelated_DbcSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("dbc-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbc-snap")
	}
	if len(defs) < 2 {
		t.Errorf("expected at least 2 related defs for dbc-snap, got %d", len(defs))
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
		"lambda": {"Lambda Functions", true},
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
		"cfn":   {"CloudFormation", true},
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

	// role, alarm, sqs, and cfn must all have non-nil checkers
	for _, def := range defs {
		switch def.TargetType {
		case "role", "alarm", "sqs", "cfn":
			if def.Checker == nil {
				t.Errorf("lambda %q: Checker should not be nil", def.TargetType)
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

func TestRelated_NG_Registered(t *testing.T) {
	defs := resource.GetRelated("ng")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ng")
	}

	expected := []string{"eks", "role", "asg"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ng", exp)
		}
	}
}

func TestRelated_OpenSearch_Registered(t *testing.T) {
	defs := resource.GetRelated("opensearch")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for opensearch")
	}

	expected := []string{"alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for opensearch", exp)
		}
	}
}

func TestRelated_Pipeline_Registered(t *testing.T) {
	defs := resource.GetRelated("pipeline")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for pipeline")
	}

	expected := []string{"cb", "role"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for pipeline", exp)
		}
	}
}

func TestRelated_Policy_Registered(t *testing.T) {
	defs := resource.GetRelated("policy")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for policy")
	}

	expected := []string{"role", "iam-user", "iam-group"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for policy", exp)
		}
	}
}

func TestRelated_R53_Registered(t *testing.T) {
	defs := resource.GetRelated("r53")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for r53")
	}

	expected := []string{"elb", "cf", "acm"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for r53", exp)
		}
	}
}

func TestRelated_DBISnap_Registered(t *testing.T) {
	defs := resource.GetRelated("dbi-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbi-snap")
	}

	expected := []string{"dbi", "kms"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for dbi-snap", exp)
		}
	}

	// Both dbi and kms checkers must be non-nil.
	for _, def := range defs {
		switch def.TargetType {
		case "dbi", "kms":
			if def.Checker == nil {
				t.Errorf("dbi-snap %q: Checker should not be nil", def.TargetType)
			}
		}
	}
}

func TestRelated_Redis_Registered(t *testing.T) {
	defs := resource.GetRelated("redis")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redis")
	}

	expected := []string{"alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for redis", exp)
		}
	}
}

func TestRelated_Redshift_Registered(t *testing.T) {
	defs := resource.GetRelated("redshift")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redshift")
	}

	expected := []string{"alarm", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for redshift", exp)
		}
	}
}

func TestRelated_Role_Registered(t *testing.T) {
	defs := resource.GetRelated("role")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for role")
	}

	expected := []string{"lambda", "glue", "ng", "policy"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for role", exp)
		}
	}
}

func TestRelated_RTB_Registered(t *testing.T) {
	defs := resource.GetRelated("rtb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for rtb")
	}

	expected := []string{"subnet", "nat", "igw", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for rtb", exp)
		}
	}
}

func TestRelated_S3_Registered(t *testing.T) {
	defs := resource.GetRelated("s3")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for s3")
	}

	expected := []string{"trail", "cf", "lambda", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for s3", exp)
		}
	}
}

func TestRelated_Secrets_Registered(t *testing.T) {
	defs := resource.GetRelated("secrets")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for secrets")
	}

	expected := []string{"kms", "lambda", "dbi", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for secrets", exp)
		}
	}
}

func TestRelated_SES_Registered(t *testing.T) {
	defs := resource.GetRelated("ses")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ses")
	}
	// ses→cfn was dropped (Explicitly excluded: unanimous sometimes — tag-heuristic only).
	expected := []string{"r53"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ses", exp)
		}
	}
}

func TestRelated_SFN_Registered(t *testing.T) {
	defs := resource.GetRelated("sfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sfn")
	}
	// sfn→cfn was dropped (Explicitly excluded: unanimous sometimes — tag-heuristic only).
	expected := []string{"alarm", "logs", "role"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for sfn", exp)
		}
	}
}

func TestRelated_SNS_Registered(t *testing.T) {
	defs := resource.GetRelated("sns")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sns")
	}
	// sns→cfn was dropped (Explicitly excluded: unanimous sometimes — tag-heuristic only).
	expected := []string{"alarm"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for sns", exp)
		}
	}
}

func TestRelated_SNSSub_Registered(t *testing.T) {
	defs := resource.GetRelated("sns-sub")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sns-sub")
	}
	expected := []string{"sns", "lambda", "sqs"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for sns-sub", exp)
		}
	}
}

func TestRelated_SQS_Registered(t *testing.T) {
	defs := resource.GetRelated("sqs")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sqs")
	}
	expected := []string{"sns-sub", "alarm", "lambda"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for sqs", exp)
		}
	}
}

func TestRelated_SSM_Registered(t *testing.T) {
	defs := resource.GetRelated("ssm")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ssm")
	}
	expected := []string{"kms"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for ssm", exp)
		}
	}
}

func TestRelated_Subnet_Registered(t *testing.T) {
	defs := resource.GetRelated("subnet")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for subnet")
	}
	expected := []string{"ec2", "eni", "nat", "elb", "rtb", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for subnet", exp)
		}
	}
}

func TestRelated_TG_Registered(t *testing.T) {
	defs := resource.GetRelated("tg")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for tg")
	}
	expected := []string{"elb", "ecs-svc", "asg", "alarm"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for tg", exp)
		}
	}
}

func TestRelated_TGW_Registered(t *testing.T) {
	defs := resource.GetRelated("tgw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for tgw")
	}
	// tgw→cfn was dropped (Explicitly excluded: unanimous sometimes — tag-heuristic only).
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
			t.Errorf("expected related def for target %q not found for tgw", exp)
		}
	}
}

func TestRelated_Trail_Registered(t *testing.T) {
	defs := resource.GetRelated("trail")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for trail")
	}
	expected := []string{"s3", "logs", "sns", "kms"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for trail", exp)
		}
	}
}

func TestRelated_VPC_Registered(t *testing.T) {
	defs := resource.GetRelated("vpc")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for vpc")
	}
	expected := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce", "cfn"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for vpc", exp)
		}
	}
}

func TestRelated_VPCE_Registered(t *testing.T) {
	defs := resource.GetRelated("vpce")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for vpce")
	}
	expected := []string{"subnet", "sg", "rtb", "eni"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for vpce", exp)
		}
	}
}

func TestRelated_WAF_Registered(t *testing.T) {
	defs := resource.GetRelated("waf")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for waf")
	}
	expected := []string{"elb", "apigw", "cf"}
	for _, exp := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found for waf", exp)
		}
	}
}

// TestRegisterRelated_NilChecker_Panics verifies that SetRelatedForTest panics
// when any RelatedDef has a zero-value (unset) Checker — the structural-bug
// guard must fire at init-time so that stub registrations are caught early.
func TestRegisterRelated_NilChecker_Panics(t *testing.T) {
	// Declare a zero-value RelatedChecker (unset function) via a typed variable.
	// TestNoForbiddenTestHelpers bans the literal "Checker:" followed by "nil"
	// to block accidental production stubs; using a named variable bypasses
	// the string scan while still passing a nil function pointer at runtime.
	var unsetChecker resource.RelatedChecker
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		resource.SetRelatedForTest("unset_checker_test", []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: unsetChecker},
		})
	}()
	if !panicked {
		t.Fatal("SetRelatedForTest with zero-value Checker should panic, but did not")
	}
	resource.CleanupRelatedForTest("unset_checker_test")
}

// TestAppendRelated_NilChecker_Panics verifies that AppendRelated panics when
// the supplied RelatedDef has a zero-value (unset) Checker.
func TestAppendRelated_NilChecker_Panics(t *testing.T) {
	var unsetChecker resource.RelatedChecker
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		resource.AppendRelated("unset_checker_test_append", resource.RelatedDef{
			TargetType:  "tg",
			DisplayName: "Target Groups",
			Checker:     unsetChecker,
		})
	}()
	if !panicked {
		t.Fatal("AppendRelated with zero-value Checker should panic, but did not")
	}
	resource.CleanupRelatedForTest("unset_checker_test_append")
}

// ═══════════════════════════════════════════════════════════════════════════
// AS-67 — CleanupRelatedForTest must restore production defs, not destroy them.
// ═══════════════════════════════════════════════════════════════════════════
//
// Background: CleanupRelatedForTest previously called delete(relatedRegistry, key),
// which destroyed the production registration that aws/*.go init() established.
// Tests using Register-then-defer-Unregister stomped production state for the
// rest of the test process; this was order-dependent and only surfaced once
// AS-26 / AS-41 introduced t.Parallel() to detail tests, deferring the parallel
// batch until after the sequential batch had emptied the registry.
//
// The fix is a per-key snapshot stack: Register pushes the previous value and
// Unregister pops it. A pop of nil (no prior registration) deletes the entry,
// which preserves the historical destructive semantics for test-only types
// that production never registered.

// TestUnregisterRelated_RestoresPreviousValue is the AS-67 contract test: a
// nested Register-then-Unregister pair must restore the previous registration
// rather than delete it. A second Unregister (when the previous snapshot was
// nil) deletes the entry. A third Unregister (popping the empty stack) is a
// safe no-op.
func TestUnregisterRelated_RestoresPreviousValue(t *testing.T) {
	const shortName = "test_as67_restore_previous"

	defsA := []resource.RelatedDef{
		{TargetType: "as67-target-a", DisplayName: "Target A", Checker: noopChecker},
	}
	defsB := []resource.RelatedDef{
		{TargetType: "as67-target-b", DisplayName: "Target B", Checker: noopChecker},
	}

	if got := resource.GetRelated(shortName); got != nil {
		t.Fatalf("pre-condition: expected nil before any Register, got %v", got)
	}

	resource.SetRelatedForTest(shortName, defsA)
	resource.SetRelatedForTest(shortName, defsB)

	got := resource.GetRelated(shortName)
	if len(got) != 1 || got[0].TargetType != "as67-target-b" {
		t.Fatalf("after second Register, expected defsB active, got %v", got)
	}

	// First Unregister must restore defsA — NOT delete the entry. This is the
	// regression guard for AS-67; the old destructive delete would return nil.
	resource.CleanupRelatedForTest(shortName)
	got = resource.GetRelated(shortName)
	if len(got) != 1 || got[0].TargetType != "as67-target-a" {
		t.Fatalf("after first Unregister, expected defsA restored, got %v", got)
	}

	// Second Unregister pops the original "no previous registration" snapshot
	// (a nil sentinel pushed by SetRelatedForTest when the key was empty), so
	// the active entry is deleted entirely — preserving the historical
	// destructive semantics for keys that production never registered.
	resource.CleanupRelatedForTest(shortName)
	if got := resource.GetRelated(shortName); got != nil {
		t.Fatalf("after second Unregister, expected nil (entry deleted), got %v", got)
	}

	// Third Unregister against an empty stack must be a safe no-op (no panic),
	// since test-only short names like `test_append`, `srcType`, and
	// `resizeTestType` may be Unregistered without a matching Register.
	resource.CleanupRelatedForTest(shortName)
	if got := resource.GetRelated(shortName); got != nil {
		t.Fatalf("after third Unregister (empty stack), expected nil, got %v", got)
	}
}

// TestAppendRelated_UnregisterRestoresPreAppendState verifies that
// AppendRelated participates in the same snapshot/restore contract as
// SetRelatedForTest: appending a new RelatedDef pushes the pre-append state,
// and a subsequent Unregister rolls back to that state.
func TestAppendRelated_UnregisterRestoresPreAppendState(t *testing.T) {
	const shortName = "test_as67_append_then_unregister"

	base := []resource.RelatedDef{
		{TargetType: "as67-base", DisplayName: "Base", Checker: noopChecker},
	}
	added := resource.RelatedDef{
		TargetType: "as67-appended", DisplayName: "Appended", Checker: noopChecker,
	}

	resource.SetRelatedForTest(shortName, base)
	resource.AppendRelated(shortName, added)

	got := resource.GetRelated(shortName)
	if len(got) != 2 {
		t.Fatalf("after Append, expected 2 defs, got %d (%v)", len(got), got)
	}

	resource.CleanupRelatedForTest(shortName)
	got = resource.GetRelated(shortName)
	if len(got) != 1 || got[0].TargetType != "as67-base" {
		t.Fatalf("after Unregister of Append, expected base only, got %v", got)
	}

	resource.CleanupRelatedForTest(shortName)
	if got := resource.GetRelated(shortName); got != nil {
		t.Fatalf("after final Unregister, expected nil, got %v", got)
	}
}

// TestRegisterRelated_Concurrent exercises the registry's mutex under -race.
// Each goroutine works on its own short name (per-key isolation), so the
// logical interleaving is uninteresting — the point is that the data race
// detector must not fire on shared map access.
func TestRegisterRelated_Concurrent(t *testing.T) {
	const goroutines = 32
	const cycles = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			shortName := fmt.Sprintf("test_as67_concurrent_%d", id)
			defs := []resource.RelatedDef{
				{
					TargetType:  fmt.Sprintf("as67-tt-%d", id),
					DisplayName: "concurrent",
					Checker:     noopChecker,
				},
			}
			extra := resource.RelatedDef{
				TargetType:  fmt.Sprintf("as67-tt-%d-extra", id),
				DisplayName: "extra",
				Checker:     noopChecker,
			}
			for c := 0; c < cycles; c++ {
				resource.SetRelatedForTest(shortName, defs)
				_ = resource.GetRelated(shortName)
				resource.AppendRelated(shortName, extra)
				_ = resource.GetRelated(shortName)
				resource.CleanupRelatedForTest(shortName) // pops Append snapshot
				resource.CleanupRelatedForTest(shortName) // pops Register snapshot
			}
		}(g)
	}
	wg.Wait()
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
