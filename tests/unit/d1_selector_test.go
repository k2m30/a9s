package unit_test

// d1_selector_test.go — one selector decides the colour and the Status phrase,
// for every type, from findings alone.
//
// The colour oracle here is always the maximum severity computed in the test,
// never the selector under test: a test that asks the selector what the answer
// should be cannot catch the selector being wrong.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1BatchTypes are the types whose classifier or phrase picker this batch
// touches: the seven raw-field classifiers, opensearch's own phrase builder,
// the three compute fallbacks, and the two security classifiers.
var d1BatchTypes = map[string]bool{
	"alarm": true, "trail": true, "ct-events": true, "rtb": true,
	"sns-sub": true, "ses": true, "ssm": true,
	"opensearch": true,
	"asg":        true, "ecs-svc": true, "ecs": true,
	"policy": true, "iam-user": true,
}

// d1WorstSeverity is the expected-side oracle: the maximum severity in the
// slice, computed here rather than asked of the code under test.
func d1WorstSeverity(fs []domain.Finding) (domain.Severity, bool) {
	if len(fs) == 0 {
		return 0, false
	}
	worst := fs[0].Severity
	for _, f := range fs[1:] {
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return worst, true
}

// ─── rows 6 and 7: the selector's own rules ─────────────────────────────────

// TestD1_TopFindingNeverShowsHealthyText pins that a severity which is neither
// an issue nor dim cannot win the Status cell. SevOK sits between dim and warn
// in the enum, so a plain maximum puts "healthy" text where the operator
// expects the reason the row is dim.
func TestD1_TopFindingNeverShowsHealthyText(t *testing.T) {
	findings := []domain.Finding{
		{Code: "probe.dim", Phrase: "routine event", Severity: domain.SevDim, Source: "wave1"},
		{Code: "probe.ok", Phrase: "healthy", Severity: domain.SevOK, Source: "wave1"},
	}
	got, ok := domain.TopFinding(findings)
	if !ok {
		t.Fatal("TopFinding reported nothing for a non-empty slice")
	}
	if got.Phrase != "routine event" {
		t.Errorf("Phrase = %q, want the dim phrase; a non-issue, non-dim severity must not win", got.Phrase)
	}
}

// TestD1_StatusPhraseCountsIssuesOnly pins the (+N) suffix: N is the number of
// other issue-severity findings. A dim finding is a state, not a problem, so
// it must not inflate the count of things wrong with the row.
func TestD1_StatusPhraseCountsIssuesOnly(t *testing.T) {
	cases := []struct {
		name     string
		findings []domain.Finding
		want     string
	}{
		{
			name: "one issue and one dim reads as a single problem",
			findings: []domain.Finding{
				{Code: "probe.broken", Phrase: "not logging", Severity: domain.SevBroken, Source: "wave1"},
				{Code: "probe.dim", Phrase: "routine event", Severity: domain.SevDim, Source: "wave1"},
			},
			want: "not logging",
		},
		{
			name: "two issues and one dim counts the other issue only",
			findings: []domain.Finding{
				{Code: "probe.broken", Phrase: "not logging", Severity: domain.SevBroken, Source: "wave1"},
				{Code: "probe.warn", Phrase: "delivery error", Severity: domain.SevWarn, Source: "wave1"},
				{Code: "probe.dim", Phrase: "routine event", Severity: domain.SevDim, Source: "wave1"},
			},
			want: "not logging (+1)",
		},
		{
			name: "dim only still explains why the row is dim",
			findings: []domain.Finding{
				{Code: "probe.dim", Phrase: "routine event", Severity: domain.SevDim, Source: "wave1"},
			},
			want: "routine event",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.StatusPhrase(tc.findings); got != tc.want {
				t.Errorf("StatusPhrase = %q, want %q", got, tc.want)
			}
		})
	}
}

// ─── rows 1, 4 and 5: colour comes from findings on every demo row ──────────

// TestD1_DemoRowColourIsTheWorstFindingSeverity pins that every demo row of
// every batch type is coloured by its own findings and by nothing else. A
// classifier that still reads raw fields — either instead of the findings or
// as a fallback after them — disagrees with this oracle on some row.
func TestD1_DemoRowColourIsTheWorstFindingSeverity(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if !d1BatchTypes[td.ShortName] {
			continue
		}
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			t.Errorf("%s: no demo fixtures, so this batch's colour rule is unwitnessed", td.ShortName)
			continue
		}
		for _, res := range mergeWave2Findings(t, td, rows, cache, clients) {
			worst, ok := d1WorstSeverity(res.Findings)
			if !ok {
				continue // no findings: the type's healthy default, not this rule
			}
			want := resource.ColorFromSeverity(worst)
			if got := td.ResolveColor(res); got != want {
				t.Errorf("%s/%s: colour %v, want %v from its worst finding (%v); findings=%+v",
					td.ShortName, res.ID, got, want, worst, res.Findings)
			}
		}
	}
}

