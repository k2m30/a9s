// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// Issue #463 — per-finding FirstSeen persisted in the availability cache
// (cache.Row.FindingFirstSeen), plus the new-since-previous-scan deltas
// surfaced through Core.FindingFirstSeenForType / NewFindingPairsSincePrev.
//
// Drives the real save path (newTestControllerAndCore + Controller.Apply +
// Controller.ApplyResourcesLoaded, the same wiring
// TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed in
// app_pilot_defects_test.go exercises) so the carry/drop/reappear logic
// inside Core.saveResourceListCache is what's actually under test, not a
// hand-built cache.Row. Two "sweeps" against the SAME Controller/session
// give saveResourceListCache's `existing, _ := store.Type(canon)` a real
// previous generation to diff against, without needing a second Controller
// (and therefore without needing a second, allowlist-violating construction
// site — see qa_controller_construction_discipline_test.go).

func TestCacheFirstSeen_PersistsAcrossSaves(t *testing.T) {
	ctrl, _ := newTestControllerAndCore(t)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	row := resource.Resource{
		ID:       "bucket-fs-persist",
		Name:     "fs-persist-bucket",
		Type:     "s3",
		Fields:   map[string]string{"region": "us-east-1"},
		Findings: []domain.Finding{finding},
	}

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{row}, nil, false)

	store1 := cache.LoadDirForTest("demo", "us-east-1")
	tf1, ok := store1.Type("s3")
	if !ok || len(tf1.Rows) != 1 {
		t.Fatalf("save 1: store.Type(s3) = (%+v, %v), want exactly 1 row", tf1, ok)
	}
	t1, ok := tf1.Rows[0].FindingFirstSeen[finding.Code]
	if !ok || t1.IsZero() {
		t.Fatalf("save 1: FindingFirstSeen[%q] missing or zero (got %v, ok=%v)", finding.Code, t1, ok)
	}

	time.Sleep(5 * time.Millisecond)
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{row}, nil, false)

	store2 := cache.LoadDirForTest("demo", "us-east-1")
	tf2, ok := store2.Type("s3")
	if !ok || len(tf2.Rows) != 1 {
		t.Fatalf("save 2: store.Type(s3) = (%+v, %v), want exactly 1 row", tf2, ok)
	}
	t2, ok := tf2.Rows[0].FindingFirstSeen[finding.Code]
	if !ok {
		t.Fatal("save 2: FindingFirstSeen entry dropped for a finding that is still present on the row")
	}
	if !t2.Equal(t1) {
		t.Errorf("save 2: FindingFirstSeen[%q] = %v, want unchanged from save 1's %v — a persisting finding must keep its original FirstSeen, not be re-stamped", finding.Code, t2, t1)
	}
}

func TestCacheFirstSeen_ResolvedFindingDropsOut(t *testing.T) {
	ctrl, _ := newTestControllerAndCore(t)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		// Source "wave1", not "wave2": a Wave-2-sourced finding's absence
		// from an incoming row is legitimately carried forward by
		// reconcileTypeFile's C6b Wave-2 carry (carryWave2ForRows) — that's
		// the flicker guard, not a resolution. Only a Wave-1 finding's
		// absence from the incoming row is a genuine resolution.
		Source: "wave1",
	}
	rowID := "bucket-fs-resolve"

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{{
		ID:       rowID,
		Name:     "fs-resolve-bucket",
		Type:     "s3",
		Fields:   map[string]string{"region": "us-east-1"},
		Findings: []domain.Finding{finding},
	}}, nil, false)

	store1 := cache.LoadDirForTest("demo", "us-east-1")
	tf1, ok := store1.Type("s3")
	if !ok || len(tf1.Rows) != 1 {
		t.Fatalf("save 1: store.Type(s3) = (%+v, %v), want exactly 1 row", tf1, ok)
	}
	if _, hasFirstSeen := tf1.Rows[0].FindingFirstSeen[finding.Code]; !hasFirstSeen {
		t.Fatalf("save 1: FindingFirstSeen[%q] missing before resolution", finding.Code)
	}

	// The finding resolves: save 2 has the same row with no findings at all.
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{{
		ID:     rowID,
		Name:   "fs-resolve-bucket",
		Type:   "s3",
		Fields: map[string]string{"region": "us-east-1"},
	}}, nil, false)

	store2 := cache.LoadDirForTest("demo", "us-east-1")
	tf2, ok := store2.Type("s3")
	if !ok || len(tf2.Rows) != 1 {
		t.Fatalf("save 2: store.Type(s3) = (%+v, %v), want exactly 1 row", tf2, ok)
	}
	if _, stillThere := tf2.Rows[0].FindingFirstSeen[finding.Code]; stillThere {
		t.Errorf("save 2: FindingFirstSeen[%q] still present after the finding resolved, want it dropped from the map", finding.Code)
	}
}

