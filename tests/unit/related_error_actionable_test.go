package unit_test

// related_error_actionable_test.go — an errored related-resource result must
// not be navigable.
//
// RelatedCheckResult's fields are unexported, buildable only via
// KnownRelated/UnknownRelated/ErrorRelated/DeferredRelated, and none of
// those can express Count>0 together with Err!=nil; the eb-rule checker
// (core/aws/lambda_related.go) returns KnownRelated(ids, truncated=true) on
// a partial ListTargetsByRule failure. The two sub-cases below (a clean
// resolved result, and an explicit ErrorRelated result) pin
// EffectiveState() behavior.

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
