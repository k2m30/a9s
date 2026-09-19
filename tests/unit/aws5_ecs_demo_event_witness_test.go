package unit

// The demo carries both event branches of the ecs-svc deployment finding, each
// on a service that already carries the finding through the stuck-tasks
// branch, so no row changes colour and no count moves.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// aws5ECSDemoRows drives the real enricher over the demo ECS fake, exactly as
// demo mode does, and returns the rows of the deployment finding per service.
func aws5ECSDemoRows(t *testing.T) map[string][]domain.DetailRow {
	t.Helper()
	fix := fixtures.NewECSFixtures()
	rows := make([]resource.Resource, 0, len(fix.Services))
	for _, s := range fix.Services {
		name := aws.ToString(s.ServiceName)
		rows = append(rows, resource.Resource{
			ID: name, Name: name, Type: "ecs-svc",
			Fields: map[string]string{"cluster": "acme-services", "service_name": name},
		})
	}
	res, err := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fakes.NewECS(), Region: "us-east-1"}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichECSServices over the demo fixtures: %v", err)
	}
	out := make(map[string][]domain.DetailRow, len(res.AttentionDetails))
	for svc, byCode := range res.AttentionDetails {
		out[svc] = byCode[d3CodeECSDeployFailed].Rows
	}
	return out
}

// The demo service with no tasks running says why it has none.
func TestECSDemo_PlacementWitnessRendersItsReason(t *testing.T) {
	rows := aws5ECSDemoRows(t)[fixtures.ECSServiceNoTasksRunning]
	if !aws5HasRow(rows, "Event", "unable to place task") {
		t.Fatalf("%s rows = %v, want the placement event", fixtures.ECSServiceNoTasksRunning, rows)
	}
	if reason := aws5RowValue(rows, "Reason"); !strings.Contains(reason, "memory") {
		t.Errorf("%s Reason row = %q, want AWS's own explanation", fixtures.ECSServiceNoTasksRunning, reason)
	}
}

// The demo service running below its desired count renders the ELB
// health-check reason.
func TestECSDemo_HealthCheckWitnessRendersItsReason(t *testing.T) {
	rows := aws5ECSDemoRows(t)[fixtures.ECSServiceBelowDesiredCount]
	if !aws5HasRow(rows, "Event", "load balancer health checks failed") {
		t.Fatalf("%s rows = %v, want the health-check event", fixtures.ECSServiceBelowDesiredCount, rows)
	}
	if reason := aws5RowValue(rows, "Reason"); !strings.Contains(reason, "502") {
		t.Errorf("%s Reason row = %q, want the codes AWS reported", fixtures.ECSServiceBelowDesiredCount, reason)
	}
}

// Every other demo service is event-free, so a later branch cannot trip on a
// row that was never meant to prove it.
func TestECSDemo_OnlyTheTwoWitnessesCarryEvents(t *testing.T) {
	for _, s := range fixtures.NewECSFixtures().Services {
		name := aws.ToString(s.ServiceName)
		switch name {
		case fixtures.ECSServiceNoTasksRunning, fixtures.ECSServiceBelowDesiredCount:
			if len(s.Events) == 0 {
				t.Errorf("%s carries no events; it is this branch's witness", name)
			}
		default:
			if len(s.Events) != 0 {
				t.Errorf("%s carries %d event(s); only the two witnesses do", name, len(s.Events))
			}
		}
	}
}
