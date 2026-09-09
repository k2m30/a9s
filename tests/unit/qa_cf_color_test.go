package unit

// qa_cf_color_test.go — the CloudFront Distributions Color function reads
// findings only.
//
// Colour derives from findings, so a resource carrying no findings is
// Healthy whatever its fields say, and the state each row names is reported
// by the finding the fetcher emits for it (see prowler_w6a_*_test.go). The
// table enumerates the states that must not colour a row on their own, so a
// raw-field branch is what it catches.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
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
			// The fields reach the type\'s own predicate, which produces the
			// finding, so the colour the table names is one the row carries for a
			// reason the detail view shows.
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(fields=%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
