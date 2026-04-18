package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestAlarmColor(t *testing.T) {
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatal("alarm not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "ok_with_actions",
			fields: map[string]string{"state": "OK", "actions_count": "2"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "ok_no_actions",
			fields: map[string]string{"state": "OK", "actions_count": "0"},
			want:   resource.ColorWarning,
		},
		{
			name:   "ok_missing_actions_treated_as_zero",
			fields: map[string]string{"state": "OK"},
			want:   resource.ColorWarning,
		},
		{
			name:   "alarm_state",
			fields: map[string]string{"state": "ALARM", "actions_count": "2"},
			want:   resource.ColorBroken,
		},
		{
			name:   "alarm_overrides_no_actions",
			fields: map[string]string{"state": "ALARM", "actions_count": "0"},
			want:   resource.ColorBroken,
		},
		{
			name:   "insufficient_data",
			fields: map[string]string{"state": "INSUFFICIENT_DATA", "actions_count": "2"},
			want:   resource.ColorWarning,
		},
		{
			name:   "unknown",
			fields: map[string]string{"state": ""},
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
