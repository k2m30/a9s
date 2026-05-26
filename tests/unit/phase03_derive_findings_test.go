// phase03_derive_findings_test.go — TDD tests for PR-03a-shim derive.
//
// Tests attention.DeriveFindings(r *domain.Resource, td resource.ResourceTypeDef)
// (2-arg form; wave2 enrichment is applied separately by applyEnrichment).
//
// Spec: docs/refactor/03-finding-model.md (PR-03a-shim)
package unit_test

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
)

// ec2TD is a reusable ResourceTypeDef for ec2 used across derive tests.
var ec2TD = resource.ResourceTypeDef{ShortName: "ec2"}

// TestDerive_NilResource_NoOp verifies that DeriveFindings does not panic when
// the resource pointer is nil — the contract is a safe no-op.
func TestDerive_NilResource_NoOp(t *testing.T) {
	// Must not panic; no assertions beyond "we got here".
	attention.DeriveFindings(nil, ec2TD)
}

// TestDerive_HealthyRow_EmptyOutputs verifies that a resource with no Status,
// no Issues, and no enrichment findings produces nil Findings and nil
// AttentionDetails — i.e. the zero-value healthy row.
func TestDerive_HealthyRow_EmptyOutputs(t *testing.T) {
	r := domain.Resource{ID: "i-healthy"}
	attention.DeriveFindings(&r, ec2TD)
	if r.Findings != nil {
		t.Errorf("Findings: got %v, want nil", r.Findings)
	}
	if r.AttentionDetails != nil {
		t.Errorf("AttentionDetails: got %v, want nil", r.AttentionDetails)
	}
}

