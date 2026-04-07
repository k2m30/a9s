package unit

// TestCTEventsViewLayout_MatchesDesignSpec asserts the ct-events column layout
// in the built-in defaults matches §8 of docs/design/ct-event-list-v2.md exactly.
//
// This test catches any accidental width drift in defaults_monitoring.go that
// would not be caught by compilation. A width change (e.g., TIME from 15→19)
// is a silent regression — the code compiles but the layout violates the spec.
//
// §8 column spec:
//   V       width=1   key="_ct.verb"
//   TIME    width=15  key="time"
//   ACTOR   width=26  key="_ct.actor"
//   ORIGIN  width=7   key="_ct.origin"
//   EVENT   width=24  path="EventName"
//   TARGET  width=36  key="_ct.target"
//   OUTCOME width=14  key="_ct.outcome"

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
)

func TestCTEventsViewLayout_MatchesDesignSpec(t *testing.T) {
	cfg := config.DefaultConfig()
	vd := config.GetViewDef(cfg, "ct-events")

	wantCols := []struct {
		title string
		width int
	}{
		{"V", 1},
		{"TIME", 15},
		{"ACTOR", 26},
		{"ORIGIN", 7},
		{"EVENT", 24},
		{"TARGET", 36},
		{"OUTCOME", 14},
	}

	if len(vd.List) != len(wantCols) {
		t.Fatalf("ct-events list has %d columns, want %d; column count must match §8 spec exactly",
			len(vd.List), len(wantCols))
	}

	for i, want := range wantCols {
		got := vd.List[i]
		if got.Title != want.title {
			t.Errorf("col %d: title = %q, want %q (§8 column order must be V/TIME/ACTOR/ORIGIN/EVENT/TARGET/OUTCOME)",
				i, got.Title, want.title)
		}
		if got.Width != want.width {
			t.Errorf("col %d (%s): width = %d, want %d (§8 exact widths required)",
				i, want.title, got.Width, want.width)
		}
	}
}

func TestCTEventsViewLayout_DetailFieldsMatchDesignSpec(t *testing.T) {
	// §8 detail fields: EventId, EventName, EventTime, EventSource, Username,
	// ReadOnly, AccessKeyId, Resources, CloudTrailEvent.
	cfg := config.DefaultConfig()
	vd := config.GetViewDef(cfg, "ct-events")

	wantDetail := []string{
		"EventId", "EventName", "EventTime", "EventSource",
		"Username", "ReadOnly", "AccessKeyId",
		"Resources", "CloudTrailEvent",
	}

	if len(vd.Detail) != len(wantDetail) {
		t.Fatalf("ct-events detail has %d fields, want %d", len(vd.Detail), len(wantDetail))
	}

	for i, want := range wantDetail {
		if vd.Detail[i] != want {
			t.Errorf("detail field %d = %q, want %q (§8 detail field list)", i, vd.Detail[i], want)
		}
	}
}
