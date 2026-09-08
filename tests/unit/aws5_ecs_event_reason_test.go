package unit

// aws5_ecs_event_reason_test.go — the ECS service event keeps AWS's own reason.
//
// The enricher matched "unable to place" and reported the fixed phrase "unable
// to place task", throwing the rest of the message away. The rest is the whole
// diagnosis: insufficient capacity, no matching ports, not enough memory. The
// phrase stays a9s's; AWS's sentence becomes a row under it.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// aws5ECSEventRows drives EnrichECSServices over one service carrying one
// recent event and returns the detail rows the deployment finding grew.
func aws5ECSEventRows(t *testing.T, message string) []domain.DetailRow {
	t.Helper()
	const svc = "acme-checkout-svc"
	recent := time.Now().Add(-1 * time.Minute)
	fake := &d3ECSFake{services: []ecstypes.Service{{
		ServiceName:  aws.String(svc),
		Status:       aws.String("ACTIVE"),
		DesiredCount: 2,
		RunningCount: 2,
		Events:       []ecstypes.ServiceEvent{{CreatedAt: &recent, Message: aws.String(message)}},
	}}}

	res, err := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: svc, Name: svc, Type: "ecs-svc",
			Fields: map[string]string{"cluster": "acme-prod", "service_name": svc},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichECSServices: %v", err)
	}
	return res.AttentionDetails[svc][d3CodeECSDeployFailed].Rows
}

// aws5RowValue returns the value of the first row with the given label.
func aws5RowValue(rows []domain.DetailRow, label string) string {
	for _, r := range rows {
		if r.Label == label {
			return r.Value
		}
	}
	return ""
}

// TestECSSvc_PlacementEventKeepsAWSReason pins the row this task is about.
// "unable to place task" alone is a scheduler problem with no cause; the
// operator needs to know whether it is memory, ports or capacity before they
// can act, and only AWS's sentence says which.
func TestECSSvc_PlacementEventKeepsAWSReason(t *testing.T) {
	rows := aws5ECSEventRows(t, "(service acme-checkout-svc) was unable to place a task because "+
		"no container instance met all of its requirements. The closest matching container-instance "+
		"0a1b2c3d has insufficient memory available. For more information, see the Troubleshooting "+
		"section of the Amazon ECS Developer Guide.")

	if got := aws5RowValue(rows, "Event"); got != "unable to place task" {
		t.Errorf("Event row = %q, want %q: the phrase stays the code's", got, "unable to place task")
	}
	reason := aws5RowValue(rows, "Reason")
	if !strings.Contains(reason, "insufficient memory available") {
		t.Errorf("Reason row = %q, want AWS's own explanation of why the task could not be placed", reason)
	}
	if strings.Contains(reason, "For more information") {
		t.Errorf("Reason row = %q: the documentation pointer is not a reason and pushes the cause off the line", reason)
	}
}

// TestECSSvc_ELBEventKeepsAWSReason is the sibling in the same loop: the ELB
// branch threw its message away the same way, and AWS puts the failing health
// check codes in it.
func TestECSSvc_ELBEventKeepsAWSReason(t *testing.T) {
	rows := aws5ECSEventRows(t, "(service acme-checkout-svc) (instance i-0a1b2c3d4e5f60001) (port 8080) "+
		"is unhealthy in (target-group acme-web) due to (reason Health checks failed with these codes: [502]).")

	if got := aws5RowValue(rows, "Event"); got != "ELB health checks failed" {
		t.Errorf("Event row = %q, want %q", got, "ELB health checks failed")
	}
	if reason := aws5RowValue(rows, "Reason"); !strings.Contains(reason, "502") {
		t.Errorf("Reason row = %q, want the codes AWS named", reason)
	}
}

// TestECSSvc_EventWithNoMarkerKeepsItsWholeMessage pins the fallback: AWS
// wording a9s has not seen must not lose its reason the way the fixed phrase
// did. The whole message is the reason when AWS introduces it with nothing.
func TestECSSvc_EventWithNoMarkerKeepsItsWholeMessage(t *testing.T) {
	const msg = "(service acme-checkout-svc) was unable to place a task; the cluster has no registered container instances."
	if reason := aws5RowValue(aws5ECSEventRows(t, msg), "Reason"); reason != msg {
		t.Errorf("Reason row = %q, want the whole message %q", reason, msg)
	}
}

// TestECSSvc_EventWithNothingAfterTheMarkerAddsNoRow pins what a probe found:
// a message that ends at the marker has no reason to show, and an empty row
// renders as a blank line under the phrase.
func TestECSSvc_EventWithNothingAfterTheMarkerAddsNoRow(t *testing.T) {
	rows := aws5ECSEventRows(t, "(service acme-checkout-svc) was unable to place a task because ")
	for _, r := range rows {
		if r.Label == "Reason" {
			t.Errorf("Reason row = %q, want no row at all when AWS wrote no reason after the marker", r.Value)
		}
	}
	if got := aws5RowValue(rows, "Event"); got != "unable to place task" {
		t.Errorf("Event row = %q, want it kept regardless", got)
	}
}
