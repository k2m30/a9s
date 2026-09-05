package unit

// wave2_fold_preserves_wave1_attention_test.go — ApplyWave2ToRow must fold
// wave-2 results onto a row without destroying what wave 1 already put there.
//
// The fold owns exactly the wave-2 slice of a row: findings whose Source is
// "wave2:<type>" and the AttentionDetail entries keyed by those findings'
// codes. Wave-1 findings and their supporting rows are written by the fetcher
// from data the fetcher alone holds, and nothing re-derives them afterwards —
// so if the fold discards them, they are gone for the rest of the row's life
// and the operator sees a finding with no evidence under it. That failure is
// invisible on any type without a wave-2 enricher, which is why it needs a
// test at the fold rather than per type.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const (
	foldWave1Code domain.FindingCode = "sg.default-with-rules"
	foldWave2Code domain.FindingCode = "sg.unused"
	foldStaleCode domain.FindingCode = "sg.stale-wave2"
)

// foldTestRow is a security group carrying a wave-1 finding with its own
// supporting rows, plus the leftovers of a previous wave-2 pass: one finding
// whose condition still holds and one that no longer does.
func foldTestRow() *domain.Resource {
	return &domain.Resource{
		ID:   "sg-0default11111111",
		Name: "default",
		Findings: []domain.Finding{
			{
				Code:     foldWave1Code,
				Phrase:   "default group allows traffic",
				Detail:   "The default group still carries rules.",
				Severity: domain.SevWarn,
				Source:   "wave1",
			},
			{
				Code:     foldStaleCode,
				Phrase:   "stale from the previous pass",
				Severity: domain.SevWarn,
				Source:   "wave2:sg",
			},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			foldWave1Code: {Rows: []domain.DetailRow{
				{Label: "Ingress rules", Value: "1", Tier: "~"},
				{Label: "Egress rules", Value: "2", Tier: "~"},
			}},
			foldStaleCode: {Rows: []domain.DetailRow{
				{Label: "Stale", Value: "yes", Tier: "~"},
			}},
		},
	}
}

func foldTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{ShortName: "sg"}
}

// TestApplyWave2ToRow_KeepsWave1AttentionRows pins the core contract: the
// wave-1 finding and every one of its supporting rows survive a wave-2 fold.
func TestApplyWave2ToRow_KeepsWave1AttentionRows(t *testing.T) {
	r := foldTestRow()

	runtime.ApplyWave2ToRow(r, foldTypeDef(),
		map[string][]domain.Finding{
			r.ID: {{
				Code:     foldWave2Code,
				Phrase:   "not attached to anything",
				Severity: domain.SevWarn,
				Source:   "wave2:sg",
			}},
		},
		map[string]map[domain.FindingCode]domain.AttentionDetail{
			r.ID: {foldWave2Code: {Rows: []domain.DetailRow{
				{Label: "Network interfaces", Value: "0", Tier: "~"},
			}}},
		},
	)

	ad, ok := r.AttentionDetails[foldWave1Code]
	if !ok {
		t.Fatalf("wave-1 AttentionDetail for %s was wiped by the wave-2 fold; AttentionDetails=%+v",
			foldWave1Code, r.AttentionDetails)
	}
	want := [][2]string{{"Ingress rules", "1"}, {"Egress rules", "2"}}
	if len(ad.Rows) != len(want) {
		t.Fatalf("wave-1 rows = %+v, want %v", ad.Rows, want)
	}
	for i, w := range want {
		if ad.Rows[i].Label != w[0] || ad.Rows[i].Value != w[1] {
			t.Errorf("wave-1 row %d = %q: %q, want %q: %q", i, ad.Rows[i].Label, ad.Rows[i].Value, w[0], w[1])
		}
	}

	if _, ok := r.AttentionDetails[foldWave2Code]; !ok {
		t.Errorf("wave-2 AttentionDetail for %s missing; AttentionDetails=%+v", foldWave2Code, r.AttentionDetails)
	}
}

// TestApplyWave2ToRow_DropsStaleWave2Attention is the other half: the fold
// owns the wave-2 entries, so an entry whose finding is no longer emitted must
// not linger. Without this the first test could be satisfied by never clearing
// anything, which would leave resolved issues on screen forever.
func TestApplyWave2ToRow_DropsStaleWave2Attention(t *testing.T) {
	r := foldTestRow()

	runtime.ApplyWave2ToRow(r, foldTypeDef(),
		map[string][]domain.Finding{
			r.ID: {{
				Code:     foldWave2Code,
				Phrase:   "not attached to anything",
				Severity: domain.SevWarn,
				Source:   "wave2:sg",
			}},
		},
		map[string]map[domain.FindingCode]domain.AttentionDetail{
			r.ID: {foldWave2Code: {Rows: []domain.DetailRow{
				{Label: "Network interfaces", Value: "0", Tier: "~"},
			}}},
		},
	)

	if ad, ok := r.AttentionDetails[foldStaleCode]; ok {
		t.Errorf("stale wave-2 AttentionDetail for %s survived the fold with rows %+v", foldStaleCode, ad.Rows)
	}
	for _, f := range r.Findings {
		if f.Code == foldStaleCode {
			t.Errorf("stale wave-2 finding %s survived the fold: %+v", foldStaleCode, f)
		}
	}
}

