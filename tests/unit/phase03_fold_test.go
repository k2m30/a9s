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
//	Test 1/RowStore: pins that applyEnrichment folds RowStore's retained rows
//	    via AmendRows (task #17 wave 1 stage 2 removed the old
//	    session.ProbeResources/ProbeTruncated "all-enrichment-done" free, so
//	    RowStore's rows for this type survive past EnrichChecked >= EnrichTotal
//	    with no special seeding needed to keep them inspectable).
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
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// slugForTest mirrors the unexported slug() function in core/semantics/attention/derive.go.
// It normalizes a phrase to a stable code suffix for use in test wantCode assertions.
// Lowercase; runs of non-alphanumerics collapse to a single dot; leading/trailing dots trimmed.
//
//	"pending maintenance" → "pending.maintenance"
//	"bucket public"       → "bucket.public"
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugForTest(phrase string) string {
	s := strings.ToLower(strings.TrimSpace(phrase))
	s = slugRe.ReplaceAllString(s, ".")
	s = strings.Trim(s, ".")
	return s
}

// applyMsg applies a message to the TUI model and returns the updated model.
// Relocated from the deleted phase03_shim_wireups_test.go (renamed from applyMsg).
func applyMsg(m tui.Model, msg tea.Msg) tui.Model {
	return tuitest.StepModel(m, msg)
}

// newRootModel builds a minimal root model suitable for the fold tests.
// It applies a WindowSizeMsg and a ClientsReadyMsg with nil clients, which is
// enough to advance the model past the initial state without triggering live AWS calls.
// Relocated from the deleted phase03_shim_wireups_test.go (renamed from newRootModel).
func newRootModel() tui.Model {
	m := tui.New("test-profile", "us-east-1",
		tui.WithNoCache(true),
	)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// ClientsReadyMsg with nil clients advances m.clients; no AWS calls are made
	// because WithNoCache(true) suppresses background availability probes.
	m = applyMsg(m, messages.ClientsReady{Clients: nil})
	return m
}

// ── Test 1: EnrichmentCheckedMsg mutates rows directly in all three caches ──

