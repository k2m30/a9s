package unit_test

// related_error_actionable_test.go — regression pin for the P2 the RelatedRowState
// migration introduced: an errored related-resource result must not be navigable.
//
// A checker can return a PARTIAL success — a positive Count with real
// ResourceIDs — alongside an aggregate Err (the lambda→eb-rule checker does
// exactly this when some ListTargetsByRule calls fail). It leaves State at the
// zero value (RelatedResolved) because it did resolve a partial count. The row
// renders as an error via Err, but the enum-only IsRelatedActionable(State,...)
// switch treated RelatedResolved + Count>0 as actionable, so Enter/click could
// still navigate off an error row. EffectiveState() folds Err back into the
// state so the error dominates.

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelatedErrorResult_NotActionable_EvenWithPositiveCount(t *testing.T) {
	// What the lambda→eb-rule checker returns on partial failure: 3 rules
	// resolved, but an aggregate error, State left at the zero value.
	errored := resource.RelatedCheckResult{
		TargetType:  "eb-rule",
		State:       domain.RelatedResolved, // zero value — the checker never set it
		Count:       3,
		ResourceIDs: []string{"rule-a", "rule-b", "rule-c"},
		Err:         errors.New("2/5 ListTargetsByRule calls failed"),
	}

	// The bug this pins: reading State DIRECTLY makes the errored row actionable.
	if !resource.IsRelatedActionable(errored.State, errored.Count, errored.Approximate) {
		t.Fatal("precondition changed: a zero-value Resolved state with Count>0 is expected to read as actionable — this is the trap EffectiveState guards")
	}

	// The fix: Err dominates the disposition.
	if got := errored.EffectiveState(); got != domain.RelatedError {
		t.Errorf("EffectiveState() = %v, want RelatedError (Err must win over a zero-value Resolved state)", got)
	}
	if resource.IsRelatedActionable(errored.EffectiveState(), errored.Count, errored.Approximate) {
		t.Errorf("an errored related result must not be actionable even with Count=%d > 0", errored.Count)
	}

	// A clean resolved result with the same count is still actionable — the fix
	// only blocks the errored case.
	clean := resource.RelatedCheckResult{TargetType: "eb-rule", Count: 3, ResourceIDs: []string{"a", "b", "c"}}
	if !resource.IsRelatedActionable(clean.EffectiveState(), clean.Count, clean.Approximate) {
		t.Errorf("a clean resolved result with Count>0 must stay actionable")
	}

	// A result already built as RelatedError is unaffected by EffectiveState.
	explicit := resource.ErrorRelated("eb-rule", errors.New("boom"))
	if got := explicit.EffectiveState(); got != domain.RelatedError {
		t.Errorf("EffectiveState() on an ErrorRelated result = %v, want RelatedError", got)
	}
}
