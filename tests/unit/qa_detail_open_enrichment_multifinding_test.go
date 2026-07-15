// qa_detail_open_enrichment_multifinding_test.go — regression pin for a P2 bug
// found by Codex in the v3.47.0 landing (core/runtime/handlers_availability.go
// around the PatchDetail construction in handleEnrichmentChecked).
//
// handleEnrichmentChecked folds allFindings (map[string][]domain.Finding — every
// independently-evaluated Wave-2 condition per resource) onto cached rows via
// applyEnrichment/AmendRows. The bug (now fixed) was that the PatchDetail
// intent emitted for ALREADY-OPEN detail views used to be built from a single
// worst-severity representative Finding per resource instead of the full
// per-resource slice:
//
//	intents = append(intents, PatchDetail{
//	    ResourceType:               msg.ResourceType,
//	    EnrichmentFindings:         allFindings,           // now: every finding, not one
//	    EnrichmentAttentionDetails: msg.AttentionDetails,
//	})
//
// core/app/intents.go's PatchDetail case iterates the per-resource slice
// and calls applyDetailFindingsForResource with every finding
// (core/app/detail_state.go), so a resource that is ALREADY OPEN in a
// detail view when a multi-finding EnrichmentChecked result arrives shows
// every independently-evaluated condition in its Attention block, not just
// the worst one.
//
// This mirrors qa_wave2_multifinding_test.go's Section 1 open-detail pattern
// (PushScreen{ScreenDetail} + EnsureDetailState + read Body.Detail.Fields for
// Path=="Attention" rows) but drives the REAL handleEnrichmentChecked path via
// the public Controller.Handle seam instead of hand-folding through
// runtime.ApplyWave2ToRow, so it pins the PatchDetail plumbing specifically
// (a different seam than #52's enricher-level fold, which this file assumes
// already fixed — the per-resource slice is what a real post-#52 enricher now
// populates on messages.EnrichmentChecked.Findings, the sole plural
// representation since the legacy-purge rename retired the single-Finding
// compat field of the same name).
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

const detailOpenMultiFindingResourceID = "ecs-svc-detail-open-multi"

var (
	detailOpenMultiFindingBroken = domain.Finding{
		Code:     domain.FindingCode("ecs-svc.deployment-failed"),
		Phrase:   "deployment failed",
		Detail:   "The service's latest deployment rolled back after failing health checks.",
		Severity: domain.SevBroken,
		Source:   "wave2:ecs-svc",
	}
	detailOpenMultiFindingWarn = domain.Finding{
		Code:     domain.FindingCode("ecs-svc.task-def-drift"),
		Phrase:   "task definition drifted from desired revision",
		Detail:   "Running tasks reference an older task definition revision than the service's desired one.",
		Severity: domain.SevWarn,
		Source:   "wave2:ecs-svc",
	}
)

// TestHandleEnrichmentChecked_DetailAlreadyOpen_MultiFinding_BothFindingsReachAttention
// opens a detail view for a resource BEFORE its Wave-2 result arrives (mirroring
// a user who navigated to the detail while enrichment was still in flight), then
// drives a real messages.EnrichmentChecked carrying TWO independently-evaluated
// findings for that resource through Controller.Handle — the same public seam
// production wires EnrichmentChecked results through for both the TUI and
// web/headless. Both findings must reach the open detail's Attention block.
func TestHandleEnrichmentChecked_DetailAlreadyOpen_MultiFinding_BothFindingsReachAttention(t *testing.T) {
	ctrl := newTestController(t)

	res := resource.Resource{
		ID:   detailOpenMultiFindingResourceID,
		Name: detailOpenMultiFindingResourceID,
		Type: "ecs-svc",
	}
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(res, "ecs-svc")

	preSnap := ctrl.Snapshot()
	if preSnap.Body.Detail == nil {
		t.Fatal("test setup: Body.Detail is nil after EnsureDetailState — cannot exercise the already-open path")
	}

	snap, _ := ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "ecs-svc",
		Issues:       1,
		Truncated:    false,
		// Findings carries every independently-evaluated condition — what a
		// real enricher's IssueEnricherResult.Findings now holds per-resource
		// (map[string][]domain.Finding, #52).
		Findings: map[string][]domain.Finding{
			detailOpenMultiFindingResourceID: {detailOpenMultiFindingBroken, detailOpenMultiFindingWarn},
		},
	})

	body := snap.Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after Handle(EnrichmentChecked) for an already-open detail")
	}

	var attentionText []string
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			attentionText = append(attentionText, f.Value)
		}
	}
	// Case-insensitive: buildAttentionEntries capitalizes the first letter for
	// display. The contract pinned here is that BOTH findings' phrases survive
	// the PatchDetail round-trip onto an already-open detail, not exact casing.
	joined := strings.ToLower(strings.Join(attentionText, " | "))

	if !strings.Contains(joined, strings.ToLower(detailOpenMultiFindingBroken.Phrase)) {
		t.Fatalf("test setup: expected the worst finding %q on the open detail's Attention block; got rows: %v", detailOpenMultiFindingBroken.Phrase, attentionText)
	}
	if !strings.Contains(joined, strings.ToLower(detailOpenMultiFindingWarn.Phrase)) {
		t.Errorf("BUG: open detail's Attention block is missing the second finding %q (got rows: %v) — core/runtime/handlers_availability.go's PatchDetail construction in handleEnrichmentChecked passes msg.Findings (the single worst-severity Finding per resource) as EnrichmentFindings instead of allFindings (the full per-resource []domain.Finding slice), so core/app/intents.go's PatchDetail case (and applyFindingToState in core/app/detail_state.go) only ever receive and apply ONE finding for a resource whose detail is already open — the second independently-evaluated condition never reaches the live view until the user closes and reopens the detail.",
			detailOpenMultiFindingWarn.Phrase, attentionText)
	}
}