// TestFold_EnrichmentCheckedMutatesRowsDirectly verifies that after the fold
// implementation handles EnrichmentCheckedMsg, the wave 2 finding is written
// DIRECTLY into the cached row's r.Findings slice — not via the parallel
// EnrichmentFindings map.
//
// Table-driven: exercises canonical-only types (ec2, s3, sg, role, ng, kms)
// and aliased types (dbi/rds, redis/elasticache) to ensure ShortName and
// cache key derivation is correct across all resource categories.
//
// Specifically for each type:
//   - Resource with Status "running" (a lifecycle phrase, filtered by wave1)
//     should yield exactly one finding after enrichment — the wave2 entry.
//   - That wave2 finding must have Phrase == ef.Phrase and Source == "wave2:<canonShort>".
//   - r.AttentionDetails[code].Rows must contain the EnrichmentFinding's Rows.
//   - The same row mutation must occur for LazyResourceCache and RowStore.
//
// Red-light today:
//   - ResourceCache and LazyResourceCache subtests PASS with the current shim
//     (DeriveFindings correctly filters "running" and emits only wave2).
//   - RowStore subtest pins that applyEnrichment's AmendRows fold reaches
//     RowStore's retained rows the same way it reaches the other two caches
//     (task #17 wave 1 stage 2 replaced the removed session.ProbeResources/
//     ProbeTruncated all-enrichment-done free with a no-op — RowStore rows
//     are never cleared on completion, so no special seeding is needed to
//     keep them inspectable).
func TestFold_EnrichmentCheckedMutatesRowsDirectly(t *testing.T) {
	// alias == "" means use canonShort as the message ResourceType (canonical-only).
	// alias != "" means use alias as the message ResourceType, assert cache under canonShort.
	cases := []struct {
		name, canonShort, alias string
		summary                 string
	}{
		{"ec2-canonical", "ec2", "", "pending maintenance"},
		{"s3-canonical", "s3", "", "bucket public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending"},
		{"sg-canonical", "sg", "", "overly permissive"},
		{"iam-role-canonical", "role", "", "unused"},
		{"ng-canonical", "ng", "", "scale failure"},
		{"kms-canonical", "kms", "", "pending deletion"},
	}

	for _, tc := range cases {
		tc := tc
		msgType := tc.canonShort
		if tc.alias != "" {
			msgType = tc.alias
		}
		wantSource := "wave2:" + tc.canonShort
		// slug: lowercase, runs of non-alphanumerics → ".", trim leading/trailing dots.
		wantCode := domain.FindingCode(tc.canonShort + "." + slugForTest(tc.summary))
		rid := "r-" + tc.canonShort

		efFinding := domain.Finding{
			Code:     wantCode,
			Phrase:   tc.summary,
			Severity: domain.SevWarn,
			Source:   wantSource,
		}
		efAttention := domain.AttentionDetail{
			Rows: []domain.DetailRow{
				{Label: "Action", Value: "test-action"},
				{Label: "Detail", Value: "test-detail"},
			},
		}

		t.Run(tc.name+"/ResourceCache", func(t *testing.T) {
			m := newRootModel()

			m.Core().Session().RowStore.Observe(tc.canonShort, []resource.Resource{
				{ID: rid, Name: "test-" + tc.canonShort, Fields: map[string]string{"status": "running"}},
			}, nil, session.OriginFetch, false)

			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType:     msgType,
				Findings:         map[string][]domain.Finding{rid: {efFinding}},
				AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {wantCode: efAttention}},
				Gen:              0,
				TypeGen:          0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if tr.Gen == 0 || len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q) is empty after EnrichmentCheckedMsg", tc.canonShort)
			}
			r := tr.Rows[0]

			// After fold: "running" is a lifecycle phrase → filtered by wave1.
			// The single finding must be the wave2 entry from the EnrichmentFinding.
			if len(r.Findings) != 1 {
				t.Errorf("ResourceCache[%q].Resources[0].Findings: got len=%d, want 1 (only wave2; wave1 'running' is lifecycle-filtered)", tc.canonShort, len(r.Findings))
			} else {
				f := r.Findings[0]
				if f.Phrase != tc.summary {
					t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, tc.summary)
				}
				if f.Source != wantSource {
					t.Errorf("Findings[0].Source: got %q, want %q", f.Source, wantSource)
				}
			}

			// AttentionDetails must carry the rows from the enrichment input.
			if len(r.AttentionDetails) == 0 {
				t.Errorf("AttentionDetails: got empty, want entry for code %q", wantCode)
			} else if detail, ok := r.AttentionDetails[wantCode]; !ok {
				t.Errorf("AttentionDetails[%q]: not found; keys: %v", wantCode, r.AttentionDetails)
			} else if len(detail.Rows) != len(efAttention.Rows) {
				t.Errorf("AttentionDetails[%q].Rows: got len=%d, want %d", wantCode, len(detail.Rows), len(efAttention.Rows))
			} else {
				for i, row := range detail.Rows {
					if row.Label != efAttention.Rows[i].Label || row.Value != efAttention.Rows[i].Value {
						t.Errorf("AttentionDetails[%q].Rows[%d]: got {%q,%q}, want {%q,%q}",
							wantCode, i, row.Label, row.Value, efAttention.Rows[i].Label, efAttention.Rows[i].Value)
					}
				}
			}
		})

		t.Run(tc.name+"/LazyResourceCache", func(t *testing.T) {
			m := newRootModel()

			m.Core().Session().RowStore.ObservePartial(tc.canonShort, []resource.Resource{
				{ID: rid, Name: "lazy-" + tc.canonShort, Fields: map[string]string{"status": "running"}},
			})

			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType:     msgType,
				Findings:         map[string][]domain.Finding{rid: {efFinding}},
				AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {wantCode: efAttention}},
				Gen:              0,
				TypeGen:          0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if tr.Gen == 0 || len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q) is empty after EnrichmentCheckedMsg", tc.canonShort)
			}
			r := tr.Rows[0]

			if len(r.Findings) != 1 {
				t.Errorf("LazyResourceCache[%q][0].Findings: got len=%d, want 1 (wave2 only; wave1 'running' is lifecycle-filtered)", tc.canonShort, len(r.Findings))
			} else if r.Findings[0].Phrase != tc.summary {
				t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, tc.summary)
			} else if r.Findings[0].Source != wantSource {
				t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, wantSource)
			}
		})

		t.Run(tc.name+"/RowStore", func(t *testing.T) {
			m := newRootModel()

			// task #17 wave 1 stage 2: the "all enrichment done" cleanup that
			// used to nil out session.ProbeResources is gone — RowStore
			// retains its rows for the session even after
			// EnrichChecked >= EnrichTotal (see handlers_availability.go's
			// doc comment on that branch). EnrichTotal is left at its
			// session.New() default; there is no cleanup left to avoid.

			// RowStore is populated via AvailabilityCheckedMsg in real usage,
			// but for the fold test we seed it directly (OriginProbe) — the
			// fold must walk RowStore's retained rows just as it walks the
			// other two caches.
			m.Core().Session().RowStore.Observe(tc.canonShort, []resource.Resource{
				{ID: rid, Name: "probe-" + tc.canonShort, Fields: map[string]string{"status": "running"}},
			}, nil, session.OriginProbe, false)

			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType:     msgType,
				Findings:         map[string][]domain.Finding{rid: {efFinding}},
				AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {wantCode: efAttention}},
				Gen:              0,
				TypeGen:          0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q).Rows is empty after EnrichmentCheckedMsg (fold path must update RowStore via AmendRows)", tc.canonShort)
			}
			r := tr.Rows[0]

			if len(r.Findings) != 1 {
				t.Errorf("RowStore.Snapshot(%q).Rows[0].Findings: got len=%d, want 1 (wave2 only; wave1 'running' is lifecycle-filtered)", tc.canonShort, len(r.Findings))
			} else if r.Findings[0].Phrase != tc.summary {
				t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, tc.summary)
			} else if r.Findings[0].Source != wantSource {
				t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, wantSource)
			}
		})
	}
}