// TestDerive_StatusOnly_SingleWave1Finding verifies that a resource with only a
// non-empty Status (and no Issues) produces exactly one wave1 Finding derived
// from the Status phrase.
func TestDerive_StatusOnly_SingleWave1Finding(t *testing.T) {
	r := domain.Resource{ID: "i-001", Status: "impaired"}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != "ec2.impaired" {
		t.Errorf("Code: got %q, want %q", f.Code, "ec2.impaired")
	}
	if f.Phrase != "impaired" {
		t.Errorf("Phrase: got %q, want %q", f.Phrase, "impaired")
	}
	if f.Source != "wave1" {
		t.Errorf("Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestDerive_StatusWithSuffix_StripsBumpFinding verifies that a Status carrying
// a "(+N)" suffix (produced by resource.BumpFindingSuffix) is stripped before
// becoming the Finding's Phrase and before the slug-based Code is formed.
// The function resource.StripFindingSuffix handles this stripping.
func TestDerive_StatusWithSuffix_StripsBumpFinding(t *testing.T) {
	r := domain.Resource{ID: "i-002", Status: "impaired (+2)"}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Phrase != "impaired" {
		t.Errorf("Phrase: got %q, want %q (suffix must be stripped)", f.Phrase, "impaired")
	}
	if f.Code != "ec2.impaired" {
		t.Errorf("Code: got %q, want %q (code derived from stripped phrase)", f.Code, "ec2.impaired")
	}
}

// TestDerive_IssuesList_OneFindingPerIssue verifies that each entry in
// r.Issues produces one wave1 Finding in the same order. Slug rule: spaces
// and non-[a-z0-9] runs collapse to a single dot, leading/trailing dots
// are trimmed.
func TestDerive_IssuesList_OneFindingPerIssue(t *testing.T) {
	r := domain.Resource{
		ID: "i-003",
		Issues: []string{
			"impaired",
			"system check failed",
			"instance check failed",
		},
	}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 3 {
		t.Fatalf("len(Findings): got %d, want 3", len(r.Findings))
	}

	cases := []struct {
		wantCode   domain.FindingCode
		wantPhrase string
		wantSource string
	}{
		{"ec2.impaired", "impaired", "wave1"},
		{"ec2.system.check.failed", "system check failed", "wave1"},
		{"ec2.instance.check.failed", "instance check failed", "wave1"},
	}
	for i, tc := range cases {
		f := r.Findings[i]
		if f.Code != tc.wantCode {
			t.Errorf("Findings[%d].Code: got %q, want %q", i, f.Code, tc.wantCode)
		}
		if f.Phrase != tc.wantPhrase {
			t.Errorf("Findings[%d].Phrase: got %q, want %q", i, f.Phrase, tc.wantPhrase)
		}
		if f.Source != tc.wantSource {
			t.Errorf("Findings[%d].Source: got %q, want %q", i, f.Source, tc.wantSource)
		}
	}
}

// TestDerive_IssuesEmpty_StatusFallback verifies that when r.Issues is nil but
// r.Status is a lifecycle steady-state phrase like "running", NO Finding is
// produced. Lifecycle steady-states are not issues — the shim must filter them.
// Findings must be nil and AttentionDetails must be nil.
//
// Pre-fix: shim emits one wave1 Finding with Phrase="running".
// Post-fix: "running" is recognized as a lifecycle phrase → Findings == nil.
func TestDerive_IssuesEmpty_StatusFallback(t *testing.T) {
	r := domain.Resource{ID: "i-004", Status: "running", Issues: nil}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 0 {
		t.Fatalf("len(Findings): got %d, want 0 (lifecycle steady-state must not produce a Finding)", len(r.Findings))
	}
	if r.AttentionDetails != nil {
		t.Errorf("AttentionDetails: got %v, want nil", r.AttentionDetails)
	}
}

// TestDerive_LifecyclePhrasesAreNotEmittedAsFindings verifies that all known
// lifecycle steady-state phrases — both healthy and terminal — produce zero
// Findings when passed as r.Status. These are lifecycle state labels, not
// issues, and must be filtered by the shim before creating wave1 Findings.
//
// Note: "inactive" is intentionally absent from this list. Several resource
// types (ECS services, ECS clusters) classify INACTIVE as broken, so it must
// not be universally filtered. See TestDerive_InactiveIsEmittedAsFinding.
//
// Pre-fix: every non-empty Status produces a Finding.
// Post-fix: lifecycle phrases are detected and skipped → Findings == nil.
func TestDerive_LifecyclePhrasesAreNotEmittedAsFindings(t *testing.T) {
	phrases := []string{
		"running", "available", "active", "in-service", "healthy",
		"terminated", "deleted", "shutting-down", "deregistered",
	}
	for _, phrase := range phrases {
		phrase := phrase
		t.Run(phrase, func(t *testing.T) {
			r := domain.Resource{ID: "i", Status: phrase}
			attention.DeriveFindings(&r, ec2TD)
			if len(r.Findings) != 0 {
				t.Errorf("Status=%q: got %d Findings, want 0 (lifecycle phrase must not emit a Finding)",
					phrase, len(r.Findings))
			}
		})
	}
}

// TestDerive_LifecyclePhrasesInIssuesAreAlsoSkipped verifies that lifecycle
// filter applies to both the Status path AND the Issues path. When r.Issues
// contains a mix of lifecycle phrases and real issue phrases, only the real
// issue phrases produce Findings.
//
// Setup: Issues = ["running", "impaired", "terminated"]
// Expected: exactly 1 Finding with Phrase="impaired" and Severity=SevBroken.
//
// Pre-fix: all three issues produce Findings (shim does not filter by phrase).
// Post-fix: "running" and "terminated" are filtered; only "impaired" survives.
func TestDerive_LifecyclePhrasesInIssuesAreAlsoSkipped(t *testing.T) {
	r := domain.Resource{
		ID:     "i-mixed",
		Issues: []string{"running", "impaired", "terminated"},
	}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1 (only non-lifecycle phrase 'impaired' must produce a Finding)", len(r.Findings))
	}
	if r.Findings[0].Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, "impaired")
	}
	if r.Findings[0].Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want SevBroken", r.Findings[0].Severity)
	}
}

