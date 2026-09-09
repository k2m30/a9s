// tui_list_reentry_verify_test.go — the TUI half of the re-entry
// re-verification pin. Opening a list, returning to the main menu and
// opening it again must not render the retained rows as verified-fresh:
// HandleNavigate returns the task for the row-store hit, so the TUI shows
// the refreshing marker and issues the AWS call however long ago the rows
// were fetched. The headless half lives in list_request_generation_test.go.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

func TestListReEntry_TUIRendersRefreshingMarker(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	m := tuitest.Sized("reentry-verify-prof", "us-east-1")
	t.Cleanup(func() { m.CloseController() })

	m = tuitest.StepModel(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	m = tuitest.StepModel(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "bucket-1", Name: "bucket-1", Type: "s3", Fields: map[string]string{"name": "bucket-1"}},
			{ID: "bucket-2", Name: "bucket-2", Type: "s3", Fields: map[string]string{"name": "bucket-2"}},
		},
		Pagination: &resource.PaginationMeta{IsTruncated: false},
		Provenance: messages.FetchProvenanceCanonicalList,
	})

	m = tuitest.StepModel(m, messages.PopView{})

	m, cmd := tuitest.Step(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	if cmd == nil {
		t.Fatalf("re-opening the retained s3 list issued no command — nothing re-verifies the retained rows")
	}

	view := tuitest.StripANSI(tuitest.Render(m))
	if !strings.Contains(view, "refreshing") {
		t.Fatalf("re-opened list renders without the refreshing marker; view:\n%s", view)
	}
}
