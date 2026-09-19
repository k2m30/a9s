package unit

// Status reaches the list column from Findings: phraseFromFindings(r.Findings)
// in extractCellValue merges wave-1 findings (from the fetcher) and wave-2
// findings (from applyEnrichment) at render time.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type testSnap struct {
	ID        string
	ParentID  string
	Type      string
	CreatedAt time.Time
}

type testParent struct {
	ID                    string
	BackupRetentionPeriod int32
}

func makeCrossRefCfg(retentionEnabled bool) awsclient.SnapshotCrossRefConfig {
	return awsclient.SnapshotCrossRefConfig{
		ParentShortName: "test-parent",
		GetParentID: func(raw any) (string, bool) {
			s, ok := raw.(testSnap)
			if !ok || s.ParentID == "" {
				return "", false
			}
			return s.ParentID, true
		},
		GetCreatedAt: func(raw any) (time.Time, bool) {
			s, ok := raw.(testSnap)
			if !ok {
				return time.Time{}, false
			}
			return s.CreatedAt, !s.CreatedAt.IsZero()
		},
		GetSnapshotType: func(raw any) (string, bool) {
			s, ok := raw.(testSnap)
			if !ok {
				return "", false
			}
			return s.Type, s.Type != ""
		},
		GetParentRetention: func(raw any) (int32, bool) {
			p, ok := raw.(testParent)
			if !ok {
				return 0, false
			}
			return p.BackupRetentionPeriod, p.BackupRetentionPeriod > 0
		},
		// The wording comes from the catalog by code, so a config with no codes
		// emits nothing.
		OrphanCode:        "dbi-snap.orphan",
		PastRetentionCode: "dbi-snap.past-retention",
		ParentRowLabel:    "Source Parent",
		RetentionEnabled:  retentionEnabled,
	}
}

func snapRes(snap testSnap) resource.Resource {
	return resource.Resource{
		ID:        snap.ID,
		RawStruct: snap,
	}
}

func snapResWithStatus(snap testSnap, status string, issues []string) resource.Resource {
	r := resource.Resource{
		ID:        snap.ID,
		Fields:    map[string]string{},
		RawStruct: snap,
	}
	if status != "" {
		r.Fields["status"] = status
	}
	for _, phrase := range issues {
		r.Findings = append(r.Findings, domain.Finding{
			Code:     domain.FindingCode("wave1." + phrase),
			Phrase:   phrase,
			Severity: domain.SevBroken,
			Source:   "wave1",
		})
	}
	return r
}

func parentCache(truncated bool, parents ...testParent) resource.ResourceCache {
	entries := make([]resource.Resource, 0, len(parents))
	for _, p := range parents {
		entries = append(entries, resource.Resource{
			ID:        p.ID,
			RawStruct: p,
		})
	}
	return resource.ResourceCache{
		"test-parent": resource.ResourceCacheEntry{
			Resources:   entries,
			IsTruncated: truncated,
		},
	}
}

func TestSnapshotCrossRef_EmptyParentCache(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := resource.ResourceCache{}

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if len(result.FieldUpdates) != 0 {
		t.Errorf("expected 0 FieldUpdates, got %d", len(result.FieldUpdates))
	}
}

func TestSnapshotCrossRef_ParentFound_NoFinding(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	parent := testParent{ID: "p1", BackupRetentionPeriod: 30}
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, parent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := result.Findings["snap-1"]; found {
		t.Errorf("expected no finding for snap-1, got: %+v", result.Findings["snap-1"])
	}
	if _, found := result.FieldUpdates["snap-1"]; found {
		t.Errorf("expected no FieldUpdates for snap-1")
	}
}

// A truncated cache may hold the parent on a later page, so a missing parent
// is no orphan.
func TestSnapshotCrossRef_TruncatedCache_NoFalseOrphan(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	existingParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(true, existingParent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := result.Findings["snap-1"]; found {
		t.Errorf("expected no orphan finding for truncated cache, got: %+v", result.Findings["snap-1"])
	}
	if _, found := result.FieldUpdates["snap-1"]; found {
		t.Errorf("expected no FieldUpdates for truncated cache")
	}
}

func TestSnapshotCrossRef_OrphanFinding(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, otherParent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	findings, found := result.Findings["snap-1"]
	if !found {
		t.Fatal("expected orphan finding for snap-1, got none")
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Errorf("expected Severity=SevBroken, got %v", finding.Severity)
	}
	// The wording is the code's, declared once in the catalog, not a string
	// this config carries.
	if want := catalog.Phrase("dbi-snap.orphan"); finding.Phrase != want {
		t.Errorf("expected Phrase=%q, got %q", want, finding.Phrase)
	}

	found = false
	for _, row := range result.AttentionDetails["snap-1"][finding.Code].Rows {
		if row.Label == "Source Parent" {
			found = true
			if !crossRefContains(row.Value, "p1") {
				t.Errorf("Source Parent row Value should contain %q, got %q", "p1", row.Value)
			}
			if !crossRefContains(row.Value, "not in loaded list") {
				t.Errorf("Source Parent row Value should contain hint %q, got %q", "not in loaded list", row.Value)
			}
		}
	}
	if !found {
		t.Errorf("expected a row with Label=%q, rows were: %+v", "Source Parent", result.AttentionDetails["snap-1"][finding.Code].Rows)
	}

	if updates, hasUpdates := result.FieldUpdates["snap-1"]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected no FieldUpdates entry for snap-1 (status overlay removed); got %v", updates)
	}
}

