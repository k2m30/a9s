package unit_test

// canonical_ref_conformance_test.go — the class guard over the whole demo
// bench. Every ID a related row counts, and every ID Enter on a navigable
// field opens, must be a row of the target type's list; anything else is a
// badge that drills into nothing or a field that opens nothing.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestRefConformance_RelatedIDsAreTargetRows runs every registered related
// checker on every demo row and requires each returned ID to be a row ID of
// the target's fixture list.
func TestRefConformance_RelatedIDsAreTargetRows(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	ctx := context.Background()

	checked := 0
	var bad []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			for _, res := range b.byType[td.ShortName] {
				for _, id := range def.Checker(ctx, clients, res, b.cache).ResourceIDs() {
					checked++
					if !b.has(def.TargetType, id) {
						bad = append(bad, fmt.Sprintf("%s → %s (%s): source %q returned %q",
							td.ShortName, def.TargetType, def.DisplayName, res.ID, id))
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no related checker returned an ID on the demo bench")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d of %d related IDs are not rows of their target type:\n  %s", len(bad), checked, strings.Join(bad, "\n  "))
	}
}

// TestRefConformance_NavigableFieldsOpenTargetRows opens every demo row's detail
// as the app renders it and requires Enter on each navigable row to hand the
// navigation a row ID of the target's list, and that ID to be the one
// resource.NavIDFromValue gives the same value — one reading for both.
//
// ct-events is left out: its navigable rows are the event body's own
// principal and target, not registered NavigableFields, and they name
// resources as they were when the call was made.
func TestRefConformance_NavigableFieldsOpenTargetRows(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)

	checked := 0
	var bad []string
	for _, td := range resource.AllResourceTypes() {
		if td.ShortName == "ct-events" {
			continue
		}
		for _, res := range b.byType[td.ShortName] {
			for _, f := range openDetail(c, td.ShortName, res) {
				if !f.IsNavigable {
					continue
				}
				checked++
				value := strings.TrimPrefix(strings.TrimSpace(f.Value), "- ")
				opens := navTarget(f)
				resolved := resource.NavIDFromValue(f.TargetType, value, b.rc(f.TargetType))
				switch {
				case !b.has(f.TargetType, opens):
					bad = append(bad, fmt.Sprintf("%s.%s → %s: source %q value %q opens %q, not a %s row",
						td.ShortName, f.Path, f.TargetType, res.ID, value, opens, f.TargetType))
				case opens != resolved:
					bad = append(bad, fmt.Sprintf("%s.%s → %s: source %q value %q opens %q but NavIDFromValue gives %q",
						td.ShortName, f.Path, f.TargetType, res.ID, value, opens, resolved))
				}
			}
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		}
	}
	if checked == 0 {
		t.Fatal("no navigable row rendered on the demo bench")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d of %d navigable rows open no row of their target type:\n  %s", len(bad), checked, strings.Join(bad, "\n  "))
	}
}
