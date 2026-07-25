package unit_test

// related_error_actionable_test.go — regression pin for the P2 the RelatedRowState
// migration introduced: an errored related-resource result must not be navigable.
//
// This test used to hand-construct a RelatedCheckResult with State left at its
// zero value (RelatedResolved), a positive Count, AND a non-nil Err
// simultaneously — the shape core/aws/lambda_related.go's eb-rule checker
// produced on a partial ListTargetsByRule failure. RelatedCheckResult's fields
// are now unexported, buildable only via KnownRelated/UnknownRelated/
// ErrorRelated/DeferredRelated, and none of those can express Count>0 together
// with Err!=nil — so that exact literal is no longer constructible from
// tests/unit (or from core/aws itself: the real eb-rule checker was migrated
// alongside this change to return KnownRelated(ids, truncated=true) on a
// partial ListTargetsByRule failure instead of a Count+Err combination,
// retiring the scenario this test existed to catch). The remaining two
// sub-cases below (a clean resolved result, and an explicit ErrorRelated
// result) are still constructible and still pin real EffectiveState()
// behavior.

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestRelatedErrorResult_NotActionable_EvenWithPositiveCount(t *testing.T) {
	// A clean resolved result with a positive count is actionable.
	clean := resource.KnownRelated("eb-rule", []string{"a", "b", "c"}, false)
	if !resource.IsRelatedActionable(clean.EffectiveState(), clean.Count(), clean.Truncated()) {
		t.Errorf("a clean resolved result with Count>0 must stay actionable")
	}

	// A result already built as RelatedError is unaffected by EffectiveState.
	explicit := resource.ErrorRelated("eb-rule", errors.New("boom"))
	if got := explicit.EffectiveState(); got != domain.RelatedError {
		t.Errorf("EffectiveState() on an ErrorRelated result = %v, want RelatedError", got)
	}
	if resource.IsRelatedActionable(explicit.EffectiveState(), explicit.Count(), explicit.Truncated()) {
		t.Errorf("an ErrorRelated result must not be actionable")
	}
}
