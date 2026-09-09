// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_field_contracts_test.go pins four facts a fetcher or a cache path
// owns and today answers twice, ambiguously, or not at all:
//
//   - the per-profile cost cache file name must distinguish two profiles that
//     differ only by a character the old sanitiser folded away;
//   - an ECS task row must carry the short task id under the key its list
//     column reads, so the column does not fall back to the whole ARN;
//   - a Lambda invocation row must describe "memory used" once, under the key
//     the column's SortKey names;
//   - a Lambda function that has been deleted is a race the enricher must
//     record, not a function that happens to have no resource policy.
package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/resource"
)

// --- row 15: the per-profile cost cache file name --------------------------

// TestCostsCachePath_DistinguishesProfilesThatDifferByOneCharacter pins that
// two configured profiles which differ only by a space or a slash never share
// one costs file. The pair-directory layout already encodes injectively; the
// costs file is the one path element left folding "team a" and "team_a"
// together, which silently serves one account's spend under the other's name.
func TestCostsCachePath_DistinguishesProfilesThatDifferByOneCharacter(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	pairs := [][2]string{
		{"example team", "example_team"},
		{"example/team", "example_team"},
		{"example team", "example/team"},
	}
	for _, p := range pairs {
		a, b := costs.CachePath(p[0]), costs.CachePath(p[1])
		if a == "" || b == "" {
			t.Fatalf("costs.CachePath returned an empty path for %q/%q — cache root unresolved", p[0], p[1])
		}
		if a == b {
			t.Errorf("costs.CachePath(%q) == costs.CachePath(%q) == %q — "+
				"two distinct profiles share one costs file, so each one's spend overwrites the other's",
				p[0], p[1], a)
		}
	}
}

// TestCostsCachePath_IsStableForOneProfile is the negative half: encoding
// must not make the path depend on anything but the profile name.
func TestCostsCachePath_IsStableForOneProfile(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	first := costs.CachePath("example-readonly")
	if first == "" {
		t.Fatal("costs.CachePath returned an empty path")
	}
	if second := costs.CachePath("example-readonly"); second != first {
		t.Errorf("costs.CachePath is not stable: %q then %q", first, second)
	}
	if !strings.HasSuffix(first, "--costs.yaml") {
		t.Errorf("costs cache path = %q, want it to keep the %q suffix", first, "--costs.yaml")
	}
}

// --- row 17: the ECS task id column ----------------------------------------

// ecsSvcTasksFake serves one running task for a service and nothing else.
type ecsSvcTasksFake struct {
	awsclient.ECSAPI
}

func (f *ecsSvcTasksFake) ListTasks(
	_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	if in.DesiredStatus != ecstypes.DesiredStatusRunning {
		return &ecs.ListTasksOutput{}, nil
	}
	return &ecs.ListTasksOutput{TaskArns: []string{wipfixECSTaskARN}}, nil
}

func (f *ecsSvcTasksFake) DescribeTasks(
	_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{
		TaskArn:           aws.String(wipfixECSTaskARN),
		ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/example-cluster"),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/example-web:5"),
		LastStatus:        aws.String("RUNNING"),
		HealthStatus:      ecstypes.HealthStatusHealthy,
		LaunchType:        ecstypes.LaunchTypeFargate,
	}}}, nil
}

const (
	wipfixECSTaskID  = "0b1c2d3e4f5061728394a5b6c7d8e9f0"
	wipfixECSTaskARN = "arn:aws:ecs:us-east-1:123456789012:task/example-cluster/" + wipfixECSTaskID
)

