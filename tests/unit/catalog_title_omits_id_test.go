package unit

// TitleOmitsID renders the detail frame title as "detail -- <Name>" instead
// of "detail -- <ID> (<Name>)", keeping a log event's long synthetic ID out
// of the title. The title string is composed in the unexported
// internal/tui/app_view.go:(Model).frameTitle, unreachable from tests/unit.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

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

func TestCatalog_TitleOmitsID_DefaultsFalseForOtherTypes(t *testing.T) {
	def := catalog.FindAny("ec2")
	if def == nil {
		t.Fatal("catalog.FindAny(\"ec2\") returned nil — top-level type not registered")
	}
	if def.TitleOmitsID {
		t.Errorf("catalog.FindAny(\"ec2\").TitleOmitsID = true, want false — normal resource types must keep showing the ID in the detail frame title")
	}
}
