package unit

// prowler_w6b_rebase_test.go — the two rebase rows folded into batch w6b.
//
// Row 12: the ecs_tasks child list renders its Health column from
// strings.ToUpper(HealthStatus), so the cell reads HEALTHY and UNKNOWN. Those
// are the SDK's enum spellings, not words an operator uses, and the
// rendered-surface ruling bans them from every surface a9s draws.
//
// Row 11 (the ecs_tasks classifier) and the CodeArtifact policy migration are
// already true on this base; the pin kept here is the one that stays useful
// after the fix, namely that no type in the batch goes back to matching a
// policy document as a string.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w6bECSTaskFakes serves the list/describe pair FetchEcsSvcTasks needs.
type w6bECSTaskListFake struct{ arns []string }

func (f *w6bECSTaskListFake) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: f.arns}, nil
}

type w6bECSTaskDescribeFake struct{ tasks []ecstypes.Task }

func (f *w6bECSTaskDescribeFake) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{Tasks: f.tasks}, nil
}

func w6bECSTask(id string, health ecstypes.HealthStatus, lastStatus string) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + id),
		LastStatus:        aws.String(lastStatus),
		DesiredStatus:     aws.String("RUNNING"),
		HealthStatus:      health,
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-web:7"),
		StartedAt:         aws.Time(time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)),
		LaunchType:        ecstypes.LaunchTypeFargate,
	}
}

func w6bFetchECSTasks(t *testing.T, tasks ...ecstypes.Task) []resource.Resource {
	t.Helper()
	arns := make([]string, 0, len(tasks))
	for _, task := range tasks {
		arns = append(arns, aws.ToString(task.TaskArn))
	}
	out, err := awsclient.FetchEcsSvcTasks(context.Background(),
		&w6bECSTaskListFake{arns: arns}, &w6bECSTaskDescribeFake{tasks: tasks},
		"arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster", "acme-web", "")
	if err != nil {
		t.Fatalf("FetchEcsSvcTasks: %v", err)
	}
	return out.Resources
}

// ─── rebase row 12: the Health column reads in words ────────────────────────

// The Health cell is drawn straight from Fields["health"], so whatever this
// field holds is what an operator reads. HEALTHY is the SDK's spelling of the
// enum, not the word.
func TestW6BECSTasks_HealthColumnIsLowercaseWords(t *testing.T) {
	for _, tc := range []struct {
		status ecstypes.HealthStatus
		want   string
	}{
		{ecstypes.HealthStatusHealthy, "healthy"},
		{ecstypes.HealthStatusUnhealthy, "unhealthy"},
		{ecstypes.HealthStatusUnknown, "unknown"},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			rs := w6bFetchECSTasks(t, w6bECSTask("task"+string(tc.status), tc.status, "RUNNING"))
			r := rs[0]
			if got := r.Fields["health"]; got != tc.want {
				t.Errorf("Fields[health] = %q, want %q", got, tc.want)
			}
		})
	}
}

// A task AWS reports no health for at all has nothing to say, which is not the
// same as reporting UNKNOWN. Lower-casing must not turn the empty string into
// a word.
func TestW6BECSTasks_NoHealthReported_StaysEmpty(t *testing.T) {
	rs := w6bFetchECSTasks(t, w6bECSTask("task-nohealth", "", "RUNNING"))
	if got := rs[0].Fields["health"]; got != "" {
		t.Errorf("Fields[health] = %q for a task with no health status; want the empty string", got)
	}
}

// The class check on row 12. The health string feeds the findings function as
// well as the column, so lower-casing the column without teaching the
// predicate about it silently retires the unhealthy finding — the row would
// read "unhealthy" and colour green.
func TestW6BECSTasks_UnhealthyTaskStillCarriesItsFinding(t *testing.T) {
	rs := w6bFetchECSTasks(t, w6bECSTask("task-sick", ecstypes.HealthStatusUnhealthy, "RUNNING"))
	// Asserted field by field rather than through pw1RequireFinding: this
	// finding predates the batch and carries no Detail sentence, and requiring
	// one here would fail for a reason row 12 is not about.
	f, ok := pw1FindFinding(rs[0].Findings, domain.FindingCode("ecs-task.health.unhealthy"))
	if !ok {
		t.Fatalf("an unhealthy task lost its finding: %+v", rs[0].Findings)
	}
	if f.Phrase != "unhealthy" || f.Severity != domain.SevBroken || f.Source != "wave1" {
		t.Errorf("unhealthy finding = %+v; want phrase %q, severity %v, source %q",
			f, "unhealthy", domain.SevBroken, "wave1")
	}
}

// ─── rebase row 11 / contract rule 6: one policy engine ─────────────────────

// A resource policy is JSON with several legal spellings of the same grant:
// "Principal":"*", {"AWS":"*"}, a list containing "*". Matching the document
// as a string gets exactly one of them right, which is why the contract routes
// every policy through iampolicy and why this stays pinned after the
// CodeArtifact migration rather than being deleted with it.
func TestW6BNoPolicyDocumentStringMatchingInCoreAWS(t *testing.T) {
	root := filepath.Join("..", "..", "core", "aws")
	needles := []string{`Principal":"*"`, `Principal": "*"`}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v", root, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(root, e.Name())
		body, err := os.ReadFile(path) //nolint:gosec // a fixed path under the repo, not user input
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, needle := range needles {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s matches a policy document as a string (%q); route it through iampolicy.Parse + Evaluate",
					path, needle)
			}
		}
	}
}
