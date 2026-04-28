// phase03_fold_test.go — TDD red-light tests for PR-03a-fold.
//
// PR-03a-fold replaces the parallel m.EnrichmentFindings map with direct
// mutation of cached row Findings/AttentionDetails via a new applyEnrichment
// method on Model. These tests define the behavior the fold PR must satisfy.
//
// ── What is being tested ────────────────────────────────────────────────────
//
//	After EnrichmentCheckedMsg is handled, every cached row of the given
//	resource type must have its r.Findings and r.AttentionDetails updated
//	in-place. The parallel Session.EnrichmentFindings map is deleted entirely.
//
// ── Red-light expectations (before PR-03a-fold) ─────────────────────────────
//
//	Test 1/ProbeResources: fails because the handler's "all-enrichment-done"
//	    cleanup (m.EnrichChecked >= m.EnrichTotal → m.ProbeResources = nil) runs
//	    before the test can inspect the cache. After fold, applyEnrichment
//	    must run BEFORE cleanup AND tests seed EnrichTotal=2 to keep ProbeResources
//	    alive for inspection. Without EnrichTotal=2, this subtest ALWAYS fails.
//
//	Test 4: Session.EnrichmentFindings still exists — the reflection check
//	    fails with "EnrichmentFindings field still exists".
//
//	Tests 2, 3, 5: currently PASS with the shim (DeriveFindings is deterministic).
//	    They serve as regression pins: if fold incorrectly appends wave2 instead
//	    of replacing, test 2 catches it (len==3 instead of 2). If fold forgets to
//	    clear wave2 on empty input, test 3 catches it. If fold wipes wave1 when
//	    writing wave2, test 5 catches it.
//
// ── Green-light (after PR-03a-fold) ─────────────────────────────────────────
//
//	applyEnrichment directly mutates r.Findings/r.AttentionDetails, the
//	parallel map is deleted, and all five tests pass.
//
// Run:
//
//	go test ./tests/unit/ -count=1 -run TestFold_ -v
package unit_test

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ── Test 1: EnrichmentCheckedMsg mutates rows directly in all three caches ──