func TestSnapshotCrossRef_PastRetention_Automated(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	const retentionDays = 7
	const ageDays = 30
	parent := testParent{ID: "p1", BackupRetentionPeriod: retentionDays}
	createdAt := time.Now().Add(-ageDays * 24 * time.Hour)
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: createdAt}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, parent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	findings, found := result.Findings["snap-1"]
	if !found {
		t.Fatal("expected past-retention finding for snap-1, got none")
	}
	finding := findings[0]
	if finding.Severity != domain.SevBroken {
		t.Errorf("expected Severity=SevBroken, got %v", finding.Severity)
	}

	// The declared wording with the days-over count in its slot; the count is
	// the only thing the enricher supplies.
	expectedDaysOver := ageDays - retentionDays
	expectedPhrase := strings.Replace(catalog.Phrase("dbi-snap.past-retention"),
		"<N>", strconv.Itoa(expectedDaysOver), 1)
	if finding.Phrase != expectedPhrase {
		t.Errorf("expected Phrase=%q, got %q", expectedPhrase, finding.Phrase)
	}

	hasParentRow := false
	hasRetentionRow := false
	hasCreatedRow := false
	for _, row := range result.AttentionDetails["snap-1"][finding.Code].Rows {
		switch row.Label {
		case "Source Parent":
			hasParentRow = true
			if !crossRefContains(row.Value, "p1") {
				t.Errorf("Source Parent row Value should contain %q, got %q", "p1", row.Value)
			}
		case "Retention":
			hasRetentionRow = true
			if !crossRefContains(row.Value, fmt.Sprintf("%d", retentionDays)) {
				t.Errorf("Retention row Value should contain %q, got %q", fmt.Sprintf("%d days", retentionDays), row.Value)
			}
		case "Created":
			hasCreatedRow = true
			wantDate := createdAt.Format("2006-01-02")
			if !crossRefContains(row.Value, wantDate) {
				t.Errorf("Created row Value should contain %q, got %q", wantDate, row.Value)
			}
		}
	}
	if !hasParentRow {
		t.Errorf("missing Source Parent row; rows: %+v", result.AttentionDetails["snap-1"][finding.Code].Rows)
	}
	if !hasRetentionRow {
		t.Errorf("missing Retention row; rows: %+v", result.AttentionDetails["snap-1"][finding.Code].Rows)
	}
	if !hasCreatedRow {
		t.Errorf("missing Created row; rows: %+v", result.AttentionDetails["snap-1"][finding.Code].Rows)
	}

	if updates, hasUpdates := result.FieldUpdates["snap-1"]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected no FieldUpdates entry for snap-1 (status overlay removed); got %v", updates)
	}
}

// The past-retention rule applies to automated snapshots only.
func TestSnapshotCrossRef_PastRetention_Manual(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	parent := testParent{ID: "p1", BackupRetentionPeriod: 7}
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "manual", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, parent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := result.Findings["snap-1"]; found {
		t.Errorf("expected no finding for manual snapshot past retention, got: %+v", result.Findings["snap-1"])
	}
	if _, found := result.FieldUpdates["snap-1"]; found {
		t.Errorf("expected no FieldUpdates for manual snapshot")
	}
}

func TestSnapshotCrossRef_ZeroRetentionParent(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	parent := testParent{ID: "p1", BackupRetentionPeriod: 0} // retention disabled
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, parent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := result.Findings["snap-1"]; found {
		t.Errorf("expected no finding for zero-retention parent, got: %+v", result.Findings["snap-1"])
	}
}