// TestD1_EveryDemoRowColourIsExplainedByAFinding pins the other direction: a
// row that is not healthy must say why. A classifier that colours from a raw
// field leaves a coloured row with nothing in its Status cell.
func TestD1_EveryDemoRowColourIsExplainedByAFinding(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if !d1BatchTypes[td.ShortName] {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, byType[td.ShortName], cache, clients) {
			if td.ResolveColor(res) == domain.ColorHealthy {
				continue
			}
			if len(res.Findings) == 0 {
				t.Errorf("%s/%s: coloured but carries no finding, so its Status cell is empty",
					td.ShortName, res.ID)
			}
		}
	}
}

// ─── row 1: the phrases the seven classifiers' branches were computing ──────

// d1ExpectedPhrases are the phrases each type's own fetcher emits on these
// fields. Pinned unchanged: a phrase moving here means an emitter was
// rewritten.
var d1ExpectedPhrases = map[string][]string{
	"alarm":     {"alarm triggered", "insufficient data", "no actions"},
	"trail":     {"not logging", "log file validation disabled"},
	"ct-events": {"destructive call", "root account activity", "routine event"},
	"rtb":       {"blackhole route (target deleted)", "no subnet associations"},
	"sns-sub":   {"endpoint has not confirmed the subscription", "endpoint deleted"},
	"ses": {
		"verification failed", "verify: temp failure", "verification not started",
		"pending verification", "sending disabled",
	},
	"ssm": {"plaintext value looks like a credential", "not modified in over 365 days"},
}

// TestD1_ClassifierPhrasesAreStillDeclared pins each phrase on its type's
// catalog literal, so deleting a branch cannot quietly take its wording with it.
func TestD1_ClassifierPhrasesAreStillDeclared(t *testing.T) {
	for short, phrases := range d1ExpectedPhrases {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Errorf("%s: not in the catalog", short)
			continue
		}
		declared := make(map[string]bool, len(td.Findings))
		for _, fd := range td.Findings {
			declared[fd.Phrase] = true
		}
		for _, p := range phrases {
			if !declared[p] {
				t.Errorf("%s: no FindingDef declares the phrase %q that its classifier branch was computing", short, p)
			}
		}
	}
}

// ─── row 2: the two private phrase pickers ──────────────────────────────────

// TestD1_OpenSearchStatusCountsEverySignalOnce pins that the background-check
// signals are findings on the row, not a separate counter. While they are
// counted outside the slice, the "(+N)" in the Status cell and the number of
// findings the detail view shows disagree.
func TestD1_OpenSearchStatusCountsEverySignalOnce(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	rows := byType["opensearch"]
	if len(rows) == 0 {
		t.Fatal("no opensearch demo fixtures")
	}
	for _, res := range rows {
		want := domain.StatusPhrase(res.Findings)
		if got := res.Fields["status"]; got != want {
			t.Errorf("opensearch/%s: Fields[status]=%q but its findings say %q; the phrase is built twice",
				res.ID, got, want)
		}
	}
}

// TestD1_SESStatusIsTheWorstFindingsPhrase pins the same for ses, whose own
// picker takes findings[0]. The quota signal is expected to be expressed as a
// severity rather than as an exception inside a selector.
func TestD1_SESStatusIsTheWorstFindingsPhrase(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	rows := byType["ses"]
	if len(rows) == 0 {
		t.Fatal("no ses demo fixtures")
	}
	for _, res := range rows {
		want := domain.StatusPhrase(res.Findings)
		if got := res.Fields["status"]; got != want {
			t.Errorf("ses/%s: Fields[status]=%q but its findings say %q", res.ID, got, want)
		}
	}
}

// ─── row 3: the ecs-svc child rows carry the parent's findings ──────────────

// d1ECSFake serves the four ECS calls both the ecs-task fetcher and the
// ecs-svc child fetcher make, over one fixed task list. The demo fake filters
// ListTasks by cluster ARN while a service row carries the cluster name, so
// the two views never meet there; a local fake makes the comparison possible.
type d1ECSFake struct {
	awsclient.ECSAPI
	tasks []ecstypes.Task
}

const d1ClusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster"

func (f *d1ECSFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{d1ClusterARN}}, nil
}

func (f *d1ECSFake) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	var arns []string
	for _, task := range f.tasks {
		// The child fetcher asks for RUNNING and STOPPED separately.
		if in.DesiredStatus != "" && string(in.DesiredStatus) != aws.ToString(task.DesiredStatus) {
			continue
		}
		arns = append(arns, aws.ToString(task.TaskArn))
	}
	return &ecs.ListTasksOutput{TaskArns: arns}, nil
}