// ── Test 2: Repeated EnrichmentCheckedMsg replaces wave2, preserves wave1 ───

// TestFold_RepeatedEnrichmentReplacesWave2 verifies that a second
// EnrichmentCheckedMsg replaces the wave2 finding, not stacks on top.
//
// Table-driven: exercises all 8 resource type categories (canonical-only and
// aliased) to ensure the replace-not-append invariant holds regardless of type.
//
// Setup per type:
//   - Seed a cached row with Status "impaired" (→ wave1 finding after derive).
//   - Send EnrichmentCheckedMsg with wave2 finding A.
//   - Send a second EnrichmentCheckedMsg with wave2 finding B.
//
// After the second call:
//   - len(r.Findings) == 2: wave1 ("impaired") + wave2 (finding B only)
//   - Findings[1].Phrase == second summary (second wave2 only, not both)
//   - Findings[0].Source == "wave1"
//   - Findings[1].Source == "wave2:<canonShort>"
//
// Red-light today: the shim's DeriveFindings is deterministic; the second call
// re-derives from m.EnrichmentFindings which holds finding B. This test should
// actually pass with the shim. However after fold, the test validates that
// applyEnrichment replaces (not appends) the wave2 slot. It is listed as
// red-light because if fold incorrectly appends, len would be 3 (wave1 + A + B).
func TestFold_RepeatedEnrichmentReplacesWave2(t *testing.T) {
	cases := []struct {
		name, canonShort, alias string
		summaryA, summaryB      string
	}{
		{"ec2-canonical", "ec2", "", "pending maintenance", "retirement scheduled"},
		{"s3-canonical", "s3", "", "bucket public", "acl updated"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "failover triggered"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "node replaced"},
		{"sg-canonical", "sg", "", "overly permissive", "rule added"},
		{"iam-role-canonical", "role", "", "unused", "policy attached"},
		{"ng-canonical", "ng", "", "scale failure", "node drain"},
		{"kms-canonical", "kms", "", "pending deletion", "key disabled"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			msgType := tc.canonShort
			if tc.alias != "" {
				msgType = tc.alias
			}
			wantSource := "wave2:" + tc.canonShort
			rid := "r-repeat-" + tc.canonShort

			efFirst := domain.Finding{
				Code:     domain.FindingCode(tc.canonShort + "." + slugForTest(tc.summaryA)),
				Phrase:   tc.summaryA,
				Severity: domain.SevWarn,
				Source:   wantSource,
			}
			efSecond := domain.Finding{
				Code:     domain.FindingCode(tc.canonShort + "." + slugForTest(tc.summaryB)),
				Phrase:   tc.summaryB,
				Severity: domain.SevBroken,
				Source:   wantSource,
			}

			m := newRootModel()
			m.Core().Session().RowStore.Observe(tc.canonShort, []resource.Resource{
				{
					ID:   rid,
					Name: "test-" + tc.canonShort,
					// W1.4a: fetchers populate Findings directly; mirror what the
					// removed derive shim produced from Status: "impaired".
					Findings: []domain.Finding{{
						Code:     domain.FindingCode(tc.canonShort + ".impaired"),
						Phrase:   "impaired",
						Severity: domain.SevBroken,
						Source:   "wave1",
					}},
				},
			}, nil, session.OriginFetch, false)

			// First enrichment: wave2 = summaryA
			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType: msgType,
				Findings:     map[string][]domain.Finding{rid: {efFirst}},
				Gen:          0,
				TypeGen:      0,
			})

			// Second enrichment: wave2 = summaryB
			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType: msgType,
				Findings:     map[string][]domain.Finding{rid: {efSecond}},
				Gen:          0,
				TypeGen:      0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if tr.Gen == 0 || len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q) is empty", tc.canonShort)
			}
			r := tr.Rows[0]

			// Expect exactly 2: wave1 (impaired) + wave2 (summaryB only).
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
			if wave2.Source != wantSource {
				t.Errorf("Findings[1].Source: got %q, want %q", wave2.Source, wantSource)
			}
			if wave2.Phrase != efSecond.Phrase {
				t.Errorf("Findings[1].Phrase: got %q, want %q (second wave2 only — must NOT stack)", wave2.Phrase, efSecond.Phrase)
			}
		})
	}
}