func TestSnapshotCrossRef_RetentionDisabled(t *testing.T) {
	t.Run("no_past_retention_when_disabled", func(t *testing.T) {
		cfg := makeCrossRefCfg(false)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		parent := testParent{ID: "p1", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
		resources := []resource.Resource{snapRes(snap)}
		cache := parentCache(false, parent)

		result, err := fn(context.Background(), nil, resources, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, found := result.Findings["snap-1"]; found {
			t.Errorf("expected no past-retention finding when RetentionEnabled=false, got: %+v", result.Findings["snap-1"])
		}
	})

	t.Run("orphan_still_fires_when_retention_disabled", func(t *testing.T) {
		cfg := makeCrossRefCfg(false)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-2", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
		resources := []resource.Resource{snapRes(snap)}
		cache := parentCache(false, otherParent)

		result, err := fn(context.Background(), nil, resources, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, found := result.Findings["snap-2"]; !found {
			t.Error("expected orphan finding when RetentionEnabled=false but parent missing; got none")
		}
	})
}

func TestSnapshotCrossRef_OrphanFinding_NoFieldUpdates_WithWave1(t *testing.T) {
	t.Run("legacy_form_single_wave1_phrase_plus_orphan", func(t *testing.T) {
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"}
		res := snapResWithStatus(snap, "unencrypted", []string{"unencrypted"})
		cache := parentCache(false, otherParent)

		result, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok := result.Findings["snap-1"]; !ok {
			t.Errorf("expected orphan Finding for snap-1 — wave-1 stacking must NOT suppress the wave-2 Finding")
		}

		if updates, hasUpdates := result.FieldUpdates["snap-1"]; hasUpdates && len(updates) != 0 {
			t.Errorf("AS-140: expected empty FieldUpdates for snap-1; got %v", updates)
		}
	})

	t.Run("legacy_form_multi_wave1_phrases_plus_orphan", func(t *testing.T) {
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"}
		res := snapResWithStatus(snap, "unencrypted (+1)", []string{"unencrypted", "publicly accessible"})
		cache := parentCache(false, otherParent)

		result, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok := result.Findings["snap-1"]; !ok {
			t.Errorf("expected orphan Finding for snap-1")
		}
		if updates, hasUpdates := result.FieldUpdates["snap-1"]; hasUpdates && len(updates) != 0 {
			t.Errorf("AS-140: expected empty FieldUpdates for snap-1; got %v", updates)
		}
	})
}

func TestSnapshotCrossRef_Idempotent(t *testing.T) {
	t.Run("orphan_idempotent", func(t *testing.T) {
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"}
		res := snapRes(snap)
		cache := parentCache(false, otherParent)

		result1, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("run1 unexpected error: %v", err)
		}
		result2, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("run2 unexpected error: %v", err)
		}

		assertResultsIdentical(t, "snap-1", result1, result2)
	})

	t.Run("past_retention_idempotent", func(t *testing.T) {
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		parent := testParent{ID: "p1", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-30 * 24 * time.Hour)}
		res := snapRes(snap)
		cache := parentCache(false, parent)

		result1, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("run1 unexpected error: %v", err)
		}
		result2, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("run2 unexpected error: %v", err)
		}

		assertResultsIdentical(t, "snap-1", result1, result2)
	})
}

func TestSnapshotCrossRef_PostPR03eShape_NoFieldUpdates(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
	snap := testSnap{ID: "snap-1", ParentID: "p1"}

	res := resource.Resource{
		ID: snap.ID,
		Findings: []domain.Finding{
			{Code: "dbi-snap.warn.unencrypted", Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields:    map[string]string{"status": "unencrypted"},
		RawStruct: snap,
	}
	cache := parentCache(false, otherParent)

	result, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings["snap-1"]; !ok {
		t.Errorf("expected orphan Finding for snap-1 even on post-PR-03e wave-1 shape")
	}

	if updates, hasUpdates := result.FieldUpdates["snap-1"]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for snap-1 (status overlay removed); got %v", updates)
	}
}

func crossRefContains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func assertResultsIdentical(t *testing.T, id string, r1, r2 awsclient.IssueEnricherResult) {
	t.Helper()

	fs1, ok1 := r1.Findings[id]
	fs2, ok2 := r2.Findings[id]

	if ok1 != ok2 {
		t.Errorf("idempotency: run1 finding present=%v, run2 finding present=%v for %q", ok1, ok2, id)
		return
	}
	if ok1 {
		f1, f2 := fs1[0], fs2[0]
		if f1.Severity != f2.Severity {
			t.Errorf("idempotency: Severity run1=%v run2=%v for %q", f1.Severity, f2.Severity, id)
		}
		if f1.Phrase != f2.Phrase {
			t.Errorf("idempotency: Phrase run1=%q run2=%q for %q", f1.Phrase, f2.Phrase, id)
		}
		rows1 := r1.AttentionDetails[id][f1.Code].Rows
		rows2 := r2.AttentionDetails[id][f2.Code].Rows
		if len(rows1) != len(rows2) {
			t.Errorf("idempotency: len(Rows) run1=%d run2=%d for %q", len(rows1), len(rows2), id)
		}
	}

	u1, ok1 := r1.FieldUpdates[id]
	u2, ok2 := r2.FieldUpdates[id]
	if ok1 != ok2 {
		t.Errorf("idempotency: run1 FieldUpdates present=%v, run2 present=%v for %q", ok1, ok2, id)
		return
	}
	if ok1 && u1["status"] != u2["status"] {
		t.Errorf("idempotency: FieldUpdates[status] run1=%q run2=%q for %q", u1["status"], u2["status"], id)
	}
}