// TestDerive_Wave2OnTopOfLifecycleStatus verifies that when r.Status is a
// lifecycle steady-state ("running") and an EnrichmentFinding is present, the
// result contains exactly one Finding — the wave2 entry — because the lifecycle
// Status was filtered and did not produce a wave1 Finding.
//
// This means Wave 2 is Findings[0] (index 0), not Findings[1].
//
// Pre-fix: "running" produces Findings[0] (wave1) and enrichment is Findings[1].
// Post-fix: "running" is filtered; enrichment becomes the sole Findings[0].
func TestDerive_Wave2OnTopOfLifecycleStatus(t *testing.T) {
	r := domain.Resource{ID: "i-w2", Status: "running"}
	// Wave-1 derive: "running" is a lifecycle phrase — no wave1 finding.
	attention.DeriveFindings(&r, ec2TD)
	// Wave-2 append (simulates applyEnrichment): directly append the wave2 finding.
	w2 := domain.Finding{
		Code:     "ec2.pending.maintenance",
		Phrase:   "pending maintenance",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}
	r.Findings = append(r.Findings, w2)
	if r.AttentionDetails == nil {
		r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail)
	}
	r.AttentionDetails[w2.Code] = domain.AttentionDetail{
		Rows: []domain.DetailRow{{Label: "Action", Value: "reboot"}},
	}

	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1 (lifecycle filtered; wave2 is the only Finding)", len(r.Findings))
	}
	if r.Findings[0].Phrase != "pending maintenance" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", r.Findings[0].Phrase, "pending maintenance")
	}
	if r.Findings[0].Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want SevBroken", r.Findings[0].Severity)
	}
	if r.Findings[0].Source != "wave2:ec2" {
		t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, "wave2:ec2")
	}
}

// TestDerive_Wave2Only_AppendsFindingAndAttentionDetails verifies the wave2
// path: when Status and Issues are empty but an EnrichmentFinding exists for
// the resource ID, one wave2 Finding is produced and AttentionDetails is
// populated with the corresponding rows.
func TestDerive_Wave2Only_AppendsFindingAndAttentionDetails(t *testing.T) {
	r := domain.Resource{ID: "i-005"}
	// Wave-1 derive: no status/issues → no wave1 findings.
	attention.DeriveFindings(&r, ec2TD)
	// Wave-2 append (simulates applyEnrichment).
	wantCode := domain.FindingCode("ec2.pending.maintenance")
	w2 := domain.Finding{
		Code:     wantCode,
		Phrase:   "pending maintenance",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}
	r.Findings = append(r.Findings, w2)
	r.AttentionDetails = map[domain.FindingCode]domain.AttentionDetail{
		wantCode: {Rows: []domain.DetailRow{{Label: "Action", Value: "reboot", Tier: ""}}},
	}

	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Source != "wave2:ec2" {
		t.Errorf("Source: got %q, want %q", f.Source, "wave2:ec2")
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity: got %v, want SevBroken", f.Severity)
	}
	if f.Phrase != "pending maintenance" {
		t.Errorf("Phrase: got %q, want %q", f.Phrase, "pending maintenance")
	}

	// Code is derived from slug of "pending maintenance" under "ec2" short name.
	if f.Code != wantCode {
		t.Errorf("Code: got %q, want %q", f.Code, wantCode)
	}

	// AttentionDetails must carry one entry keyed by the finding's code.
	if len(r.AttentionDetails) != 1 {
		t.Fatalf("len(AttentionDetails): got %d, want 1", len(r.AttentionDetails))
	}
	ad, ok := r.AttentionDetails[wantCode]
	if !ok {
		t.Fatalf("AttentionDetails missing key %q", wantCode)
	}
	if len(ad.Rows) != 1 {
		t.Fatalf("len(AttentionDetails[%q].Rows): got %d, want 1", wantCode, len(ad.Rows))
	}
	row := ad.Rows[0]
	if row.Label != "Action" {
		t.Errorf("Row.Label: got %q, want %q", row.Label, "Action")
	}
	if row.Value != "reboot" {
		t.Errorf("Row.Value: got %q, want %q", row.Value, "reboot")
	}
	if row.Tier != "" {
		t.Errorf("Row.Tier: got %q, want %q (empty — no tier on FindingRow)", row.Tier, "")
	}
}

