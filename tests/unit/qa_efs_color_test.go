package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEfsColor verifies the EFS Color function reads Resource.Fields["status"]
// (the derived §4 phrase written by the fetcher + Wave-2 enricher), strips any
// (+N) suffix, and maps to the spec's state bucket. Written against the current
// contract: Color is phrase-driven, not field-derived from life_cycle_state.
func TestEfsColor(t *testing.T) {
	td := resource.FindResourceType("efs")
	if td == nil {
		t.Fatal("efs not registered")
	}

	cases := []struct {
		name   string
		status string
		want   resource.Color
	}{
		{name: "healthy_blank", status: "", want: resource.ColorHealthy},
		{name: "creating_warning", status: "creating", want: resource.ColorWarning},
		{name: "updating_warning", status: "updating", want: resource.ColorWarning},
		{name: "deleting_warning", status: "deleting", want: resource.ColorWarning},
		{name: "error_broken", status: "error", want: resource.ColorBroken},
		{name: "no_mount_targets_broken", status: "no mount targets", want: resource.ColorBroken},
		{name: "mount_target_down_broken", status: "mount target down", want: resource.ColorBroken},
		{name: "multi_w1_suffix_stripped", status: "no mount targets (+1)", want: resource.ColorBroken},
		{name: "w1_w2_stack_suffix_stripped", status: "mount target down (+1)", want: resource.ColorBroken},
		{name: "warning_with_suffix", status: "updating (+1)", want: resource.ColorWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				Fields: map[string]string{"status": tc.status},
			}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("Color(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
