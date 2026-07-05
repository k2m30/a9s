// runtime_wave2_carry_test.go — pins for C6b (docs/design/cache-requirements.md
// clause C6b, defect row D17): Wave-2 carry at every row-replacing write.
//
// D17 (glyphs/Status blink, on restart AND mid-session): every Wave-1
// sweep-completion save wrote pre-enrichment rows, and reconcileTypeFile's
// "refreshed rows win" rule (rules 3/4, internal/runtime/probes.go:162) let
// those bare rows strip Findings and enricher Fields (e.g. "status") from
// the type file; the analogous in-memory write into session.ProbeResources
// (handleAvailabilityChecked, internal/runtime/handlers_availability.go:278)
// blanked the visible list the same way until re-enrichment.
//
// C6b's fix: when an accepted rows-carrying observation lacks Wave-2 data
// that the rows it replaces have, the write carries forward — per row ID —
// the replaced rows' wave2:-sourced Findings and the type's registered
// enricher Fields (IssueEnricherFieldKeys). A Wave-2-sourced observation for
// the type supersedes carried data (healed/resolved issues still clear).
// Wave-1 findings never carry (their absence in a fresh fetch means
// resolved).
//
// Covers, in order:
//
//  1. TestReconcileTypeFile_Wave2Carry_RefreshDropsWave1KeepsWave2 — disk
//     chokepoint (reconcileTypeFile via the same seam
//     runtime_reconciletypefile_test.go uses): a bare rows-carrying refresh
//     must carry forward row X's wave2 finding + Fields["status"], and drop
//     row Y's wave1 finding.
//  2. TestReconcileTypeFile_Wave2SourcedObservation_ClearsCarriedData — a
//     Wave-2-sourced observation for the type (healed row X, no findings)
//     must NOT carry forward the old wave2 finding/status — carried data
//     clears, it is not immortal.
//  3. TestHandleAvailabilityChecked_Wave2Carry_ProbeResourcesKeepFindingAndStatus —
//     in-memory probe-store carry (handleAvailabilityChecked): a fresh bare
//     Wave-1 probe result must not blank ProbeResources' carried wave2
//     finding + status; a subsequent enrichment-completion for the type
//     replaces them.
//  4. TestRestartSeed_S3_Wave2FindingAndStatusVisibleOnFirstRender — restart
//     end-to-end: a type file on disk with wave2 findings + status seeds the
//     FIRST rendered list state (glyph + status) via the disk-store fallback
//     HandleNavigate/list-open already exercises, with no enrichment probe
//     run this session.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network. Fake profile/region/resource IDs only.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — disk chokepoint: reconcileTypeFile is unexported, so it is
// exercised the same way runtime_reconciletypefile_test.go does: through the
// exported Core.SaveResourceListCache seam, which is documented (probes.go:
// 438-440) to route every rows-carrying write through reconcileTypeFile's
// rules 1/3/4.
// ────────────────────────────────────────────────────────────────────────────

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
	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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
	// shape a fresh Wave-1 sweep-completion save produces at HEAD.
	freshRows := []cache.Row{
		{ID: "s3-bucket-x", Name: "s3-bucket-x"},
		{ID: "s3-bucket-y", Name: "s3-bucket-y"},
	}
	if err := c.SaveResourceListCache(shortName, freshRows, 2, true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
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

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — a Wave-2-sourced observation for the type supersedes carried data:
// driven through the REAL Wave-2-completion path (messages.EnrichmentChecked
// with the enrichment queue already drained, mirroring
// TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen in
// app_pilot_defects_test.go), a healed observation (no findings for
// s3-bucket-x this time) must clear the previously carried wave2 finding +
// status on disk, not re-carry it forever. This exercises the real
// wave2Authoritative=true call site (handleEnrichmentChecked's "all done"
// branch -> snapshotProbeResourcesForSave(true) ->
// saveResourceListCacheWave2Complete via the TaskKindSaveCache executor
// case), executed with core.ExecuteTask exactly as DrainSync would — no
// hand-built flag needed since the real event path reaches it.
// ────────────────────────────────────────────────────────────────────────────

func TestReconcileTypeFile_Wave2SourcedObservation_ClearsCarriedData(t *testing.T) {
	const wave2ClearProfile = "wave2-clear-prof"
	const wave2ClearRegion = "us-east-1"
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	store := cache.LoadDir(wave2ClearProfile, wave2ClearRegion)
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
	ctrl := app.New(core)

	// Seed ProbeResources with the CURRENT session's bare Wave-1-observed row
	// (no findings of its own yet) — mirrors what a live availability sweep
	// would have populated this session before enrichment ran.
	// snapshotProbeResourcesForSave (which handleEnrichmentChecked's "all
	// done" branch calls) reads from ProbeResources — an empty/nil map here
	// produces a nil save payload and never touches disk at all, which would
	// trivially (and wrongly) "pass" this test without exercising the
	// Wave-2-completion save path.
	core.Session().ProbeResources = map[string][]resource.Resource{
		"s3": {{ID: "s3-bucket-x", Name: "s3-bucket-x", Type: "s3"}},
	}
	core.Session().ProbeTruncated = map[string]bool{"s3": false}

	// Enrichment queue already drained (EnrichChecked reaches EnrichTotal once
	// this single result lands) so handleEnrichmentChecked's "all done" branch
	// fires immediately — mirrors the DEF-7 precedent's single-type shape.
	core.Session().EnrichTotal = 1
	core.Session().EnrichChecked = 0
	core.Session().EnrichQueue = nil

	// Healed observation: no Findings for s3-bucket-x this time — the
	// enrichment probe re-ran and found nothing.
	_, tasks := ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Issues:       0,
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

	reloaded := cache.LoadDir(wave2ClearProfile, wave2ClearRegion)
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

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — in-memory probe-store carry: handleAvailabilityChecked
// (internal/runtime/handlers_availability.go:278) overwrites
// session.ProbeResources[canon] with msg.Resources unconditionally. A fresh
// bare Wave-1 probe result (same IDs, no findings) must not blank the
// carried wave2 finding + status that a prior enrichment pass wrote into
// ProbeResources.
// ────────────────────────────────────────────────────────────────────────────

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
	c.Session().ProbeResources = map[string][]resource.Resource{
		"s3": {enrichedRow},
	}
	c.Session().ProbeTruncated = map[string]bool{"s3": false}
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

	got, ok := c.Session().ProbeResources["s3"]
	if !ok {
		t.Fatal("ProbeResources[\"s3\"] missing after handleAvailabilityChecked")
	}
	if len(got) != 1 {
		t.Fatalf("ProbeResources[\"s3\"] has %d entries, want 1", len(got))
	}
	row := got[0]
	if len(row.Findings) != 1 {
		t.Fatalf("ProbeResources row Findings = %+v, want 1 carried wave2 finding — a bare Wave-1 probe result must not blank the in-memory carried finding (C6b)", row.Findings)
	}
	if got := row.Findings[0].Source; got != "wave2:s3" {
		t.Errorf("ProbeResources row Findings[0].Source = %q, want %q", got, "wave2:s3")
	}
	if got := row.Fields["status"]; got != "public access block incomplete" {
		t.Errorf(`ProbeResources row Fields["status"] = %q, want %q (C6b in-memory carry)`, got, "public access block incomplete")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — restart seed end-to-end: a type file on disk whose rows carry
// wave2 findings + status must seed the FIRST rendered list state (glyph +
// status) with no enrichment run this session. Mirrors
// TestListOpen_NoProbeResourcesButDiskStoreHasRows_SeedsFromDiskStore's
// disk-store fallback pattern (app_cache_first_seeding_test.go) — a fresh
// session (no ProbeResources observed for s3) opening the s3 list must seed
// from the on-disk Store via HandleNavigate's rowsFromCacheRows fallback,
// which is documented (handlers_availability.go:562) to copy Fields/Findings
// verbatim onto the seeded resource.Resource rows.
// ────────────────────────────────────────────────────────────────────────────

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

	ctrl := app.New(core)
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
	if row.Decorator != app.DecoratorError {
		t.Errorf("Rows[0].Decorator = %q, want %q — the FIRST rendered frame must already show the carried wave2 finding's glyph, no enrichment re-run needed (D17/C6b)", row.Decorator, app.DecoratorError)
	}

	// HandleNavigate's disk-store fallback (handlers_navigate.go:205-224) seeds
	// the list SCREEN's own state directly via NavigateResult.CachedEntry
	// (internal/app/navigate.go), not session.ResourceCache/ProbeResources —
	// those session-level maps are only ever written by a live availability
	// probe (handleAvailabilityChecked) or an enrichment rerun, neither of
	// which ran this session. Body.List.Rows above is therefore the correct
	// (and only) place to observe the disk-seeded finding/status on the
	// FIRST rendered frame.
	if row.Severity != "broken" {
		t.Errorf(`Rows[0].Severity = %q, want "broken" (resolveListDecoratorFull's SevBroken branch) — the FIRST render must already reflect the carried wave2 finding, no enrichment needed`, row.Severity)
	}
}