// TestFetchEcsSvcTasks_TaskIDColumnKeyCarriesTheShortID pins row 17: the
// ecs-task list column is keyed "task_id". A row that does not carry that key
// falls through to the reflected TaskArn path and renders the whole ARN in a
// 38-column cell, on live and on replay alike.
func TestFetchEcsSvcTasks_TaskIDColumnKeyCarriesTheShortID(t *testing.T) {
	fake := &ecsSvcTasksFake{}
	res, err := awsclient.FetchEcsSvcTasks(context.Background(), fake, fake,
		"example-cluster", "example-web", "")
	if err != nil {
		t.Fatalf("FetchEcsSvcTasks: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("fetched %d rows, want 1", len(res.Resources))
	}
	row := res.Resources[0]

	td := resource.FindResourceType("ecs-task")
	if td == nil {
		t.Fatal("resource type ecs-task is not registered")
	}
	var idKey string
	for _, col := range td.Columns {
		if col.Title == "Task ID" {
			idKey = col.Key
		}
	}
	if idKey == "" {
		t.Fatal("the ecs-task catalog has no column titled \"Task ID\"")
	}

	got := row.Fields[idKey]
	if got != wipfixECSTaskID {
		t.Errorf("Fields[%q] = %q, want the short task id %q — the Task ID column reads this key, "+
			"and an absent key falls back to the reflected TaskArn path, which is the whole ARN",
			idKey, got, wipfixECSTaskID)
	}
	if strings.HasPrefix(got, "arn:") {
		t.Errorf("the Task ID cell renders %q, an ARN — the short id is what identifies a task on screen", got)
	}
	// The whole ARN is still available for the console link and the detail body.
	if row.Fields["task_arn"] != wipfixECSTaskARN {
		t.Errorf("task_arn = %q, want the full ARN preserved %q", row.Fields["task_arn"], wipfixECSTaskARN)
	}
}

// --- row 20: memory used, once ---------------------------------------------

type cwlogsReportFake struct {
	awsclient.CWLogsFilterLogEventsAPI
}

func (f *cwlogsReportFake) FilterLogEvents(
	_ context.Context, _ *cwlogs.FilterLogEventsInput, _ ...func(*cwlogs.Options),
) (*cwlogs.FilterLogEventsOutput, error) {
	msg := "REPORT RequestId: 8e1a2b3c-4d5e-6f70-8192-a3b4c5d6e7f8\t" +
		"Duration: 512.34 ms\tBilled Duration: 513 ms\tMemory Size: 512 MB\tMax Memory Used: 137 MB\t"
	return &cwlogs.FilterLogEventsOutput{Events: []cwlogstypes.FilteredLogEvent{{
		Message:       aws.String(msg),
		Timestamp:     aws.Int64(1767225600000),
		LogStreamName: aws.String("2026/01/01/[$LATEST]abcdef"),
	}}}, nil
}

// TestFetchLambdaInvocations_MemoryUsedIsOneField pins row 20: "memory used
// in MB" is one fact. Writing it under two keys leaves the list column's
// SortKey pointing at whichever copy survives the next edit, and a SortKey
// naming a key no row carries sorts every row equal.
func TestFetchLambdaInvocations_MemoryUsedIsOneField(t *testing.T) {
	res, err := awsclient.FetchLambdaInvocations(context.Background(), &cwlogsReportFake{},
		"example-fn", "/aws/lambda/example-fn", "")
	if err != nil {
		t.Fatalf("FetchLambdaInvocations: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("fetched %d rows, want 1", len(res.Resources))
	}
	fields := res.Resources[0].Fields

	_, plain := fields["memory_used_mb"]
	_, raw := fields["memory_used_mb_raw"]
	if plain && raw {
		t.Errorf("the row carries both memory_used_mb=%q and memory_used_mb_raw=%q — "+
			"one fact, one field; memory_used is the formatted one",
			fields["memory_used_mb"], fields["memory_used_mb_raw"])
	}

	for _, col := range config.DefaultViewDef("lambda_invocations").List {
		key := col.SortKey
		if key == "" {
			key = col.Key
		}
		if key == "" {
			continue
		}
		if _, ok := fields[key]; !ok {
			t.Errorf("column %q sorts on %q, a key no invocation row carries — every row sorts equal",
				col.Title, key)
		}
	}
	if got := fields["memory_used"]; got != "137/512 MB" {
		t.Errorf("memory_used = %q, want %q — the displayed field is unaffected by the collapse",
			got, "137/512 MB")
	}
}

// --- row 25: a deleted function is not a policy-less one -------------------

// lambdaDeletedFunctionFake answers ResourceNotFoundException to every call,
// the way Lambda answers for a function that has been deleted since the list
// fetch that produced the row.
type lambdaDeletedFunctionFake struct {
	awsclient.LambdaAPI
}

func lambdaNotFound() error {
	return &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "Function not found"}
}

func (f *lambdaDeletedFunctionFake) GetPolicy(
	_ context.Context, _ *lambda.GetPolicyInput, _ ...func(*lambda.Options),
) (*lambda.GetPolicyOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaDeletedFunctionFake) ListFunctionUrlConfigs(
	_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options),
) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaDeletedFunctionFake) GetFunction(
	_ context.Context, _ *lambda.GetFunctionInput, _ ...func(*lambda.Options),
) (*lambda.GetFunctionOutput, error) {
	return nil, lambdaNotFound()
}

