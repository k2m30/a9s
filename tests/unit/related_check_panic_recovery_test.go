// related_check_panic_recovery_test.go — pins the upcoming fix to
// relatedCheckCmd's panic-recovery closure
// (internal/tui/runtime_adapter_related.go:396-406).
//
// Today, when a related checker panics, the recover() branch discards the
// panic value entirely and returns a bare
// messages.RelatedCheckResult{..., Result: resource.UnknownRelated(def.TargetType)}
// with no error anywhere on the message — a checker crash is completely
// silent to the operator. The fix must surface the panic as an error on
// LazyAddError, the message's only error-carrying field today
// (core/runtime/messages/event.go), which Core.HandleRelatedCheckResult
// already converts into a FlashIntent{IsError:true}
// (core/runtime/handlers_resources.go:285-289) — an existing, wired
// surfacing path a coder can reuse rather than adding a new field. Result
// itself must stay resource.UnknownRelated so the existing blank,
// navigable-row rendering fallback (core/resource/related.go:232-240) is
// unaffected.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestRelatedCheckCmd_CheckerPanic_SurfacesErrorAndFallsBackToUnknown
// registers a related checker that panics, drives it through the real
// relatedCheckCmd fan-out (by opening the resource's detail view, which
// begins a DetailOperation and dispatches the related-check task directly),
// and asserts the recovered result carries a non-nil, descriptive
// LazyAddError while Result is preserved as resource.UnknownRelated.
func TestRelatedCheckCmd_CheckerPanic_SurfacesErrorAndFallsBackToUnknown(t *testing.T) {
	const (
		srcType    = "test-related-panic-source"
		targetType = "test-related-panic-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Panic Recovery Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				panic("boom")
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	m := tui.New("testprofile", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-panic-001"}
	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResult received — a checker panic must still be recovered and reported, not left unhandled")
	}

	if resultMsg.LazyAddError == nil {
		t.Fatal("RelatedCheckResult.LazyAddError is nil after a checker panic — the panic must be surfaced as an error, not silently discarded")
	}
	if !strings.Contains(resultMsg.LazyAddError.Error(), "boom") {
		t.Errorf("LazyAddError = %q, want it to mention the panic value %q", resultMsg.LazyAddError.Error(), "boom")
	}
	if !strings.Contains(resultMsg.LazyAddError.Error(), targetType) {
		t.Errorf("LazyAddError = %q, want it to mention the target type %q", resultMsg.LazyAddError.Error(), targetType)
	}

	// Field-by-field, not reflect.DeepEqual (govet deepequalerrors flags
	// DeepEqual on a struct carrying an error field): UnknownRelated leaves
	// every field but TargetType/State at its zero value.
	wantResult := resource.UnknownRelated(targetType)
	got := resultMsg.Result
	if got.TargetType() != wantResult.TargetType() ||
		got.State() != wantResult.State() ||
		got.Count() != wantResult.Count() ||
		got.Truncated() != wantResult.Truncated() ||
		len(got.ResourceIDs()) != 0 ||
		len(got.FetchFilter()) != 0 {
		t.Errorf("Result = %+v, want %+v — the UnknownRelated rendering fallback must be preserved on a checker panic", got, wantResult)
	}
}
