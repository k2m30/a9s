package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEbsSnapColor(t *testing.T) {
	td := resource.FindResourceType("ebs-snap")
	if td == nil {
		t.Fatal("ebs-snap not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "state_completed",
			fields: map[string]string{"state": "completed"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state_pending",
			fields: map[string]string{"state": "pending"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state_error",
			fields: map[string]string{"state": "error"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state_recoverable",
			fields: map[string]string{"state": "recoverable"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state_recovering",
			fields: map[string]string{"state": "recovering"},
			want:   resource.ColorBroken,
		},
		{
			name:   "encrypted_false",
			fields: map[string]string{"state": "completed", "encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "encrypted_true",
			fields: map[string]string{"state": "completed", "encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name: "automated_old",
			fields: map[string]string{
				"state":       "completed",
				"description": "Created by CreateImage(i-abc)",
				"started":     time.Now().AddDate(-2, 0, 0).Format(time.RFC3339),
			},
			want: resource.ColorWarning,
		},
		{
			name: "automated_recent",
			fields: map[string]string{
				"state":       "completed",
				"description": "Created by CreateImage(i-abc)",
				"started":     time.Now().Format(time.RFC3339),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "manual_old",
			fields: map[string]string{
				"state":       "completed",
				"description": "manual snap",
				"started":     time.Now().AddDate(-2, 0, 0).Format(time.RFC3339),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "orphan_flag",
			fields: map[string]string{
				"state":         "completed",
				"volume_id":     "vol-deleted",
				"volume_orphan": "true",
			},
			want: resource.ColorWarning,
		},
		{
			name: "broken_overrides_warning",
			fields: map[string]string{
				"state":     "error",
				"encrypted": "false",
			},
			want: resource.ColorBroken,
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