// TestFold_EnrichmentCheckedMutatesRowsDirectly verifies that after the fold
// implementation handles EnrichmentCheckedMsg, the wave 2 finding is written
// DIRECTLY into the cached row's r.Findings slice — not via the parallel
// EnrichmentFindings map.
//
// Specifically:
//   - Resource with Status "running" (a lifecycle phrase, filtered by wave1)
//     should yield exactly one finding after enrichment — the wave2 entry.
//   - That wave2 finding must have Phrase == ef.Summary and Source == "wave2:ec2".
//   - r.AttentionDetails[code].Rows must contain the EnrichmentFinding's Rows.
//   - The same row mutation must occur for LazyResourceCache and ProbeResources.
//
// Red-light today:
//   - ResourceCache and LazyResourceCache subtests PASS with the current shim
//     (DeriveFindings correctly filters "running" and emits only wave2).
//   - ProbeResources subtest FAILS: the handler's all-enrichment-done cleanup
//     (EnrichChecked >= EnrichTotal → ProbeResources = nil) fires before the
//     test can inspect the cache. The subtest seeds EnrichTotal=2 to prevent
//     the cleanup, making this a genuine red-light until fold is implemented.
func TestFold_EnrichmentCheckedMutatesRowsDirectly(t *testing.T) {
	const (
		rid        = "i-1"
		wantPhrase = "pending maintenance"
		wantSource = "wave2:ec2"
	)

	ef := resource.EnrichmentFinding{
		Severity: "~",
		Summary:  wantPhrase,
		Rows: []resource.FindingRow{
			{Label: "Action", Value: "instance-retirement"},
			{Label: "Earliest Target", Value: "2026-05-01"},
		},
	}

	wantCode := domain.FindingCode("ec2.pending.maintenance")

	t.Run("ResourceCache", func(t *testing.T) {
		m := newShimModel()

		m.ResourceCache["ec2"] = &session.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: rid, Name: "test-ec2", Status: "running"},
			},
		}

		m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
			ResourceType: "ec2",
			Findings:     map[string]resource.EnrichmentFinding{rid: ef},
			Gen:          0, // test-injection bypass
			TypeGen:      0, // test-injection bypass
		})

		entry, ok := m.ResourceCache["ec2"]
		if !ok || len(entry.Resources) == 0 {
			t.Fatal("ResourceCache[ec2] is empty after EnrichmentCheckedMsg")
		}
		r := entry.Resources[0]

		// After fold: "running" is a lifecycle phrase → filtered by wave1.
		// The single finding must be the wave2 entry from the EnrichmentFinding.
		if len(r.Findings) != 1 {
			t.Errorf("ResourceCache[ec2].Resources[0].Findings: got len=%d, want 1 (only wave2; wave1 'running' is lifecycle-filtered)", len(r.Findings))
		} else {
			f := r.Findings[0]
			if f.Phrase != wantPhrase {
				t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, wantPhrase)
			}
			if f.Source != wantSource {
				t.Errorf("Findings[0].Source: got %q, want %q", f.Source, wantSource)
			}
		}

		// AttentionDetails must carry the EnrichmentFinding's Rows.
		if len(r.AttentionDetails) == 0 {
			t.Errorf("AttentionDetails: got empty, want entry for code %q", wantCode)
		} else if detail, ok := r.AttentionDetails[wantCode]; !ok {
			t.Errorf("AttentionDetails[%q]: not found; keys: %v", wantCode, r.AttentionDetails)
		} else if len(detail.Rows) != len(ef.Rows) {
			t.Errorf("AttentionDetails[%q].Rows: got len=%d, want %d", wantCode, len(detail.Rows), len(ef.Rows))
		} else {
			for i, row := range detail.Rows {
				if row.Label != ef.Rows[i].Label || row.Value != ef.Rows[i].Value {
					t.Errorf("AttentionDetails[%q].Rows[%d]: got {%q,%q}, want {%q,%q}",
						wantCode, i, row.Label, row.Value, ef.Rows[i].Label, ef.Rows[i].Value)
				}
			}
		}
	})

	t.Run("LazyResourceCache", func(t *testing.T) {
		m := newShimModel()

		m.LazyResourceCache["ec2"] = []resource.Resource{
			{ID: rid, Name: "lazy-ec2", Status: "running"},
		}

		m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
			ResourceType: "ec2",
			Findings:     map[string]resource.EnrichmentFinding{rid: ef},
			Gen:          0,
			TypeGen:      0,
		})

		lazySlice, ok := m.LazyResourceCache["ec2"]
		if !ok || len(lazySlice) == 0 {
			t.Fatal("LazyResourceCache[ec2] is empty after EnrichmentCheckedMsg")
		}
		r := lazySlice[0]

		if len(r.Findings) != 1 {
			t.Errorf("LazyResourceCache[ec2][0].Findings: got len=%d, want 1 (wave2 only; wave1 'running' is lifecycle-filtered)", len(r.Findings))
		} else if r.Findings[0].Phrase != wantPhrase {
			t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, wantPhrase)
		} else if r.Findings[0].Source != wantSource {
			t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, wantSource)
		}
	})

	t.Run("ProbeResources", func(t *testing.T) {
		m := newShimModel()

		// Prevent the "all enrichment done" cleanup path (app_handlers_availability.go:
		// if m.EnrichChecked >= m.EnrichTotal { m.ProbeResources = nil }).
		// After EnrichChecked++ fires (0→1), we need 1 < EnrichTotal to avoid
		// the cleanup so ProbeResources remains inspectable. Setting EnrichTotal=2
		// simulates "one type still pending", keeping the cache alive.
		m.EnrichTotal = 2

		// ProbeResources is initialized via AvailabilityCheckedMsg in real usage,
		// but for the fold test we set it directly — the fold must walk ProbeResources
		// just as it walks the other two caches.
		if m.ProbeResources == nil {
			m.ProbeResources = make(map[string][]resource.Resource)
		}
		m.ProbeResources["ec2"] = []resource.Resource{
			{ID: rid, Name: "probe-ec2", Status: "running"},
		}

		m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
			ResourceType: "ec2",
			Findings:     map[string]resource.EnrichmentFinding{rid: ef},
			Gen:          0,
			TypeGen:      0,
		})

		probeSlice, ok := m.ProbeResources["ec2"]
		if !ok || len(probeSlice) == 0 {
			t.Fatal("ProbeResources[ec2] is empty after EnrichmentCheckedMsg (fold path must update ProbeResources before cleanup)")
		}
		r := probeSlice[0]

		if len(r.Findings) != 1 {
			t.Errorf("ProbeResources[ec2][0].Findings: got len=%d, want 1 (wave2 only; wave1 'running' is lifecycle-filtered)", len(r.Findings))
		} else if r.Findings[0].Phrase != wantPhrase {
			t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, wantPhrase)
		} else if r.Findings[0].Source != wantSource {
			t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, wantSource)
		}
	})
}

