package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestAmiColor_StateAndDeprecation(t *testing.T) {
	td := resource.FindResourceType("ami")
	if td == nil {
		t.Fatal("ami not registered")
	}

	pastYear := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	futureYear := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		// State-based cases.
		{
			name:   "state=available",
			fields: map[string]string{"state": "available"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=pending",
			fields: map[string]string{"state": "pending"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=transient",
			fields: map[string]string{"state": "transient"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=failed",
			fields: map[string]string{"state": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=error",
			fields: map[string]string{"state": "error"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=invalid",
			fields: map[string]string{"state": "invalid"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=deregistered",
			fields: map[string]string{"state": "deregistered"},
			want:   resource.ColorDim,
		},
		{
			name:   "state=disabled",
			fields: map[string]string{"state": "disabled"},
			want:   resource.ColorDim,
		},
		// Deprecation cases — only apply when state=available.
		{
			name:   "state=available+deprecation_in_past",
			fields: map[string]string{"state": "available", "deprecation_time": pastYear},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=available+deprecation_in_future",
			fields: map[string]string{"state": "available", "deprecation_time": futureYear},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=available+deprecation_time_empty",
			fields: map[string]string{"state": "available", "deprecation_time": ""},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=available+deprecation_time_invalid",
			fields: map[string]string{"state": "available", "deprecation_time": "not-a-date"},
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
