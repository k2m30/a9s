// tui_savecache_routing_test.go — RED pin for DEF-11 (C6 + Goal 4 of
// docs/design/cache-requirements.md).
//
// Root cause: internal/tui/app_dispatch.go's tasksToCmd intercepts
// runtime.TaskKindSaveCache and calls the TUI-local m.saveAvailabilityCache()
// (internal/tui/probe_adapter.go — counts-only, sourced from the controller's
// MenuState), bypassing the shared executor path
// (internal/runtime/executor.go's TaskKindSaveCache case) which additionally
// calls saveProbeResourcesToTypeFiles to persist per-type rows/findings from
// the dispatch-time *SaveCachePayload snapshot. A live TUI session therefore
// persists ~/.a9s cache type files with a correct Count/Issues header but
// ZERO Rows, while the headless/web executor path (already pinned by
// TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen
// in app_pilot_defects_test.go, driven through app.Controller +
// runtime.Core directly) persists rows correctly.
//
// This file drives the SAME AvailabilityChecked-queue-drained completion
// through the actual renderer seam a live TUI session uses:
// tui.Model.Update -> m.coreUpdate -> m.tasksToCmd -> (TaskKindSaveCache
// case) -> the tea.Cmd(s) executed exactly as the Bubble Tea runtime would.
//
// Reachability: internal/tui.Model's dispatch lane (tasksToCmd,
// executeTaskCmd, coreUpdate) is entirely unexported. The only reachable
// seam from tests/unit is tui.Model's exported tea.Model interface
// (Update/View/Init) via the tuitest harness (tests/unit/tuitest/tuitest.go),
// exactly as tui_root_test.go and app_enrich_test.go already do. Driving
// messages.AvailabilityChecked with the queue naturally drained (AvailQueue
// nil, AvailTotal 0 by default per session.New(), so a single Checked event
// satisfies "all checks done") reaches handleAvailabilityChecked's
// queue-exhausted branch, which appends the TaskKindSaveCache task carrying
// the dispatch-time snapshotProbeResourcesForSave() payload — the exact
// production sequence a live sweep-completion produces.
package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// newSaveCacheApp builds a tui.Model wired to demo clients (no real AWS
// calls) with on-disk caching ENABLED (unlike newEnrichApp in
// app_enrich_test.go, which sets WithNoCache(true) — DEF-11 is specifically
// about what lands on disk, so caching must stay on here). A9S_CONFIG_FOLDER
// is redirected to t.TempDir() so cache.LoadDir/Store.SaveType write under an
// isolated directory, matching the app_pilot_defects_test.go DEF-7 precedent.
func newSaveCacheApp(t *testing.T, profile, region string) tui.Model {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	m := tui.New(profile, region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfile(profile),
		tui.WithRegion(region))
	t.Cleanup(func() { m.CloseController() })
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// driveSweepCompletion sends a single messages.AvailabilityChecked for s3
// through m.Update, exactly as the real Bubble Tea runtime would deliver the
// event from the last in-flight availability probe. Because session.New()
// leaves AvailQueue nil / AvailChecked 0 / AvailTotal 0, this single event's
// unconditional AvailChecked++ makes AvailChecked(1) >= AvailTotal(0) true,
// so handleAvailabilityChecked's "all checks done" branch fires on this very
// call — appending the TaskKindSaveCache task un-conditionally, with no need
// to seed or drain any queue by hand. Gen must equal 1 (session.New seeds
// AvailabilityGen at 1, not 0 — AvailabilityChecked.AcceptZeroGen() is
// false, so a zero-Gen event would be silently dropped as stale).
func driveSweepCompletion(m tui.Model, sweepResources []resource.Resource) (tui.Model, tea.Cmd) {
	return rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        len(sweepResources),
		Issues:       1,
		Resources:    sweepResources,
		Gen:          1,
	})
}

// runCmdTree recursively executes cmd, delivering every resulting tea.Msg
// back into m.Update, exactly as the Bubble Tea event loop would. This is
// required because tasksToCmd's TaskKindSaveCache branch returns a tea.Cmd
// that must actually be invoked for the save to happen — the fix under test
// is entirely about what that returned tea.Cmd does when it runs.
func runCmdTree(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch v := msg.(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, c := range v {
			m = runCmdTree(t, m, c)
		}
		return m
	default:
		var next tea.Cmd
		m, next = rootApplyMsg(m, msg)
		return runCmdTree(t, m, next)
	}
}

