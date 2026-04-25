package unit

// aws_snapshot_cross_ref_test.go — TDD-first tests for EnrichSnapshotCrossRef helper.
//
// The helper (internal/aws/snapshot_cross_ref.go) does NOT exist yet. These
// tests are intentionally uncompilable until the coder creates it.
//
// Design: the helper is a parameterized IssueEnricherFunc factory that handles
// the orphan + past-retention cross-ref pattern shared across dbi-snap, dbc-snap,
// and future snapshot types.
//
// Test strategy:
//   - All stubs (testSnap, testParent) are defined inline — no AWS SDK imports.
//   - cfg is built via makeCrossRefCfg helper — one canonical config per run.
//   - Each subtest is independent; no shared state between runs.

import (
	"context"
	"fmt"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Inline stubs — no AWS SDK dependency
// ---------------------------------------------------------------------------

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

// makeCrossRefCfg builds a SnapshotCrossRefConfig backed by testSnap / testParent.
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
		OrphanPhrase:    "orphan: source parent deleted",
		ParentRowLabel:  "Source Parent",
		RetentionPhrase: func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) },
		RetentionEnabled: retentionEnabled,
	}
}

// snapRes builds a minimal resource.Resource for enricher input.
func snapRes(snap testSnap) resource.Resource {
	return resource.Resource{
		ID:        snap.ID,
		Status:    "",
		RawStruct: snap,
	}
}

// snapResWithStatus builds a resource.Resource with a pre-existing Status and
// Issues slice (simulates a Wave-1 fetcher that emitted findings before Wave 2).
func snapResWithStatus(snap testSnap, status string, issues []string) resource.Resource {
	return resource.Resource{
		ID:        snap.ID,
		Status:    status,
		Issues:    issues,
		RawStruct: snap,
	}
}