// ── Test 3: EnrichmentCheckedMsg with nil Findings clears wave2 ──────────────

// TestFold_EmptyEnrichmentClearsWave2 verifies that sending
// EnrichmentCheckedMsg with nil/empty Findings removes wave2 entries from
// cached rows while preserving wave1.
//
// Table-driven: exercises all 8 resource type categories (canonical-only and
// aliased) to ensure the clear-wave2 invariant holds regardless of type.
//
// Setup per type:
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
	cases := []struct {
		name, canonShort, alias string
		summary                 string
	}{
		{"ec2-canonical", "ec2", "", "pending maintenance"},
		{"s3-canonical", "s3", "", "bucket public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending"},
		{"sg-canonical", "sg", "", "overly permissive"},
		{"iam-role-canonical", "role", "", "unused"},
		{"ng-canonical", "ng", "", "scale failure"},
		{"kms-canonical", "kms", "", "pending deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			msgType := tc.canonShort
			if tc.alias != "" {
				msgType = tc.alias
			}
			wantWave2Source := "wave2:" + tc.canonShort
			rid := "r-empty-" + tc.canonShort

			efInitialCode := domain.FindingCode(tc.canonShort + "." + slugForTest(tc.summary))
			efInitialFinding := domain.Finding{
				Code:     efInitialCode,
				Phrase:   tc.summary,
				Severity: domain.SevWarn,
				Source:   wantWave2Source,
			}
			efInitialAttention := domain.AttentionDetail{
				Rows: []domain.DetailRow{{Label: "Action", Value: "test-retirement"}},
			}

			m := newRootModel()
			m.Core().Session().RowStore.Observe(tc.canonShort, []resource.Resource{
				{
					ID:   rid,
					Name: "test-" + tc.canonShort,
					// W1.4a: fetchers populate Findings directly; mirror what the
					// removed derive shim produced from Status: "impaired".
					Findings: []domain.Finding{{
						Code:     domain.FindingCode(tc.canonShort + ".impaired"),
						Phrase:   "impaired",
						Severity: domain.SevBroken,
						Source:   "wave1",
					}},
				},
			}, nil, session.OriginFetch, false)

			// Seed wave2 finding.
			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType:     msgType,
				Findings:         map[string][]domain.Finding{rid: {efInitialFinding}},
				AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {efInitialCode: efInitialAttention}},
				Gen:              0,
				TypeGen:          0,
			})

			// Verify wave2 was set before clearing.
			{
				tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
				if tr.Gen == 0 || len(tr.Rows) == 0 {
					t.Fatalf("RowStore.Snapshot(%q) empty after initial enrichment", tc.canonShort)
				}
				hasWave2 := false
				for _, f := range tr.Rows[0].Findings {
					if f.Source == wantWave2Source {
						hasWave2 = true
						break
					}
				}
				if !hasWave2 {
					t.Logf("pre-clear wave2 not present in %q — shim not yet wired; continuing to assert clear behavior", tc.canonShort)
				}
			}

			// Send empty Findings — must clear wave2.
			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType: msgType,
				Findings:     nil,
				Gen:          0,
				TypeGen:      0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if tr.Gen == 0 || len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q) is empty after empty EnrichmentCheckedMsg", tc.canonShort)
			}
			r := tr.Rows[0]

			// After clearing: only wave1 ("impaired") should remain.
			for _, f := range r.Findings {
				if f.Source == wantWave2Source {
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
		})
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

