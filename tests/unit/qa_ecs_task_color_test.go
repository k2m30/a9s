package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEcsTaskColor(t *testing.T) {
	td := resource.FindResourceType("ecs-task")
	if td == nil {
		t.Fatal("ecs-task not registered")
	}

	cases := []struct {
		name         string
		lastStatus   string
		stopCode     string
		healthStatus string
		want         resource.Color
	}{
		{
			name:       "running",
			lastStatus: "RUNNING",
			want:       resource.ColorHealthy,
		},
		{
			name:       "pending",
			lastStatus: "PENDING",
			want:       resource.ColorWarning,
		},
		{
			name:       "provisioning",
			lastStatus: "PROVISIONING",
			want:       resource.ColorWarning,
		},
		{
			name:       "stopping",
			lastStatus: "STOPPING",
			want:       resource.ColorWarning,
		},
		{
			name:       "stopped_user",
			lastStatus: "STOPPED",
			stopCode:   "UserInitiated",
			want:       resource.ColorDim,
		},
		{
			name:       "stopped_failed",
			lastStatus: "STOPPED",
			stopCode:   "TaskFailedToStart",
			want:       resource.ColorBroken,
		},
		{
			name:       "stopped_essential_exit",
			lastStatus: "STOPPED",
			stopCode:   "EssentialContainerExited",
			want:       resource.ColorBroken,
		},
		{
			name:       "stopped_no_code",
			lastStatus: "STOPPED",
			want:       resource.ColorDim,
		},
		{
			name:         "unhealthy_running",
			lastStatus:   "RUNNING",
			healthStatus: "UNHEALTHY",
			want:         resource.ColorBroken,
		},
		{
			name: "empty",
			want: resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			if tc.lastStatus != "" {
				fields["last_status"] = tc.lastStatus
			}
			if tc.stopCode != "" {
				fields["stop_code"] = tc.stopCode
			}
			if tc.healthStatus != "" {
				fields["health_status"] = tc.healthStatus
			}
			got := td.Color(resource.Resource{Fields: fields})
			if got != tc.want {
				t.Errorf("Color(last_status=%q, stop_code=%q, health_status=%q) = %v, want %v",
					tc.lastStatus, tc.stopCode, tc.healthStatus, got, tc.want)
			}
		})
	}
}
