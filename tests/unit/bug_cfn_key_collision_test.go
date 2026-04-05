package unit

// bug_cfn_key_collision_test.go — Test revealing the CFN key collision bug.
//
// Bug: internal/resource/types_cicd.go:19 defines the cfn_resources ChildViewDef
// with Key: "r" (lowercase). keys.go:108 also binds ToggleRelated to "r".
// Both bindings fire on the same key, causing silent shadowing on CFN detail views.
//
// The fix changes the cfn_resources ChildViewDef Key to "R" (uppercase).
// This test FAILS with current code (Key == "r") and passes after the fix (Key == "R").

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestCFN_ResourcesChildViewDef_UsesUppercaseR asserts that the cfn_resources
// ChildViewDef on the "cfn" resource type uses key "R" (uppercase), not "r".
// Key "r" collides with the ToggleRelated binding in keys.go:108, causing silent
// shadowing on CFN detail views.
func TestCFN_ResourcesChildViewDef_UsesUppercaseR(t *testing.T) {
	cfn := resource.FindResourceType("cfn")
	if cfn == nil {
		t.Fatal("resource type 'cfn' not found in registry")
	}

	var cfnResourcesDef *resource.ChildViewDef
	for i := range cfn.Children {
		if cfn.Children[i].ChildType == "cfn_resources" {
			cfnResourcesDef = &cfn.Children[i]
			break
		}
	}
	if cfnResourcesDef == nil {
		t.Fatal("cfn_resources ChildViewDef not found on 'cfn' resource type")
	}

	if cfnResourcesDef.Key == "r" {
		t.Errorf("BUG: cfn_resources ChildViewDef has Key %q which collides with "+
			"ToggleRelated binding 'r' in keys.go — fix: change to \"R\"",
			cfnResourcesDef.Key)
	}
	if cfnResourcesDef.Key != "R" {
		t.Errorf("cfn_resources ChildViewDef Key = %q; want \"R\"", cfnResourcesDef.Key)
	}
}
