package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestSecretsColor(t *testing.T) {
	td := resource.FindResourceType("secrets")
	if td == nil {
		t.Fatal("secrets not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name: "healthy",
			fields: map[string]string{
				"rotation_enabled": "Yes",
				"last_accessed":    time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
				"last_changed":     time.Now().AddDate(0, 0, -30).Format("2006-01-02"),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "no_rotation",
			fields: map[string]string{
				"rotation_enabled": "No",
				"last_accessed":    time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
				"last_changed":     time.Now().AddDate(0, 0, -30).Format("2006-01-02"),
			},
			want: resource.ColorWarning,
		},
		{
			// A secret nobody has read in 200 days is dormant, and the fetcher
			// says so in the status field: secrets.go:64 sets DORMANT on that
			// exact threshold and that exact field. The case used to omit the
			// status and expect the classifier to re-derive dormancy from
			// last_accessed, which is a row no fetch can produce and a second
			// place for the same rule to live. It carries the status the
			// fetcher writes instead.
			name: "stale_access_is_dormant",
			fields: map[string]string{
				"status":           "DORMANT",
				"rotation_enabled": "Yes",
				"last_accessed":    time.Now().AddDate(0, 0, -200).Format("2006-01-02"),
				"last_changed":     time.Now().AddDate(0, 0, -30).Format("2006-01-02"),
			},
			want: resource.ColorWarning,
		},
		{
			name: "stale_change",
			fields: map[string]string{
				"rotation_enabled": "Yes",
				"last_accessed":    time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
				"last_changed":     time.Now().AddDate(0, 0, -400).Format("2006-01-02"),
			},
			want: resource.ColorWarning,
		},
		{
			name: "recent_access_just_inside_threshold",
			fields: map[string]string{
				"rotation_enabled": "Yes",
				"last_accessed":    time.Now().AddDate(0, 0, -170).Format("2006-01-02"),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "empty_dates_ok",
			fields: map[string]string{
				"rotation_enabled": "Yes",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "invalid_date_ignored",
			fields: map[string]string{
				"rotation_enabled": "Yes",
				"last_accessed":    "not-a-date",
			},
			want: resource.ColorHealthy,
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
