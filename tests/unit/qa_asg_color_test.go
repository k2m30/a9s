package unit

// qa_asg_color_test.go — Color contract pin for Auto Scaling Groups.
//
// Each case is non-tautological: fields are set to realistic values that a
// real fetcher would produce. The expected colors are read from the production
// Color func in internal/resource/types_compute.go.
//
// Skipped cases (not in production Color func):
//   - in_service / instances / suspended_processes fields — production Color
//     only reads Fields["status"]; those fields are not consulted.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestAsgColor(t *testing.T) {
	td := resource.FindResourceType("asg")
	if td == nil {
		t.Fatal("asg not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			// Empty status is the normal operating state for an ASG.
			name:   "empty_status",
			fields: map[string]string{"status": ""},
			want:   resource.ColorHealthy,
		},
		{
			// "Delete in progress" is set by AWS when the ASG is being deleted.
			// Production Color returns ColorWarning (transitional, noteworthy).
			name:   "delete_in_progress",
			fields: map[string]string{"status": "Delete in progress"},
			want:   resource.ColorWarning,
		},
		{
			// Unknown status value — production Color falls through to the default
			// return of ColorHealthy (no fallbackColor call for ASG).
			name:   "unknown_status",
			fields: map[string]string{"status": "some-unknown-state"},
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
