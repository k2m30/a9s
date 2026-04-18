package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEcsColor(t *testing.T) {
	td := resource.FindResourceType("ecs")
	if td == nil {
		t.Fatal("ecs not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "active",
			fields: map[string]string{"status": "ACTIVE"},
			want:   resource.ColorHealthy,
		},
		{
			// Per docs/attention-signals.md: ecs Cluster INACTIVE → Broken
			// (cluster has been deleted; downstream resources may still
			// reference it). Was Dim — corrected per doc + PR273 contract.
			name:   "inactive",
			fields: map[string]string{"status": "INACTIVE"},
			want:   resource.ColorBroken,
		},
		{
			name:   "provisioning",
			fields: map[string]string{"status": "PROVISIONING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deprovisioning",
			fields: map[string]string{"status": "DEPROVISIONING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed",
			fields: map[string]string{"status": "FAILED"},
			want:   resource.ColorBroken,
		},
		{
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
