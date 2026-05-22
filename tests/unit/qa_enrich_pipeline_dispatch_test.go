package unit

// qa_enrich_pipeline_dispatch_test.go — Regression test for the CodePipeline
// enrichment dispatch bug (issue #017-issue-counts-attention-filter).
//
// Root cause: Wave 1 stores CodePipeline probe resources under ShortName
// "pipeline" (internal/resource/types_cicd.go:51), but buildEnrichQueue's
// order slice and the EnricherRegistry both use key "pipe". Result: buildEnrichQueue
// never sees "pipeline" in probeResources and the enricher is never dispatched.
//
// Fix: rename both "pipe" → "pipeline" in buildEnrichQueue's order slice
// (internal/tui/app_fetchers.go:537) and in EnricherRegistry
// (internal/aws/pipeline_issue_enrichment.go).
//
// This test seeds probeResources["pipeline"] via AvailabilityCheckedMsg (the
// same path that Wave 1 uses at the end of the availability-probe cycle), then
// checks whether the returned cmd contains an EnrichmentCheckedMsg for "pipeline".
// It FAILS today because buildEnrichQueue returns an empty queue (key mismatch).
// It PASSES after the coder renames the key.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// pipelineProbeResources returns a minimal slice of fake CodePipeline resources
// suitable for seeding probeResources["pipeline"] in Wave 1 dispatch tests.
func pipelineProbeResources() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "my-deploy-pipeline",
			Name: "my-deploy-pipeline",
			Fields: map[string]string{
				"name":          "my-deploy-pipeline",
				"pipeline_type": "V2",
				"version":       "3",
			},
		},
	}
}

// collectEnrichmentMsgs executes a cmd (possibly a BatchMsg) and returns all
// EnrichmentCheckedMsg values found, up to two levels of nesting.
// Unlike extractMsg, this helper does NOT call t.Fatal — it just returns what it finds.
func collectEnrichmentMsgs(cmd tea.Cmd) []messages.EnrichmentChecked {
	if cmd == nil {
		return nil
	}
	var found []messages.EnrichmentChecked
	visit := func(msg tea.Msg) {
		if m, ok := msg.(messages.EnrichmentChecked); ok {
			found = append(found, m)
		}
	}
	top := cmd()
	visit(top)
	if batch, ok := top.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			subMsg := sub()
			visit(subMsg)
			if subBatch, ok := subMsg.(tea.BatchMsg); ok {
				for _, inner := range subBatch {
					if inner == nil {
						continue
					}
					innerMsg := inner()
					visit(innerMsg)
				}
			}
		}
	}
	return found
}

// TestBuildEnrichQueue_DispatchesCodePipeline verifies that when probeResources
// contains a "pipeline" entry (seeded by Wave 1), buildEnrichQueue includes
// "pipeline" in the queue and probeEnrichment is dispatched.
//
// FAILS today: "pipe" key in buildEnrichQueue order slice doesn't match the
// "pipeline" key stored in probeResources → empty queue → no dispatch.
// PASSES after fix: rename "pipe" → "pipeline" in order slice and EnricherRegistry.
func TestBuildEnrichQueue_DispatchesCodePipeline(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	// isDemo must be false (default) so startEnrichment is not skipped.

	// Deliver AvailabilityCheckedMsg to seed probeResources["pipeline"] and
	// trigger the availability-probe finalization path that calls startEnrichment.
	// availTotal starts at 0; after incrementing availChecked to 1, 1 >= 0 → finalize.
	// session.New seeds AvailabilityGen=1 (AS-659) — stamp the live value so
	// the AvailabilityChecked stale guard (AcceptZeroGen=false) accepts it.
	_, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "pipeline",
		Count:        1,
		Truncated:    false,
		Gen:          m.Core().Session().AvailabilityGen,
		Resources:    pipelineProbeResources(),
	})

	if cmd == nil {
		t.Fatal("AvailabilityCheckedMsg should return a non-nil cmd (at minimum a cache-save cmd)")
	}

	// Execute the returned cmd tree and look for EnrichmentCheckedMsg for "pipeline".
	// If buildEnrichQueue includes "pipeline", probeEnrichment is dispatched and will
	// return EnrichmentCheckedMsg{ResourceType: "pipeline", Err: "AWS clients not initialized"}.
	// If not dispatched (bug), no EnrichmentCheckedMsg is produced.
	found := collectEnrichmentMsgs(cmd)

	dispatched := false
	for _, msg := range found {
		if msg.ResourceType == "pipeline" {
			dispatched = true
			break
		}
	}

	if !dispatched {
		t.Errorf("buildEnrichQueue did not dispatch enrichment for %q; "+
			"expected EnrichmentCheckedMsg{ResourceType: \"pipeline\"} in cmd tree, "+
			"got %d EnrichmentCheckedMsg(s): %v\n"+
			"Likely cause: buildEnrichQueue order slice uses \"pipe\" instead of \"pipeline\"",
			"pipeline", len(found), found)
	}
}
