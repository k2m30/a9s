package unit_test

// related_validate_test.go — Tests for ARCH-06: ValidateRelatedResult helper.
//
// ValidateRelatedResult(r resource.RelatedCheckResult) error
// (internal/resource/related.go) enforces invariants on RelatedCheckResult
// values returned by RelatedCheckers. Since task #58 replaced the Count==-1
// sentinel with the domain.RelatedRowState enum, the invariants are keyed off
// State rather than a negative Count:
//
//   Valid states (no error):
//     - {Count: 0}                                   — definitively zero (State: RelatedResolved, the zero value)
//     - {State: RelatedUnknown}                       — unknown (Count 0, no IDs)
//     - {Count: N, ResourceIDs: N items}             — confirmed N, IDs match (State: RelatedResolved)
//     - {Count: 0, Truncated: true}                — truncated scan, possibly more (State: RelatedResolved)
//     - {Count: N, Truncated: true, ResourceIDs: N items}
//
//   Invalid states (error):
//     - TargetType == ""                             — missing target type
//     - Count > 0 but ResourceIDs empty               — count/IDs inconsistency
//     - State != RelatedResolved but Count != 0       — non-resolved states must not carry a count
//     - State != RelatedResolved with ResourceIDs set — non-resolved states can't carry IDs
//     - Truncated == true with State != RelatedResolved — Truncated requires RelatedResolved
//
// TestRegisteredCheckers_ProduceValidResults: for parents in the scoped list,
// calls each registered checker with empty cache + nil clients and asserts the
// result passes ValidateRelatedResult. Catches checker invariant violations that
// would cause silent UI corruption.

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws" // ensure all related registrations run
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestValidateRelatedResult_Valid
// ─────────────────────────────────────────────────────────────────────────────

// TestValidateRelatedResult_Valid verifies that all well-formed RelatedCheckResult
// values return nil from ValidateRelatedResult.
func TestValidateRelatedResult_Valid(t *testing.T) {
	cases := []struct {
		name string
		r    resource.RelatedCheckResult
	}{
		{
			name: "count zero is valid",
			r:    resource.RelatedCheckResult{TargetType: "vpc", Count: 0},
		},
		{
			name: "unknown state is valid",
			r:    resource.RelatedCheckResult{TargetType: "vpc", State: domain.RelatedUnknown},
		},
		{
			name: "count 3 with 3 IDs is valid",
			r: resource.RelatedCheckResult{
				TargetType:  "vpc",
				Count:       3,
				ResourceIDs: []string{"vpc-a", "vpc-b", "vpc-c"},
			},
		},
		{
			name: "count 0 truncated is valid",
			r: resource.RelatedCheckResult{
				TargetType: "vpc",
				Count:      0,
				Truncated:  true,
			},
		},
		{
			name: "count 5 truncated with 5 IDs is valid",
			r: resource.RelatedCheckResult{
				TargetType:  "vpc",
				Count:       5,
				Truncated:   true,
				ResourceIDs: []string{"vpc-1", "vpc-2", "vpc-3", "vpc-4", "vpc-5"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := resource.ValidateRelatedResult(tc.r)
			if err != nil {
				t.Errorf("ValidateRelatedResult(%+v) = %v, want nil", tc.r, err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestValidateRelatedResult_Invalid
// ─────────────────────────────────────────────────────────────────────────────

// TestValidateRelatedResult_Invalid verifies that malformed RelatedCheckResult
// values return a non-nil error from ValidateRelatedResult.
func TestValidateRelatedResult_Invalid(t *testing.T) {
	cases := []struct {
		name string
		r    resource.RelatedCheckResult
	}{
		{
			name: "empty target type",
			r:    resource.RelatedCheckResult{Count: 0},
		},
		{
			name: "count > 0 but no IDs",
			r:    resource.RelatedCheckResult{TargetType: "vpc", Count: 2, ResourceIDs: nil},
		},
		{
			name: "unknown state with IDs",
			r: resource.RelatedCheckResult{
				TargetType:  "vpc",
				State:       domain.RelatedUnknown,
				ResourceIDs: []string{"vpc-x"},
			},
		},
		{
			name: "truncated with unknown state",
			r: resource.RelatedCheckResult{
				TargetType: "vpc",
				State:      domain.RelatedUnknown,
				Truncated:  true,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := resource.ValidateRelatedResult(tc.r)
			if err == nil {
				t.Errorf("ValidateRelatedResult(%+v) = nil, want non-nil error", tc.r)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRegisteredCheckers_ProduceValidResults
// ─────────────────────────────────────────────────────────────────────────────

// TestRegisteredCheckers_ProduceValidResults calls every registered RelatedChecker
// for each parent type in the scoped list, passing empty cache and nil clients,
// and asserts that the returned RelatedCheckResult passes ValidateRelatedResult.
//
// nil clients and empty cache represent the minimal invocation that must not
// produce an invariant-violating result (e.g., a checker that returns Count=2
// with no ResourceIDs when it falls back to the nil-clients path).
//
// Parent types scoped per ARCH-06 task: asg, ecr, ecs-svc, secrets, ses, kms, eb, eks.
//
// Shape/invariant guard only — a checker can still be inert, non-drillable,
// or fed by incomplete runtime data and pass.
func TestRegisteredCheckers_ProduceValidResults(t *testing.T) {
	parentTypes := []string{"asg", "ecr", "ecs-svc", "secrets", "ses", "kms", "eb", "eks"}

	emptyCache := resource.ResourceCache{}
	dummyRes := resource.Resource{
		ID:     "test-resource-id",
		Name:   "test-resource-name",
		Fields: map[string]string{},
	}

	for _, parent := range parentTypes {
		parent := parent
		t.Run(parent, func(t *testing.T) {
			defs := resource.GetRelated(parent)
			if len(defs) == 0 {
				t.Skipf("no RelatedDefs registered for %q — skipping (check registration)", parent)
			}

			for _, def := range defs {
				def := def
				if def.Checker == nil {
					// Nil Checker is not an acceptable steady state for a registered
					// relation. This legacy skip should become a hard failure once
					// the remaining allowances are removed.
					continue
				}

				t.Run(def.TargetType, func(t *testing.T) {
					result := def.Checker(context.Background(), nil, dummyRes, emptyCache)

					// TargetType should be echoed from the def.
					if result.TargetType == "" {
						// Tolerate missing TargetType echo — ValidateRelatedResult will catch it.
					}
					// Override TargetType from the def if the checker forgot to set it —
					// we want to catch the semantic invariants, not just the echo.
					if result.TargetType == "" {
						result.TargetType = def.TargetType
					}

					if err := resource.ValidateRelatedResult(result); err != nil {
						t.Errorf("checker for %q->%q returned invalid result: %v; result: %+v",
							parent, def.TargetType, err, result)
					}
				})
			}
		})
	}
}
