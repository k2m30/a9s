package unit

// lazy_add_flash_error_test.go — pins the FlashIntent wiring in
// Core.HandleRelatedCheckResult (core/runtime/handlers_resources.go) that
// converts a non-nil LazyAddError into a visible operator notification.
//
// internal/tui/app.go's messages.RelatedCheckResult case calls m.ctrl.Handle
// directly and forwards only the returned TaskRequests, never the ViewState
// or its Flash — unlike Model.applyIntents (app_dispatch.go), which re-emits
// a FlashIntent as a messages.Flash cmd. The FlashIntent this handler emits
// therefore never reaches m.flash (or a returned cmd), so this test verifies
// the rendered view and stays RED until that TUI-side gap is closed.

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ---------------------------------------------------------------------------
// Test 1 — total FetchByIDs failure emits FlashMsg with "related-fetch" prefix
// ---------------------------------------------------------------------------

// TestLazyAddError_EmitsFlashMsg verifies that when FetchByIDs returns a
// non-nil error (and no partial results), the app handler emits a FlashMsg
// with IsError=true and text containing "related-fetch" + the error string.
//
// Regression pin for the `if msg.LazyAddError != nil` branch added in app.go.
func TestLazyAddError_EmitsFlashMsg(t *testing.T) {
	const (
		srcType    = "test-flash-error-source"
		targetType = "test-flash-error-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Flash Error Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{"id-boom-001", "id-boom-002"},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return nil, errors.New("simulated boom")
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-flash-001"}

	// Step 1: open detail for srcRes — begins a DetailOperation and
	// dispatches the related-check task directly — then collect the
	// resulting RelatedCheckResult.
	m, startCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, startCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received from opening srcType detail")
	}
	if resultMsg.LazyAddError == nil {
		t.Fatal("expected LazyAddError to be set when FetchByIDs returns an error")
	}

	// Step 2: apply the RelatedCheckResultMsg to the model. HandleRelatedCheckResult
	// applies the LazyAddError as a FlashIntent (core/runtime/handlers_resources.go),
	// mutating Controller/Model flash state directly rather than returning a
	// messages.Flash cmd, so the error surfaces in the rendered header.
	m, _ = rootApplyMsg(m, resultMsg)

	view := stripANSI(rootViewContent(m))
	for _, want := range []string{"related-fetch", "simulated boom"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to show flash containing %q, got:\n%s", want, view)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2 — partial success + error emits FlashMsg AND caches the partial result
// ---------------------------------------------------------------------------

// TestLazyAddError_PartialSuccess_StillEmitsFlashMsg verifies that when
// FetchByIDs returns (partialResults, error), the app handler:
//   - merges the partial results into lazyResourceCache, AND
//   - still emits a FlashMsg for the operator.
//
// Regression pin for the partial-success path where both the merge loop at
// app.go:594-610 and the flash branch at app.go:615-624 must both execute.
func TestLazyAddError_PartialSuccess_StillEmitsFlashMsg(t *testing.T) {
	const (
		srcType    = "test-flash-partial-source"
		targetType = "test-flash-partial-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Flash Partial Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{"id-ok", "id-bad"},
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		// Partial success: 1 resolved, 1 missing.
		partial := []resource.Resource{{ID: "id-ok", Name: "ok-resource"}}
		return partial, errors.New("id-bad: denied")
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-partial-001"}

	// Step 1: open detail for srcRes and collect the resulting RelatedCheckResult.
	m, startCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, startCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received from opening srcType detail")
	}
	if resultMsg.LazyAddError == nil {
		t.Fatal("expected LazyAddError to be set for partial FetchByIDs failure")
	}

	// The partial result must be present on the message itself (app.go merge loop
	// reads from msg.LazyAddedResources).
	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 || lazySlice[0].ID != "id-ok" {
		t.Errorf("LazyAddedResources[%s] = %v, want 1 resource with ID=id-ok", targetType, lazySlice)
	}

	// Step 2: apply the result to the model. HandleRelatedCheckResult applies
	// the LazyAddError as a FlashIntent (core/runtime/handlers_resources.go),
	// mutating Controller/Model flash state directly rather than returning a
	// messages.Flash cmd, so the error surfaces in the rendered header.
	m, _ = rootApplyMsg(m, resultMsg)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "id-bad") {
		t.Errorf("expected view to show flash containing %q, got:\n%s", "id-bad", view)
	}
	if !strings.Contains(view, "related-fetch") {
		t.Errorf("expected view to show flash containing %q, got:\n%s", "related-fetch", view)
	}
}
