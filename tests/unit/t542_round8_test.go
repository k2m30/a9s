package unit_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
)

// A subscription with no ARN yet (pending confirmation) has no name CloudTrail
// records it under, so its detail offers no CloudTrail Events row at all; a
// confirmed subscription keeps the row.
func TestSNSSubDetail_CloudTrailRowOnlyWithASubscriptionARN(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)
	var pending, confirmed int
	for _, sub := range b.byType["sns-sub"] {
		openDetail(c, "sns-sub", sub)
		shown := false
		for _, blk := range c.Snapshot().Body.Detail.Related {
			shown = shown || blk.Name == "CloudTrail Events"
		}
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		wantShown := strings.HasPrefix(sub.ID, "arn:")
		if wantShown {
			confirmed++
		} else {
			pending++
		}
		if shown != wantShown {
			t.Errorf("subscription %s: CloudTrail Events row shown = %v, want %v", sub.ID, shown, wantShown)
		}
	}
	if pending == 0 || confirmed == 0 {
		t.Fatalf("demo bench has %d subscriptions without an ARN and %d with one; both are needed", pending, confirmed)
	}
}

// The demo's security-group and Lambda CloudTrail events name their resource
// in resources[], so their TARGET row opens that row.
func TestDemoCTEvents_SGAndLambdaTargetsAreLinks(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)
	for id, want := range map[string][2]string{
		"evt-sg-web-alb-authorize-001":        {"sg", "sg-0aaa111111111111a"},
		"evt-lambda-data-pipeline-update-001": {"lambda", "data-pipeline-transform"},
	} {
		fields := openDetail(c, "ct-events", b.row(t, "ct-events", id))
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		i := slices.IndexFunc(fields, t542IsTargetRow)
		if i < 0 {
			t.Fatalf("%s: no TARGET row", id)
		}
		if f := fields[i]; !f.IsNavigable || f.TargetType != want[0] || navTarget(f) != want[1] || !b.has(want[0], want[1]) {
			t.Errorf("%s: TARGET %s %q navigable=%v type=%q opens %q, want %s %q", id, f.Key, f.Value, f.IsNavigable, f.TargetType, navTarget(f), want[0], want[1])
		}
	}
}