// TestApplyWave2ToRow_EmptyResultKeepsWave1 pins the common case on a type
// with a wave-2 enricher that found nothing. The early return for an empty
// finding set must not be a shortcut past the wave-1 rows — this is the path
// every healthy row in the account takes.
func TestApplyWave2ToRow_EmptyResultKeepsWave1(t *testing.T) {
	r := foldTestRow()

	runtime.ApplyWave2ToRow(r, foldTypeDef(),
		map[string][]domain.Finding{},
		map[string]map[domain.FindingCode]domain.AttentionDetail{},
	)

	if _, ok := r.AttentionDetails[foldWave1Code]; !ok {
		t.Errorf("wave-1 AttentionDetail for %s was wiped by a wave-2 pass that emitted nothing; AttentionDetails=%+v",
			foldWave1Code, r.AttentionDetails)
	}
	if _, ok := r.AttentionDetails[foldStaleCode]; ok {
		t.Errorf("stale wave-2 AttentionDetail for %s survived an empty wave-2 pass", foldStaleCode)
	}
}

// TestApplyWave2ToRow_RepeatedFoldIsStable pins idempotence across the
// refreshes a long-lived row actually sees: folding the same result twice must
// leave the row identical, not accumulate or erode entries.
func TestApplyWave2ToRow_RepeatedFoldIsStable(t *testing.T) {
	findings := map[string][]domain.Finding{
		"sg-0default11111111": {{
			Code:     foldWave2Code,
			Phrase:   "not attached to anything",
			Severity: domain.SevWarn,
			Source:   "wave2:sg",
		}},
	}
	details := map[string]map[domain.FindingCode]domain.AttentionDetail{
		"sg-0default11111111": {foldWave2Code: {Rows: []domain.DetailRow{
			{Label: "Network interfaces", Value: "0", Tier: "~"},
		}}},
	}

	r := foldTestRow()
	runtime.ApplyWave2ToRow(r, foldTypeDef(), findings, details)
	runtime.ApplyWave2ToRow(r, foldTypeDef(), findings, details)

	if len(r.AttentionDetails) != 2 {
		t.Errorf("AttentionDetails = %+v, want exactly the wave-1 and wave-2 entries", r.AttentionDetails)
	}
	if len(r.Findings) != 2 {
		t.Errorf("Findings = %+v, want exactly the wave-1 and wave-2 findings", r.Findings)
	}
}

// TestApplyWave2ToRow_NeverWritesThroughSharedBacking pins the property that
// lets every caller hand the fold a shallow row copy: the fold must replace
// Findings and AttentionDetails with freshly allocated ones, never append into
// the spare capacity of the slice the caller still shares with its pre-Amend
// rows.
//
// The amend callbacks in core/app, core/runtime and internal/tui all do
// copy(out, rows), which duplicates the row structs but not the arrays behind
// their slice headers. If the fold ever grew Findings in place, the write
// would land in the store's own backing array and in any snapshot taken
// before the amend — a corruption no caller could see or defend against.
// One test here covers all four call sites.
func TestApplyWave2ToRow_NeverWritesThroughSharedBacking(t *testing.T) {
	// A row whose Findings slice has room to grow, as a fetcher's append loop
	// naturally leaves it.
	backing := make([]domain.Finding, 1, 8)
	backing[0] = domain.Finding{
		Code: foldWave1Code, Phrase: "default group allows traffic",
		Severity: domain.SevWarn, Source: "wave1",
	}
	callerDetails := map[domain.FindingCode]domain.AttentionDetail{
		foldWave1Code: {Rows: []domain.DetailRow{{Label: "Ingress rules", Value: "1", Tier: "~"}}},
	}

	// What the store still sees: the same array, and the same map.
	shared := backing[:cap(backing)]
	r := &domain.Resource{ID: "sg-0default11111111", Findings: backing, AttentionDetails: callerDetails}

	runtime.ApplyWave2ToRow(r, foldTypeDef(),
		map[string][]domain.Finding{
			r.ID: {{Code: foldWave2Code, Phrase: "not attached to anything", Severity: domain.SevWarn, Source: "wave2:sg"}},
		},
		map[string]map[domain.FindingCode]domain.AttentionDetail{
			r.ID: {foldWave2Code: {Rows: []domain.DetailRow{{Label: "Network interfaces referencing", Value: "0", Tier: "~"}}}},
		},
	)

	if _, ok := w3FindingByCode(r.Findings, foldWave2Code); !ok {
		t.Fatalf("precondition: the fold did not append the wave-2 finding; Findings=%+v", r.Findings)
	}
	for i := 1; i < len(shared); i++ {
		if shared[i].Code != "" {
			t.Errorf("the fold wrote %q into shared spare capacity at index %d; the store's own rows would see it",
				shared[i].Code, i)
		}
	}
	if shared[0].Code != foldWave1Code {
		t.Errorf("shared[0] = %q, want the caller's untouched wave-1 finding %q", shared[0].Code, foldWave1Code)
	}

	// The caller's map must likewise be left alone, not adopted and extended.
	if _, ok := callerDetails[foldWave2Code]; ok {
		t.Error("the fold wrote the wave-2 AttentionDetail into the caller's own map instead of a fresh one")
	}
	if len(callerDetails) != 1 {
		t.Errorf("caller's AttentionDetails = %+v, want its single original entry", callerDetails)
	}
}
