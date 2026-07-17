// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Issue #463 — per-finding FirstSeen persisted in the availability cache
// (cache.Row.FindingFirstSeen), plus the new-since-previous-scan deltas
// surfaced through app.FindingsGroup (OldestFirstSeen / NewSincePrev).
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

	store1 := cache.LoadDir("demo", "us-east-1")
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

	store2 := cache.LoadDir("demo", "us-east-1")
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
		// Source "wave1", not "wave2:s3": a Wave-2-sourced finding's absence
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

	store1 := cache.LoadDir("demo", "us-east-1")
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

	store2 := cache.LoadDir("demo", "us-east-1")
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
	store1 := cache.LoadDir("demo", "us-east-1")
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

	store3 := cache.LoadDir("demo", "us-east-1")
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

	dir := cache.Dir("migrate-profile", "us-east-1")
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

	store := cache.LoadDir("migrate-profile", "us-east-1")

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

func findFindingsGroup(t *testing.T, groups []app.FindingsGroup, code domain.FindingCode) app.FindingsGroup {
	t.Helper()
	for _, g := range groups {
		if g.Code == code {
			return g
		}
	}
	t.Fatalf("FindingsOverview() has no group for code %q (groups=%+v)", code, groups)
	return app.FindingsGroup{}
}

func TestFindingsOverview_FirstSeenAndNewSincePrev(t *testing.T) {
	ctrl, _ := newTestControllerAndCore(t)
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

	groups1, _ := ctrl.FindingsOverview()
	g1 := findFindingsGroup(t, groups1, finding.Code)
	if g1.NewSincePrev < 1 {
		t.Errorf("save 1: NewSincePrev = %d, want >= 1 — no previous on-disk generation existed, so this (row, code) pair is new", g1.NewSincePrev)
	}
	if g1.OldestFirstSeen.IsZero() {
		t.Error("save 1: OldestFirstSeen is zero, want a real timestamp once the finding has been persisted")
	}

	time.Sleep(5 * time.Millisecond)
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{row}, nil, false)

	groups2, _ := ctrl.FindingsOverview()
	g2 := findFindingsGroup(t, groups2, finding.Code)
	if g2.NewSincePrev != 0 {
		t.Errorf("save 2: NewSincePrev = %d, want 0 — the (row, code) pair already existed in the previous on-disk generation", g2.NewSincePrev)
	}
	if !g2.OldestFirstSeen.Equal(g1.OldestFirstSeen) {
		t.Errorf("save 2: OldestFirstSeen = %v, want unchanged from save 1's %v", g2.OldestFirstSeen, g1.OldestFirstSeen)
	}
}
