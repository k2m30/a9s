package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRdsSnapColor(t *testing.T) {
	td := resource.FindResourceType("rds-snap")
	if td == nil {
		t.Fatal("rds-snap not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "available",
			fields: map[string]string{"status": "available", "encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"status": "creating", "encrypted": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "copying",
			fields: map[string]string{"status": "copying", "encrypted": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_restore",
			fields: map[string]string{"status": "incompatible-restore"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_parameters",
			fields: map[string]string{"status": "incompatible-parameters"},
			want:   resource.ColorBroken,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"status": "available", "encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "broken_overrides_unencrypted",
			fields: map[string]string{"status": "failed", "encrypted": "false"},
			want:   resource.ColorBroken,
		},
		{
			name:   "empty",
			fields: map[string]string{"status": ""},
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
