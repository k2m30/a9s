package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestOpensearchColor(t *testing.T) {
	td := resource.FindResourceType("opensearch")
	if td == nil {
		t.Fatal("opensearch not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "green_default",
			fields: map[string]string{"cluster_health": "Green"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "yellow",
			fields: map[string]string{"cluster_health": "Yellow"},
			want:   resource.ColorWarning,
		},
		{
			name:   "red",
			fields: map[string]string{"cluster_health": "Red"},
			want:   resource.ColorBroken,
		},
		{
			name:   "deleted",
			fields: map[string]string{"deleted": "true", "cluster_health": "Red"},
			want:   resource.ColorDim,
		},
		{
			name:   "isolated",
			fields: map[string]string{"domain_processing_status": "Isolated"},
			want:   resource.ColorBroken,
		},
		{
			name:   "processing",
			fields: map[string]string{"processing": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "upgrade_processing",
			fields: map[string]string{"upgrade_processing": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "status_failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "status_creating",
			fields: map[string]string{"status": "creating"},
			want:   resource.ColorWarning,
		},
		{
			name:   "update_available",
			fields: map[string]string{"service_software_update_available": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"encryption_at_rest_enabled": "false"},
			want:   resource.ColorWarning,
		},
		{
			name: "broken_overrides_warning",
			fields: map[string]string{
				"cluster_health":             "Red",
				"encryption_at_rest_enabled": "false",
			},
			want: resource.ColorBroken,
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
				t.Errorf("Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
