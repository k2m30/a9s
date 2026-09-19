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

// Per-finding FirstSeen persisted in the availability cache
// (cache.Row.FindingFirstSeen), read back off the file the save wrote.
// A first observation time cannot be reconstructed once it is not recorded.

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

	ctrl.WaitForCacheWrites()
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

	ctrl.WaitForCacheWrites()
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
		// A Wave-2 finding absent from an incoming row is carried forward
		// (carryWave2ForRows, the flicker guard); only a Wave-1 finding's absence
		// is a resolution.
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

	ctrl.WaitForCacheWrites()
	store1 := cache.LoadDirForTest("demo", "us-east-1")
	tf1, ok := store1.Type("s3")
	if !ok || len(tf1.Rows) != 1 {
		t.Fatalf("save 1: store.Type(s3) = (%+v, %v), want exactly 1 row", tf1, ok)
	}
	if _, hasFirstSeen := tf1.Rows[0].FindingFirstSeen[finding.Code]; !hasFirstSeen {
		t.Fatalf("save 1: FindingFirstSeen[%q] missing before resolution", finding.Code)
	}

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{{
		ID:     rowID,
		Name:   "fs-resolve-bucket",
		Type:   "s3",
		Fields: map[string]string{"region": "us-east-1"},
	}}, nil, false)

	ctrl.WaitForCacheWrites()
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
		// Wave-1: a carried-forward Wave-2 finding would make the reappearance
		// indistinguishable from never having left.
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
	ctrl.WaitForCacheWrites()
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

	ctrl.WaitForCacheWrites()
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