// TestDerive_Wave1AndWave2_Combined verifies that when both r.Issues and an
// EnrichmentFinding are present, wave1 Findings come first (in Issues order),
// followed by the single wave2 Finding. AttentionDetails carries only the
// wave2 entry (wave1 findings have no structured rows).
func TestDerive_Wave1AndWave2_Combined(t *testing.T) {
	r := domain.Resource{
		ID:     "i-006",
		Issues: []string{"impaired"},
	}
	// Wave-1 derive.
	attention.DeriveFindings(&r, ec2TD)
	// Wave-2 append (simulates applyEnrichment).
	w2 := domain.Finding{
		Code:     "ec2.scheduled.reboot",
		Phrase:   "scheduled reboot",
		Severity: domain.SevWarn,
		Source:   "wave2:ec2",
	}
	r.Findings = append(r.Findings, w2)
	r.AttentionDetails = map[domain.FindingCode]domain.AttentionDetail{
		w2.Code: {Rows: []domain.DetailRow{{Label: "Window", Value: "sat 02:00"}}},
	}

	if len(r.Findings) != 2 {
		t.Fatalf("len(Findings): got %d, want 2", len(r.Findings))
	}

	wave1 := r.Findings[0]
	if wave1.Code != "ec2.impaired" {
		t.Errorf("Findings[0].Code: got %q, want %q", wave1.Code, "ec2.impaired")
	}
	if wave1.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", wave1.Source, "wave1")
	}

	wave2 := r.Findings[1]
	if wave2.Source != "wave2:ec2" {
		t.Errorf("Findings[1].Source: got %q, want %q", wave2.Source, "wave2:ec2")
	}
	if wave2.Code != "ec2.scheduled.reboot" {
		t.Errorf("Findings[1].Code: got %q, want %q", wave2.Code, "ec2.scheduled.reboot")
	}

	// AttentionDetails has only the wave2 entry.
	if len(r.AttentionDetails) != 1 {
		t.Fatalf("len(AttentionDetails): got %d, want 1 (wave1 has no detail rows)", len(r.AttentionDetails))
	}
	if _, ok := r.AttentionDetails[wave2.Code]; !ok {
		t.Errorf("AttentionDetails missing wave2 key %q", wave2.Code)
	}
}

// TestDerive_Wave2SeverityMapping verifies the three severity translations:
// "!" -> SevBroken, "~" -> SevWarn, any other value -> SevDim.
func TestDerive_Wave2SeverityMapping(t *testing.T) {
	cases := []struct {
		inputSev  string
		wantSev   domain.Severity
		wantLabel string
	}{
		{"!", domain.SevBroken, "SevBroken"},
		{"~", domain.SevWarn, "SevWarn"},
		{"unknown", domain.SevDim, "SevDim"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.inputSev, func(t *testing.T) {
			r := domain.Resource{ID: "i-sev"}
			// Wave-2 append (simulates applyEnrichment).
			w2 := domain.Finding{
				Code:     "ec2.test.sev",
				Phrase:   "test phrase",
				Severity: tc.wantSev,
				Source:   "wave2:ec2",
			}
			r.Findings = append(r.Findings, w2)
			if len(r.Findings) != 1 {
				t.Fatalf("len(Findings): got %d, want 1", len(r.Findings))
			}
			if r.Findings[0].Severity != tc.wantSev {
				t.Errorf("Severity(%q): got %v, want %s", tc.inputSev, r.Findings[0].Severity, tc.wantLabel)
			}
		})
	}
}