func TestCacheFirstSeen_ReappearingFindingFreshStamp(t *testing.T) {
	ctrl, _ := newTestControllerAndCore(t)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		// Source "wave1": see TestCacheFirstSeen_ResolvedFindingDropsOut —
		// a Wave-2-sourced finding's absence from an incoming row would be
		// carried forward by C6b instead of genuinely resolving, which
		// would make the later reappearance indistinguishable from "never
		// left".
		Source: "wave1",
	}
	rowID := "bucket-fs-reappear"
	withFinding := resource.Resource{
		ID: rowID, Name: "fs-reappear-bucket", Type: "s3",
		Fields:   map[string]string{"region": "us-east-1"},
		Findings: []domain.Finding{finding},
	}
	withoutFinding := resource.Resource{
		ID: rowID, Name: "fs-reappear-bucket", Type: "s3",
		Fields: map[string]string{"region": "us-east-1"},
	}

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{withFinding}, nil, false)
	store1 := cache.LoadDirForTest("demo", "us-east-1")
	tf1, ok := store1.Type("s3")
	if !ok || len(tf1.Rows) != 1 {
		t.Fatalf("save 1: store.Type(s3) = (%+v, %v), want exactly 1 row", tf1, ok)
	}
	t1, ok := tf1.Rows[0].FindingFirstSeen[finding.Code]
	if !ok || t1.IsZero() {
		t.Fatalf("save 1: FindingFirstSeen[%q] missing or zero", finding.Code)
	}

	time.Sleep(5 * time.Millisecond)
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{withoutFinding}, nil, false)

	time.Sleep(5 * time.Millisecond)
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{withFinding}, nil, false)

	store3 := cache.LoadDirForTest("demo", "us-east-1")
	tf3, ok := store3.Type("s3")
	if !ok || len(tf3.Rows) != 1 {
		t.Fatalf("save 3: store.Type(s3) = (%+v, %v), want exactly 1 row", tf3, ok)
	}
	t3, ok := tf3.Rows[0].FindingFirstSeen[finding.Code]
	if !ok || t3.IsZero() {
		t.Fatalf("save 3: FindingFirstSeen[%q] missing or zero after reappearing", finding.Code)
	}
	if !t3.After(t1) {
		t.Errorf("save 3: FindingFirstSeen[%q] = %v, want a fresh stamp strictly after save 1's %v — a resolved-then-reappearing finding must not resurrect its old FirstSeen", finding.Code, t3, t1)
	}
}

// writeTypeFile marshals tf as-is (including a caller-chosen Version that
// Store.Put would otherwise overwrite) and writes it directly to
// <dir>/<shortName>.yaml, bypassing Store/Put entirely — the only way to
// construct an on-disk file pinned to a specific schema version for a
// migration test.
func writeTypeFile(t *testing.T, dir, shortName string, tf cache.TypeFile) {
	t.Helper()
	data, err := yaml.Marshal(tf)
	if err != nil {
		t.Fatalf("yaml.Marshal(%s): %v", shortName, err)
	}
	path := filepath.Join(dir, shortName+".yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestCacheFirstSeen_V1FileMigratesWithSavedAtFloor(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	dir := cache.DirForTest("migrate-profile", "us-east-1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}

	savedAt := time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)
	findingA := domain.Finding{Code: "vpc-no-flow-logs", Phrase: "flow logs disabled", Severity: domain.SevWarn}
	findingB := domain.Finding{Code: "vpc-default-sg-open", Phrase: "default SG allows all", Severity: domain.SevBroken}

	v1File := cache.TypeFile{
		Version:      1,
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{
				ID:       "vpc-migrate-1",
				Name:     "legacy-vpc",
				Fields:   map[string]string{"cidr": "10.0.0.0/16"},
				Findings: []domain.Finding{findingA, findingB},
			},
		},
		SavedAt: savedAt,
	}
	writeTypeFile(t, dir, "vpc", v1File)

	v0File := v1File
	v0File.Version = 0
	writeTypeFile(t, dir, "ec2", v0File)

	v3File := v1File
	v3File.Version = 3
	writeTypeFile(t, dir, "lambda", v3File)

	store := cache.LoadDirForTest("migrate-profile", "us-east-1")

	vpcTF, ok := store.Type("vpc")
	if !ok {
		t.Fatal(`Type("vpc") missing — a version-1 file must still load once SchemaVersion is 2 (migration, not rejection)`)
	}
	if len(vpcTF.Rows) != 1 {
		t.Fatalf("migrated vpc TypeFile has %d rows, want 1", len(vpcTF.Rows))
	}
	row := vpcTF.Rows[0]
	for _, f := range []domain.Finding{findingA, findingB} {
		got, ok := row.FindingFirstSeen[f.Code]
		if !ok {
			t.Errorf("migrated row missing FindingFirstSeen[%q]", f.Code)
			continue
		}
		if !got.Equal(savedAt) {
			t.Errorf("migrated row FindingFirstSeen[%q] = %v, want the v1 file's SavedAt floor %v", f.Code, got, savedAt)
		}
	}

	if _, ok := store.Type("ec2"); ok {
		t.Error(`Type("ec2") should be absent — a version-0 file is still an unsupported schema and must be skipped, not treated as a migration target`)
	}
	if _, ok := store.Type("lambda"); ok {
		t.Error(`Type("lambda") should be absent — a version-3 file is still an unsupported schema and must be skipped, not treated as a migration target`)
	}
}

