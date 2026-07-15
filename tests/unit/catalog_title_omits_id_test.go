package unit

// catalog_title_omits_id_test.go — TDD-red skeleton for the TitleOmitsID
// catalog flag (bug fix: log-event detail frame title must not show the
// giant synthetic event ID, e.g.
//   detail -- 39760596764200534672029196979118035848442573100344082434 ({"level":"INFO",...)
//
// TODO(coder): this test currently FAILS TO COMPILE because
// catalog.ResourceTypeDef has no TitleOmitsID field yet. Add:
//
//	type ResourceTypeDef struct {
//	    ...
//	    // TitleOmitsID, when true, renders the detail frame title as
//	    // "detail -- <Name>" instead of "detail -- <ID> (<Name>)".
//	    TitleOmitsID bool
//	}
//
// and set TitleOmitsID: true on the "log_events" and
// "lambda_invocation_logs" child-type catalog entries (internal/aws install
// site — wherever those ChildViewDef/ResourceTypeDef entries are built).
//
// There is no clean, already-exported unit seam for the actual
// "detail -- <ID> (<Name>)" string composition: that logic lives in the
// unexported internal/tui/app_view.go:(Model).frameTitle, which reads
// unexported rendererState/app.ViewState fields and cannot be driven from
// tests/unit without further production plumbing (e.g. exposing a
// Controller-level DetailFrameTitle that consults catalog.TitleOmitsID,
// analogous to the existing (unrelated) Controller.DetailFrameTitle at
// internal/app/detail_state.go:412, which today returns bare Name/ID with no
// "detail -- " prefix and no ID-suppression logic at all).
//
// This skeleton pins the catalog-level contract (the part of the fix that
// IS unit-testable without new plumbing) so the coder has a red test driving
// the field addition. A follow-up test (or an extension of this one) should
// assert the actual frame-title string once the coder exposes a seam.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// TestCatalog_TitleOmitsID_SetForLogEventChildTypes verifies that the
// "log_events" and "lambda_invocation_logs" child-type catalog entries opt
// out of showing the raw resource ID in the detail frame title.
func TestCatalog_TitleOmitsID_SetForLogEventChildTypes(t *testing.T) {
	cases := []struct {
		shortName string
	}{
		{"log_events"},
		{"lambda_invocation_logs"},
	}

	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			def := catalog.FindChild(tc.shortName)
			if def == nil {
				t.Fatalf("catalog.FindChild(%q) returned nil — child type not registered", tc.shortName)
			}
			if !def.TitleOmitsID {
				t.Errorf("catalog.FindChild(%q).TitleOmitsID = false, want true (clean-Name bug fix requires suppressing the raw ID in the detail frame title)", tc.shortName)
			}
		})
	}
}

// TestCatalog_TitleOmitsID_DefaultsFalseForOtherTypes verifies that ordinary
// resource types (e.g. "ec2") are unaffected — their frame title must still
// include the ID per the existing "detail -- <ID> (<Name>)" contract.
func TestCatalog_TitleOmitsID_DefaultsFalseForOtherTypes(t *testing.T) {
	def := catalog.Find("ec2")
	if def == nil {
		t.Fatal("catalog.Find(\"ec2\") returned nil — top-level type not registered")
	}
	if def.TitleOmitsID {
		t.Errorf("catalog.Find(\"ec2\").TitleOmitsID = true, want false — normal resource types must keep showing the ID in the detail frame title")
	}
}
