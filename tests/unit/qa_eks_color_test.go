package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEksColor(t *testing.T) {
	td := resource.FindResourceType("eks")
	if td == nil {
		t.Fatal("eks not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "active",
			fields: map[string]string{"status": "ACTIVE"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"status": "CREATING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "updating",
			fields: map[string]string{"status": "UPDATING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deleting",
			fields: map[string]string{"status": "DELETING"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed",
			fields: map[string]string{"status": "FAILED"},
			want:   resource.ColorBroken,
		},
		{
			name:   "active_with_issues",
			fields: map[string]string{"status": "ACTIVE", "health_issues_count": "1"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed_with_issues",
			fields: map[string]string{"status": "FAILED", "health_issues_count": "2"},
			want:   resource.ColorBroken,
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
