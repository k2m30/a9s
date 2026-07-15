package unit

// qa_ami_color_test.go — Color contract pin for AMIs.
//
// Since the color-findings-conformance wave (qa_color_findings_conformance_test.go),
// colorAMI is colorFromAnyFinding-only (core/aws/catalog_compute.go) — it
// has NO raw-field fallback at all. Every non-healthy case here attaches a
// Finding shaped exactly like the real fetcher (core/aws/ami.go, wave1
// state/deprecation Findings, codes in ami_codes.go). Fields are kept for
// realism/context only — they are no longer read by Color.
//
// colorAMI is a bare `colorFromAnyFinding(r) or ColorHealthy` — it does not
// branch on finding Code, only on Severity and Source-prefix ("wave1" or
// "wave2:"). One representative case per severity tier (plus the no-finding
// Healthy anchor) exercises every branch colorAMI can take; per-state
// wave1-emission mapping (which AWS state produces which code/severity) is
// pinned at the fetcher layer, not here.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestAmiColor_StateAndDeprecation(t *testing.T) {
	td := resource.FindResourceType("ami")
	if td == nil {
		t.Fatal("ami not registered")
	}

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
	}{
		{
			name:   "state=available",
			fields: map[string]string{"state": "available"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=pending",
			fields: map[string]string{"state": "pending"},
			findings: []domain.Finding{
				{Code: "ami.state.pending", Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			},
			want: resource.ColorWarning,
		},
		{
			name:   "state=failed",
			fields: map[string]string{"state": "failed"},
			findings: []domain.Finding{
				{Code: "ami.state.failed", Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:   "state=deregistered",
			fields: map[string]string{"state": "deregistered"},
			findings: []domain.Finding{
				{Code: "ami.state.dim", Phrase: "deregistered", Severity: domain.SevDim, Source: "wave1"},
			},
			want: resource.ColorDim,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields, Findings: tc.findings})
			if got != tc.want {
				t.Errorf("Color(%v, findings=%v) = %v, want %v", tc.fields, tc.findings, got, tc.want)
			}
		})
	}
}