// TestDerive_Deterministic_RepeatedCallsIdentical verifies that calling
// DeriveFindings twice with identical inputs produces identical outputs.
// The second call must REPLACE r.Findings and r.AttentionDetails, never
// append to them.
func TestDerive_Deterministic_RepeatedCallsIdentical(t *testing.T) {
	r := domain.Resource{
		ID:     "i-007",
		Issues: []string{"impaired"},
	}

	// First call: wave1 derive only.
	attention.DeriveFindings(&r, ec2TD)
	findings1 := make([]domain.Finding, len(r.Findings))
	copy(findings1, r.Findings)
	details1 := r.AttentionDetails

	// Second call with same inputs — must produce identical result, not append.
	attention.DeriveFindings(&r, ec2TD)

	if !reflect.DeepEqual(findings1, r.Findings) {
		t.Errorf("Findings changed on second call: first=%v second=%v", findings1, r.Findings)
	}
	if !reflect.DeepEqual(details1, r.AttentionDetails) {
		t.Errorf("AttentionDetails changed on second call")
	}
	// Specifically verify the count hasn't doubled (append would produce 2 not 1).
	if len(r.Findings) != 1 {
		t.Errorf("len(Findings) after second call: got %d, want 1 (replace, not append)", len(r.Findings))
	}
}

// TestDerive_Wave2BridgeOnSecondCall is the critical Wave-2-bridge test from
// the spec: the shim must NOT early-return when len(r.Findings) > 0.
//
// First call with no enrichment produces wave1 only.
// Second call with the same r but enrichment populated must produce wave1 + wave2.
func TestDerive_Wave2BridgeOnSecondCall(t *testing.T) {
	r := domain.Resource{
		ID:     "i-008",
		Issues: []string{"impaired"},
	}

	// First pass: no enrichment.
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 1 {
		t.Fatalf("after first call: len(Findings): got %d, want 1", len(r.Findings))
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("after first call: Findings[0].Source: got %q, want wave1", r.Findings[0].Source)
	}

	// Second pass: re-derive wave1, then append wave2 (simulates applyEnrichment).
	attention.DeriveFindings(&r, ec2TD)
	w2 := domain.Finding{
		Code:     "ec2.pending.maintenance",
		Phrase:   "pending maintenance",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}
	r.Findings = append(r.Findings, w2)
	if len(r.Findings) != 2 {
		t.Fatalf("after second call: len(Findings): got %d, want 2 (wave1 + wave2, no early-return)", len(r.Findings))
	}
	if r.Findings[1].Source != "wave2:ec2" {
		t.Errorf("after second call: Findings[1].Source: got %q, want wave2:ec2", r.Findings[1].Source)
	}
}

// TestDerive_NoEarlyReturnOnExistingFindings verifies that pre-existing stale
// entries in r.Findings are replaced (not preserved) when the shim derives a
// healthy row (no Status, no Issues, no enrichment). The result must be nil,
// never the pre-populated slice.
func TestDerive_NoEarlyReturnOnExistingFindings(t *testing.T) {
	r := domain.Resource{
		ID: "i-009",
		Findings: []domain.Finding{
			{Code: "manual", Phrase: "stale", Severity: domain.SevBroken, Source: "manual"},
		},
	}
	attention.DeriveFindings(&r, ec2TD)
	if len(r.Findings) != 0 {
		t.Errorf("len(Findings): got %d, want 0 (shim must replace stale entries on healthy row)", len(r.Findings))
	}
}

// TestDerive_InactiveIsEmittedAsFinding pins that "inactive" is NOT
// universally ignorable lifecycle noise. Several types — notably ECS
// services and ECS clusters in internal/resource/types_compute.go —
// classify INACTIVE as broken. The shim must emit a Finding so the
// downstream IsIssue / detail-view paths can surface it.
//
// The shim's coarse phrase→severity mapping classifies "inactive" via
// the default branch (SevWarn). Per-category PRs (03c containers) will
// refine the canonical code/severity for ECS specifically. Until then,
// SevWarn is sufficient: row color follows td.ResolveColor (which ECS
// types map to ColorBroken) and IsIssue is true for SevWarn.
func TestDerive_InactiveIsEmittedAsFinding(t *testing.T) {
	r := domain.Resource{ID: "i", Status: "inactive"}
	td := resource.ResourceTypeDef{ShortName: "ecs-svc"}
	attention.DeriveFindings(&r, td)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings count = %d, want 1 (inactive must emit a Finding — see ECS type policy)", len(r.Findings))
	}
	if r.Findings[0].Phrase != "inactive" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "inactive")
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source = %q, want %q", r.Findings[0].Source, "wave1")
	}
	if !r.Findings[0].Severity.IsIssue() {
		t.Errorf("Findings[0].Severity = %v, expected IsIssue() == true (so ctrl+z and detail attention surface it)", r.Findings[0].Severity)
	}
}

