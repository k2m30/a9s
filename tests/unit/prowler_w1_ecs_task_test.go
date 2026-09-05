package unit

// prowler_w1_ecs_task_test.go — behavioural pins for the ecs-task posture
// signals of batch w1: privileged, host-namespace, writable-root, no-logging
// and env-secret.
//
// All five read the task definition, which EnrichECSTasks fetches once per
// distinct definition ARN. The tests drive the real enricher so the batching,
// the per-item failure handling and the finding shapes are pinned together.

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	pw1ECSTaskCodePrivileged    = domain.FindingCode("ecs-task.privileged")
	pw1ECSTaskCodeHostNamespace = domain.FindingCode("ecs-task.host-namespace")
	pw1ECSTaskCodeWritableRoot  = domain.FindingCode("ecs-task.writable-root")
	pw1ECSTaskCodeNoLogging     = domain.FindingCode("ecs-task.no-logging")
	pw1ECSTaskCodeEnvSecret     = domain.FindingCode("ecs-task.env-secret")
)

const pw1TaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-app:7"

// pw1ECSTaskFake serves DescribeTasks and DescribeTaskDefinition and counts
// the per-definition describes so the memoisation contract can be asserted.
type pw1ECSTaskFake struct {
	awsclient.ECSAPI
	tasks       map[string]ecstypes.Task           // task id → task
	defs        map[string]ecstypes.TaskDefinition // definition ARN → definition
	defErr      map[string]error                   // definition ARN → error
	tasksErr    error
	defRequests []string
}

func (f *pw1ECSTaskFake) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if f.tasksErr != nil {
		return nil, f.tasksErr
	}
	var out []ecstypes.Task
	for _, ref := range in.Tasks {
		// The enricher passes task IDs, the fetcher passes full ARNs.
		id := ref
		if i := strings.LastIndex(ref, "/"); i >= 0 {
			id = ref[i+1:]
		}
		if task, ok := f.tasks[id]; ok {
			out = append(out, task)
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: out}, nil
}

func (f *pw1ECSTaskFake) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	arn := aws.ToString(in.TaskDefinition)
	f.defRequests = append(f.defRequests, arn)
	if err, ok := f.defErr[arn]; ok {
		return nil, err
	}
	def, ok := f.defs[arn]
	if !ok {
		return nil, errors.New("ClientException: task definition not found")
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &def}, nil
}

func (f *pw1ECSTaskFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{"arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster"}}, nil
}

func (f *pw1ECSTaskFake) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	arns := make([]string, 0, len(f.tasks))
	for _, task := range f.tasks {
		arns = append(arns, aws.ToString(task.TaskArn))
	}
	sort.Strings(arns)
	return &ecs.ListTasksOutput{TaskArns: arns}, nil
}

// pw1Task builds a running task pointing at defARN.
func pw1Task(id, defARN string) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + id),
		ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster"),
		TaskDefinitionArn: aws.String(defARN),
		LastStatus:        aws.String("RUNNING"),
		DesiredStatus:     aws.String("RUNNING"),
		LaunchType:        ecstypes.LaunchTypeFargate,
	}
}

// pw1HealthyContainer is a container definition that trips none of the five
// posture rules: unprivileged, read-only root, a log driver, no secrets.
func pw1HealthyContainer(name string) ecstypes.ContainerDefinition {
	return ecstypes.ContainerDefinition{
		Name:                   aws.String(name),
		Image:                  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/" + name + ":1.4.2"),
		Essential:              aws.Bool(true),
		Privileged:             aws.Bool(false),
		ReadonlyRootFilesystem: aws.Bool(true),
		LogConfiguration: &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs,
			Options: map[string]string{
				"awslogs-group":  "/ecs/acme-app",
				"awslogs-region": "us-east-1",
			},
		},
		Environment: []ecstypes.KeyValuePair{
			{Name: aws.String("APP_ENV"), Value: aws.String("production")},
			{Name: aws.String("DB_PASSWORD_ARN"), Value: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf")},
		},
		Secrets: []ecstypes.Secret{{
			Name:      aws.String("DB_PASSWORD"),
			ValueFrom: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf"),
		}},
	}
}

// pw1NoRefContainer is pw1HealthyContainer without the Secrets Manager ARN
// environment variable, so a test can observe one scanner rule in isolation.
func pw1NoRefContainer(name string) ecstypes.ContainerDefinition {
	c := pw1HealthyContainer(name)
	c.Environment = []ecstypes.KeyValuePair{{Name: aws.String("APP_ENV"), Value: aws.String("production")}}
	return c
}