// ── Test 2: Repeated EnrichmentCheckedMsg replaces wave2, preserves wave1 ───

// TestFold_RepeatedEnrichmentReplacesWave2 verifies that a second
// EnrichmentCheckedMsg replaces the wave2 finding, not stacks on top.
//
// Setup:
//   - Seed a cached row with Status "impaired" (→ wave1 finding after derive).
//   - Send EnrichmentCheckedMsg with wave2 finding A ("pending maintenance").
//   - Send a second EnrichmentCheckedMsg with wave2 finding B ("retirement scheduled").
//
// After the second call:
//   - len(r.Findings) == 2: wave1 ("impaired") + wave2 ("retirement scheduled")
//   - Findings[1].Phrase == "retirement scheduled" (second wave2 only, not both)
//   - Findings[0].Source == "wave1"
//   - Findings[1].Source == "wave2:ec2"
//
// Red-light today: the shim's DeriveFindings is deterministic; the second call
// re-derives from m.EnrichmentFindings which holds finding B. This test should
// actually pass with the shim. However after fold, the test validates that
// applyEnrichment replaces (not appends) the wave2 slot. It is listed as
// red-light because if fold incorrectly appends, len would be 3 (wave1 + A + B).
func TestFold_RepeatedEnrichmentReplacesWave2(t *testing.T) {
	const rid = "i-2"

	efFirst := resource.EnrichmentFinding{
		Severity: "~",
		Summary:  "pending maintenance",
	}
	efSecond := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "retirement scheduled",
	}

	m := newShimModel()
	m.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{
			{ID: rid, Name: "test-ec2", Status: "impaired"},
		},
	}

	// First enrichment: wave2 = "pending maintenance"
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Findings:     map[string]resource.EnrichmentFinding{rid: efFirst},
		Gen:          0,
		TypeGen:      0,
	})

	// Second enrichment: wave2 = "retirement scheduled"
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Findings:     map[string]resource.EnrichmentFinding{rid: efSecond},
		Gen:          0,
		TypeGen:      0,
	})

	entry, ok := m.ResourceCache["ec2"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[ec2] is empty")
	}
	r := entry.Resources[0]

	// Expect exactly 2: wave1 (impaired) + wave2 (retirement scheduled).
	// If fold appends instead of replacing, we'd get 3 (wave1 + both wave2s).
	if len(r.Findings) != 2 {
		t.Errorf("Findings len: got %d, want 2 (wave1+wave2); findings: %v", len(r.Findings), r.Findings)
		return
	}

	wave1 := r.Findings[0]
	if wave1.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want \"wave1\"", wave1.Source)
	}
	if wave1.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want \"impaired\"", wave1.Phrase)
	}

	wave2 := r.Findings[1]
	if wave2.Source != "wave2:ec2" {
		t.Errorf("Findings[1].Source: got %q, want \"wave2:ec2\"", wave2.Source)
	}
	if wave2.Phrase != efSecond.Summary {
		t.Errorf("Findings[1].Phrase: got %q, want %q (second wave2 only — must NOT stack)", wave2.Phrase, efSecond.Summary)
	}
}

// ── Test 3: EnrichmentCheckedMsg with nil Findings clears wave2 ──────────────

