package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// ---------------------------------------------------------------------------
// Test struct definitions (local to test — no AWS SDK dependency)
// ---------------------------------------------------------------------------

// testEC2Instance mirrors a minimal AWS EC2 struct shape for fieldlist tests.
// No JSON tags — AWS SDK Go v2 structs have none, so field-name matching applies.
type testEC2Instance struct {
	VpcId        *string
	SubnetId     *string
	State        testEC2State
	InstanceType string
}

type testEC2State struct {
	Name string
	Code int
}

// ---------------------------------------------------------------------------
// T002 — ExtractFieldList tests
// ---------------------------------------------------------------------------

// TestExtractFieldList_ScalarField verifies that a scalar pointer field is
// resolved correctly and returned as a single non-header FieldItem.
func TestExtractFieldList_ScalarField(t *testing.T) {
	obj := testEC2Instance{VpcId: new("vpc-abc")}
	paths := []string{"VpcId"}

	items := fieldpath.ExtractFieldList(obj, nil, paths, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 FieldItem, got %d", len(items))
	}
	item := items[0]
	if item.Path != "VpcId" {
		t.Errorf("Path: expected %q, got %q", "VpcId", item.Path)
	}
	if item.Key != "VpcId" {
		t.Errorf("Key: expected %q, got %q", "VpcId", item.Key)
	}
	if item.Value != "vpc-abc" {
		t.Errorf("Value: expected %q, got %q", "vpc-abc", item.Value)
	}
	if item.IsNavigable {
		t.Error("IsNavigable: expected false for non-navigable field")
	}
	if item.IsHeader {
		t.Error("IsHeader: expected false for scalar field")
	}
}

// TestExtractFieldList_NavigableScalar verifies that a scalar field listed in
// the navigable map is annotated with IsNavigable=true and the correct TargetType.
func TestExtractFieldList_NavigableScalar(t *testing.T) {
	obj := testEC2Instance{VpcId: new("vpc-abc")}
	paths := []string{"VpcId"}
	navigable := map[string]string{"VpcId": "vpc"}

	items := fieldpath.ExtractFieldList(obj, nil, paths, navigable)

	if len(items) != 1 {
		t.Fatalf("expected 1 FieldItem, got %d", len(items))
	}
	item := items[0]
	if !item.IsNavigable {
		t.Error("IsNavigable: expected true for navigable field")
	}
	if item.TargetType != "vpc" {
		t.Errorf("TargetType: expected %q, got %q", "vpc", item.TargetType)
	}
}

// TestExtractFieldList_StructField verifies that a struct-typed field produces
// a header FieldItem followed by sub-field items.
func TestExtractFieldList_StructField(t *testing.T) {
	obj := testEC2Instance{
		State: testEC2State{Name: "running", Code: 16},
	}
	paths := []string{"State"}

	items := fieldpath.ExtractFieldList(obj, nil, paths, nil)

	if len(items) == 0 {
		t.Fatal("expected at least 1 FieldItem for struct field, got 0")
	}

	// First item must be the header.
	header := items[0]
	if header.Path != "State" {
		t.Errorf("header Path: expected %q, got %q", "State", header.Path)
	}
	if !header.IsHeader {
		t.Error("IsHeader: expected true for struct field header")
	}
	if header.Value != "" {
		t.Errorf("header Value: expected empty string, got %q", header.Value)
	}

	// Remaining items should be sub-fields.
	if len(items) < 2 {
		t.Fatalf("expected at least 2 FieldItems (header + sub-fields) for struct, got %d", len(items))
	}
	for _, sub := range items[1:] {
		if !sub.IsSubField {
			t.Errorf("sub-field item %q: expected IsSubField=true", sub.Key)
		}
		if sub.IndentLevel != 1 {
			t.Errorf("sub-field item %q: expected IndentLevel=1, got %d", sub.Key, sub.IndentLevel)
		}
	}
}

