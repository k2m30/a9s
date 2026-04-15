package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEfsColor(t *testing.T) {
	td := resource.FindResourceType("efs")
	if td == nil {
		t.Fatal("efs not registered")
	}

	cases := []struct {
		name           string
		lifeCycleState string
		mountTargets   string
		want           resource.Color
	}{
		{
			name:           "available",
			lifeCycleState: "available",
			mountTargets:   "2",
			want:           resource.ColorHealthy,
		},
		{
			name:           "no_mount_targets",
			lifeCycleState: "available",
			mountTargets:   "0",
			want:           resource.ColorBroken,
		},
		{
			name:           "creating",
			lifeCycleState: "creating",
			mountTargets:   "2",
			want:           resource.ColorWarning,
		},
		{
			name:           "updating",
			lifeCycleState: "updating",
			mountTargets:   "2",
			want:           resource.ColorWarning,
		},
		{
			name:           "deleting",
			lifeCycleState: "deleting",
			mountTargets:   "2",
			want:           resource.ColorWarning,
		},
		{
			name:           "error",
			lifeCycleState: "error",
			mountTargets:   "2",
			want:           resource.ColorBroken,
		},
		{
			name:           "broken_overrides",
			lifeCycleState: "error",
			mountTargets:   "0",
			want:           resource.ColorBroken,
		},
		{
			name:           "empty",
			lifeCycleState: "",
			mountTargets:   "",
			want:           resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				Fields: map[string]string{
					"life_cycle_state": tc.lifeCycleState,
					"mount_targets":    tc.mountTargets,
				},
			}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("Color(life_cycle_state=%q, mount_targets=%q) = %v, want %v",
					tc.lifeCycleState, tc.mountTargets, got, tc.want)
			}
		})
	}
}
