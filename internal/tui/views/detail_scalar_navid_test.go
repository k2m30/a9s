// detail_scalar_navid_test.go — Pin 3 regression pin for scalar NavID extraction.
//
// Verifies that buildFieldList() applies NavIDFromValue to top-level scalar
// navigable fields (IsNavigable=true, IsSubField=false) when they are enumerated
// via the path-form branch (detailPaths != nil).
//
// Context: ExtractFieldList marks IsNavigable only when a non-nil navMap is passed.
// The path-form branch (when detailPaths is configured) passes navMap; the flat
// no-detailPaths branch passes nil. Tests here use a ViewsConfig that supplies
// detailPaths so navMap is active.
//
// Pre-fix: the post-processing loop in buildFieldList only applied NavIDFromValue
// to YAML sub-fields (IsSubField=true), so a Lambda Role ARN at the top level
// would have IsNavigable=true but NavID="" — navigation then used the full ARN
// as the target ID, causing "not found" when looking up the role by name.
//
// Post-fix: the loop applies NavIDFromValue to ALL items where
//   IsNavigable=true && !IsSubField && TargetType!="" && Value!=""
// yielding NavID="my-lambda-role" from the full Role ARN.
package views

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

// lambdaViewCfg returns a ViewsConfig that includes "Role" in the lambda detail paths,
// so that buildFieldList uses the path-form branch and passes navMap to ExtractFieldList.
func lambdaViewCfg() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"lambda": {
				Detail: []config.DetailField{
					{Path: "Role"},
					{Path: "Runtime"},
					{Path: "Handler"},
					{Path: "MemorySize"},
					{Path: "Timeout"},
				},
			},
		},
	}
}

// TestBuildFieldList_ScalarNavigableField_AppliesNavIDFromValue verifies that
// after buildFieldList() runs, the FieldItem for the "Role" field on a lambda
// resource has NavID set to the bare role name ("my-lambda-role"), not the full
// ARN. This confirms that the post-processing loop covers top-level scalar fields.
//
// The test exercises buildFieldList indirectly via refreshViewportContent()
// (which calls buildFieldList when m.fieldList is nil).
func TestBuildFieldList_ScalarNavigableField_AppliesNavIDFromValue(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"
	const wantNavID = "my-lambda-role"

	res := resource.Resource{
		ID:   "arn:aws:lambda:us-east-1:123456789012:function:my-fn",
		Name: "my-fn",
		Fields: map[string]string{
			"Role":        roleARN,
			"Runtime":     "go1.x",
			"Handler":     "bootstrap",
			"MemorySize":  "128",
			"Timeout":     "30",
			"FunctionArn": "arn:aws:lambda:us-east-1:123456789012:function:my-fn",
		},
	}

	// Register "role" as navigable for lambda.
	resource.RegisterNavigableFields("lambda", []resource.NavigableField{
		{FieldPath: "Role", TargetType: "role"},
	})
	defer resource.UnregisterNavigableFields("lambda")

	// Use a ViewsConfig that includes "Role" in detailPaths so the path-form
	// branch is taken — this branch passes navMap to ExtractFieldList, enabling
	// IsNavigable annotation on top-level scalar fields.
	m := NewDetail(res, "lambda", lambdaViewCfg(), keys.Default())
	m.SetSize(120, 40)

	// Force buildFieldList by triggering viewport content refresh.
	m.refreshViewportContent()

	if len(m.fieldList) == 0 {
		t.Fatal("fieldList is empty after refreshViewportContent — buildFieldList may have failed")
	}

	// Find the "Role" field item in the field list.
	var roleItem *fieldpath.FieldItem
	for i := range m.fieldList {
		item := &m.fieldList[i]
		if item.Path == "Role" || item.Key == "Role" {
			roleItem = item
			break
		}
	}
	if roleItem == nil {
		t.Fatalf("fieldList does not contain a FieldItem with Path/Key='Role'; items: %v", m.fieldList)
	}

	if !roleItem.IsNavigable {
		t.Errorf("Role FieldItem.IsNavigable = false, want true (registered as NavigableField)")
	}
	if roleItem.TargetType != "role" {
		t.Errorf("Role FieldItem.TargetType = %q, want %q", roleItem.TargetType, "role")
	}
	if roleItem.Value != roleARN {
		t.Errorf("Role FieldItem.Value = %q, want %q", roleItem.Value, roleARN)
	}
	if roleItem.NavID != wantNavID {
		t.Errorf("Role FieldItem.NavID = %q, want %q\n"+
			"buildFieldList must apply NavIDFromValue to top-level scalar navigable fields, "+
			"not just YAML sub-fields", roleItem.NavID, wantNavID)
	}
}

// TestBuildFieldList_ScalarNavigableField_NoExtractor_NavIDEmpty verifies that
// when a target type has no registered NavID extractor (e.g., "subnet"), the
// NavID remains "" and the full value is used for navigation.
// This ensures the post-processing loop doesn't overwrite with an empty string.
func TestBuildFieldList_ScalarNavigableField_NoExtractor_NavIDEmpty(t *testing.T) {
	const subnetID = "subnet-0aaa111111111111a"

	res := resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"SubnetId":     subnetID,
			"InstanceType": "t3.large",
		},
	}

	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "SubnetId", TargetType: "subnet"},
	})
	defer resource.UnregisterNavigableFields("ec2")

	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{
					{Path: "SubnetId"},
					{Path: "InstanceType"},
				},
			},
		},
	}

	m := NewDetail(res, "ec2", cfg, keys.Default())
	m.SetSize(120, 40)
	m.refreshViewportContent()

	if len(m.fieldList) == 0 {
		t.Fatal("fieldList is empty after refreshViewportContent")
	}

	for _, item := range m.fieldList {
		if item.Path == "SubnetId" || item.Key == "SubnetId" {
			if !item.IsNavigable {
				t.Errorf("SubnetId FieldItem.IsNavigable = false, want true")
			}
			// "subnet" has no NavID extractor — NavID must remain empty.
			// The loop condition `navID != "" && navID != item.Value` must protect this.
			if item.NavID != "" {
				t.Errorf("SubnetId FieldItem.NavID = %q, want %q "+
					"(subnet has no NavID extractor; full value is used for navigation)",
					item.NavID, "")
			}
			return
		}
	}
	t.Fatalf("fieldList does not contain a FieldItem with Path/Key='SubnetId'; items: %v", m.fieldList)
}