// ── CodeRabbit PR #310 finding A: Ctrl+R on resource list leaves stale wave2 ──

// TestFold_CtrlROnList_ClearsActiveRowFindings verified that pressing Ctrl+R
// while viewing a resource list cleared stale wave2 findings from the rows
// held by the active ResourceListModel IMMEDIATELY at keypress time. That
// contract is superseded: eagerly blanking findings on Ctrl+R produced a
// real, user-visible flicker for the full AWS round-trip between the
// keypress and the rerun's EnrichmentChecked arrival. Findings are now
// stale-until-replaced, not blank-until-replaced — this test asserts both
// halves: the wave2 finding must still be present immediately after Ctrl+R
// (no blank window), and it must be genuinely removed once a fresh
// EnrichmentChecked result lands that no longer contains it (real
// replacement still works, it just doesn't fire eagerly at keypress time).
//
// See qa_glyph_continuity_test.go's TestRerunStart_KeepsVisibleFindingsUntilReplaced
// for the from-scratch pin of the same corrected invariant.
func TestFold_CtrlROnList_ClearsActiveRowFindings(t *testing.T) {
	const (
		rid        = "i-ctrl-r"
		wantSource = "wave2:ec2"
	)

	efFinding := domain.Finding{
		Code:     "ec2.pending.maintenance",
		Phrase:   "pending maintenance",
		Severity: domain.SevBroken,
		Source:   wantSource,
	}
	efAttention := domain.AttentionDetail{
		Rows: []domain.DetailRow{
			{Label: "Action", Value: "instance-retirement"},
		},
	}

	m := newRootModel()

	// Step 1: seed RowStore so NavigateMsg gets a cache hit and creates
	// a ResourceListModel holding this slice.
	m.Core().Session().RowStore.Observe("ec2", []resource.Resource{
		{ID: rid, Name: "test-ec2", Fields: map[string]string{"status": "running"}},
	}, nil, session.OriginFetch, false)

	// Step 2: stamp wave2 findings into the cached row via EnrichmentCheckedMsg.
	m = applyMsg(m, messages.EnrichmentChecked{
		ResourceType:     "ec2",
		Findings:         map[string][]domain.Finding{rid: {efFinding}},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {efFinding.Code: efAttention}},
		Gen:              0,
		TypeGen:          0,
	})

	// Step 3: navigate to the ec2 list view — cache hit path creates a
	// ResourceListModel from entry.Resources (the slice with wave2 findings).
	m = applyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Step 4: confirm the row has wave2 findings before Ctrl+R.
	preResources := m.ActiveListResources()
	if len(preResources) == 0 {
		t.Fatal("ActiveListResources: empty before Ctrl+R — navigate did not push ResourceListModel")
	}
	hasWave2Pre := false
	for _, f := range preResources[0].Findings {
		if f.Source == wantSource {
			hasWave2Pre = true
			break
		}
	}
	if !hasWave2Pre {
		t.Fatal("pre-condition failed: expected wave2 finding present on the active row before Ctrl+R")
	}

	// Step 5: send Ctrl+R — this triggers handleRefresh on the resource list path.
	m = applyMsg(m, tea.KeyPressMsg{Code: -1, Text: "\x12"})

	// Assertion (corrected), half 1: immediately after Ctrl+R, BEFORE any
	// fresh enrichment result has landed, the wave2 finding must still be
	// present — no blank window (stale-until-replaced).
	postKeypress := m.ActiveListResources()
	if len(postKeypress) == 0 {
		t.Fatal("ActiveListResources: empty after Ctrl+R — ResourceListModel unexpectedly absent")
	}
	hasWave2PostKeypress := false
	for _, r := range postKeypress {
		for _, f := range r.Findings {
			if f.Source == wantSource {
				hasWave2PostKeypress = true
			}
		}
	}
	if !hasWave2PostKeypress {
		t.Error("wave2 finding vanished immediately after Ctrl+R, before any fresh result landed — findings must be stale-until-replaced, not blanked eagerly at keypress time")
	}

	// Step 6: deliver a fresh EnrichmentChecked result that genuinely omits
	// rid (resource recovered) — this MUST remove the finding.
	m = applyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{},
		Gen:          0,
		TypeGen:      0,
	})

	// Assertion, half 2: after the fresh result lands and genuinely omits
	// rid, no wave2 entry should remain on the rows visible in the active
	// ResourceListModel.
	postFreshResult := m.ActiveListResources()
	if len(postFreshResult) == 0 {
		t.Fatal("ActiveListResources: empty after the fresh EnrichmentChecked result — ResourceListModel unexpectedly absent")
	}
	for _, r := range postFreshResult {
		for _, f := range r.Findings {
			if f.Source == wantSource {
				t.Errorf(
					"resource %q still has stale wave2 finding after a fresh EnrichmentChecked result that omitted it: Source=%q Phrase=%q",
					r.ID, f.Source, f.Phrase,
				)
			}
		}
	}
}

