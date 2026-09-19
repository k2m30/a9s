package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

// TestCtEventsRightColumnEndToEnd verifies that RelatedNavigateMsgs produced
// by the right-column row activation path are handled correctly by the root
// tui.Model.
//
// E1: Typed rows (Count>0) — must not produce FlashMsg error.
// E2: Pivot rows (FetchFilter non-empty) — must push a new view.
// E3: Unknown TargetType — must produce FlashMsg with IsError=true.
func TestCtEventsRightColumnEndToEnd(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter verifies that for
// every ct-events fixture where the pivot checkers return a FetchFilter,
// the full dispatch path pushes a new view (E2 exhaustive variant).
func TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}