// pw1TaskDef builds a task definition from containers with the safe defaults.
func pw1TaskDef(arn string, containers ...ecstypes.ContainerDefinition) ecstypes.TaskDefinition {
	return ecstypes.TaskDefinition{
		TaskDefinitionArn:       aws.String(arn),
		Family:                  aws.String("acme-app"),
		Revision:                7,
		Status:                  ecstypes.TaskDefinitionStatusActive,
		NetworkMode:             ecstypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecstypes.Compatibility{ecstypes.CompatibilityFargate},
		ContainerDefinitions:    containers,
	}
}

func pw1ECSTaskResource(id, defARN string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"task_id":         id,
			"cluster":         "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster",
			"status":          "RUNNING",
			"task_definition": defARN,
			"arn":             "arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + id,
		},
	}
}

func pw1EnrichECSTasks(t *testing.T, fake *pw1ECSTaskFake, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichECSTasks(context.Background(), &awsclient.ServiceClients{ECS: fake}, rs, nil)
	if err != nil && fake.tasksErr == nil && len(fake.defErr) == 0 {
		t.Fatalf("EnrichECSTasks: %v", err)
	}
	return res
}

// pw1RunOneTask wires one task at pw1TaskDefARN through the enricher.
func pw1RunOneTask(t *testing.T, id string, def ecstypes.TaskDefinition) awsclient.IssueEnricherResult {
	t.Helper()
	fake := &pw1ECSTaskFake{
		tasks: map[string]ecstypes.Task{id: pw1Task(id, pw1TaskDefARN)},
		defs:  map[string]ecstypes.TaskDefinition{pw1TaskDefARN: def},
	}
	return pw1EnrichECSTasks(t, fake, pw1ECSTaskResource(id, pw1TaskDefARN))
}

// ─── row 7: ecs-task.privileged ─────────────────────────────────────────────

// TestECSTask_Privileged_ContainerEscapesIsolation pins the Broken finding and
// one row per privileged container: a privileged container has the host's
// device access, so a compromise inside it is a compromise of the instance.
func TestECSTask_Privileged_ContainerEscapesIsolation(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4444"
	priv := pw1HealthyContainer("sidecar")
	priv.Privileged = aws.Bool(true)
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"), priv))

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodePrivileged,
		"privileged container", domain.SevBroken, "wave2:ecs-task")
	rows := pw1Rows(res, id, pw1ECSTaskCodePrivileged)
	pw1RequireRow(t, rows, "Container", "sidecar")
	for _, r := range rows {
		if r.Value == "app" {
			t.Errorf("unprivileged container 'app' listed as a privileged container: %+v", rows)
		}
	}
}

// TestECSTask_Privileged_UnprivilegedIsHealthy pins the negative case.
func TestECSTask_Privileged_UnprivilegedIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4445"
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app")))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodePrivileged)
}

// TestECSTask_Privileged_NilIsNotPrivileged pins that an absent Privileged
// pointer is the AWS default (false), not an unknown to flag.
func TestECSTask_Privileged_NilIsNotPrivileged(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4446"
	c := pw1HealthyContainer("app")
	c.Privileged = nil
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodePrivileged)
}

// ─── row 8: ecs-task.host-namespace ─────────────────────────────────────────

// TestECSTask_HostNamespace_HostNetworkMode pins the warning and that only the
// condition that applies is cited.
func TestECSTask_HostNamespace_HostNetworkMode(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4447"
	def := pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"))
	def.NetworkMode = ecstypes.NetworkModeHost
	res := pw1RunOneTask(t, id, def)

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeHostNamespace,
		"shares the host network or process namespace", domain.SevWarn, "wave2:ecs-task")
	rows := pw1Rows(res, id, pw1ECSTaskCodeHostNamespace)
	pw1RequireRow(t, rows, "Network mode", "host")
	for _, r := range rows {
		if r.Label == "Process namespace" {
			t.Errorf("PidMode cited on a task that does not share the PID namespace: %+v", rows)
		}
	}
}