// TestFold_EmptyEnrichmentClearsWave2 verifies that sending
// EnrichmentCheckedMsg with nil/empty Findings removes wave2 entries from
// cached rows while preserving wave1.
//
// Setup:
//   - Seed a cached row with Status "impaired" (→ wave1 finding).
//   - Send first EnrichmentCheckedMsg with a wave2 finding.
//   - Send second EnrichmentCheckedMsg with Findings = nil.
//
// After the second call:
//   - len(r.Findings) == 1 (wave1 only; wave2 cleared)
//   - r.Findings[0].Source == "wave1"
//   - r.AttentionDetails is nil or empty (wave2 detail removed)
//
// Red-light today: same reasoning as Test 2. After fold, applyEnrichment
// with empty perResource must strip wave2 entries. The shim path currently
// re-derives and correctly clears wave2 since DeriveFindings uses the updated
// (now empty) m.EnrichmentFindings[type]. This test guards against fold
// implementations that forget to clear the wave2 slot on empty input.
func TestFold_EmptyEnrichmentClearsWave2(t *testing.T) {
	const rid = "i-3"

	efInitial := resource.EnrichmentFinding{
		Severity: "~",
		Summary:  "pending maintenance",
		Rows:     []resource.FindingRow{{Label: "Action", Value: "retirement"}},
	}

	m := newShimModel()
	m.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{
			{ID: rid, Name: "test-ec2", Status: "impaired"},
		},
	}

	// Seed wave2 finding.
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Findings:     map[string]resource.EnrichmentFinding{rid: efInitial},
		Gen:          0,
		TypeGen:      0,
	})

	// Verify wave2 was set before clearing.
	{
		entry := m.ResourceCache["ec2"]
		if entry == nil || len(entry.Resources) == 0 {
			t.Fatal("ResourceCache[ec2] empty after initial enrichment")
		}
		hasWave2 := false
		for _, f := range entry.Resources[0].Findings {
			if f.Source == "wave2:ec2" {
				hasWave2 = true
				break
			}
		}
		if !hasWave2 {
			t.Log("pre-clear wave2 not present — shim not yet wired; continuing to assert clear behavior")
		}
	}

	// Send empty Findings — must clear wave2.
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Findings:     nil,
		Gen:          0,
		TypeGen:      0,
	})

	entry, ok := m.ResourceCache["ec2"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[ec2] is empty after empty EnrichmentCheckedMsg")
	}
	r := entry.Resources[0]

	// After clearing: only wave1 ("impaired") should remain.
	for _, f := range r.Findings {
		if f.Source == "wave2:ec2" {
			t.Errorf("Findings still contains wave2 entry after nil Findings: %+v", f)
		}
	}

	// Wave1 must be preserved.
	wave1Present := false
	for _, f := range r.Findings {
		if f.Source == "wave1" && f.Phrase == "impaired" {
			wave1Present = true
			break
		}
	}
	if !wave1Present {
		t.Errorf("wave1 finding (impaired) was lost after empty EnrichmentCheckedMsg; Findings: %v", r.Findings)
	}

	// AttentionDetails must not contain wave2 entries.
	if len(r.AttentionDetails) > 0 {
		for code := range r.AttentionDetails {
			t.Errorf("AttentionDetails still contains entry for code %q after wave2 clear", code)
		}
	}
}

// ── Test 4: Session.EnrichmentFindings field is deleted ────────────────────

// TestFold_EnrichmentFindingsFieldDeleted verifies at compile+runtime that
// session.Session does NOT have an EnrichmentFindings field.
//
// This test FAILS before PR-03a-fold (the field still exists in Session) and
// PASSES after (the field is deleted from the struct).
//
// The reflection approach is used per the spec: it avoids a compilation
// dependency on the deleted field while still catching regressions if the
// field is re-added.
func TestFold_EnrichmentFindingsFieldDeleted(t *testing.T) {
	s := session.New()
	v := reflect.ValueOf(s).Elem()
	if _, found := v.Type().FieldByName("EnrichmentFindings"); found {
		t.Error("Session.EnrichmentFindings field still exists; PR-03a-fold deletes it")
	}
}

// ── Test 5: Wave1 + wave2 coexist when row enters via Site 4 (CachedPages) ──

