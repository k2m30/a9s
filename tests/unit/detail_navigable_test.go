package unit_test

// detail_navigable_test.go — tests for resource.IsFieldNavigableForTest, the
// resource-package predicate the live buildDetailFieldItems/RenderDetail
// path uses — see tests/unit/detail_ports_test.go's header comment item 7.
// RenderDetail's rendering/Enter-key coverage is in detail_ports_test.go and
// detail_livepath_migration_test.go.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestIsFieldNavigableForTest_MatchFound(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
	})

	f := resource.IsFieldNavigableForTest("ec2", "VpcId")
	if f == nil {
		t.Fatal("IsFieldNavigable: expected non-nil for registered field VpcId")
	}
	if f.TargetType != "vpc" {
		t.Errorf("IsFieldNavigable: TargetType: want %q, got %q", "vpc", f.TargetType)
	}
}

func TestIsFieldNavigableForTest_NoMatch(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	f := resource.IsFieldNavigableForTest("ec2", "SubnetId")
	if f != nil {
		t.Errorf("IsFieldNavigable: expected nil for unregistered field SubnetId, got %+v", f)
	}
}

func TestIsFieldNavigableForTest_UnknownType(t *testing.T) {
	f := resource.IsFieldNavigableForTest("rds", "VpcId")
	if f != nil {
		t.Errorf("IsFieldNavigable: expected nil for unregistered type, got %+v", f)
	}
}