// TestECSTask_HostNamespace_HostPidMode pins the second half of the OR: a task
// in awsvpc mode that shares the host PID namespace is still flagged.
func TestECSTask_HostNamespace_HostPidMode(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4448"
	def := pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"))
	def.PidMode = ecstypes.PidModeHost
	res := pw1RunOneTask(t, id, def)

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeHostNamespace,
		"shares the host network or process namespace", domain.SevWarn, "wave2:ecs-task")
	rows := pw1Rows(res, id, pw1ECSTaskCodeHostNamespace)
	pw1RequireRow(t, rows, "Process namespace", "host")
	for _, r := range rows {
		if r.Label == "Network mode" {
			t.Errorf("NetworkMode cited on an awsvpc task: %+v", rows)
		}
	}
}

// TestECSTask_HostNamespace_BothConditionsCiteBothRows pins that a task doing
// both gets one finding carrying both rows, not two findings under one code.
func TestECSTask_HostNamespace_BothConditionsCiteBothRows(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4449"
	def := pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"))
	def.NetworkMode = ecstypes.NetworkModeHost
	def.PidMode = ecstypes.PidModeHost
	res := pw1RunOneTask(t, id, def)

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeHostNamespace,
		"shares the host network or process namespace", domain.SevWarn, "wave2:ecs-task")
	rows := pw1Rows(res, id, pw1ECSTaskCodeHostNamespace)
	pw1RequireRow(t, rows, "Network mode", "host")
	pw1RequireRow(t, rows, "Process namespace", "host")
}

// TestECSTask_HostNamespace_AwsvpcIsHealthy pins the negative case.
func TestECSTask_HostNamespace_AwsvpcIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444a"
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app")))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodeHostNamespace)
}

// ─── row 9: ecs-task.writable-root ──────────────────────────────────────────

// TestECSTask_WritableRoot_ExplicitFalse pins the warning for a container that
// declares a writable root filesystem.
func TestECSTask_WritableRoot_ExplicitFalse(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444b"
	c := pw1HealthyContainer("app")
	c.ReadonlyRootFilesystem = aws.Bool(false)
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeWritableRoot,
		"writable root filesystem", domain.SevWarn, "wave2:ecs-task")
	pw1RequireRow(t, pw1Rows(res, id, pw1ECSTaskCodeWritableRoot), "Container", "app")
}

// TestECSTask_WritableRoot_NilIsWritable pins the one place nil is not
// unknown: ECS defaults ReadonlyRootFilesystem to false, so an unset pointer
// means the root filesystem really is writable.
func TestECSTask_WritableRoot_NilIsWritable(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444c"
	c := pw1HealthyContainer("app")
	c.ReadonlyRootFilesystem = nil
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))
	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeWritableRoot,
		"writable root filesystem", domain.SevWarn, "wave2:ecs-task")
}

// TestECSTask_WritableRoot_ReadOnlyIsHealthy pins the negative case.
func TestECSTask_WritableRoot_ReadOnlyIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444d"
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app")))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodeWritableRoot)
}

// TestECSTask_WritableRoot_CitesOnlyTheWritableContainers pins that a mixed
// definition names the offending container and not its hardened neighbour.
func TestECSTask_WritableRoot_CitesOnlyTheWritableContainers(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444e"
	bad := pw1HealthyContainer("legacy")
	bad.ReadonlyRootFilesystem = aws.Bool(false)
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app"), bad))

	rows := pw1Rows(res, id, pw1ECSTaskCodeWritableRoot)
	pw1RequireRow(t, rows, "Container", "legacy")
	for _, r := range rows {
		if r.Value == "app" {
			t.Errorf("read-only container 'app' cited as writable: %+v", rows)
		}
	}
}

// ─── row 10: ecs-task.no-logging ────────────────────────────────────────────

// TestECSTask_NoLogging_MissingLogConfiguration pins the warning: without a
// log driver the container's output is unreachable after the task stops.
func TestECSTask_NoLogging_MissingLogConfiguration(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd444f"
	c := pw1HealthyContainer("app")
	c.LogConfiguration = nil
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))

	pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeNoLogging,
		"container without log driver", domain.SevWarn, "wave2:ecs-task")
	pw1RequireRow(t, pw1Rows(res, id, pw1ECSTaskCodeNoLogging), "Container", "app")
}

// TestECSTask_NoLogging_AwslogsIsHealthy pins the negative case.
func TestECSTask_NoLogging_AwslogsIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4450"
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app")))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodeNoLogging)
}

// ─── row 11: ecs-task.env-secret ────────────────────────────────────────────

