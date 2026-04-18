package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEcsSvcColor(t *testing.T) {
	td := resource.FindResourceType("ecs-svc")
	if td == nil {
		t.Fatal("ecs-svc not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "active_balanced",
			fields: map[string]string{"status": "ACTIVE", "running_count": "3", "desired_count": "3"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "draining",
			fields: map[string]string{"status": "DRAINING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "inactive",
			fields: map[string]string{"status": "INACTIVE"},
			want:   resource.ColorBroken,
		},
		{
			name:   "under_capacity",
			fields: map[string]string{"status": "ACTIVE", "running_count": "1", "desired_count": "3"},
			want:   resource.ColorWarning,
		},
		{
			name:   "zero_running",
			fields: map[string]string{"status": "ACTIVE", "running_count": "0", "desired_count": "3"},
			want:   resource.ColorBroken,
		},
		{
			name:   "zero_desired",
			fields: map[string]string{"status": "ACTIVE", "running_count": "0", "desired_count": "0"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "desired_empty",
			fields: map[string]string{"status": "ACTIVE", "desired_count": ""},
			want:   resource.ColorHealthy,
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