// TestFold_AttentionDetailsCarryAcrossEntryPoints verifies that a resource
// entering via Site 4 (RelatedCheckResultMsg.CachedPages) receives wave1
// findings at entry, then wave2 findings at enrichment, and both coexist in
// r.Findings with wave1 first, wave2 second.
//
// This tests the invariant that the fold path does not wipe wave1 when
// writing wave2.
//
// Entry sequence:
//  1. RelatedCheckResultMsg with CachedPages — row enters ResourceCache with
//     Status "impaired" → wave1 finding derived at entry (shim site #4).
//  2. EnrichmentCheckedMsg with wave2 finding — applyEnrichment must preserve
//     wave1 and append/replace wave2 slot.
//
// Assertions:
//   - len(r.Findings) == 2
//   - Findings[0].Source == "wave1", Findings[0].Phrase == "impaired"
//   - Findings[1].Source == "wave2:ec2", Findings[1].Phrase == "pending maintenance"
//   - Findings[1].Code == "ec2.pending.maintenance"
//   - r.AttentionDetails["ec2.pending.maintenance"].Rows is non-empty
//
// Red-light today: site #4 shim may or may not be wired, and fold is not yet
// implemented; the test asserts the post-fold steady-state.
func TestFold_AttentionDetailsCarryAcrossEntryPoints(t *testing.T) {
	const (
		rid        = "i-site4"
		wantPhrase = "pending maintenance"
	)

	wantCode := domain.FindingCode("ec2.pending.maintenance")

	ef := resource.EnrichmentFinding{
		Severity: "~",
		Summary:  wantPhrase,
		Rows:     []resource.FindingRow{{Label: "Action", Value: "instance-retirement"}},
	}

	m := newShimModel()

	// Step 1: resource enters via Site 4 (CachedPages).
	m = shimApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: "src-1",
		DefDisplayName:   "EC2 Instances",
		Result:           resource.RelatedCheckResult{TargetType: "ec2", Count: 1},
		Generation:       0,
		CachedPages: map[string]resource.ResourceCacheEntry{
			"ec2": {
				Resources: []resource.Resource{
					{ID: rid, Name: "site4-ec2", Status: "impaired"},
				},
			},
		},
	})

	// Verify entry-point wave1 was set (shim site #4 must be wired for this to hold).
	{
		entry := m.ResourceCache["ec2"]
		if entry == nil || len(entry.Resources) == 0 {
			t.Fatal("ResourceCache[ec2] empty after RelatedCheckResultMsg — site 4 shim not wired")
		}
		if len(entry.Resources[0].Findings) == 0 {
			t.Log("wave1 Finding not yet set at entry point — site 4 shim not wired; will still assert post-enrichment state")
		}
	}

	// Step 2: Wave 2 enrichment arrives.
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Findings:     map[string]resource.EnrichmentFinding{rid: ef},
		Gen:          0,
		TypeGen:      0,
	})

	entry, ok := m.ResourceCache["ec2"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[ec2] is empty after EnrichmentCheckedMsg")
	}
	r := entry.Resources[0]

	// Both wave1 and wave2 must coexist.
	if len(r.Findings) != 2 {
		t.Errorf("Findings len: got %d, want 2 (wave1+wave2); findings: %v", len(r.Findings), r.Findings)
		return
	}

	// Pin order: wave1 first, wave2 second.
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want \"wave1\"", r.Findings[0].Source)
	}
	if r.Findings[0].Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want \"impaired\"", r.Findings[0].Phrase)
	}
	if r.Findings[1].Source != "wave2:ec2" {
		t.Errorf("Findings[1].Source: got %q, want \"wave2:ec2\"", r.Findings[1].Source)
	}
	if r.Findings[1].Phrase != wantPhrase {
		t.Errorf("Findings[1].Phrase: got %q, want %q", r.Findings[1].Phrase, wantPhrase)
	}
	if r.Findings[1].Code != wantCode {
		t.Errorf("Findings[1].Code: got %q, want %q", r.Findings[1].Code, wantCode)
	}

	// AttentionDetails must carry the wave2 rows.
	if detail, ok := r.AttentionDetails[wantCode]; !ok {
		t.Errorf("AttentionDetails[%q]: not found", wantCode)
	} else if len(detail.Rows) == 0 {
		t.Errorf("AttentionDetails[%q].Rows: empty, want non-empty", wantCode)
	} else if detail.Rows[0].Label != ef.Rows[0].Label {
		t.Errorf("AttentionDetails[%q].Rows[0].Label: got %q, want %q", wantCode, detail.Rows[0].Label, ef.Rows[0].Label)
	}
}