// TestECSTask_EnvSecret_PlaintextEnvironmentVariable pins the Broken finding,
// the container citation and the Where:Kind row — and that the credential
// value itself never appears anywhere in the emitted finding.
func TestECSTask_EnvSecret_PlaintextEnvironmentVariable(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4451"
	c := pw1HealthyContainer("app")
	c.Environment = append(c.Environment, ecstypes.KeyValuePair{
		Name:  aws.String("DB_PASSWORD"),
		Value: aws.String("hunter2hunter2"),
	})
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))

	f := pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodeEnvSecret,
		"credential in container environment", domain.SevBroken, "wave2:ecs-task")
	rows := pw1Rows(res, id, pw1ECSTaskCodeEnvSecret)
	pw1RequireRow(t, rows, "Container", "app")
	pw1RequireRow(t, rows, "DB_PASSWORD", "keyword")

	if f.Phrase == "hunter2hunter2" || f.Detail == "hunter2hunter2" {
		t.Errorf("credential value leaked into the finding")
	}
	for _, r := range rows {
		if r.Value == "hunter2hunter2" || r.Label == "hunter2hunter2" {
			t.Errorf("credential value leaked into an AttentionDetail row: %+v", r)
		}
	}
}

// TestECSTask_EnvSecret_SecretsManagerWiringIsHealthy pins the negative case
// that matters most: the recommended shape, where the environment carries an
// ARN and the real value arrives through the Secrets block, must stay silent.
func TestECSTask_EnvSecret_SecretsManagerWiringIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4452"
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, pw1HealthyContainer("app")))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodeEnvSecret)
}

// TestECSTask_EnvSecret_PlaceholderIsHealthy pins that a documented
// placeholder is not a leak. A bare "contains PASSWORD" check fails here.
func TestECSTask_EnvSecret_PlaceholderIsHealthy(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4453"
	c := pw1NoRefContainer("app")
	c.Environment = append(c.Environment,
		ecstypes.KeyValuePair{Name: aws.String("API_TOKEN"), Value: aws.String("changeme")},
		ecstypes.KeyValuePair{Name: aws.String("CLIENT_SECRET"), Value: aws.String("${CLIENT_SECRET}")},
	)
	res := pw1RunOneTask(t, id, pw1TaskDef(pw1TaskDefARN, c))
	pw1RequireNoFinding(t, res.Findings[id], pw1ECSTaskCodeEnvSecret)
}

// ─── cross-cutting ──────────────────────────────────────────────────────────

// TestECSTask_AllFiveConditionsOnOneDefinitionAreFiveFindings pins
// independence: a definition that trips every rule produces one finding per
// rule, each with its own code, phrase and rows.
func TestECSTask_AllFiveConditionsOnOneDefinitionAreFiveFindings(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4454"
	c := pw1NoRefContainer("app")
	c.Privileged = aws.Bool(true)
	c.ReadonlyRootFilesystem = aws.Bool(false)
	c.LogConfiguration = nil
	c.Environment = append(c.Environment, ecstypes.KeyValuePair{
		Name:  aws.String("ADMIN_PASSWORD"),
		Value: aws.String("s3cr3t-value-9"),
	})
	def := pw1TaskDef(pw1TaskDefARN, c)
	def.NetworkMode = ecstypes.NetworkModeHost

	res := pw1RunOneTask(t, id, def)
	for _, code := range []domain.FindingCode{
		pw1ECSTaskCodePrivileged,
		pw1ECSTaskCodeHostNamespace,
		pw1ECSTaskCodeWritableRoot,
		pw1ECSTaskCodeNoLogging,
		pw1ECSTaskCodeEnvSecret,
	} {
		if _, ok := pw1FindFinding(res.Findings[id], code); !ok {
			t.Errorf("missing finding %q on a definition that trips every rule: %+v", code, res.Findings[id])
		}
	}
}

// TestECSTask_TaskDefinitionDescribedOncePerDistinctARN pins the batching
// contract: two tasks from the same definition cost one describe, not two.
func TestECSTask_TaskDefinitionDescribedOncePerDistinctARN(t *testing.T) {
	const idA = "0aaaa1111bbbb2222cccc3333dddd4455"
	const idB = "0aaaa1111bbbb2222cccc3333dddd4456"
	priv := pw1HealthyContainer("app")
	priv.Privileged = aws.Bool(true)

	fake := &pw1ECSTaskFake{
		tasks: map[string]ecstypes.Task{
			idA: pw1Task(idA, pw1TaskDefARN),
			idB: pw1Task(idB, pw1TaskDefARN),
		},
		defs: map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1TaskDef(pw1TaskDefARN, priv)},
	}
	res := pw1EnrichECSTasks(t, fake, pw1ECSTaskResource(idA, pw1TaskDefARN), pw1ECSTaskResource(idB, pw1TaskDefARN))

	if len(fake.defRequests) != 1 {
		t.Errorf("DescribeTaskDefinition called %d times for one definition shared by two tasks: %v",
			len(fake.defRequests), fake.defRequests)
	}
	for _, id := range []string{idA, idB} {
		pw1RequireFinding(t, res.Findings[id], pw1ECSTaskCodePrivileged,
			"privileged container", domain.SevBroken, "wave2:ecs-task")
	}
}

