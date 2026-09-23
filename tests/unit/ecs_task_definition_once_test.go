package unit_test

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// onceECSFake answers DescribeTaskDefinition from defs and records the ARN of
// every call that reaches it, so a test can count the reads one detail open
// costs. err, when set, is returned for every ARN.
type onceECSFake struct {
	fakeECSForSvcPivots
	defs map[string]*ecstypes.TaskDefinition
	err  error

	mu    sync.Mutex
	calls map[string]int
}

func (f *onceECSFake) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	arn := aws.ToString(in.TaskDefinition)
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[arn]++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: f.defs[arn]}, nil
}

func (f *onceECSFake) callsPerARN() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.calls))
	for k, v := range f.calls {
		out[k] = v
	}
	return out
}

func newOnceECSFake(def *ecstypes.TaskDefinition, err error) *onceECSFake {
	return &onceECSFake{
		defs: map[string]*ecstypes.TaskDefinition{aws.ToString(def.TaskDefinitionArn): def},
		err:  err,
	}
}

// onceECSCache holds the rows the service's and task's pivots join against.
// Every other target type gets a present, empty entry: an account holding
// none of that type is a loaded, complete answer, and it keeps a row from
// falling through to a live fetch no session was opened for.
func onceECSCache(shortNames ...string) resource.ResourceCache {
	cache := resource.ResourceCache{}
	for _, shortName := range shortNames {
		for _, def := range resource.GetRelated(shortName) {
			cache[def.TargetType] = resource.ResourceCacheEntry{Resources: []resource.Resource{}}
		}
	}
	joined := resource.ResourceCache{
		"logs": {Resources: claimLogGroups("/ecs/acme-api", "/ecs/acme-api-sidecar", "/ecs/acme-api-worker")},
		"ecr": {Resources: []resource.Resource{{
			ID: "acme-api", Name: "acme-api",
			Fields: map[string]string{"repository_name": "acme-api", "uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-api"},
		}}},
		"secrets": {Resources: []resource.Resource{{ID: "acme/api/db", Name: "acme/api/db", Fields: map[string]string{}}}},
		"role":    {Resources: []resource.Resource{{ID: "acme-api-task", Name: "acme-api-task", Fields: map[string]string{}}}},
	}
	for target, entry := range joined {
		cache[target] = entry
	}
	return cache
}

// onceRelatedAnswers runs the real fan-out of one detail open: every related
// def registered for shortName, in registration order, against one set of
// clients and one operation. It returns each row's answer keyed by its
// target type.
func onceRelatedAnswers(ctx context.Context, shortName string, clients *awsclient.ServiceClients, src resource.Resource, cache resource.ResourceCache) map[string]resource.RelatedCheckResult {
	out := map[string]resource.RelatedCheckResult{}
	for _, def := range resource.GetRelated(shortName) {
		out[def.TargetType] = def.Checker(ctx, clients, src, cache)
	}
	return out
}

// onceAssertSameAnswers reports every row whose ids or coverage changed
// between two fan-outs of the same open.
func onceAssertSameAnswers(t *testing.T, label string, before, after map[string]resource.RelatedCheckResult) {
	t.Helper()
	for target, b := range before {
		a, ok := after[target]
		if !ok {
			t.Errorf("%s: the %s row is missing from the second fan-out", label, target)
			continue
		}
		assertSameIDs(t, label+" -> "+target, a.ResourceIDs(), b.ResourceIDs())
		if a.Coverage() != b.Coverage() {
			t.Errorf("%s -> %s: coverage = %v, want %v", label, target, a.Coverage(), b.Coverage())
		}
	}
}

const onceTaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-api:7"