// TestCacheFirstSeen_NewSincePrevViaControllerSave pins the same #463
// carry/drop behaviour the deleted Controller.FindingsOverview used to expose:
// the FIRST non-authoritative save records the (row, code) pair as new and
// stamps its FirstSeen, and a SECOND save of the same row records no new pair
// and does not restamp. The probe moved from the aggregate to the two Core
// accessors that fed it (FindingFirstSeenForType, NewFindingPairsSincePrev),
// which is where the save path actually writes. Do not restore an aggregate
// assertion: FindingsOverview was deleted as a caller-less API, not renamed.
func TestCacheFirstSeen_NewSincePrevViaControllerSave(t *testing.T) {
	ctrl, core := newTestControllerAndCore(t)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	row := resource.Resource{
		ID:       "bucket-fov-1",
		Name:     "fov-bucket",
		Type:     "s3",
		Fields:   map[string]string{"region": "us-east-1"},
		Findings: []domain.Finding{finding},
	}

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{row}, nil, false)

	if got := core.NewFindingPairsSincePrev()["s3"][finding.Code]; got < 1 {
		t.Errorf("save 1: NewFindingPairsSincePrev()[s3][%q] = %d, want >= 1 — no previous on-disk generation existed, so this (row, code) pair is new", finding.Code, got)
	}
	first := core.FindingFirstSeenForType("s3")[row.ID][finding.Code]
	if first.IsZero() {
		t.Fatal("save 1: FirstSeen is zero, want a real timestamp once the finding has been persisted")
	}

	time.Sleep(5 * time.Millisecond)
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{row}, nil, false)

	if got := core.NewFindingPairsSincePrev()["s3"][finding.Code]; got != 0 {
		t.Errorf("save 2: NewFindingPairsSincePrev()[s3][%q] = %d, want 0 — the (row, code) pair already existed in the previous on-disk generation", finding.Code, got)
	}
	if second := core.FindingFirstSeenForType("s3")[row.ID][finding.Code]; !second.Equal(first) {
		t.Errorf("save 2: FirstSeen = %v, want unchanged from save 1's %v", second, first)
	}
}

