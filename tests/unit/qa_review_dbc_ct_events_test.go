package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws" // trigger init chain
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestReview_DBC_RegistersCTEvents confirms that ct-events is present in dbc's
// related-def list. Relevant to reviewer P2 claim that dbc lost ct-events.
// ct-events is auto-appended for every resource type by zzz_ct_events_all_related.go.
func TestReview_DBC_RegistersCTEvents(t *testing.T) {
	defs := resource.GetRelated("dbc")
	found := false
	for _, d := range defs {
		if d.TargetType == "ct-events" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(defs))
		for _, d := range defs {
			names = append(names, d.TargetType)
		}
		t.Errorf("dbc must register a ct-events related def; got %d defs: %v", len(defs), names)
	}
}
