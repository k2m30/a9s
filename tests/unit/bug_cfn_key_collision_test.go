package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// Key "r" is bound to ToggleRelated; a child view on "r" would be
// shadowed on CFN detail views.
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
