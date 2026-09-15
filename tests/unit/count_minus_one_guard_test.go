package unit

// count_minus_one_guard_test.go — originally an AST-based guard ensuring that
// raw `Count: -1` struct literals in core/aws/*_related*.go only appeared
// in legitimately-guarded positions (nil checks, error checks, type-assertion
// failures, or FetchFilter-navigation paths); any `Count: -1` NOT inside one
// of those guards was the anti-pattern purged in Batch B.
//
// Task #58 replaced the Count==-1 sentinel with the domain.RelatedRowState
// enum, and every producer in internal/ now routes through a state
// constructor (UnknownRelated / ErrorRelated / DeferredRelated /
// LoadingRelated) — there are no more `RelatedCheckResult{Count: -1}`
// literals left in production code to scan for. The AST guard this file used
// to carry (TestNoCountMinusOneInReverseScanCheckers, scoped narrowly to
// truncated-condition Count:-1 in core/aws/*_related*.go) is fully
// subsumed by the comprehensive gate in qa_related_state_no_sentinel_gate_test.go,
// which scans ALL of internal/ for a negative Count/count literal on any of
// the four related result/row types (RelatedCheckResult / DetailRelatedRow /
// RelatedBlock / rightColumnRow), not just the truncated-if-guarded subset —
// so it was deleted here as redundant rather than kept permanently vacuous.
//
// The two tests below are unrelated to the sentinel encoding — they pin
// relatedResultTrunc's shape and its ValidateRelatedResult validity,
// both still-current, unchanged contracts — so they are kept.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Test 2: TruncatedResult result passes ValidateRelatedResult
// ---------------------------------------------------------------------------

// TestValidateRelatedResult_TruncatedResult_IsValid asserts that the result
// produced by TruncatedResult satisfies the ValidateRelatedResult invariants.
// This pins the contract: Truncated=true + Count=0 must be a valid state.
func TestValidateRelatedResult_TruncatedResult_IsValid(t *testing.T) {
	result := resource.KnownRelated("vpc", nil, true)
	if err := resource.ValidateRelatedResult(result); err != nil {
		t.Errorf("ValidateRelatedResult(TruncatedResult(\"vpc\")) returned error: %v; want nil", err)
	}
}