// TestNewFindingPairs_SurviveWave2CompletionSave pins the fix for the defect
// external review found in probes.go's #463 wiring: saveResourceListCache
// called session.SetNewFindingPairs unconditionally on every save,
// wholesale-replacing the type's delta — so a Wave-2-completion save (which
// diffs against the just-written Wave-1 generation, where the Wave-1 pair
// already has FirstSeen and is therefore no longer "new") stomped out the
// Wave-1 sweep's own new-pair record before anything could read it.
//
// Intended contract: a non-authoritative (Wave-1-style) save REPLACES the
// type's delta (a fresh one-step scan baseline); a Wave-2-authoritative save
// (a Wave2Answered type, the handleEnrichmentChecked "all done" dispatch) MERGES
// into it instead. Drives the real executor entry point
// (Core.ExecuteTask(TaskKindSaveCache)) directly, the same production seam
// TestExecuteTask_SaveCache_ExactIssueCount_SurvivesRowDerivedRecomputation in
// runtime_savecache_regressions_test.go uses, toggling
// SaveCachePayload.Wave2Answered for the two save kinds under test — this
// package's TestCacheFirstSeen_NewSincePrevViaControllerSave never exercises
// a Wave-2-authoritative save at all, only saveResourceListCache's non-
// authoritative branch via Controller.ApplyResourcesLoaded.
func TestNewFindingPairs_SurviveWave2CompletionSave(t *testing.T) {
	const shortName = "ec2"
	c := newSaveCacheRegressionCore(t, false)
	ctx := context.Background()

	wave1Finding := domain.Finding{Code: "ec2-impaired", Phrase: "instance status check failed", Severity: domain.SevBroken, Source: "wave1"}
	row1 := resource.Resource{ID: saveRegID("nfp", 0), Name: saveRegID("nfp", 0), Type: shortName, Findings: []domain.Finding{wave1Finding}}

	execSaveCache := func(resources []resource.Resource, wave2Complete bool) {
		t.Helper()
		// cachegen row 3: Wave-2 authority is per type (SaveCachePayload.
		// Wave2Answered), so a failed probe in a sweep cannot make that
		// sweep's save persist its type as clean. The two save kinds this
		// test toggles are unchanged otherwise.
		var wave2Answered map[string]bool
		if wave2Complete {
			wave2Answered = map[string]bool{shortName: true}
		}
		payload := &runtime.SaveCachePayload{
			Resources:     map[string][]resource.Resource{shortName: resources},
			Truncated:     map[string]bool{shortName: false},
			Wave2Answered: wave2Answered,
		}
		ev, err := c.ExecuteTask(ctx, runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
			Payload: payload,
		})
		if err != nil {
			t.Fatalf("ExecuteTask(TaskKindSaveCache, wave2Answered=%v): %v", wave2Complete, err)
		}
		if flash, ok := ev.(messages.Flash); ok && flash.IsError {
			t.Fatalf("ExecuteTask(TaskKindSaveCache, wave2Answered=%v) returned an error flash: %s", wave2Complete, flash.Text)
		}
	}

	// Save 1 — Wave-1-style sweep completion introduces a new wave1-sourced
	// pair on row1.
	execSaveCache([]resource.Resource{row1}, false)
	pairs1 := c.NewFindingPairsSincePrev()[shortName]
	if pairs1[wave1Finding.Code] < 1 {
		t.Fatalf("save 1: NewFindingPairsSincePrev()[%q][%q] = %d, want >= 1", shortName, wave1Finding.Code, pairs1[wave1Finding.Code])
	}

	// Save 2 — Wave-2 completion for the SAME type: row1/wave1Finding is
	// unchanged (already in the previous generation, so NOT a new pair on
	// this save's own diff), and a second row carries a genuinely new
	// wave2-sourced finding.
	wave2Finding := domain.Finding{Code: "ec2-untagged", Phrase: "missing required tags", Severity: domain.SevWarn, Source: "wave2:ec2"}
	row2 := resource.Resource{ID: saveRegID("nfp", 1), Name: saveRegID("nfp", 1), Type: shortName, Findings: []domain.Finding{wave2Finding}}
	execSaveCache([]resource.Resource{row1, row2}, true)

	pairs2 := c.NewFindingPairsSincePrev()[shortName]
	if pairs2[wave1Finding.Code] < 1 {
		t.Errorf("save 2 (Wave-2-answered): NewFindingPairsSincePrev()[%q][%q] = %d, want >= 1 — a Wave-2-authoritative save must MERGE into the type's delta, not replace it and drop the Wave-1 sweep's own new pair", shortName, wave1Finding.Code, pairs2[wave1Finding.Code])
	}
	if pairs2[wave2Finding.Code] < 1 {
		t.Errorf("save 2 (Wave-2-answered): NewFindingPairsSincePrev()[%q][%q] = %d, want >= 1 — the new Wave-2 finding's own pair must also be recorded", shortName, wave2Finding.Code, pairs2[wave2Finding.Code])
	}

	// Save 3 — a subsequent Wave-1-style save with no new findings: both
	// rows/findings are unchanged from save 2, so this save's own diff is
	// empty. Non-authoritative saves REPLACE the delta, so the type's
	// recorded delta must reset to empty here, not keep carrying save 2's
	// (now-stale) counts forward.
	execSaveCache([]resource.Resource{row1, row2}, false)
	pairs3 := c.NewFindingPairsSincePrev()[shortName]
	if n := pairs3[wave1Finding.Code]; n != 0 {
		t.Errorf("save 3: NewFindingPairsSincePrev()[%q][%q] = %d, want 0 — a non-authoritative save with no new pairs must REPLACE (reset) the type's delta, not carry save 2's stale count forward", shortName, wave1Finding.Code, n)
	}
	if n := pairs3[wave2Finding.Code]; n != 0 {
		t.Errorf("save 3: NewFindingPairsSincePrev()[%q][%q] = %d, want 0 — a non-authoritative save with no new pairs must REPLACE (reset) the type's delta, not carry save 2's stale count forward", shortName, wave2Finding.Code, n)
	}
}
