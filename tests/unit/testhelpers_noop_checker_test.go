package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// noopChecker is a RelatedChecker that returns zero results. Use it in
// RelatedDef structs when the test only needs the def to be non-nil
// (e.g. to trigger right-column rendering) but doesn't need real data.
var noopChecker resource.RelatedChecker = func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{Count: 0}
}

// unregisterEC2Related masks ec2 related defs with an empty slice for the
// duration of t and restores the prior state on cleanup. Uses SetRelatedForTest
// to push a new snapshot frame rather than popping the stack — CleanupRelatedForTest
// would restore the previous production registration rather than clearing defs.
func unregisterEC2Related(t *testing.T) {
	t.Helper()
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })
}

// replaceEC2Related registers defs for "ec2" and restores the originals on
// cleanup so tests that temporarily override related defs don't leave the
// registry poisoned for subsequent tests running in shuffled order.
func replaceEC2Related(t *testing.T, defs []resource.RelatedDef) {
	t.Helper()
	resource.SetRelatedForTest("ec2", defs)
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })
}

// replaceEC2NavigableFields registers navigable fields for "ec2" and restores
// the prior ACTIVE registry state on cleanup. Snapshots the ACTIVE registry
// (not the merged one) so an empty active stays empty after the test —
// otherwise we'd promote production DEFAULT fields into ACTIVE and poison
// later tests that check active-only navigability.
func replaceEC2NavigableFields(t *testing.T, fields []resource.NavigableField) {
	t.Helper()
	orig := resource.GetActiveNavigableFields("ec2")
	resource.SetNavigableFieldsForTest("ec2", fields)
	t.Cleanup(func() {
		if orig == nil {
			resource.CleanupNavigableFieldsForTest("ec2")
		} else {
			resource.SetNavigableFieldsForTest("ec2", orig)
		}
	})
}

// unregisterEC2NavigableFields strips ec2 navigable fields from the ACTIVE
// registry for the duration of t and restores them on cleanup. Used by tests
// that need a clean "no active navigable fields" state for ec2.
func unregisterEC2NavigableFields(t *testing.T) {
	t.Helper()
	orig := resource.GetActiveNavigableFields("ec2")
	resource.CleanupNavigableFieldsForTest("ec2")
	t.Cleanup(func() {
		if orig == nil {
			resource.CleanupNavigableFieldsForTest("ec2")
		} else {
			resource.SetNavigableFieldsForTest("ec2", orig)
		}
	})
}