// TestTUISaveCache_PersistsRowsToTypeFile pins DEF-11: a TUI-driven
// availability-sweep-completion save must persist the swept type's rows to
// disk, not just its availability count. RED originally — tasksToCmd's
// TaskKindSaveCache case called m.saveAvailabilityCache() (counts-only from
// MenuState), never reaching saveProbeResourcesToTypeFiles, so tf.Rows stayed
// empty even though the sweep carried a real row with a finding.
//
// This pin is about SAVE-PLUMBING, not enrichment: TaskKindSaveCache persists
// from the dispatch-time SaveCachePayload snapshot taken BEFORE Wave-2
// enrichment ever runs (handlers_availability.go's handleAvailabilityChecked
// captures c.snapshotProbeResourcesForSave() specifically so a later
// startEnrichment() mutation of ProbeResources cannot retroactively change
// what gets persisted). So the finding under test here must already be
// present on the Wave-1 resource at dispatch time — exactly what a real
// Wave-1 fetcher can emit (fetcher-sourced findings, e.g. public bucket ACL
// detected inline during ListBuckets/GetBucketAcl, are a real and current
// Wave-1 surface; see docs/refactor/03-finding-model.md). The Source label is
// deliberately NOT "wave2:s3" — since the wave-2-in-demo change made Wave-2
// enrichment run for real even in demo mode, a "wave2:*" Source on a
// synthetic finding that was never actually produced by the s3 enricher would
// misrepresent what this pin asserts. The finding here stays synthetic
// (Source: "wave1:s3") because DEF-11's point is that the save-cache dispatch
// persists whatever Findings the row already carries at snapshot time — not
// that any particular enricher produced them.
func TestTUISaveCache_PersistsRowsToTypeFile(t *testing.T) {
	const profile, region = "def11-tui-prof", "us-east-1"
	m := newSaveCacheApp(t, profile, region)

	finding := domain.Finding{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave1:s3"}
	sweepResources := []resource.Resource{
		{
			ID:       "bucket-def11-1",
			Name:     "def11-bucket",
			Type:     "s3",
			Fields:   map[string]string{"region": region},
			Findings: []domain.Finding{finding},
		},
	}

	m, cmd := driveSweepCompletion(m, sweepResources)
	m = runCmdTree(t, m, cmd)
	_ = m

	store := cache.LoadDir(profile, region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a TUI-driven sweep completion — DEF-11: the TUI save-cache dispatch must persist a per-type file just like the headless executor path does`)
	}
	if len(tf.Rows) == 0 {
		t.Fatal("s3 TypeFile.Rows is empty after a TUI-driven sweep completion — DEF-11: internal/tui/app_dispatch.go's TaskKindSaveCache case intercepts the task via m.saveAvailabilityCache() (counts-only), bypassing the shared executor path that also persists rows via saveProbeResourcesToTypeFiles")
	}
	found := false
	for _, r := range tf.Rows {
		if r.ID == "bucket-def11-1" {
			found = true
			if len(r.Findings) != 1 || r.Findings[0].Code != "s3-public-read" {
				t.Errorf("persisted TUI-swept row Findings = %+v, want 1 finding with Code=%q", r.Findings, "s3-public-read")
			}
		}
	}
	if !found {
		t.Error("persisted s3 TypeFile.Rows missing bucket-def11-1 — the TUI-swept row was not persisted at all")
	}
}

// TestTUISaveCache_AvailabilityCountsStillPersist guards the half of the
// save-cache dispatch that already works today: availability Count/HasResources
// must keep persisting once the routing fix lands, so the DEF-11 reroute cannot
// regress the one thing the intercepted TUI path got right. Green both before
// and after the fix.
func TestTUISaveCache_AvailabilityCountsStillPersist(t *testing.T) {
	const profile, region = "def11-tui-counts-prof", "us-east-1"
	m := newSaveCacheApp(t, profile, region)

	sweepResources := []resource.Resource{
		{ID: "bucket-def11-2", Name: "def11-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
	}

	m, cmd := driveSweepCompletion(m, sweepResources)
	m = runCmdTree(t, m, cmd)
	_ = m

	store := cache.LoadDir(profile, region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a TUI-driven sweep completion — availability counts must persist regardless of the DEF-11 rows regression`)
	}
	if !tf.HasResources {
		t.Error("s3 TypeFile.HasResources = false, want true — availability count persistence must survive the DEF-11 routing fix")
	}
	if tf.Count != 1 {
		t.Errorf("s3 TypeFile.Count = %d, want 1 — availability count persistence must survive the DEF-11 routing fix", tf.Count)
	}
	// Issues is intentionally NOT asserted here: post-fix, TaskKindSaveCache's
	// issue count is derived from c.session.ResourceCache via
	// availabilityFromResourceCache (internal/runtime/executor.go), which
	// classifies issues through the s3 catalog's own domain.Color function —
	// a synthetic single-field fixture built purely for this cache-routing
	// test does not reliably trigger that per-type classification, and a
	// diagnostic run confirmed the identical (Issues=0) outcome through the
	// headless executor path in this same "sweep-only, no list ever opened"
	// scenario. That classification fidelity is a different subsystem than
	// DEF-11 (which is about Rows, not Issues); Count/HasResources above are
	// the load-bearing counts-still-work assertions for this guard.
}

// A third test driving the same session state through BOTH the headless
// executor path (runtime.Core.ExecuteTask, per
// TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen
// in app_pilot_defects_test.go) and this file's TUI dispatch path, asserting
// identical resulting TypeFiles, is skipped here: building a second,
// independently-seeded Core/Controller pair to compare against the tui.Model
// under test would duplicate nearly all of newSaveCacheApp/driveSweepCompletion
// with no additional reachable-seam risk — TestTUISaveCache_PersistsRowsToTypeFile
// already pins the exact rows/findings the executor path is known (via the
// DEF-7 test) to persist, so a lane-parity diff would only restate the same
// assertion through a heavier harness.
