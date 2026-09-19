// Wave-2 carry at every row-replacing write
// (docs/design/cache-requirements.md).
//
// When an accepted rows-carrying observation lacks Wave-2 data that the rows
// it replaces have, the write carries forward — per row ID — the replaced
// rows' wave2:-sourced Findings and the type's registered enricher Fields
// (IssueEnricherFieldKeys), both into the type file (reconcileTypeFile,
// core/runtime/probes.go) and into the in-memory RowStore
// (handleAvailabilityChecked). Without it a Wave-1 sweep-completion save of
// pre-enrichment rows would strip glyphs and Status on restart and
// mid-session. A Wave-2-sourced observation for the type supersedes carried
// data (healed/resolved issues still clear). Wave-1 findings never carry
// (their absence in a fresh fetch means resolved).
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network. Fake profile/region/resource IDs only.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// reconcileTypeFile is unexported, so it is exercised the same way
// runtime_reconciletypefile_test.go does: through the exported
// Core.SaveResourceListCache seam, which routes every rows-carrying write
// through reconcileTypeFile's rules 1/3/4.

const (
	wave2CarryProfile = "wave2-carry-prof"
	wave2CarryRegion  = "us-east-1"
)

func TestReconcileTypeFile_Wave2Carry_RefreshDropsWave1KeepsWave2(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "wave2carry"

	// newSaveCacheRegressionCore (runtime_savecache_regressions_test.go)
	// bootstraps its Core against saveRegProfile/saveRegRegion, NOT
	// wave2CarryProfile/wave2CarryRegion — the seed must land in the same
	// pair the Core under test actually reads/writes, or SaveResourceListCache
	// below sees a fresh empty store and there is nothing to carry from.
	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	existingRows := []cache.Row{
		{
			ID:   "s3-bucket-x",
			Name: "s3-bucket-x",
			Fields: map[string]string{
				"status": "public access block incomplete",
			},
			Findings: []domain.Finding{
				{
					Code:     domain.FindingCode("s3-public-access"),
					Phrase:   "public access block incomplete",
					Severity: domain.SevBroken,
					Source:   "wave2:s3",
				},
			},
		},
		{
			ID:   "s3-bucket-y",
			Name: "s3-bucket-y",
			Findings: []domain.Finding{
				{
					Code:     domain.FindingCode("s3-fetcher-issue"),
					Phrase:   "bucket policy denies read",
					Severity: domain.SevWarn,
					Source:   "wave1",
				},
			},
		},
	}
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 2, Exact: true, Rows: existingRows,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// Same-depth, non-subset refresh shape (rule 4): same IDs, but bare rows
	// with NO Findings and NO status — the accepted "refreshed rows win"
	// shape a fresh Wave-1 sweep-completion save produces.
	freshRows := []cache.Row{
		{ID: "s3-bucket-x", Name: "s3-bucket-x"},
		{ID: "s3-bucket-y", Name: "s3-bucket-y"},
	}
	if err := c.SaveResourceListCache(c.Pair(), shortName, freshRows, 2, true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after Wave-2-carry refresh write")
	}
	if len(tf.Rows) != 2 {
		t.Fatalf("TypeFile.Rows has %d entries, want 2", len(tf.Rows))
	}

	var rowX, rowY *cache.Row
	for i := range tf.Rows {
		switch tf.Rows[i].ID {
		case "s3-bucket-x":
			rowX = &tf.Rows[i]
		case "s3-bucket-y":
			rowY = &tf.Rows[i]
		}
	}
	if rowX == nil {
		t.Fatal("row s3-bucket-x missing from reconciled TypeFile")
	}
	if rowY == nil {
		t.Fatal("row s3-bucket-y missing from reconciled TypeFile")
	}

	if len(rowX.Findings) != 1 {
		t.Fatalf("row X Findings = %+v, want 1 carried wave2 finding (C6b) — a bare refresh must not strip Wave-2 data", rowX.Findings)
	}
	if got := rowX.Findings[0].Source; got != "wave2:s3" {
		t.Errorf("row X Findings[0].Source = %q, want %q", got, "wave2:s3")
	}
	if got := rowX.Findings[0].Phrase; got != "public access block incomplete" {
		t.Errorf("row X Findings[0].Phrase = %q, want %q", got, "public access block incomplete")
	}
	if got := rowX.Fields["status"]; got != "public access block incomplete" {
		t.Errorf(`row X Fields["status"] = %q, want %q (C6b: enricher Fields carry with the finding)`, got, "public access block incomplete")
	}

	if len(rowY.Findings) != 0 {
		t.Errorf("row Y Findings = %+v, want 0 — a wave1 finding absent from a fresh fetch means resolved and must NOT carry (C6b)", rowY.Findings)
	}
}

