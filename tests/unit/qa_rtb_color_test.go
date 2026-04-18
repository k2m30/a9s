package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRtbColor(t *testing.T) {
	td := resource.FindResourceType("rtb")
	if td == nil {
		t.Fatal("rtb not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "healthy_main",
			fields: map[string]string{"blackhole_routes_count": "0", "associations_count": "2", "is_main": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "healthy_associated",
			fields: map[string]string{"blackhole_routes_count": "0", "associations_count": "2", "is_main": "false"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "blackhole",
			fields: map[string]string{"blackhole_routes_count": "1", "associations_count": "2", "is_main": "false"},
			want:   resource.ColorBroken,
		},
		{
			name:   "orphan",
			fields: map[string]string{"blackhole_routes_count": "0", "associations_count": "0", "is_main": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "empty_main_ok",
			fields: map[string]string{"blackhole_routes_count": "0", "associations_count": "0", "is_main": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "broken_overrides_orphan",
			fields: map[string]string{"blackhole_routes_count": "2", "associations_count": "0", "is_main": "false"},
			want:   resource.ColorBroken,
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
