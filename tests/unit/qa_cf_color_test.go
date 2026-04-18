package unit

// qa_cf_color_test.go — Behavioral tests for the CloudFront Distributions Color function.
//
// CodeRabbit PR-273 finding: internal/resource/types_dns_cdn.go:50-58 ignores the
// "enabled" field entirely. docs/attention-signals.md specifies: Enabled==false → Dim,
// regardless of status. The current colorer returns ColorHealthy for "Deployed" whether
// or not enabled is set, and does not return ColorDim at all.
//
// These tests will FAIL until the production colorer is fixed to check enabled first.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestCloudFrontColor(t *testing.T) {
	td := resource.FindResourceType("cf")
	if td == nil {
		t.Fatal("cf (CloudFront) type not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			// Deployed + enabled=true → nominal operation → ColorHealthy.
			name:   "deployed_enabled",
			fields: map[string]string{"status": "Deployed", "enabled": "true"},
			want:   resource.ColorHealthy,
		},
		{
			// Deployed + enabled=false → distribution is disabled → ColorDim.
			// docs/attention-signals.md: Enabled==false → Dim for cf.
			// Current colorer returns ColorHealthy (ignores enabled) — FAILS.
			name:   "deployed_disabled",
			fields: map[string]string{"status": "Deployed", "enabled": "false"},
			want:   resource.ColorDim,
		},
		{
			// InProgress + enabled=true → deploying → ColorWarning.
			name:   "inprogress_enabled",
			fields: map[string]string{"status": "InProgress", "enabled": "true"},
			want:   resource.ColorWarning,
		},
		{
			// InProgress + enabled=false → disabled wins over in-progress → ColorDim.
			// docs/attention-signals.md: disabled distributions are Dim regardless of status.
			// Current colorer returns ColorWarning (ignores enabled) — FAILS.
			name:   "inprogress_disabled",
			fields: map[string]string{"status": "InProgress", "enabled": "false"},
			want:   resource.ColorDim,
		},
		{
			// Empty fields → no signals → ColorHealthy (default).
			name:   "empty",
			fields: map[string]string{},
			want:   resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(fields=%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