// ── CodeRabbit PR #310 finding B: main-menu Ctrl+R leaves stale wave2 in cache ──

// TestFold_MainMenuCtrlR_ClearsAllCachedWave2 verifies that pressing Ctrl+R
// while on the main menu clears wave2 findings from ALL cached resource rows
// across all types.
//
// The pre-fix bug (PR #310 CodeRabbit finding B):
//
//	handleRefresh on the main-menu path resets side maps (ProbeResources,
//	EnrichmentRan, etc.) but never touches m.Core().Session().ResourceCache. Rows in ResourceCache
//	retain stale r.Findings from the previous enrichment wave. When the user
//	navigates back to a list, they see stale wave2 markers until a fresh
//	EnrichmentCheckedMsg arrives and overwrites them.
//
// Note: this model is built WITHOUT WithNoCache so handleRefresh does not
// return early on the main-menu path (noCache=true short-circuits at line 349).
func TestFold_MainMenuCtrlR_ClearsAllCachedWave2(t *testing.T) {
	const (
		ec2ID    = "i-menu-ctrlr"
		s3ID     = "bucket-menu-ctrlr"
		ec2Short = "ec2"
		s3Short  = "s3"
	)

	efEC2Finding := domain.Finding{
		Code:     "ec2.ec2.retirement",
		Phrase:   "ec2 retirement",
		Severity: domain.SevBroken,
		Source:   "wave2:" + ec2Short,
	}
	efEC2Attention := domain.AttentionDetail{
		Rows: []domain.DetailRow{{Label: "Action", Value: "retirement"}},
	}
	efS3Finding := domain.Finding{
		Code:     "s3.s3.public.access",
		Phrase:   "s3 public access",
		Severity: domain.SevWarn,
		Source:   "wave2:" + s3Short,
	}
	efS3Attention := domain.AttentionDetail{
		Rows: []domain.DetailRow{{Label: "Policy", Value: "public-read"}},
	}

	// Build a model WITHOUT WithNoCache so main-menu Ctrl+R is not a no-op.
	// ClientsReadyMsg with nil clients is enough to advance past init state.
	m := tui.New("test-profile", "us-east-1")
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = applyMsg(m, messages.ClientsReady{Clients: nil})

	// Step 1: seed RowStore for ec2 and s3 with running resources.
	m.Core().Session().RowStore.Observe(ec2Short, []resource.Resource{
		{ID: ec2ID, Name: "test-ec2", Fields: map[string]string{"status": "running"}},
	}, nil, session.OriginFetch, false)
	m.Core().Session().RowStore.Observe(s3Short, []resource.Resource{
		{ID: s3ID, Name: "test-bucket", Fields: map[string]string{"status": "running"}},
	}, nil, session.OriginFetch, false)

	// Step 2: stamp wave2 findings via EnrichmentCheckedMsg for both types.
	m = applyMsg(m, messages.EnrichmentChecked{
		ResourceType:     ec2Short,
		Findings:         map[string][]domain.Finding{ec2ID: {efEC2Finding}},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{ec2ID: {efEC2Finding.Code: efEC2Attention}},
		Gen:              0,
		TypeGen:          0,
	})
	m = applyMsg(m, messages.EnrichmentChecked{
		ResourceType:     s3Short,
		Findings:         map[string][]domain.Finding{s3ID: {efS3Finding}},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{s3ID: {efS3Finding.Code: efS3Attention}},
		Gen:              0,
		TypeGen:          0,
	})

	// Step 3: confirm wave2 entries are present before Ctrl+R.
	for _, tc := range []struct {
		short  string
		rid    string
		source string
	}{
		{ec2Short, ec2ID, "wave2:" + ec2Short},
		{s3Short, s3ID, "wave2:" + s3Short},
	} {
		tr := m.Core().Session().RowStore.Snapshot(tc.short)
		if tr.Gen == 0 || len(tr.Rows) == 0 {
			t.Fatalf("pre-Ctrl+R: RowStore.Snapshot(%q) empty — enrichment not wired", tc.short)
		}
		hasWave2 := false
		for _, f := range tr.Rows[0].Findings {
			if f.Source == tc.source {
				hasWave2 = true
				break
			}
		}
		if !hasWave2 {
			t.Logf("pre-Ctrl+R: wave2 not present in %q — applyEnrichment not yet wired; continuing", tc.short)
		}
	}

	// Step 4: send Ctrl+R while on the main menu.
	m = applyMsg(m, tea.KeyPressMsg{Code: -1, Text: "\x12"})

	// Assertion: RowStore entries for both types must have NO wave2 findings.
	// Pre-fix: RowStore is untouched by main-menu Ctrl+R, so wave2 persists.
	// Post-fix: handleRefresh on main-menu path must also clear wave2 from all
	// cached rows (iterate over RowStore and call applyEnrichment per type).
	for _, tc := range []struct {
		short  string
		source string
	}{
		{ec2Short, "wave2:" + ec2Short},
		{s3Short, "wave2:" + s3Short},
	} {
		tr := m.Core().Session().RowStore.Snapshot(tc.short)
		if tr.Gen == 0 {
			// Cache entry deleted on Ctrl+R is also acceptable — no stale wave2.
			continue
		}
		for _, r := range tr.Rows {
			for _, f := range r.Findings {
				if f.Source == tc.source {
					t.Errorf(
						"RowStore.Snapshot(%q)[%q] still has stale wave2 finding after main-menu Ctrl+R: "+
							"Source=%q Phrase=%q; fix: main-menu Ctrl+R must clear wave2 from all cached rows",
						tc.short, r.ID, f.Source, f.Phrase,
					)
				}
			}
		}
	}
}

