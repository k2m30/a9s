package unit_test

// detail_resize_test.go tests that DetailModel sets the pendingRelatedDispatch
// flag after a narrow→wide resize so the root model's tea.WindowSizeMsg handler
// can dispatch RelatedCheckStartedMsg via TakePendingRelatedDispatch().
//
// Feature: 010-related-infra-fixes (issue #223)
//
// T010: TestDetailModel_NarrowToWideResize_DispatchesRelatedCheck
//   Verifies that after first-paint → narrow → wide-again, TakePendingRelatedDispatch()
//   returns true and clears the flag. This simulates what app.go does after
//   propagateSize() to dispatch RelatedCheckStartedMsg.
//
// T011: TestDetailModel_FirstPaint_NoDoubleDispatch
//   Regression guard: first-paint SetSize(wide) must NOT set pendingRelatedDispatch
//   (first-paint dispatch is handled by the NeedsRelatedCheck path, not this flag).

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

const resizeTestType = "ec2-resize-test"

// makeResizeDetailModel creates a DetailModel for the resize-dispatch tests.
// No SetSize is called — the caller controls all size calls.
func makeResizeDetailModel() views.DetailModel {
	res := resource.Resource{
		ID:   "i-resize001",
		Name: "resize-test-instance",
		Fields: map[string]string{
			"InstanceId": "i-resize001",
			"ImageId":    "ami-12345678",
		},
	}
	k := keys.Default()
	return views.NewDetail(res, resizeTestType, nil, k)
}

// ---------------------------------------------------------------------------
// T010: TestDetailModel_NarrowToWideResize_DispatchesRelatedCheck
// ---------------------------------------------------------------------------
// Given: a DetailModel with RelatedDefs registered
// When:
//  1. SetSize(80, 40) — first paint (m.ready=false → m.ready=true, right col auto-shown)
//  2. SetSize(30, 40) — narrow: right col hides, rightColAutoShown=false
//  3. SetSize(80, 40) — wide again: m.ready=true → pendingRelatedDispatch must be set
//  4. TakePendingRelatedDispatch() — simulates app.go calling this after propagateSize()
//
// Then: TakePendingRelatedDispatch() returns true (flag set); second call returns false (cleared).
//
// This verifies the contract between DetailModel and app.go's WindowSizeMsg handler.
// ---------------------------------------------------------------------------

func TestDetailModel_NarrowToWideResize_DispatchesRelatedCheck(t *testing.T) {
	resource.RegisterRelated(resizeTestType, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer resource.UnregisterRelated(resizeTestType)

	d := makeResizeDetailModel()

	// Step 1: first paint — m.ready transitions false→true, right col auto-shown.
	d.SetSize(80, 40)

	// Step 2: narrow — right col hides, rightColAutoShown reset to false.
	d.SetSize(30, 40)

	// Step 3: wide again — m.ready is now true; pendingRelatedDispatch must be set.
	d.SetSize(80, 40)

	// Step 4: TakePendingRelatedDispatch() simulates what app.go does after propagateSize().
	if !d.TakePendingRelatedDispatch() {
		t.Fatal("T010: after narrow→wide resize with m.ready==true, TakePendingRelatedDispatch() must return true")
	}
	// Flag must be cleared after the call.
	if d.TakePendingRelatedDispatch() {
		t.Fatal("T010: TakePendingRelatedDispatch() must return false on the second call (flag must be cleared)")
	}
}

// ---------------------------------------------------------------------------
// T011: TestDetailModel_FirstPaint_NoDoubleDispatch
// ---------------------------------------------------------------------------
// Given: a DetailModel with RelatedDefs registered
// When:
//  1. SetSize(80, 40) — first paint (m.ready was false, so pendingRelatedDispatch
//                       must NOT be set even though right col is auto-shown)
//  2. TakePendingRelatedDispatch()
//
// Then: returns false (first-paint dispatch is handled by NeedsRelatedCheck, not this flag).
// ---------------------------------------------------------------------------

func TestDetailModel_FirstPaint_NoDoubleDispatch(t *testing.T) {
	resource.RegisterRelated(resizeTestType, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer resource.UnregisterRelated(resizeTestType)

	d := makeResizeDetailModel()

	// First paint: m.ready is false, so the pending flag must NOT be set.
	d.SetSize(80, 40)

	if d.TakePendingRelatedDispatch() {
		t.Fatal("T011: first-paint SetSize must NOT set pendingRelatedDispatch; TakePendingRelatedDispatch() must return false")
	}
}
