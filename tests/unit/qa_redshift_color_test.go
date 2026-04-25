package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRedshiftColor exercises the Redshift Color function directly against
// the raw AWS enums it reads. Post-§4 phrase migration (2026-04-24) the
// Color func reads Fields["cluster_status"] (raw ClusterStatus) instead
// of Fields["status"] (derived §4 phrase). Availability/publicly/encrypted
// keys are unchanged.
func TestRedshiftColor(t *testing.T) {
	td := resource.FindResourceType("redshift")
	if td == nil {
		t.Fatal("redshift not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "available",
			fields: map[string]string{"cluster_status": "available", "encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"cluster_status": "creating"},
			want:   resource.ColorWarning,
		},
		{
			name:   "modifying",
			fields: map[string]string{"cluster_status": "modifying"},
			want:   resource.ColorWarning,
		},
		{
			name:   "resizing",
			fields: map[string]string{"cluster_status": "resizing"},
			want:   resource.ColorWarning,
		},
		{
			name:   "rebooting",
			fields: map[string]string{"cluster_status": "rebooting"},
			want:   resource.ColorWarning,
		},
		{
			name:   "renaming",
			fields: map[string]string{"cluster_status": "renaming"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deleting",
			fields: map[string]string{"cluster_status": "deleting"},
			want:   resource.ColorWarning,
		},
		{
			name:   "incompatible_hsm",
			fields: map[string]string{"cluster_status": "incompatible-hsm"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_network",
			fields: map[string]string{"cluster_status": "incompatible-network"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_parameters",
			fields: map[string]string{"cluster_status": "incompatible-parameters"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_restore",
			fields: map[string]string{"cluster_status": "incompatible-restore"},
			want:   resource.ColorBroken,
		},
		{
			name:   "hardware_failure",
			fields: map[string]string{"cluster_status": "hardware-failure"},
			want:   resource.ColorBroken,
		},
		{
			name:   "storage_full",
			fields: map[string]string{"cluster_status": "storage-full"},
			want:   resource.ColorBroken,
		},
		{
			name:   "availability_unavailable",
			fields: map[string]string{"cluster_status": "available", "cluster_availability_status": "Unavailable"},
			want:   resource.ColorBroken,
		},
		{
			name:   "availability_failed",
			fields: map[string]string{"cluster_status": "available", "cluster_availability_status": "Failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "availability_maintenance",
			fields: map[string]string{"cluster_status": "available", "cluster_availability_status": "Maintenance"},
			want:   resource.ColorWarning,
		},
		{
			name:   "availability_modifying",
			fields: map[string]string{"cluster_status": "available", "cluster_availability_status": "Modifying"},
			want:   resource.ColorWarning,
		},
		{
			name:   "publicly_accessible",
			fields: map[string]string{"cluster_status": "available", "publicly_accessible": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"cluster_status": "available", "encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name: "broken_overrides_warning",
			fields: map[string]string{
				"cluster_status":      "hardware-failure",
				"encrypted":           "false",
				"publicly_accessible": "true",
			},
			want: resource.ColorBroken,
		},
		{
			name:   "empty",
			fields: map[string]string{"cluster_status": ""},
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