// A Wave-2-sourced observation for the type supersedes carried data: driven
// through the real Wave-2-completion path (messages.EnrichmentChecked with the
// enrichment queue already drained), a healed observation (no findings for
// s3-bucket-x) clears the previously carried wave2 finding + status on disk
// rather than re-carrying it forever. This exercises the
// wave2Authoritative=true call site (handleEnrichmentChecked's "all done"
// branch -> snapshotProbeResourcesForSave(true) ->
// saveResourceListCacheWave2Complete via the TaskKindSaveCache executor case),
// executed with core.ExecuteTask as DrainSync would.

func TestReconcileTypeFile_Wave2SourcedObservation_ClearsCarriedData(t *testing.T) {
	const wave2ClearProfile = "wave2-clear-prof"
	const wave2ClearRegion = "us-east-1"
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	store := cache.LoadDirForTest(wave2ClearProfile, wave2ClearRegion)
	store.Put("s3", cache.TypeFile{
		HasResources: true, Count: 1, Exact: true,
		Rows: []cache.Row{
			{
				ID:   "s3-bucket-x",
				Name: "s3-bucket-x",
				Fields: map[string]string{
					"status": "public access block incomplete",
				},
				Findings: []domain.Finding{
					{
						Code:     domain.FindingCode("s3-public-access"),
						Phrase:   "public access block incomplete",
						Severity: domain.SevBroken,
						Source:   "wave2:s3",
					},
				},
			},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed SaveType(s3): %v", err)
	}

	s := session.New()
	s.Profile = wave2ClearProfile
	s.Region = wave2ClearRegion
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	// Seed RowStore with the CURRENT session's bare Wave-1-observed row (no
	// findings of its own yet) — mirrors what a live availability sweep would
	// have populated this session before enrichment ran.
	// snapshotRowStoreForSave (which handleEnrichmentChecked's "all done"
	// branch calls) reads from RowStore — a never-observed type here produces
	// a nil save payload and never touches disk at all, which would trivially
	// (and wrongly) "pass" this test without exercising the
	// Wave-2-completion save path.
	core.Session().RowStore.Observe("s3", []resource.Resource{
		{ID: "s3-bucket-x", Name: "s3-bucket-x", Type: "s3"},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	// Enrichment queue already drained (EnrichChecked reaches EnrichTotal once
	// this single result lands) so handleEnrichmentChecked's "all done" branch
	// fires immediately.
	core.Session().EnrichTotal = 1
	core.Session().EnrichChecked = 0
	core.Session().EnrichQueue = nil
	core.Session().EnrichSweepMembers = map[string]bool{"s3": true}

	// Healed observation: no Findings for s3-bucket-x this time — the
	// enrichment probe re-ran and found nothing.
	_, tasks := ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Findings:     nil,
	})

	hasSaveCacheTask := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindSaveCache {
			hasSaveCacheTask = true
			ev, err := core.ExecuteTask(t.Context(), tk)
			if err != nil {
				t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
			}
			if ev != nil {
				ctrl.Handle(ev)
			}
		}
	}
	if !hasSaveCacheTask {
		t.Fatal("handleEnrichmentChecked (queue drained) returned no TaskKindSaveCache task — test assumption broken, cannot exercise the Wave-2-completion save path")
	}

	reloaded := cache.LoadDirForTest(wave2ClearProfile, wave2ClearRegion)
	tf, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal("TypeFile missing after healed Wave-2-completion write")
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	row := tf.Rows[0]
	if len(row.Findings) != 0 {
		t.Errorf("row Findings = %+v, want 0 — a Wave-2-completion observation confirming the issue is healed must CLEAR the carried finding, not re-carry it forever (C6b)", row.Findings)
	}
	if got := row.Fields["status"]; got != "" {
		t.Errorf(`row Fields["status"] = %q, want empty — carried status must clear when Wave-2 confirms healed (C6b)`, got)
	}
}

// In-memory probe-store carry: handleAvailabilityChecked
// (core/runtime/handlers_availability.go) calls ObserveRows(canon,
// msg.Resources, ...) after carryWave2ForResources folds the previous RowStore
// rows' Wave-2 data into the fresh probe result. A fresh bare Wave-1 probe
// result (same IDs, no findings) must not blank the carried wave2 finding +
// status that a prior enrichment pass wrote into RowStore.

func TestHandleAvailabilityChecked_Wave2Carry_ProbeResourcesKeepFindingAndStatus(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newSaveCacheRegressionCore(t, false)

	enrichedRow := resource.Resource{
		ID:   "s3-bucket-x",
		Name: "s3-bucket-x",
		Type: "s3",
		Fields: map[string]string{
			"status": "public access block incomplete",
		},
		Findings: []domain.Finding{
			{
				Code:     domain.FindingCode("s3-public-access"),
				Phrase:   "public access block incomplete",
				Severity: domain.SevBroken,
				Source:   "wave2:s3",
			},
		},
	}
	c.Session().RowStore.Observe("s3", []resource.Resource{enrichedRow}, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	c.Session().AvailChecked = 0
	c.Session().AvailTotal = 1
	c.Session().AvailQueue = nil

	bareRow := resource.Resource{
		ID:   "s3-bucket-x",
		Name: "s3-bucket-x",
		Type: "s3",
	}
	_, _ = c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        1,
		Truncated:    false,
		Resources:    []resource.Resource{bareRow},
		Gen:          c.AvailabilityGen(),
	})

	snap := c.Session().RowStore.Snapshot("s3")
	if len(snap.Rows) != 1 {
		t.Fatalf("RowStore.Snapshot(\"s3\").Rows has %d entries, want 1", len(snap.Rows))
	}
	row := snap.Rows[0]
	if len(row.Findings) != 1 {
		t.Fatalf("RowStore row Findings = %+v, want 1 carried wave2 finding — a bare Wave-1 probe result must not blank the in-memory carried finding (C6b)", row.Findings)
	}
	if got := row.Findings[0].Source; got != "wave2:s3" {
		t.Errorf("RowStore row Findings[0].Source = %q, want %q", got, "wave2:s3")
	}
	if got := row.Fields["status"]; got != "public access block incomplete" {
		t.Errorf(`RowStore row Fields["status"] = %q, want %q (C6b in-memory carry)`, got, "public access block incomplete")
	}
}

