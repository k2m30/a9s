package unit

// qa_enrich_pipeline_dispatch_test.go — Regression test for the CodePipeline
// enrichment dispatch bug (issue #017-issue-counts-attention-filter).
//
// Root cause: Wave 1 stores CodePipeline probe resources under ShortName
// "pipeline" (core/resource/types_cicd.go:51), but buildEnrichQueue's
// order slice and the EnricherRegistry both use key "pipe". Result: buildEnrichQueue
// never sees "pipeline" in probeResources and the enricher is never dispatched.
//
// Fix: rename both "pipe" → "pipeline" in buildEnrichQueue's order slice
// (internal/tui/app_fetchers.go:537) and in EnricherRegistry
// (core/aws/pipeline_issue_enrichment.go).
//
// This test seeds probeResources["pipeline"] via AvailabilityCheckedMsg (the
// same path that Wave 1 uses at the end of the availability-probe cycle), then
// checks whether the returned cmd contains an EnrichmentCheckedMsg for "pipeline".
// It FAILS today because buildEnrichQueue returns an empty queue (key mismatch).
// It PASSES after the coder renames the key.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
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

// maxEnrichmentDrainIterations defensively bounds collectEnrichmentMsgs'
// redeliver-until-quiescence loop. A correct windowed startEnrichment
// dispatches at most one follow-up TaskKindProbeEnrich per EnrichmentChecked
// completion (core/runtime/handlers_availability.go's refill branch), so a
// full drain must terminate within roughly the total number of registered
// Wave2 enrichers (49 in the real catalog as of this writing). A refill bug
// that re-dispatches or never terminates fails the test instead of hanging
// the suite.
const maxEnrichmentDrainIterations = 200

// collectEnrichmentMsgs executes cmd (recursing through nested tea.BatchMsg
// to any depth) and collects every messages.EnrichmentChecked found.
//
// startEnrichment (core/runtime/handlers_availability.go) dispatches only a
// bounded initial window of TaskKindProbeEnrich tasks
// (enrichDispatchWindow=4); the remaining queued types are enriched only as
// each EnrichmentChecked completion is redelivered through the model and its
// window-refill branch (handleEnrichmentChecked) fires the next one.
// Executing cmd exactly once therefore only ever observes the first window's
// worth of results — this helper redelivers every collected
// EnrichmentChecked back through m via rootApplyMsg (the established
// send-a-message-through-Update pattern, see tests/unit/tui_root_test.go)
// and keeps collecting from whatever cmd that redelivery produces, until no
// new message is produced (quiescence).
//
// Unlike extractMsg, this helper does NOT call t.Fatal on a missing/short
// result — it just returns what it finds; it only fails the test if the
// drain fails to quiesce (see maxEnrichmentDrainIterations).
func collectEnrichmentMsgs(t *testing.T, m tui.Model, cmd tea.Cmd) []messages.EnrichmentChecked {
	t.Helper()
	var found []messages.EnrichmentChecked
	pending := extractEnrichmentChecked(cmd)

	for iterations := 0; len(pending) > 0; iterations++ {
		if iterations >= maxEnrichmentDrainIterations {
			t.Fatalf("collectEnrichmentMsgs: did not quiesce within %d iterations — suspect a refill loop (collected so far: %d)", maxEnrichmentDrainIterations, len(found))
		}
		msg := pending[0]
		pending = pending[1:]
		found = append(found, msg)

		var next tea.Cmd
		m, next = rootApplyMsg(m, msg)
		pending = append(pending, extractEnrichmentChecked(next)...)
	}
	return found
}

// extractEnrichmentChecked executes cmd (recursing through nested
// tea.BatchMsg to any depth) and returns every messages.EnrichmentChecked
// leaf found. Non-EnrichmentChecked messages are ignored — matching this
// helper's pre-windowing behavior, which only ever cared about this one
// message type.
func extractEnrichmentChecked(cmd tea.Cmd) []messages.EnrichmentChecked {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []messages.EnrichmentChecked
		for _, sub := range batch {
			out = append(out, extractEnrichmentChecked(sub)...)
		}
		return out
	}
	if ec, ok := msg.(messages.EnrichmentChecked); ok {
		return []messages.EnrichmentChecked{ec}
	}
	return nil
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
	m, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
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
	found := collectEnrichmentMsgs(t, m, cmd)

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
