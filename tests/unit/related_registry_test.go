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

// ─── compile-time reference to context so the import is used ────────────────
// RelatedChecker requires context.Context; verify the type is usable.
var _ resource.RelatedChecker = func(
	_ context.Context,
	_ interface{},
	_ resource.Resource,
	_ resource.ResourceCache,
) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{}
}
