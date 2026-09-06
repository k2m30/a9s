package unit_test

// views_detail_rightcol_hide_zeros_test.go — whether a zero-resolved pivot row
// reaches the right column.
//
// The four ct-events self-pivot rows (CT events by AccessKeyId / Username /
// EventName / SharedEventId) are filters, not counts. Each returns either a
// navigation-mode Deferred result carrying a FetchFilter, or — when the field
// the filter would use is absent — a resolved zero. The resolved-zero case is
// the one under test: a filter that would return nothing is not a row worth
// offering, so it must not render as "(0)".
//
// Typed groups from other resource types (e.g. "EC2 Instances (0)") are
// deliberately allowed to render a zero: there the zero is the answer.
//
// The two tests here replace a pair that were skipped pending a cold-cache
// harness. They now drive the real checkers through the Controller the app
// itself builds the right column from, so the behaviour has a live test rather
// than a deferred one.

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rel2SelfPivotNames are the four ct-events rows that are filters rather than
// counts.
var rel2SelfPivotNames = []string{
	"CT events by AccessKeyId",
	"CT events by Username",
	"CT events by EventName",
	"CT events by SharedEventId",
}

// rel2RightColumnBlocks runs every registered ct-events checker against the
// fixture and returns the right column the Controller renders from them.
func rel2RightColumnBlocks(t *testing.T, fixture resource.Resource) []app.RelatedBlock {
	t.Helper()
	c := buildCTEventsRightColController(t, fixture)
	for _, def := range resource.GetRelated("ct-events") {
		if def.Checker == nil {
			continue
		}
		result := def.Checker(context.Background(), nil, fixture, resource.ResourceCache{})
		errMsg := ""
		if result.Err() != nil {
			errMsg = result.Err().Error()
		}
		c.ApplyDetailRelatedResultForResource("ct-events", fixture.ID, def.DisplayName, def.TargetType,
			result.EffectiveState(), result.Count(), false, errMsg, result.Truncated(),
			result.ResourceIDs(), result.FetchFilter())
	}
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after applying the checker results")
	}
	return body.Related
}

// TestCtEventsSelfPivotZeroRowsAreNotOffered asserts the narrow rule across
// every demo fixture: a self-pivot row whose checker resolved zero is not
// presented as a countable row. A "(0)" there invites the operator to open a
// filter that is already known to match nothing.
func TestCtEventsSelfPivotZeroRowsAreNotOffered(t *testing.T) {
	fixtures := loadAllCTFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("no ct-events demo fixtures")
	}

	checked := 0
	for _, fixture := range fixtures {
		blocks := rel2RightColumnBlocks(t, fixture)
		for _, def := range resource.GetRelated("ct-events") {
			if def.Checker == nil || !rel2IsSelfPivot(def.DisplayName) {
				continue
			}
			r := def.Checker(context.Background(), nil, fixture, resource.ResourceCache{})
			if r.EffectiveState() != domain.RelatedResolved || r.Count() != 0 {
				continue
			}
			checked++
			if i := ctRelatedRowIndex(blocks, def.DisplayName); i >= 0 && blocks[i].Actionable {
				t.Errorf("%s: %q resolved zero and is still offered as actionable", fixture.ID, def.DisplayName)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no self-pivot checker resolved zero across the demo fixtures; the rule under test was never exercised")
	}
}

// rel2IsSelfPivot reports whether a ct-events related row is one of the four
// filters rather than a typed group.
func rel2IsSelfPivot(displayName string) bool {
	for _, n := range rel2SelfPivotNames {
		if n == displayName {
			return true
		}
	}
	return false
}

// TestCtEventsTypedZeroRowsStillRender is the counterpart that keeps the rule
// narrow: a typed group from another resource type renders its zero, because
// there the zero is the answer an operator came for.
func TestCtEventsTypedZeroRowsStillRender(t *testing.T) {
	fixtures := loadAllCTFixtures(t)
	selfPivot := make(map[string]bool, len(rel2SelfPivotNames))
	for _, n := range rel2SelfPivotNames {
		selfPivot[n] = true
	}

	for _, fixture := range fixtures {
		for _, b := range rel2RightColumnBlocks(t, fixture) {
			if selfPivot[b.Name] || b.State != domain.RelatedResolved || b.Count != 0 {
				continue
			}
			return // a typed zero row is present, which is what this pins
		}
	}
	t.Fatal("no typed zero row rendered across the demo fixtures; either the demo lost its zero-resolved pivots or they are being suppressed too")
}