// lambdaLivePolicylessFake is the healthy counterpart: the function exists,
// it simply has no resource policy and no function URL.
type lambdaLivePolicylessFake struct {
	awsclient.LambdaAPI
}

func (f *lambdaLivePolicylessFake) GetPolicy(
	_ context.Context, _ *lambda.GetPolicyInput, _ ...func(*lambda.Options),
) (*lambda.GetPolicyOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaLivePolicylessFake) ListFunctionUrlConfigs(
	_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options),
) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return &lambda.ListFunctionUrlConfigsOutput{}, nil
}

func (f *lambdaLivePolicylessFake) GetFunction(
	_ context.Context, _ *lambda.GetFunctionInput, _ ...func(*lambda.Options),
) (*lambda.GetFunctionOutput, error) {
	return &lambda.GetFunctionOutput{}, nil
}

func wipfixLambdaRow() []resource.Resource {
	return []resource.Resource{{
		ID:     "example-fn",
		Name:   "example-fn",
		Fields: map[string]string{"function_name": "example-fn", "runtime": "python3.13"},
	}}
}

// TestEnrichLambdaPosture_DeletedFunctionIsRecordedAsUninspected pins row 25:
// Lambda answers ResourceNotFoundException both for "this function has no
// resource policy" (healthy, a fact) and for "this function no longer exists"
// (a race, nothing was inspected). Reading the code alone cannot tell them
// apart, so a deleted function is silently reported as posture-clean.
func TestEnrichLambdaPosture_DeletedFunctionIsRecordedAsUninspected(t *testing.T) {
	clients := &awsclient.ServiceClients{Lambda: &lambdaDeletedFunctionFake{}}
	res, err := awsclient.EnrichLambdaPosture(context.Background(), clients, wipfixLambdaRow(), nil)
	if err != nil {
		t.Fatalf("EnrichLambdaPosture returned an error for a vanished function: %v — "+
			"a deleted resource is a race, not a failure to report", err)
	}
	if _, marked := res.TruncatedIDs["example-fn"]; !marked {
		t.Errorf("TruncatedIDs[example-fn] = false for a function that no longer exists — " +
			"nothing about its posture was inspected, so the row must not read as clean")
	}
	if got := codesOf(res.Findings["example-fn"]); len(got) != 0 {
		t.Errorf("findings for a vanished function = %v, want none", got)
	}
}

// TestEnrichLambdaPosture_PolicylessLiveFunctionIsClean is the negative half:
// the same error code from GetPolicy on a function that does exist is the
// healthy answer, and must leave the row inspected and finding-free.
func TestEnrichLambdaPosture_PolicylessLiveFunctionIsClean(t *testing.T) {
	clients := &awsclient.ServiceClients{Lambda: &lambdaLivePolicylessFake{}}
	res, err := awsclient.EnrichLambdaPosture(context.Background(), clients, wipfixLambdaRow(), nil)
	if err != nil {
		t.Fatalf("EnrichLambdaPosture: %v", err)
	}
	if _, marked := res.TruncatedIDs["example-fn"]; marked {
		t.Errorf("TruncatedIDs[example-fn] = true for a live function with no resource policy — " +
			"that is the healthy answer, fully inspected")
	}
	if got := codesOf(res.Findings["example-fn"]); len(got) != 0 {
		t.Errorf("findings for a policy-less live function = %v, want none", got)
	}
}