func (f *d1ECSFake) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	want := map[string]bool{}
	for _, a := range in.Tasks {
		want[a] = true
	}
	var out []ecstypes.Task
	for _, task := range f.tasks {
		if len(want) == 0 || want[aws.ToString(task.TaskArn)] {
			out = append(out, task)
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: out}, nil
}

func (f *d1ECSFake) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
		TaskDefinitionArn: in.TaskDefinition,
		Family:            aws.String("acme-app"),
		Revision:          7,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		ContainerDefinitions: []ecstypes.ContainerDefinition{{
			Name:                   aws.String("app"),
			Image:                  aws.String("acme/app:1.0"),
			Privileged:             aws.Bool(false),
			ReadonlyRootFilesystem: aws.Bool(true),
			LogConfiguration:       &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs},
		}},
	}}, nil
}

// d1Task builds one task in the shape DescribeTasks returns.
func d1Task(id, lastStatus, desiredStatus, health string, stopCode ecstypes.TaskStopCode) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + id),
		ClusterArn:        aws.String(d1ClusterARN),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-app:7"),
		LastStatus:        aws.String(lastStatus),
		DesiredStatus:     aws.String(desiredStatus),
		HealthStatus:      ecstypes.HealthStatus(health),
		StopCode:          stopCode,
		LaunchType:        ecstypes.LaunchTypeFargate,
	}
}

// TestD1_ChildTaskRowsCarryTheSameFindingsAsTheParentType pins that a task
// reached through its service shows what the same task shows in the ecs-task
// list. The child fetcher computes only the six transitional states, so an
// unhealthy task and a crash-stopped one are silent in the child view while
// the parent type reports both.
func TestD1_ChildTaskRowsCarryTheSameFindingsAsTheParentType(t *testing.T) {
	fake := &d1ECSFake{tasks: []ecstypes.Task{
		d1Task("aaaa1111bbbb2222cccc3333dddd4444", "RUNNING", "RUNNING", "UNHEALTHY", ""),
		d1Task("bbbb2222cccc3333dddd4444eeee5555", "STOPPED", "STOPPED", "UNKNOWN", ecstypes.TaskStopCodeTaskFailedToStart),
	}}
	clients := &awsclient.ServiceClients{ECS: fake}

	parentTD := resource.FindResourceType("ecs-task")
	if parentTD == nil || parentTD.Fetcher == nil {
		t.Fatal("ecs-task has no catalog Fetcher")
	}
	parentPage, err := parentTD.Fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("ecs-task fetch: %v", err)
	}
	parentByARN := map[string][]domain.Finding{}
	for _, r := range parentPage.Resources {
		parentByARN[r.Fields["arn"]] = r.Findings
	}

	child, err := awsclient.FetchEcsSvcTasks(context.Background(), fake, fake, d1ClusterARN, "acme-app", "")
	if err != nil {
		t.Fatalf("child fetch: %v", err)
	}
	if len(child.Resources) != len(fake.tasks) {
		t.Fatalf("child view returned %d rows for %d tasks", len(child.Resources), len(fake.tasks))
	}

	for _, row := range child.Resources {
		want, ok := parentByARN[row.Fields["task_arn"]]
		if !ok {
			t.Errorf("task %s is in the child view but not in the ecs-task list", row.ID)
			continue
		}
		if !d1SameCodes(row.Findings, want) {
			t.Errorf("task %s: child view carries %v, ecs-task carries %v",
				row.ID, d1Codes(row.Findings), d1Codes(want))
		}
	}
}

func d1Codes(fs []domain.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f.Code))
	}
	return out
}

// d1SameCodes compares two finding slices as multisets of codes.
func d1SameCodes(a, b []domain.Finding) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[domain.FindingCode]int{}
	for _, f := range a {
		seen[f.Code]++
	}
	for _, f := range b {
		seen[f.Code]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// ─── standard batch test: no supporting row restates its phrase ─────────────

// TestD1_DetailAttentionNeverRepeatsItself is the per-batch U11 sweep over
// every demo row of every type this batch touches.
func TestD1_DetailAttentionNeverRepeatsItself(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if !d1BatchTypes[td.ShortName] {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, byType[td.ShortName], cache, clients) {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := normalizeRowText(f.Phrase)
				for _, row := range ad.Rows {
					if normalizeRowText(row.Label+" "+row.Value) == phrase ||
						normalizeRowText(row.Value) == phrase {
						t.Errorf("%s/%s %s: row %q: %q adds nothing to the phrase %q",
							td.ShortName, res.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
				}
			}
		}
	}
}