// TestECSTask_TaskDefinitionErrorMarksOnlyItsOwnTasks pins the partial-failure
// contract: a definition that cannot be read leaves its tasks in TruncatedIDs
// while tasks on a readable definition are still evaluated.
func TestECSTask_TaskDefinitionErrorMarksOnlyItsOwnTasks(t *testing.T) {
	const badARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-denied:1"
	const badID = "0aaaa1111bbbb2222cccc3333dddd4457"
	const goodID = "0aaaa1111bbbb2222cccc3333dddd4458"
	priv := pw1HealthyContainer("app")
	priv.Privileged = aws.Bool(true)

	fake := &pw1ECSTaskFake{
		tasks: map[string]ecstypes.Task{
			badID:  pw1Task(badID, badARN),
			goodID: pw1Task(goodID, pw1TaskDefARN),
		},
		defs:   map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1TaskDef(pw1TaskDefARN, priv)},
		defErr: map[string]error{badARN: errors.New("AccessDeniedException: ecs:DescribeTaskDefinition")},
	}
	res := pw1EnrichECSTasks(t, fake, pw1ECSTaskResource(badID, badARN), pw1ECSTaskResource(goodID, pw1TaskDefARN))

	if !res.TruncatedIDs[badID] {
		t.Errorf("TruncatedIDs missing %s after its task definition could not be read", badID)
	}
	pw1RequireFinding(t, res.Findings[goodID], pw1ECSTaskCodePrivileged,
		"privileged container", domain.SevBroken, "wave2:ecs-task")
	if res.TruncatedIDs[goodID] {
		t.Errorf("healthy neighbour %s wrongly marked truncated", goodID)
	}
}

// TestECSTask_DemoBench_EachSignalHasExactlyOneWitness pins the demo fixture
// contract for all five ecs-task signals.
func TestECSTask_DemoBench_EachSignalHasExactlyOneWitness(t *testing.T) {
	td := catalog.Find("ecs-task")
	if td == nil || td.Fetcher == nil {
		t.Fatal("ecs-task has no catalog Fetcher")
	}
	clients := &awsclient.ServiceClients{ECS: fakes.NewECS()}
	page, ferr := td.Fetcher(context.Background(), clients, "")
	if ferr != nil {
		t.Fatalf("fetching demo ecs-task rows: %v", ferr)
	}
	res, eerr := awsclient.EnrichECSTasks(context.Background(), clients, page.Resources, nil)
	if eerr != nil {
		t.Fatalf("EnrichECSTasks(demo): %v", eerr)
	}

	// These five signals live on the task definition, not the task, so every
	// task running the witness's definition carries them. The demo contract is
	// therefore "the witness's definition and no other definition", which is
	// what makes exactly one row shape appear on the bench.
	defOf := map[string]string{}
	for _, r := range page.Resources {
		defOf[r.ID] = r.Fields["task_definition"]
	}
	for code, witness := range map[domain.FindingCode]string{
		pw1ECSTaskCodePrivileged:    fixtures.ECSTaskPrivileged,
		pw1ECSTaskCodeHostNamespace: fixtures.ECSTaskHostNamespace,
		pw1ECSTaskCodeWritableRoot:  fixtures.ECSTaskWritableRoot,
		pw1ECSTaskCodeNoLogging:     fixtures.ECSTaskNoLogging,
		pw1ECSTaskCodeEnvSecret:     fixtures.ECSTaskEnvSecret,
	} {
		wantDef, known := defOf[witness]
		if !known {
			t.Errorf("%s: witness task %s is not in the demo list", code, witness)
			continue
		}
		carried := false
		for id, fs := range res.Findings {
			if _, ok := pw1FindFinding(fs, code); !ok {
				continue
			}
			if id == witness {
				carried = true
			}
			if defOf[id] != wantDef {
				t.Errorf("%s: task %s (definition %s) carries the finding; only %s should",
					code, id, defOf[id], wantDef)
			}
		}
		if !carried {
			t.Errorf("%s: witness task %s does not carry the finding", code, witness)
		}
	}
}