// parentCache builds a ResourceCache with "test-parent" entries.
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSnapshotCrossRef_EmptyParentCache verifies that when the "test-parent"
// key is absent from the cache the enricher returns zero findings and no error.
func TestSnapshotCrossRef_EmptyParentCache(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := resource.ResourceCache{} // no "test-parent" key at all

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

// TestSnapshotCrossRef_ParentFound_NoFinding verifies that when the snapshot's
// parent IS present in the cache and the snapshot is within retention, no
// finding is emitted.
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

// TestSnapshotCrossRef_TruncatedCache_NoFalseOrphan verifies that when the
// cache IsTruncated=true and the parent is NOT found, no orphan finding is
// emitted (avoids false positives when the parent might exist in a later page).
func TestSnapshotCrossRef_TruncatedCache_NoFalseOrphan(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	// cache has a different parent, IsTruncated=true
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

// TestSnapshotCrossRef_OrphanFinding verifies that when the parent is NOT in
// the cache and the cache is NOT truncated, a full orphan finding is emitted.
func TestSnapshotCrossRef_OrphanFinding(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	// cache has "p2" but snap references "p1"
	otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
	snap := testSnap{ID: "snap-1", ParentID: "p1", Type: "automated", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)}
	resources := []resource.Resource{snapRes(snap)}
	cache := parentCache(false, otherParent)

	result, err := fn(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	finding, found := result.Findings["snap-1"]
	if !found {
		t.Fatal("expected orphan finding for snap-1, got none")
	}

	if finding.Severity != "!" {
		t.Errorf("expected Severity=%q, got %q", "!", finding.Severity)
	}
	if finding.Summary != "orphan: source parent deleted" {
		t.Errorf("expected Summary=%q, got %q", "orphan: source parent deleted", finding.Summary)
	}

	// Must contain a row with Label="Source Parent" and Value containing "p1" and the hint.
	found = false
	for _, row := range finding.Rows {
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
		t.Errorf("expected a row with Label=%q, rows were: %+v", "Source Parent", finding.Rows)
	}

	// FieldUpdates must carry the orphan phrase as "status".
	updates, hasUpdates := result.FieldUpdates["snap-1"]
	if !hasUpdates {
		t.Fatal("expected FieldUpdates for snap-1, got none")
	}
	if updates["status"] != "orphan: source parent deleted" {
		t.Errorf("expected FieldUpdates[status]=%q, got %q", "orphan: source parent deleted", updates["status"])
	}
}

// TestSnapshotCrossRef_PastRetention_Automated verifies that an automated
// snapshot whose age exceeds the parent's BackupRetentionPeriod gets a
// past-retention finding.
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

	finding, found := result.Findings["snap-1"]
	if !found {
		t.Fatal("expected past-retention finding for snap-1, got none")
	}
	if finding.Severity != "!" {
		t.Errorf("expected Severity=%q, got %q", "!", finding.Severity)
	}

	// Summary should match "automated, <N>d past retention" where N ≈ ageDays - retentionDays.
	expectedDaysOver := ageDays - retentionDays
	expectedPhrase := fmt.Sprintf("automated, %dd past retention", expectedDaysOver)
	if finding.Summary != expectedPhrase {
		t.Errorf("expected Summary=%q, got %q", expectedPhrase, finding.Summary)
	}

	// Rows must contain Source Parent, Retention, Created entries.
	hasParentRow := false
	hasRetentionRow := false
	hasCreatedRow := false
	for _, row := range finding.Rows {
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
		t.Errorf("missing Source Parent row; rows: %+v", finding.Rows)
	}
	if !hasRetentionRow {
		t.Errorf("missing Retention row; rows: %+v", finding.Rows)
	}
	if !hasCreatedRow {
		t.Errorf("missing Created row; rows: %+v", finding.Rows)
	}

	// FieldUpdates must carry the past-retention phrase as "status".
	updates, hasUpdates := result.FieldUpdates["snap-1"]
	if !hasUpdates {
		t.Fatal("expected FieldUpdates for snap-1, got none")
	}
	if updates["status"] != expectedPhrase {
		t.Errorf("expected FieldUpdates[status]=%q, got %q", expectedPhrase, updates["status"])
	}
}

// TestSnapshotCrossRef_PastRetention_Manual verifies that a manual snapshot
// past retention does NOT trigger the past-retention finding (rule applies to
// "automated" only).
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

// TestSnapshotCrossRef_ZeroRetentionParent verifies that when the parent has
// BackupRetentionPeriod=0 the past-retention rule does not fire.
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

// TestSnapshotCrossRef_RetentionDisabled verifies that when RetentionEnabled=false
// the past-retention rule never fires even when age > retention, and the orphan
// rule is unaffected.
func TestSnapshotCrossRef_RetentionDisabled(t *testing.T) {
	t.Run("no_past_retention_when_disabled", func(t *testing.T) {
		cfg := makeCrossRefCfg(false) // RetentionEnabled=false
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		// Same setup that would normally produce a past-retention finding.
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
		cfg := makeCrossRefCfg(false) // RetentionEnabled=false
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		// cache has "p2", snap references "p1" — should still orphan.
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

// TestSnapshotCrossRef_FieldUpdatesStatusMerge verifies BumpFindingSuffix
// integration: when the resource already has Wave-1 issues the enricher's new
// phrase bumps the suffix correctly.
func TestSnapshotCrossRef_FieldUpdatesStatusMerge(t *testing.T) {
	t.Run("case_A_single_wave1_phrase_plus_orphan", func(t *testing.T) {
		// Pre-existing status: "unencrypted" (one Wave-1 phrase, no suffix).
		// Helper adds 1 cross-ref phrase (orphan).
		// Expected merged status: "unencrypted (+1)".
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"} // orphan — p1 not in cache
		res := snapResWithStatus(snap, "unencrypted", []string{"unencrypted"})
		cache := parentCache(false, otherParent)

		result, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		updates, hasUpdates := result.FieldUpdates["snap-1"]
		if !hasUpdates {
			t.Fatal("expected FieldUpdates for snap-1, got none")
		}
		const wantStatus = "unencrypted (+1)"
		if updates["status"] != wantStatus {
			t.Errorf("case A: expected status=%q, got %q", wantStatus, updates["status"])
		}
	})

	t.Run("case_B_multi_wave1_phrases_plus_orphan", func(t *testing.T) {
		// Pre-existing status: "unencrypted (+1)" (two Wave-1 phrases, suffix already set).
		// Helper adds 1 cross-ref phrase (orphan).
		// Expected merged status: "unencrypted (+2)".
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"} // orphan
		res := snapResWithStatus(snap, "unencrypted (+1)", []string{"unencrypted", "publicly accessible"})
		cache := parentCache(false, otherParent)

		result, err := fn(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		updates, hasUpdates := result.FieldUpdates["snap-1"]
		if !hasUpdates {
			t.Fatal("expected FieldUpdates for snap-1, got none")
		}
		const wantStatus = "unencrypted (+2)"
		if updates["status"] != wantStatus {
			t.Errorf("case B: expected status=%q, got %q", wantStatus, updates["status"])
		}
	})
}

// TestSnapshotCrossRef_Idempotent verifies that running the enricher twice on
// the same inputs produces identical output (no suffix accumulation on re-runs).
func TestSnapshotCrossRef_Idempotent(t *testing.T) {
	t.Run("orphan_idempotent", func(t *testing.T) {
		cfg := makeCrossRefCfg(true)
		fn := awsclient.EnrichSnapshotCrossRef(cfg)

		otherParent := testParent{ID: "p2", BackupRetentionPeriod: 7}
		snap := testSnap{ID: "snap-1", ParentID: "p1"} // orphan
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// crossRefContains is a simple substring helper local to this test file.
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

// assertResultsIdentical checks that two IssueEnricherResults are identical for
// a specific resource ID (Findings + FieldUpdates).
func assertResultsIdentical(t *testing.T, id string, r1, r2 awsclient.IssueEnricherResult) {
	t.Helper()

	f1, ok1 := r1.Findings[id]
	f2, ok2 := r2.Findings[id]

	if ok1 != ok2 {
		t.Errorf("idempotency: run1 finding present=%v, run2 finding present=%v for %q", ok1, ok2, id)
		return
	}
	if ok1 {
		if f1.Severity != f2.Severity {
			t.Errorf("idempotency: Severity run1=%q run2=%q for %q", f1.Severity, f2.Severity, id)
		}
		if f1.Summary != f2.Summary {
			t.Errorf("idempotency: Summary run1=%q run2=%q for %q", f1.Summary, f2.Summary, id)
		}
		if len(f1.Rows) != len(f2.Rows) {
			t.Errorf("idempotency: len(Rows) run1=%d run2=%d for %q", len(f1.Rows), len(f2.Rows), id)
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
