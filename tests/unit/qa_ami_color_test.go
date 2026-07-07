package unit

// qa_ami_color_test.go — Color contract pin for AMIs.
//
// Since the color-findings-conformance wave (qa_color_findings_conformance_test.go),
// colorAMI is colorFromAnyFinding-only (internal/aws/catalog_compute.go) — it
// has NO raw-field fallback at all. Every non-healthy case here attaches a
// Finding shaped exactly like the real fetcher (internal/aws/ami.go, wave1
// state/deprecation Findings, codes in ami_codes.go). Fields are kept for
// realism/context only — they are no longer read by Color.

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestAmiColor_StateAndDeprecation(t *testing.T) {
	td := resource.FindResourceType("ami")
	if td == nil {
		t.Fatal("ami not registered")
	}

	pastYear := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	futureYear := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
	}{
		// State-based cases.
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
			name:   "state=transient",
			fields: map[string]string{"state": "transient"},
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
			name:   "state=error",
			fields: map[string]string{"state": "error"},
			findings: []domain.Finding{
				{Code: "ami.state.failed", Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:   "state=invalid",
			fields: map[string]string{"state": "invalid"},
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
		{
			name:   "state=disabled",
			fields: map[string]string{"state": "disabled"},
			findings: []domain.Finding{
				{Code: "ami.state.dim", Phrase: "disabled", Severity: domain.SevDim, Source: "wave1"},
			},
			want: resource.ColorDim,
		},
		// Deprecation cases — only apply when state=available.
		{
			name:   "state=available+deprecation_in_past",
			fields: map[string]string{"state": "available", "deprecation_time": pastYear},
			findings: []domain.Finding{
				{
					Code: "ami.deprecated", Phrase: "deprecated",
					Detail:   "DeprecationTime has passed — AWS no longer recommends this AMI for new launches.",
					Severity: domain.SevWarn, Source: "wave1",
				},
			},
			want: resource.ColorWarning,
		},
		{
			name:   "state=available+deprecation_in_future",
			fields: map[string]string{"state": "available", "deprecation_time": futureYear},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=available+deprecation_time_empty",
			fields: map[string]string{"state": "available", "deprecation_time": ""},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=available+deprecation_time_invalid",
			fields: map[string]string{"state": "available", "deprecation_time": "not-a-date"},
			want:   resource.ColorHealthy,
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