// pw1ECSTeardownStates are the lifecycle states in which a task is on its way
// out. A task in any of them is gone as far as posture goes: nothing about its
// definition can be acted on through this row any more.
var pw1ECSTeardownStates = []string{"STOPPED", "STOPPING", "DEPROVISIONING", "DEACTIVATING"}

// pw1BadDefinition is a task definition that trips all five posture rules.
func pw1BadDefinition() ecstypes.TaskDefinition {
	c := pw1NoRefContainer("app")
	c.Privileged = aws.Bool(true)
	c.ReadonlyRootFilesystem = aws.Bool(false)
	c.LogConfiguration = nil
	c.Environment = append(c.Environment,
		ecstypes.KeyValuePair{Name: aws.String("ADMIN_PASSWORD"), Value: aws.String("s3cr3t-value-9")})
	def := pw1TaskDef(pw1TaskDefARN, c)
	def.NetworkMode = ecstypes.NetworkModeHost
	return def
}

// TestECSTask_PostureSignalsSilentThroughoutTeardown pins that every state in
// which a task is being torn down silences the posture rules, not STOPPED
// alone. A task that is stopping is as unactionable as one that has stopped.
func TestECSTask_PostureSignalsSilentThroughoutTeardown(t *testing.T) {
	for _, state := range pw1ECSTeardownStates {
		t.Run(state, func(t *testing.T) {
			const id = "0aaaa1111bbbb2222cccc3333dddd4460"
			task := pw1Task(id, pw1TaskDefARN)
			task.LastStatus = aws.String(state)
			task.DesiredStatus = aws.String("STOPPED")

			r := pw1ECSTaskResource(id, pw1TaskDefARN)
			r.Fields["status"] = state
			r.Fields["last_status"] = state

			fake := &pw1ECSTaskFake{
				tasks: map[string]ecstypes.Task{id: task},
				defs:  map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1BadDefinition()},
			}
			res := pw1EnrichECSTasks(t, fake, r)

			for _, code := range []domain.FindingCode{
				pw1ECSTaskCodePrivileged,
				pw1ECSTaskCodeHostNamespace,
				pw1ECSTaskCodeWritableRoot,
				pw1ECSTaskCodeNoLogging,
				pw1ECSTaskCodeEnvSecret,
			} {
				if _, ok := pw1FindFinding(res.Findings[id], code); ok {
					t.Errorf("%s emitted for a task in state %s", code, state)
				}
			}
		})
	}
}

// TestECSTask_TeardownStatesKeepTheirLifecycleFinding pins the other half of
// the same rule: silencing posture must not silence the state itself. A
// stopping task still says so in its Status cell.
func TestECSTask_TeardownStatesKeepTheirLifecycleFinding(t *testing.T) {
	want := map[string]string{
		"STOPPING":       "stopping",
		"DEPROVISIONING": "deprovisioning",
		"DEACTIVATING":   "deactivating",
		"STOPPED":        "stopped",
	}
	td := catalog.Find("ecs-task")
	if td == nil || td.Fetcher == nil {
		t.Fatal("ecs-task has no catalog Fetcher")
	}
	for _, state := range pw1ECSTeardownStates {
		t.Run(state, func(t *testing.T) {
			const id = "0aaaa1111bbbb2222cccc3333dddd4461"
			task := pw1Task(id, pw1TaskDefARN)
			task.LastStatus = aws.String(state)
			task.DesiredStatus = aws.String("STOPPED")

			fake := &pw1ECSTaskFake{
				tasks: map[string]ecstypes.Task{id: task},
				defs:  map[string]ecstypes.TaskDefinition{pw1TaskDefARN: pw1BadDefinition()},
			}
			page, err := td.Fetcher(context.Background(), &awsclient.ServiceClients{ECS: fake}, "")
			if err != nil {
				t.Fatalf("ecs-task fetcher: %v", err)
			}
			r := pw1ResourceByID(t, page.Resources, id)
			if len(r.Findings) == 0 {
				t.Fatalf("state %s lost its lifecycle finding entirely", state)
			}
			if r.Findings[0].Phrase != want[state] {
				t.Errorf("state %s: lifecycle phrase = %q, want %q", state, r.Findings[0].Phrase, want[state])
			}
		})
	}
}