// TestExtractFieldList_NilObjWithFields verifies that a nil obj falls back to
// the pre-formatted fields map for value resolution.
func TestExtractFieldList_NilObjWithFields(t *testing.T) {
	fields := map[string]string{"Name": "test-instance"}
	paths := []string{"Name"}

	items := fieldpath.ExtractFieldList(nil, fields, paths, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 FieldItem, got %d", len(items))
	}
	if items[0].Value != "test-instance" {
		t.Errorf("Value: expected %q, got %q", "test-instance", items[0].Value)
	}
}

// TestExtractFieldList_UnknownPath verifies that a path not found in either
// the obj or the fields map returns a FieldItem with Value="-".
func TestExtractFieldList_UnknownPath(t *testing.T) {
	obj := testEC2Instance{}
	paths := []string{"NonExistent"}

	items := fieldpath.ExtractFieldList(obj, nil, paths, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 FieldItem for unknown path, got %d", len(items))
	}
	if items[0].Value != "-" {
		t.Errorf("Value: expected %q for unknown path, got %q", "-", items[0].Value)
	}
}

// TestExtractFieldList_EmptyPaths verifies that an empty paths slice returns a
// non-nil empty slice (never nil).
func TestExtractFieldList_EmptyPaths(t *testing.T) {
	obj := testEC2Instance{VpcId: new("vpc-abc")}

	items := fieldpath.ExtractFieldList(obj, nil, []string{}, nil)

	if items == nil {
		t.Error("expected non-nil slice for empty paths, got nil")
	}
	if len(items) != 0 {
		t.Errorf("expected len=0 for empty paths, got %d", len(items))
	}
}

// TestExtractFieldList_FieldsMapPrecedence verifies that the pre-formatted
// fields map takes precedence over reflection on the obj struct.
func TestExtractFieldList_FieldsMapPrecedence(t *testing.T) {
	obj := testEC2Instance{VpcId: new("vpc-from-struct")}
	fields := map[string]string{"VpcId": "vpc-from-map"}
	paths := []string{"VpcId"}

	items := fieldpath.ExtractFieldList(obj, fields, paths, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 FieldItem, got %d", len(items))
	}
	if items[0].Value != "vpc-from-map" {
		t.Errorf("Value: expected fields map to win, got %q", items[0].Value)
	}
}

// TestExtractFieldList_MultiplePaths verifies that multiple paths produce
// FieldItems in the same order as the paths slice.
func TestExtractFieldList_MultiplePaths(t *testing.T) {
	obj := testEC2Instance{
		VpcId:        new("vpc-abc"),
		InstanceType: "t3.medium",
	}
	paths := []string{"VpcId", "InstanceType"}

	items := fieldpath.ExtractFieldList(obj, nil, paths, nil)

	// Each scalar path yields exactly 1 item; total must be at least 2.
	if len(items) < 2 {
		t.Fatalf("expected at least 2 FieldItems for 2 scalar paths, got %d", len(items))
	}

	// Find items by Path to tolerate struct-expansion inserting extra items.
	foundVpc := false
	foundType := false
	vpcIdx := -1
	typeIdx := -1
	for i, it := range items {
		if it.Path == "VpcId" {
			foundVpc = true
			vpcIdx = i
		}
		if it.Path == "InstanceType" {
			foundType = true
			typeIdx = i
		}
	}

	if !foundVpc {
		t.Error("expected FieldItem with Path=VpcId")
	}
	if !foundType {
		t.Error("expected FieldItem with Path=InstanceType")
	}
	// VpcId comes before InstanceType in the paths slice — order must be preserved.
	if foundVpc && foundType && vpcIdx > typeIdx {
		t.Errorf("order: VpcId (idx=%d) must come before InstanceType (idx=%d)", vpcIdx, typeIdx)
	}

	// Spot-check values.
	for _, it := range items {
		switch it.Path {
		case "VpcId":
			if it.Value != "vpc-abc" {
				t.Errorf("VpcId Value: expected %q, got %q", "vpc-abc", it.Value)
			}
		case "InstanceType":
			if it.Value != "t3.medium" {
				t.Errorf("InstanceType Value: expected %q, got %q", "t3.medium", it.Value)
			}
		}
	}
}
