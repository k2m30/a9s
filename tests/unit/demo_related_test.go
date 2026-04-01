package unit_test

// demo_related_test.go — T025: verifies that the demo-mode related checker for
// EC2 is registered and returns the expected hardcoded results.
//
// These tests read init()-registered data from the production demo registry; no
// cleanup is required.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestDemoRelatedChecker_EC2(t *testing.T) {
	checker := resource.GetRelatedDemo("ec2")
	if checker == nil {
		t.Fatal("expected demo checker for ec2, got nil")
	}

	res := resource.Resource{ID: "i-demo001", Name: "demo-instance"}
	results := checker(res)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Build map for easier per-type assertions.
	byType := make(map[string]resource.RelatedCheckResult, len(results))
	for _, r := range results {
		byType[r.TargetType] = r
	}

	// TG: count=1 with one resource ID.
	if tg, ok := byType["tg"]; !ok {
		t.Error("missing tg result")
	} else {
		if tg.Count != 1 {
			t.Errorf("tg count: expected 1, got %d", tg.Count)
		}
		if len(tg.ResourceIDs) != 1 {
			t.Errorf("tg resource IDs: expected 1, got %d", len(tg.ResourceIDs))
		}
	}

	// ASG: count=1.
	if asg, ok := byType["asg"]; !ok {
		t.Error("missing asg result")
	} else if asg.Count != 1 {
		t.Errorf("asg count: expected 1, got %d", asg.Count)
	}

	alarmResult, alarmFound := byType["alarm"]
	if !alarmFound {
		t.Error("missing alarm result")
	} else if alarmResult.Count != 2 {
		t.Errorf("alarm count: expected 2, got %d", alarmResult.Count)
	}

	// CFN: count=0 (no stacks reference this instance).
	if cfn, ok := byType["cfn"]; !ok {
		t.Error("missing cfn result")
	} else if cfn.Count != 0 {
		t.Errorf("cfn count: expected 0, got %d", cfn.Count)
	}
}

func TestDemoRelatedChecker_Unknown(t *testing.T) {
	checker := resource.GetRelatedDemo("unknown_type")
	if checker != nil {
		t.Error("expected nil demo checker for unknown type")
	}
}
