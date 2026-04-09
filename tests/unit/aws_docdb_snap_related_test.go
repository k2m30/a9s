package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_DocdbSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("docdb-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for docdb-snap")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"dbc": {"DocumentDB Cluster", true},
		"kms": {"KMS Key", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("docdb-snap %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("docdb-snap %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("docdb-snap %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}
