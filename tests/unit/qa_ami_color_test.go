package unit

// Color contract for AMIs.
//
// colorAMI is `colorFromAnyFinding(r) or ColorHealthy`
// (core/aws/catalog_compute.go): it reads finding Severity and Source prefix
// ("wave1" or "wave2:") only, so one case per severity tier plus the
// no-finding Healthy anchor covers every branch. Each finding is shaped like
// the fetcher's (core/aws/ami.go, codes in ami_codes.go); which AWS state
// produces which code is pinned at the fetcher layer.

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
