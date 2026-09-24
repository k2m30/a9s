// A related checker that panics is a defect in that pivot. RunRelatedDef
// (core/runtime/executor.go) recovers it on the TUI lane and reports it as
// that pivot's error — Result is the pivot's
// ErrorRelated, which Core.HandleRelatedCheckResult announces as
// "related <type>: …" — and no by-ID fetch ran, so LazyAddError stays nil.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestRelatedCheckCmd_CheckerPanic_IsThatPivotsError registers a related
// checker that panics, drives it through the TUI's related-check fan-out (by
// opening the resource's detail view, which begins a DetailOperation and
// dispatches the related-check task directly), and asserts the recovered
// result is the pivot's error naming the panic.
func TestRelatedCheckCmd_CheckerPanic_IsThatPivotsError(t *testing.T) {
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

	m := newBlessedModel(t, "testprofile", "us-east-1", tui.WithNoCache(true))
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

	if resultMsg.LazyAddError != nil {
		t.Errorf("LazyAddError = %v: no by-ID fetch ran, the checker panicked", resultMsg.LazyAddError)
	}
	got := resultMsg.Result
	if got.TargetType() != targetType || got.EffectiveState() != domain.RelatedError || got.Err() == nil {
		t.Fatalf("Result = state %v target %q err %v, want the %s pivot's error", got.EffectiveState(), got.TargetType(), got.Err(), targetType)
	}
	if !strings.Contains(got.Err().Error(), "boom") {
		t.Errorf("Result error = %q, want it to name the panic value %q", got.Err().Error(), "boom")
	}
	if len(got.ResourceIDs()) != 0 {
		t.Errorf("Result IDs = %v, want none: a panicked checker counted nothing", got.ResourceIDs())
	}
}
