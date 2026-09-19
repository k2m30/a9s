package unit

// Ctrl+R on a top-level list must NOT
// clear the active ResourceListModel's findings immediately — they stay
// applied (stale-until-replaced) until the rerun's fresh EnrichmentChecked
// result actually lands, avoiding a user-visible flicker across the full
// AWS round-trip. Both halves are pinned: no blank window before the fresh
// result lands, and real removal once it genuinely omits the finding.
//
// The identity-column marker under horizontal scroll (resolveIdentityColumn
// running on the full pre-hscroll column list) is pinned by
// list_ports_test.go's
// TestWave3IdentityColParity_EnrichmentFindingsWithHScroll_AllResourceTypes.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// -----------------------------------------------------------------------------
// Ctrl+R keeps findings visible until the fresh result lands, rather than
// blanking them at keypress time
// -----------------------------------------------------------------------------

// isVisibleUnderCtrlZ toggles the ctrl+z attention filter on m, reports
// whether needle is visible, and toggles it back off. A row carrying a
// finding is never ColorHealthy, so colorEC2 (colorFromAnyFinding-only,
// core/aws/catalog_compute.go) draws no "! "/"~ " glyph for it; ctrl+z
// survival is the renderer-agnostic check for an applied issue row.
func isVisibleUnderCtrlZ(m tui.Model, needle string) (tui.Model, bool) {
	m, _ = rootApplyMsg(m, ctrlZ())
	visible := strings.Contains(stripANSI(m.View().Content), needle)
	m, _ = rootApplyMsg(m, ctrlZ())
	return m, visible
}

func TestCtrlR_RetainsActiveListFindingsUntilFreshEnrichment(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	resources := rerunEC2Resources()
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources, Provenance: messages.FetchProvenanceCanonicalList,
	})

	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))

	var visible bool
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Fatal("pre-condition failed: expected web-server-1 to survive ctrl+z (finding applied) before Ctrl+R")
	}

	// ec2 has a registered Wave-2 enricher, so Ctrl+R bumps
	// EnrichmentTypeGen["ec2"] from 0 to 1; the rerun's EnrichmentChecked
	// carries that value through the per-type generation guard
	// (core/runtime/handlers_availability.go).
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// No fetch or enrichment response has landed yet, so the old finding is
	// still the best-known truth.
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Error("Ctrl+R must NOT blank the active list's findings before the fresh result lands; web-server-1 no longer survives ctrl+z immediately after keypress")
	}

	// The fresh result carries the generation the rerun produced (TypeGen=1)
	// and omits the finding. TypeGen=0 would prove nothing: the per-type
	// generation guard treats TypeGen=0 as "not a tracked rerun" and accepts
	// it regardless of the real EnrichmentTypeGen value.
	recovered := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings:     map[string][]domain.Finding{},
		Gen:          0,
		TypeGen:      1,
	}
	m, _ = rootApplyMsg(m, recovered)

	_, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if visible {
		t.Error("a finding absent from the fresh, generation-matched enrichment result must be removed once that result lands; web-server-1 still survives ctrl+z")
	}
}
