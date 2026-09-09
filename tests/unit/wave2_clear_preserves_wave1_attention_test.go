package unit_test

// wave2_clear_preserves_wave1_attention_test.go — the wave-2 CLEAR paths, the
// sibling class of the wave-2 FOLD.
//
// ApplyWave2ToRow drops only the entries keyed by outgoing wave-2 findings.
// Three other sites do the same job — strip wave-2 findings from a row
// before a refresh — and must not pair that strip with
// `AttentionDetails = nil`, which destroys the wave-1 supporting rows the
// fetcher alone can produce, so that a Ctrl+R loses them.
//
// All three route through runtime.ApplyWave2ToRow with nil findings and nil
// details: that call already means "drop the wave-2 slice, keep the rest"
// and is pinned by TestApplyWave2ToRow_EmptyResultKeepsWave1.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	clearWave1Code domain.FindingCode = "sg.default-with-rules"
	clearWave2Code domain.FindingCode = "sg.unused"
)

// seedClearRow registers a throwaway enricher type and seeds one row carrying
// a wave-1 finding with supporting rows plus a wave-2 finding with its own.
func seedClearRow(t *testing.T, c *runtime.Core, rt string) {
	t.Helper()
	awsclient.SetWave2EnricherForTest(t, rt, awsclient.IssueEnricher{Fn: awsclient.InFetcherWave2Sentinel, Priority: 100})

	c.ObserveRows(rt, []resource.Resource{{
		ID:   "res-001",
		Name: "res-001",
		Findings: []domain.Finding{
			{Code: clearWave1Code, Phrase: "default group allows traffic", Severity: domain.SevWarn, Source: "wave1"},
			{Code: clearWave2Code, Phrase: "not attached to anything", Severity: domain.SevWarn, Source: "wave2:" + rt},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			clearWave1Code: {Rows: []domain.DetailRow{
				{Label: "Ingress rules", Value: "1", Tier: "~"},
				{Label: "Egress rules", Value: "2", Tier: "~"},
			}},
			clearWave2Code: {Rows: []domain.DetailRow{
				{Label: "Network interfaces referencing", Value: "0", Tier: "~"},
			}},
		},
	}}, nil, session.OriginFetch, false)
}

// assertWave1Survived checks the row kept its wave-1 finding and evidence and
// lost only the wave-2 pair.
func assertWave1Survived(t *testing.T, c *runtime.Core, rt, via string) {
	t.Helper()
	rows := c.AnyLaneResources(rt)
	if len(rows) != 1 {
		t.Fatalf("%s: rows = %d, want 1", via, len(rows))
	}
	r := rows[0]

	ad, ok := r.AttentionDetails[clearWave1Code]
	if !ok {
		t.Errorf("%s: wave-1 AttentionDetail for %s was wiped; AttentionDetails=%+v", via, clearWave1Code, r.AttentionDetails)
	} else if len(ad.Rows) != 2 {
		t.Errorf("%s: wave-1 rows = %+v, want the 2 seeded rows", via, ad.Rows)
	}
	if _, ok := r.AttentionDetails[clearWave2Code]; ok {
		t.Errorf("%s: wave-2 AttentionDetail for %s survived the clear", via, clearWave2Code)
	}

	var codes []domain.FindingCode
	for _, f := range r.Findings {
		codes = append(codes, f.Code)
	}
	if len(codes) != 1 || codes[0] != clearWave1Code {
		t.Errorf("%s: Findings = %v, want exactly the wave-1 finding", via, codes)
	}
}

// TestRefreshListEnrichment_KeepsWave1Attention covers the list Ctrl+R path
// (core/runtime/handlers_resources.go stripWave2FindingsRows).
func TestRefreshListEnrichment_KeepsWave1Attention(t *testing.T) {
	const rt = "test-clear-refresh-src"
	c, _ := newTestControllerWithCore(t)
	seedClearRow(t, c, rt)

	c.RefreshListEnrichment(rt) //nolint:errcheck // the returned gen token is not under test here

	assertWave1Survived(t, c, rt, "RefreshListEnrichment")
}

// TestClearAllWave2Findings_KeepsWave1Attention covers the main-menu Ctrl+R
// path, which reaches the same stripWave2FindingsRows for every retained type.
func TestClearAllWave2Findings_KeepsWave1Attention(t *testing.T) {
	const rt = "test-clear-all-src"
	c, _ := newTestControllerWithCore(t)
	seedClearRow(t, c, rt)

	c.ClearAllWave2Findings()

	assertWave1Survived(t, c, rt, "ClearAllWave2Findings")
}

// TestControllerClearRowFindings_KeepsWave1Attention covers the headless
// controller's own row stores (core/app/list_body.go clearRowFindings), which
// clears in parallel with the runtime and must agree with it.
func TestControllerClearRowFindings_KeepsWave1Attention(t *testing.T) {
	const rt = "test-clear-ctrl-src"
	c, ctrl := newTestControllerWithCore(t)
	seedClearRow(t, c, rt)

	ctrl.ClearRowFindings(rt)

	assertWave1Survived(t, c, rt, "Controller.ClearRowFindings")
}