func onceTaskDefinition() *ecstypes.TaskDefinition {
	return &ecstypes.TaskDefinition{
		Family:            aws.String("acme-api"),
		Revision:          7,
		TaskDefinitionArn: aws.String(onceTaskDefARN),
		TaskRoleArn:       aws.String("arn:aws:iam::123456789012:role/acme-api-task"),
		ExecutionRoleArn:  aws.String("arn:aws:iam::123456789012:role/acme-api-exec"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{{
			Name:  aws.String("app"),
			Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-api:v1.4.2"),
			LogConfiguration: &ecstypes.LogConfiguration{
				LogDriver: ecstypes.LogDriverAwslogs,
				Options:   map[string]string{"awslogs-group": "/ecs/acme-api", "awslogs-region": "us-east-1"},
			},
			Secrets: []ecstypes.Secret{{
				Name:      aws.String("DB_PASSWORD"),
				ValueFrom: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/api/db-AbCdEf"),
			}},
		}},
	}
}

func onceECSTaskRow() resource.Resource {
	return resource.Resource{
		ID:   "1a2b3c4d5e6f708192a3b4c5d6e7f809",
		Name: "1a2b3c4d5e6f708192a3b4c5d6e7f809",
		Fields: map[string]string{
			"task_id":         "1a2b3c4d5e6f708192a3b4c5d6e7f809",
			"cluster":         "acme-prod",
			"task_definition": onceTaskDefARN,
			"status":          "RUNNING",
			"task_role":       "arn:aws:iam::123456789012:role/acme-api-task",
			"execution_role":  "arn:aws:iam::123456789012:role/acme-api-exec",
		},
		RawStruct: ecstypes.Task{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-prod/1a2b3c4d5e6f708192a3b4c5d6e7f809"),
			ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"),
			TaskDefinitionArn: aws.String(onceTaskDefARN),
			LastStatus:        aws.String("RUNNING"),
			LaunchType:        ecstypes.LaunchTypeFargate,
		},
	}
}

// TestECSDetailOpenReadsOneTaskDefinitionOnce pins the cost of opening an ECS
// service's or task's detail. Several of its rows describe the same thing —
// the images, the log groups and the secrets all live in the one task
// definition the workload runs — and a task definition revision is immutable,
// so reading it a second time within one open returns the same bytes for
// another ecs:DescribeTaskDefinition against the account's quota. One open
// reads each distinct definition once, and every row answers exactly what it
// answers when it reads alone.
func TestECSDetailOpenReadsOneTaskDefinitionOnce(t *testing.T) {
	cache := onceECSCache("ecs-svc", "ecs-task")

	for _, tc := range []struct {
		shortName string
		src       resource.Resource
	}{
		{"ecs-svc", ecsSvcWithTaskDef("acme-api-svc", onceTaskDefARN)},
		{"ecs-task", onceECSTaskRow()},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			alone := newOnceECSFake(onceTaskDefinition(), nil)
			baseline := onceRelatedAnswers(context.Background(), tc.shortName,
				&awsclient.ServiceClients{ECS: alone, Region: "us-east-1"}, tc.src, cache)

			shared := newOnceECSFake(onceTaskDefinition(), nil)
			clients := &awsclient.ServiceClients{ECS: awsclient.NewCoalescingECS(shared), Region: "us-east-1"}
			ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(11))
			answers := onceRelatedAnswers(ctx, tc.shortName, clients, tc.src, cache)

			if enricher := resource.GetDetailEnricher(tc.shortName); enricher != nil {
				dctx := &awsclient.DetailEnrichmentCtx{Clients: clients, OpID: domain.Gen(11)}
				_, _ = enricher(ctx, dctx, tc.src) //nolint:errcheck // the enricher's own errors are another row's contract; this test counts calls
			}

			for arn, n := range shared.callsPerARN() {
				if n != 1 {
					t.Errorf("%s detail open read %s %d times, want 1: every row of one open shares the definition the workload runs", tc.shortName, arn, n)
				}
			}
			if got := len(shared.callsPerARN()); got != 1 {
				t.Errorf("%s detail open described %d distinct task definitions, want 1: %v", tc.shortName, got, shared.callsPerARN())
			}
			onceAssertSameAnswers(t, tc.shortName, baseline, answers)
		})
	}
}

// TestECSDetailOpenCostsOneRefusedReadWhateverTheRowCount pins what a
// definition the account refuses costs. A refusal is AWS's answer about that
// definition, the same however often it is asked, so one open asks once and
// every row reads the same answer; two rows of a screen cannot disagree about
// what the workload runs. A throttle is no answer about the definition and is
// asked again by the next row.
func TestECSDetailOpenCostsOneRefusedReadWhateverTheRowCount(t *testing.T) {
	refusal := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: ecs:DescribeTaskDefinition", Fault: smithy.FaultClient}
	cache := onceECSCache("ecs-svc", "ecs-task")

	for _, tc := range []struct {
		shortName string
		src       resource.Resource
	}{
		{"ecs-svc", ecsSvcWithTaskDef("acme-api-svc", onceTaskDefARN)},
		{"ecs-task", onceECSTaskRow()},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			alone := newOnceECSFake(onceTaskDefinition(), refusal)
			aloneClients := &awsclient.ServiceClients{ECS: awsclient.NewCoalescingECS(alone), Region: "us-east-1"}
			checkerByTarget(t, tc.shortName, "logs")(
				awsclient.WithDetailOp(context.Background(), domain.Gen(12)), aloneClients, tc.src, cache)
			oneRow := alone.callsPerARN()[onceTaskDefARN]
			if oneRow == 0 {
				t.Fatalf("%s: the logs row did not reach DescribeTaskDefinition, so there is no single-row cost to compare against", tc.shortName)
			}

			whole := newOnceECSFake(onceTaskDefinition(), refusal)
			wholeClients := &awsclient.ServiceClients{ECS: awsclient.NewCoalescingECS(whole), Region: "us-east-1"}
			onceRelatedAnswers(awsclient.WithDetailOp(context.Background(), domain.Gen(13)), tc.shortName, wholeClients, tc.src, cache)

			if got := whole.callsPerARN()[onceTaskDefARN]; got != oneRow {
				t.Errorf("%s detail open spent %d attempts on the refused definition; one row reading it alone spends %d", tc.shortName, got, oneRow)
			}
		})
	}
}