// TestDerive_MigratedRowPreservesFetcherFindings pins that DeriveFindings
// does NOT clobber wave1 Findings emitted directly by a migrated fetcher.
//
// Pre-fix: r.Status="" and r.Issues=nil → shim derives empty wave1, replaces
// r.Findings → fetcher's emissions wiped.
// Post-fix: shim early-returns or preserves wave1 when both legacy inputs
// are empty.
func TestDerive_MigratedRowPreservesFetcherFindings(t *testing.T) {
	r := domain.Resource{
		ID:     "i-1",
		Type:   "ec2",
		Status: "",  // migrated fetcher writes nothing
		Issues: nil,
		Fields: map[string]string{"state": "stopped"},
		Findings: []domain.Finding{
			{Code: "ec2.state.stopped.server", Phrase: "stopped", Severity: domain.SevBroken, Source: "wave1"},
		},
	}
	td := resource.ResourceTypeDef{ShortName: "ec2"}
	attention.DeriveFindings(&r, td)

	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1 — shim must preserve fetcher-emitted wave1 entries when r.Status/r.Issues are empty", len(r.Findings))
	}
	if r.Findings[0].Code != "ec2.state.stopped.server" {
		t.Errorf("Findings[0].Code = %q, want %q (migrated row should keep fetcher emission)", r.Findings[0].Code, "ec2.state.stopped.server")
	}
	if r.Findings[0].Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity = %v, want SevBroken", r.Findings[0].Severity)
	}
}

// TestDerive_MigratedRowMergesWave2WithFetcherWave1 pins that wave2 enrichment
// STILL merges correctly even when the row is from a migrated fetcher
// (wave1 from fetcher + wave2 from enrichment map).
//
// Pre-fix: shim re-derives wave1 from empty Status/Issues → produces empty
// wave1, then appends wave2 → only wave2 survives (fetcher wave1 wiped).
// Post-fix: shim detects pre-populated Findings with Source="wave1" and uses
// them as-is, then appends wave2 → two entries total.
func TestDerive_MigratedRowMergesWave2WithFetcherWave1(t *testing.T) {
	r := domain.Resource{
		ID:   "i-1",
		Type: "ec2",
		Findings: []domain.Finding{
			{Code: "ec2.state.stopping", Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"},
		},
	}
	td := resource.ResourceTypeDef{ShortName: "ec2"}
	// Wave-1 derive: shim preserves pre-populated fetcher wave1 entries.
	attention.DeriveFindings(&r, td)
	// Wave-2 append (simulates applyEnrichment).
	w2 := domain.Finding{
		Code:     "ec2.instance.status.impaired",
		Phrase:   "instance status: impaired",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}
	r.Findings = append(r.Findings, w2)
	r.AttentionDetails = map[domain.FindingCode]domain.AttentionDetail{
		w2.Code: {Rows: []domain.DetailRow{{Label: "Instance Status", Value: "impaired"}}},
	}

	if len(r.Findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2 (wave1 from fetcher + wave2 from enrichment)", len(r.Findings))
	}
	// Order: wave1 first, wave2 second
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source = %q, want wave1", r.Findings[0].Source)
	}
	if r.Findings[1].Source != "wave2:ec2" {
		t.Errorf("Findings[1].Source = %q, want wave2:ec2", r.Findings[1].Source)
	}
}
