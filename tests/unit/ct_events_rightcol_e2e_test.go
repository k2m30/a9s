package unit_test

// ct_events_rightcol_e2e_test.go — Layer 5 end-to-end tests for ct-events
// right-column row activation feeding into the root tui.Model.
//
// These tests need rewrite onto the cold-cache harness (T047-T049, Phase 5 rewrite).

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// TestCtEventsRightColumnEndToEnd verifies that RelatedNavigateMsgs produced
// by the right-column row activation path are handled correctly by the root
// tui.Model.
//
// E1: Typed rows (Count>0) — must not produce FlashMsg error.
// E2: Pivot rows (Count=-1, FetchFilter non-empty) — must push a new view.
// E3: Unknown TargetType — must produce FlashMsg with IsError=true.
func TestCtEventsRightColumnEndToEnd(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter verifies that for
// every ct-events fixture where the pivot checkers return Count=-1+FetchFilter,
// the full dispatch path pushes a new view (E2 exhaustive variant).
func TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}
