package unit_test

// qa_networking_color_phrase_agree_test.go — ruling M for the five networking
// types.
//
// The row colour takes the worst severity on the row. The Status cell took the
// first issue-severity finding in slice order (colorFromWave1 still selects
// that way). A row whose findings arrive warn-then-broken therefore renders red
// while reading as the warning, so the operator sees the colour that says
// "drop everything" next to the sentence for the lesser problem.
//
// This is the demo-bench half of tests/unit/prowler_w1_status_phrase_test.go on
// main, which pins domain.TopFinding's own rules. That selector does not exist
// in this worktree yet, so the invariant is asserted directly: whichever
// finding the cell names must be one of the worst-severity findings on the row.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// netWorstSeverity returns the highest severity among r's findings, and false
// when the row carries none.
func netWorstSeverity(r resource.Resource) (domain.Severity, bool) {
	worst, ok := domain.Severity(0), false
	for _, f := range r.Findings {
		if !ok || f.Severity > worst {
			worst, ok = f.Severity, true
		}
	}
	return worst, ok
}

// netColorForSeverity mirrors the display mapping core/aws applies, so the
// test computes the expected colour from the worst finding rather than
// trusting the classifier it is checking.
func netColorForSeverity(sev domain.Severity) domain.Color {
	switch sev {
	case domain.SevBroken:
		return domain.ColorBroken
	case domain.SevWarn:
		return domain.ColorWarning
	case domain.SevDim:
		return domain.ColorDim
	default:
		return domain.ColorHealthy
	}
}

// TestNetworkingColorAndPhrase_ComeFromTheSameFinding sweeps every demo row of
// the five networking types and fails when the Status cell names a finding
// that is not among the worst-severity ones — the case where the colour and
// the words disagree about what is wrong.
func TestNetworkingColorAndPhrase_ComeFromTheSameFinding(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	demoClients := demo.NewServiceClients()

	var bad []string
	for _, short := range netTypes {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s: not registered", short)
		}
		fixtures := byType[short]
		if len(fixtures) == 0 {
			t.Fatalf("%s: no demo fixtures — this gate cannot see the type", short)
		}
		merged := mergeWave2Findings(t, *td, fixtures, cache, demoClients)

		for _, res := range merged {
			worst, ok := netWorstSeverity(res)
			if !ok {
				continue
			}
			if got, want := td.ResolveColor(res), netColorForSeverity(worst); got != want {
				bad = append(bad, fmt.Sprintf("%s/%s: row colour = %v, want %v from the worst finding on the row %v",
					short, res.ID, got, want, netPhrases(res)))
			}
			cell, found := listStatusCellFor(t, *td, merged, res.ID)
			if !found || strings.TrimSpace(cell) == "" {
				continue
			}

			// The cell leads with one finding's phrase and may append "(+N)".
			var named []domain.Finding
			for _, f := range res.Findings {
				if f.Phrase != "" && strings.Contains(cell, f.Phrase) {
					named = append(named, f)
				}
			}
			if len(named) == 0 {
				bad = append(bad, fmt.Sprintf("%s/%s: Status cell %q names none of the row's findings %v",
					short, res.ID, cell, netPhrases(res)))
				continue
			}
			agrees := false
			for _, f := range named {
				if f.Severity == worst {
					agrees = true
				}
			}
			if !agrees {
				bad = append(bad, fmt.Sprintf(
					"%s/%s: colour comes from a %v finding but the Status cell %q names only %v — "+
						"the row's colour and its words disagree about what is wrong",
					short, res.ID, worst, cell, netSeverities(named)))
			}
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d row(s) colour from one finding and read as another:\n  %s", len(bad), strings.Join(bad, "\n  "))
}

func netPhrases(r resource.Resource) []string {
	out := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		out = append(out, fmt.Sprintf("%q(%v)", f.Phrase, f.Severity))
	}
	return out
}

func netSeverities(fs []domain.Finding) []domain.Severity {
	out := make([]domain.Severity, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Severity)
	}
	return out
}

// TestNetworkingColorAndPhrase_WarnThenBrokenRow is the adversarial case the
// demo bench does not contain: a row carrying a warning ahead of a broken
// finding in slice order. Selecting the head of the slice picks the warning,
// so the row renders yellow while the worst thing wrong is broken, and the
// Status cell reads as the lesser problem. Without this the sweep above passes
// simply because no fixture mixes severities on one row.
func TestNetworkingColorAndPhrase_WarnThenBrokenRow(t *testing.T) {
	cases := []struct {
		shortName string
		fields    map[string]string
		warn      domain.Finding
		broken    domain.Finding
	}{
		{
			shortName: "subnet",
			fields: map[string]string{
				"subnet_id": "subnet-0mixedseverity0", "name": "acme-mixed",
				"vpc_id": "vpc-0abc123", "cidr_block": "10.0.9.0/24",
				"availability_zone": "us-east-1a", "state": "unavailable",
				"available_ips": "250",
			},
			warn:   domain.Finding{Code: "subnet.auto-public-ip", Phrase: "auto-assigns public IPs", Severity: domain.SevWarn, Source: "wave1"},
			broken: domain.Finding{Code: "subnet.state.unavailable", Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"},
		},
		{
			shortName: "vpce",
			fields: map[string]string{
				"vpce_id": "vpce-0mixedseverity0", "service_name": "com.amazonaws.us-east-1.s3",
				"type": "Gateway", "state": "Failed", "vpc_id": "vpc-0abc123",
			},
			warn:   domain.Finding{Code: "vpce.policy-open", Phrase: "endpoint policy allows any principal", Severity: domain.SevWarn, Source: "wave1"},
			broken: domain.Finding{Code: "vpce.state.failed", Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			td := resource.FindResourceType(tc.shortName)
			if td == nil {
				t.Fatalf("%s: not registered", tc.shortName)
			}
			id := tc.fields[tc.shortName+"_id"]
			row := resource.Resource{
				ID: id, Name: "acme-mixed", Fields: tc.fields,
				Findings: []domain.Finding{tc.warn, tc.broken},
			}

			if got := td.ResolveColor(row); got != domain.ColorBroken {
				t.Errorf("row colour = %v, want ColorBroken: the row carries a broken finding (%q) "+
					"but the classifier took the warning (%q) that merely sits first in the slice",
					got, tc.broken.Phrase, tc.warn.Phrase)
			}
			cell, found := listStatusCellFor(t, *td, []resource.Resource{row}, row.ID)
			if !found {
				t.Fatal("no Status cell rendered for the mixed-severity row")
			}
			if !strings.Contains(cell, tc.broken.Phrase) {
				t.Errorf("Status cell = %q, want it to name the broken finding %q that decides the colour",
					cell, tc.broken.Phrase)
			}
		})
	}
}