// ── Test 5: Wave1 + wave2 coexist when row enters via Site 4 (CachedPages) ──

// TestFold_AttentionDetailsCarryAcrossEntryPoints verifies that a resource
// entering via Site 4 (RelatedCheckResultMsg.CachedPages) receives wave1
// findings at entry, then wave2 findings at enrichment, and both coexist in
// r.Findings with wave1 first, wave2 second.
//
// Table-driven: exercises all 8 resource type categories (canonical-only and
// aliased) to ensure the fold path preserves wave1 regardless of type.
//
// Entry sequence:
//  1. RelatedCheckResultMsg with CachedPages — row enters ResourceCache with
//     Status "impaired" → wave1 finding derived at entry (shim site #4).
//  2. EnrichmentCheckedMsg with wave2 finding — applyEnrichment must preserve
//     wave1 and append/replace wave2 slot.
//
// Assertions per type:
//   - len(r.Findings) == 2
//   - Findings[0].Source == "wave1", Findings[0].Phrase == "impaired"
//   - Findings[1].Source == "wave2:<canonShort>", Findings[1].Phrase == summary
//   - Findings[1].Code == "<canonShort>.<slug(summary)>"
//   - r.AttentionDetails[<code>].Rows is non-empty
//
// Red-light today: site #4 shim may or may not be wired, and fold is not yet
// implemented; the test asserts the post-fold steady-state.
func TestFold_AttentionDetailsCarryAcrossEntryPoints(t *testing.T) {
	cases := []struct {
		name, canonShort, alias string
		summary                 string
	}{
		{"ec2-canonical", "ec2", "", "pending maintenance"},
		{"s3-canonical", "s3", "", "bucket public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending"},
		{"sg-canonical", "sg", "", "overly permissive"},
		{"iam-role-canonical", "role", "", "unused"},
		{"ng-canonical", "ng", "", "scale failure"},
		{"kms-canonical", "kms", "", "pending deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			msgType := tc.canonShort
			if tc.alias != "" {
				msgType = tc.alias
			}
			wantSource := "wave2:" + tc.canonShort
			wantCode := domain.FindingCode(tc.canonShort + "." + slugForTest(tc.summary))
			rid := "r-site4-" + tc.canonShort

			efFinding := domain.Finding{
				Code:     wantCode,
				Phrase:   tc.summary,
				Severity: domain.SevWarn,
				Source:   wantSource,
			}
			efAttention := domain.AttentionDetail{
				Rows: []domain.DetailRow{{Label: "Action", Value: "test-retirement"}},
			}

			m := newRootModel()

			// Step 1: resource enters via Site 4 (CachedPages).
			m = applyMsg(m, messages.RelatedCheckResult{
				ResourceType:     tc.canonShort,
				SourceResourceID: "src-1",
				DefDisplayName:   tc.canonShort + " Resources",
				Result:           resource.RelatedCheckResult{TargetType: tc.canonShort, Count: 1},
				Generation:       0,
				CachedPages: map[string]resource.ResourceCacheEntry{
					tc.canonShort: {
						Resources: []resource.Resource{
							{
								ID:   rid,
								Name: "site4-" + tc.canonShort,
								// W1.4a: fetchers populate Findings directly; mirror what the
								// removed derive shim produced from Status: "impaired".
								Findings: []domain.Finding{{
									Code:     domain.FindingCode(tc.canonShort + ".impaired"),
									Phrase:   "impaired",
									Severity: domain.SevBroken,
									Source:   "wave1",
								}},
							},
						},
					},
				},
			})

			// Verify entry-point wave1 was set (shim site #4 must be wired for this to hold).
			{
				tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
				if tr.Gen == 0 || len(tr.Rows) == 0 {
					t.Fatalf("RowStore.Snapshot(%q) empty after RelatedCheckResultMsg — site 4 shim not wired", tc.canonShort)
				}
				if len(tr.Rows[0].Findings) == 0 {
					t.Logf("wave1 Finding not yet set at entry point for %q — site 4 shim not wired; will still assert post-enrichment state", tc.canonShort)
				}
			}

			// Step 2: Wave 2 enrichment arrives.
			m = applyMsg(m, messages.EnrichmentChecked{
				ResourceType:     msgType,
				Findings:         map[string][]domain.Finding{rid: {efFinding}},
				AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{rid: {wantCode: efAttention}},
				Gen:              0,
				TypeGen:          0,
			})

			tr := m.Core().Session().RowStore.Snapshot(tc.canonShort)
			if tr.Gen == 0 || len(tr.Rows) == 0 {
				t.Fatalf("RowStore.Snapshot(%q) is empty after EnrichmentCheckedMsg", tc.canonShort)
			}
			r := tr.Rows[0]

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
			if r.Findings[1].Source != wantSource {
				t.Errorf("Findings[1].Source: got %q, want %q", r.Findings[1].Source, wantSource)
			}
			if r.Findings[1].Phrase != tc.summary {
				t.Errorf("Findings[1].Phrase: got %q, want %q", r.Findings[1].Phrase, tc.summary)
			}
			if r.Findings[1].Code != wantCode {
				t.Errorf("Findings[1].Code: got %q, want %q", r.Findings[1].Code, wantCode)
			}

			// AttentionDetails must carry the wave2 rows.
			if detail, ok := r.AttentionDetails[wantCode]; !ok {
				t.Errorf("AttentionDetails[%q]: not found", wantCode)
			} else if len(detail.Rows) == 0 {
				t.Errorf("AttentionDetails[%q].Rows: empty, want non-empty", wantCode)
			} else if detail.Rows[0].Label != efAttention.Rows[0].Label {
				t.Errorf("AttentionDetails[%q].Rows[0].Label: got %q, want %q", wantCode, detail.Rows[0].Label, efAttention.Rows[0].Label)
			}
		})
	}
}