// Restart seed end-to-end: a type file on disk whose rows carry wave2
// findings + status seeds the first rendered list state (glyph + status) with
// no enrichment run this session — a fresh session opening the s3 list seeds
// from the on-disk Store via HandleNavigate's rowsFromCacheRows fallback,
// which copies Fields/Findings verbatim onto the seeded resource.Resource rows.

func TestRestartSeed_S3_Wave2FindingAndStatusVisibleOnFirstRender(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	core := runtime.Bootstrap(wave2CarryProfile, wave2CarryRegion, nil)
	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{
				ID:   "s3-bucket-x",
				Name: "s3-bucket-x",
				Fields: map[string]string{
					"status": "public access block incomplete",
				},
				Findings: []domain.Finding{
					{
						Code:     domain.FindingCode("s3-public-access"),
						Phrase:   "public access block incomplete",
						Severity: domain.SevBroken,
						Source:   "wave2:s3",
					},
				},
			},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := ctrl.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 list with a disk-store Wave-2-carrying seed")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a populated on-disk per-type cache must seed the list immediately")
	}
	if len(lb.Rows) != 1 {
		t.Fatalf("Body.List.Rows has %d entries, want 1", len(lb.Rows))
	}
	row := lb.Rows[0]
	if row.ResourceID != "s3-bucket-x" {
		t.Fatalf("Rows[0].ResourceID = %q, want %q", row.ResourceID, "s3-bucket-x")
	}
	// colorS3 is colorFromAnyFinding-only (core/aws/catalog_databases.go), so
	// the carried SevBroken Finding resolves td.ResolveColor(r) to ColorBroken
	// directly and no glyph is produced: the row renders as a full broken row
	// via colorTag (the WHOLE row is coloured) and Severity=="issue" (see
	// resolveListRowSeverity's IsIssue() branch). Whole-row colour is asserted
	// through colorTag and Severity above.

	// HandleNavigate's disk-store fallback (handlers_navigate.go:205-224) seeds
	// the list SCREEN's own state directly via NavigateResult.CachedEntry
	// (core/app/navigate.go), not session.ResourceCache/ProbeResources —
	// those session-level maps are only ever written by a live availability
	// probe (handleAvailabilityChecked) or an enrichment rerun, neither of
	// which ran this session. Body.List.Rows above is therefore the correct
	// (and only) place to observe the disk-seeded finding/status on the
	// FIRST rendered frame.
	if row.Severity != "issue" {
		t.Errorf(`Rows[0].Severity = %q, want "issue" (resolveListRowSeverity's IsIssue() branch, once ResolveColor() is ColorBroken directly) — the FIRST render must already reflect the carried wave2 finding, no enrichment needed`, row.Severity)
	}
}
