// The TUI lane's sweep-completion save
// persists the swept type's rows, not only its count
// (docs/design/cache-requirements.md). The headless lane's equivalent is
// TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen
// in app_pilot_defects_test.go.
//
// The drive goes through the seam a live session uses: tui.Model.Update ->
// coreUpdate -> dispatchTaskRequests -> the tea.Cmd the shared executor's
// TaskKindSaveCache case builds, executed exactly as the Bubble Tea runtime
// would. That dispatch lane is unexported, so the only reachable seam from
// tests/unit is tui.Model's tea.Model interface via the tuitest harness
// (tests/unit/tuitest/tuitest.go). A single messages.AvailabilityChecked
// with the queue already drained (AvailQueue nil, AvailTotal 0 per
// session.New()) reaches handleAvailabilityChecked's queue-exhausted branch,
// which appends the TaskKindSaveCache task carrying the dispatch-time
// snapshotProbeResourcesForSave() payload — the exact production sequence a
// live sweep completion produces.
package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// newSaveCacheApp builds a tui.Model wired to demo clients (no real AWS
// calls) with on-disk caching ENABLED (unlike newEnrichApp in
// app_enrich_test.go, which sets WithNoCache(true)), since what lands on disk
// is the subject. A9S_CONFIG_FOLDER is redirected to t.TempDir() so
// cache.LoadDirForTest/Store.SaveType write under an isolated directory.
func newSaveCacheApp(t *testing.T, profile, region string) tui.Model {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	m := newBlessedModel(t, profile, region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfileForTest(profile),
		tui.WithRegionForTest(region))
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
// back into m.Update, exactly as the Bubble Tea event loop would. The save
// happens inside the tea.Cmd dispatchTaskRequests returns, so that cmd must
// actually run for the type file to be written.
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

// TestTUISaveCache_PersistsRowsToTypeFile: a TUI-driven
// availability-sweep-completion save persists the swept type's rows to disk,
// not just its availability count.
//
// TaskKindSaveCache persists from the dispatch-time SaveCachePayload snapshot
// taken BEFORE Wave-2 enrichment runs (handleAvailabilityChecked captures
// c.snapshotProbeResourcesForSave() so a later startEnrichment() mutation of
// ProbeResources cannot change what gets persisted), so the finding must
// already be on the Wave-1 resource at dispatch time, as a fetcher-sourced
// finding is. Wave-2 enrichment runs in demo mode too, so the synthetic
// finding carries Source "wave1:s3" rather than a "wave2:*" label the s3
// enricher never produced.
func TestTUISaveCache_PersistsRowsToTypeFile(t *testing.T) {
	const profile, region = "savecache-tui-prof", "us-east-1"
	m := newSaveCacheApp(t, profile, region)

	finding := domain.Finding{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave1:s3"}
	sweepResources := []resource.Resource{
		{
			ID:       "bucket-savecache-1",
			Name:     "savecache-bucket",
			Type:     "s3",
			Fields:   map[string]string{"region": region},
			Findings: []domain.Finding{finding},
		},
	}

	m, cmd := driveSweepCompletion(m, sweepResources)
	m = runCmdTree(t, m, cmd)
	_ = m

	store := cache.LoadDirForTest(profile, region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a TUI-driven sweep completion — save-cache routing: the TUI save-cache dispatch must persist a per-type file just like the headless executor path does`)
	}
	if len(tf.Rows) == 0 {
		t.Fatal("s3 TypeFile.Rows is empty after a TUI-driven sweep completion — save-cache routing: internal/tui/app_dispatch.go's TaskKindSaveCache case intercepts the task via m.saveAvailabilityCache() (counts-only), bypassing the shared executor path that also persists rows via saveProbeResourcesToTypeFiles")
	}
	found := false
	for _, r := range tf.Rows {
		if r.ID == "bucket-savecache-1" {
			found = true
			if len(r.Findings) != 1 || r.Findings[0].Code != "s3-public-read" {
				t.Errorf("persisted TUI-swept row Findings = %+v, want 1 finding with Code=%q", r.Findings, "s3-public-read")
			}
		}
	}
	if !found {
		t.Error("persisted s3 TypeFile.Rows missing bucket-savecache-1 — the TUI-swept row was not persisted at all")
	}
}

// TestTUISaveCache_AvailabilityCountsStillPersist: availability
// Count/HasResources persist through the same save-cache dispatch.
func TestTUISaveCache_AvailabilityCountsStillPersist(t *testing.T) {
	const profile, region = "savecache-tui-counts-prof", "us-east-1"
	m := newSaveCacheApp(t, profile, region)

	sweepResources := []resource.Resource{
		{ID: "bucket-savecache-2", Name: "savecache-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
	}

	m, cmd := driveSweepCompletion(m, sweepResources)
	m = runCmdTree(t, m, cmd)
	_ = m

	store := cache.LoadDirForTest(profile, region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a TUI-driven sweep completion — availability counts must persist regardless of the rows-persistence regression`)
	}
	if !tf.HasResources {
		t.Error("s3 TypeFile.HasResources = false, want true — availability count persistence must survive the save-cache routing fix")
	}
	if tf.Count != 1 {
		t.Errorf("s3 TypeFile.Count = %d, want 1 — availability count persistence must survive the save-cache routing fix", tf.Count)
	}
}
