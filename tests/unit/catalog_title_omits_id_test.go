package unit

// catalog_title_omits_id_test.go — the TitleOmitsID catalog flag: a
// log-event detail frame title must not show the giant synthetic event ID,
// e.g.
//   detail -- 39760596764200534672029196979118035848442573100344082434 ({"level":"INFO",...)
//
// TitleOmitsID renders the detail frame title as "detail -- <Name>" instead
// of "detail -- <ID> (<Name>)", and is set on the "log_events" and
// "lambda_invocation_logs" child-type catalog entries. This file pins the
// catalog-level contract; the frame-title string itself is composed in the
// unexported internal/tui/app_view.go:(Model).frameTitle and is not
// reachable from tests/unit.

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
			def := catalog.ChildOnly(tc.shortName)
			if def == nil {
				t.Fatalf("catalog.ChildOnly(%q) returned nil — child type not registered", tc.shortName)
			}
			if !def.TitleOmitsID {
				t.Errorf("catalog.ChildOnly(%q).TitleOmitsID = false, want true (clean-Name bug fix requires suppressing the raw ID in the detail frame title)", tc.shortName)
			}
		})
	}
}

// TestCatalog_TitleOmitsID_DefaultsFalseForOtherTypes verifies that ordinary
// resource types (e.g. "ec2") are unaffected — their frame title must still
// include the ID per the existing "detail -- <ID> (<Name>)" contract.
func TestCatalog_TitleOmitsID_DefaultsFalseForOtherTypes(t *testing.T) {
	def := catalog.FindAny("ec2")
	if def == nil {
		t.Fatal("catalog.FindAny(\"ec2\") returned nil — top-level type not registered")
	}
	if def.TitleOmitsID {
		t.Errorf("catalog.FindAny(\"ec2\").TitleOmitsID = true, want false — normal resource types must keep showing the ID in the detail frame title")
	}
}
