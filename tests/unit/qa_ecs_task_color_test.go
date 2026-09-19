package unit

// Color contract pin for ECS Tasks.
//
// colorECSTask (core/aws/catalog_compute.go) prefers colorFromAnyFinding,
// falling back to a raw last_status switch; stop_code and health_status
// reach the colour only through Findings (ecsTaskStructuralFindings,
// core/aws/ecs_task_codes.go), so cases exercising them attach the matching
// Finding.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
		findings     []domain.Finding
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
			findings: []domain.Finding{
				{Code: "ecs-task.stop-code.failed", Phrase: "stopped: TaskFailedToStart", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:       "stopped_essential_exit",
			lastStatus: "STOPPED",
			stopCode:   "EssentialContainerExited",
			findings: []domain.Finding{
				{Code: "ecs-task.stop-code.failed", Phrase: "stopped: EssentialContainerExited", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
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
			findings: []domain.Finding{
				{Code: "ecs-task.health.unhealthy", Phrase: "unhealthy", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name: "empty",
			want: resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// "status", not "last_status": the classifier reads the one name the
			// column persists.
			fields := map[string]string{}
			if tc.lastStatus != "" {
				fields["status"] = tc.lastStatus
			}
			if tc.stopCode != "" {
				fields["stop_code"] = tc.stopCode
			}
			if tc.healthStatus != "" {
				fields["health_status"] = tc.healthStatus
			}
			got := td.Color(resource.Resource{Fields: fields, Findings: tc.findings})
			if got != tc.want {
				t.Errorf("Color(status=%q, stop_code=%q, health_status=%q, findings=%v) = %v, want %v",
					tc.lastStatus, tc.stopCode, tc.healthStatus, tc.findings, got, tc.want)
			}
		})
	}
}
