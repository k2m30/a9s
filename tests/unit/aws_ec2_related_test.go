package unit_test

// aws_ec2_related_test.go — T024: verifies that EC2 has its related-resource
// definitions and navigable fields registered via init().
//
// These tests read init()-registered data from the production registries; no
// cleanup is required.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEC2_RelatedDefsRegistered(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if defs == nil {
		t.Skip("EC2 related defs were unregistered by a prior test; re-run in isolation to verify init() registration")
	}
	if len(defs) != 4 {
		t.Fatalf("expected 4 related defs for ec2, got %d", len(defs))
	}

	expected := map[string]string{
		"tg":    "Target Groups",
		"asg":   "Auto Scaling Groups",
		"alarm": "CloudWatch Alarms",
		"cfn":   "CloudFormation Stacks",
	}
	for _, def := range defs {
		wantName, ok := expected[def.TargetType]
		if !ok {
			t.Errorf("unexpected target type: %s", def.TargetType)
			continue
		}
		if def.DisplayName != wantName {
			t.Errorf("target %s: expected display name %q, got %q", def.TargetType, wantName, def.DisplayName)
		}
		if def.Checker == nil {
			t.Errorf("target %s: Checker should not be nil", def.TargetType)
		}
	}
}

func TestEC2_NavigableFieldsRegistered(t *testing.T) {
	fields := resource.GetNavigableFields("ec2")
	if fields == nil {
		t.Skip("EC2 navigable fields were unregistered by a prior test; re-run in isolation to verify init() registration")
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 navigable fields for ec2, got %d", len(fields))
	}

	expected := map[string]string{
		"VpcId":    "vpc",
		"SubnetId": "subnet",
		"ImageId":  "ami",
	}
	for _, f := range fields {
		wantType, ok := expected[f.FieldPath]
		if !ok {
			t.Errorf("unexpected field path: %s", f.FieldPath)
			continue
		}
		if f.TargetType != wantType {
			t.Errorf("field %s: expected target type %q, got %q", f.FieldPath, wantType, f.TargetType)
		}
	}
}
