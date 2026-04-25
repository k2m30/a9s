package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestDbcSnapColor(t *testing.T) {
	td := resource.FindResourceType("dbc-snap")
	if td == nil {
		t.Fatal("dbc-snap not registered")
	}

	twoYearsAgo := time.Now().AddDate(-2, 0, 0).Format("2006-01-02 15:04")
	recentTime := time.Now().Format("2006-01-02 15:04")

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "available",
			fields: map[string]string{"status": "available", "storage_encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"status": "creating", "storage_encrypted": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"status": "available", "storage_encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name: "manual_old",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": twoYearsAgo,
			},
			want: resource.ColorWarning,
		},
		{
			name: "automated_old",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "automated",
				"snapshot_create_time": twoYearsAgo,
			},
			want: resource.ColorHealthy,
		},
		{
			name: "manual_recent",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": recentTime,
			},
			want: resource.ColorHealthy,
		},
		{
			name:   "broken_overrides_unencrypted",
			fields: map[string]string{"status": "failed", "storage_encrypted": "false"},
			want:   resource.ColorBroken,
		},
		{
			name: "invalid_date_ignored",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": "garbage",
			},
			want: resource.ColorHealthy,
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
