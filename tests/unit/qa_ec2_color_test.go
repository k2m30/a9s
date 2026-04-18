package unit

// qa_ec2_color_test.go — Color contract pin for EC2 Instances.
//
// Each case is non-tautological: fields are set to realistic values that a
// real fetcher would produce. The expected colors are read from the production
// Color func in internal/resource/types_compute.go.
//
// Skipped cases (not in production Color func):
//   - state_reason_code / state_transition_reason parsing — production Color
//     does not read these fields; would be a tautology against a non-existent path.
//   - long-stopped >30d — production Color has no time-based stopped logic.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEc2Color(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			// Healthy running instance with ok status checks.
			name:   "running",
			fields: map[string]string{"state": "running", "system_status": "ok", "instance_status": "ok"},
			want:   resource.ColorHealthy,
		},
		{
			// Pending instance — transitioning state.
			name:   "pending",
			fields: map[string]string{"state": "pending"},
			want:   resource.ColorWarning,
		},
		{
			// Stopping instance — transitional, not broken.
			name:   "stopping",
			fields: map[string]string{"state": "stopping"},
			want:   resource.ColorWarning,
		},
		{
			// Shutting-down instance — transitional, not terminal.
			name:   "shutting_down",
			fields: map[string]string{"state": "shutting-down"},
			want:   resource.ColorWarning,
		},
		{
			// Stopped instance — user-initiated stop. Warning, not Broken
			// (intentional shutdown should not fire an alert).
			name:   "stopped_intentional",
			fields: map[string]string{"state": "stopped"},
			want:   resource.ColorWarning,
		},
		{
			// Stopped instance with Server.* reason code — AWS forced the stop
			// (capacity issue). This is unexpected and warrants Broken.
			name: "stopped_capacity",
			fields: map[string]string{
				"state":             "stopped",
				"state_reason_code": "Server.InsufficientInstanceCapacity",
			},
			want: resource.ColorBroken,
		},
		{
			// Terminated instance — end-of-life state.
			name:   "terminated",
			fields: map[string]string{"state": "terminated"},
			want:   resource.ColorDim,
		},
		{
			// Running instance with impaired instance status check (Wave 2 enricher
			// sets Fields["instance_status"]="impaired"). Must override state color.
			name: "impaired_via_enricher",
			fields: map[string]string{
				"state":           "running",
				"instance_status": "impaired",
				"system_status":   "ok",
			},
			want: resource.ColorBroken,
		},
		{
			// Running instance with impaired system status check (Wave 2 enricher
			// sets Fields["system_status"]="impaired"). Must override state color.
			name: "system_impaired",
			fields: map[string]string{
				"state":           "running",
				"system_status":   "impaired",
				"instance_status": "ok",
			},
			want: resource.ColorBroken,
		},
		{
			// Running instance with initializing status checks — transitional, not broken.
			name: "initializing_status",
			fields: map[string]string{
				"state":           "running",
				"system_status":   "initializing",
				"instance_status": "initializing",
			},
			want: resource.ColorWarning,
		},
		{
			// Empty state field — treated as "running" per production fallback.
			name:   "empty_state",
			fields: map[string]string{"state": ""},
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
