// aws_related_checker_round2_test.go pins the related-panel checker
// mechanism quoted from the golden per-type specs in docs/resources/*.md
// for a second set of pivots, driven on realistic data. Mirrors the harness
// patterns in aws_related_checker_mechanism_test.go.
package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/aws/smithy-go"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// checkerByTarget (source, target string) resource.RelatedChecker is already
// declared in aws_iam_policies_related_test.go — reused here rather than
// redeclared.

// ---------------------------------------------------------------------------
// 1. checkAlarmCTEvents — alarm.md / ct-events cross-ref field key.
//
// checkAlarmCTEvents (core/aws/alarm_related_extra.go:181) reads
// evRes.Fields["event_source"], but FetchCloudTrailEventsPage
// (core/aws/ct_events.go:248) writes the AWS EventSource value under
// Fields["source"], never "event_source". The Contains() guard permanently
// compares against "" — this pivot can never match real ct-events data.
// ---------------------------------------------------------------------------

func TestAlarm_Related_CTEvents_MatchesBySourceField(t *testing.T) {
	alarmRes := resource.Resource{ID: "high-cpu-alarm", Name: "high-cpu-alarm"}

	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "event-1",
					Name: "PutMetricAlarm",
					Fields: map[string]string{
						// ct_events.go emits Fields["source"], not "event_source"
						// (core/aws/ct_events.go:248).
						"source":     "monitoring.amazonaws.com",
						"event_name": "PutMetricAlarm",
					},
				},
			},
		},
	}

	checker := checkerByTarget(t, "alarm", "ct-events")
	result := checker(context.Background(), nil, alarmRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (checker must read Fields[\"source\"], the key ct_events.go actually writes)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// 2. checkECRCFN — ecr.md aws:cloudformation:stack-name tag cross-ref.
//
// ecrCFNStackName (core/aws/ecr_related.go:125-128) reads
// res.Fields["cfn_stack_name"], a key FetchECRRepositoriesPage
// (core/aws/ecr.go) never populates — no ListTagsForResource call is
// wired anywhere in the ECR fetch/check path. The correct mechanism calls
// ecr:ListTagsForResource (one call per open repo, in budget) and matches
// the aws:cloudformation:stack-name tag against the cfn cache.
// ---------------------------------------------------------------------------

type fakeECRListTagsForResource struct {
	awsclient.ECRAPI
	repoArn string
	tags    map[string]string
	calls   int
}

func (f *fakeECRListTagsForResource) ListTagsForResource(_ context.Context, params *ecr.ListTagsForResourceInput, _ ...func(*ecr.Options)) (*ecr.ListTagsForResourceOutput, error) {
	f.calls++
	if params.ResourceArn == nil || *params.ResourceArn != f.repoArn {
		return &ecr.ListTagsForResourceOutput{}, nil
	}
	var tags []ecrtypes.Tag
	for k, v := range f.tags {
		tags = append(tags, ecrtypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return &ecr.ListTagsForResourceOutput{Tags: tags}, nil
}

func TestECR_Related_CFN_ResolvesViaListTagsForResource(t *testing.T) {
	repoArn := "arn:aws:ecr:us-east-1:123456789012:repository/checkout-service"
	repo := ecrtypes.Repository{
		RepositoryName: aws.String("checkout-service"),
		RepositoryArn:  aws.String(repoArn),
	}
	repoRes := resource.Resource{
		ID:        "checkout-service",
		Name:      "checkout-service",
		RawStruct: repo,
		Fields:    map[string]string{"repository_name": "checkout-service", "uri": repoArn},
	}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "checkout-stack", Name: "checkout-stack", Fields: map[string]string{"stack_name": "checkout-stack"}},
			},
		},
	}

	fake := &fakeECRListTagsForResource{
		repoArn: repoArn,
		tags:    map[string]string{"aws:cloudformation:stack-name": "checkout-stack"},
	}
	clients := &awsclient.ServiceClients{ECR: fake}

	checker := checkerByTarget(t, "ecr", "cfn")
	result := checker(context.Background(), clients, repoRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecr.md — ListTagsForResource aws:cloudformation:stack-name -> cfn cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "checkout-stack" {
		t.Fatalf("ResourceIDs = %v, want [checkout-stack]", result.ResourceIDs())
	}
	if fake.calls == 0 {
		t.Fatalf("ListTagsForResource was never called — checker must fetch tags per open repo")
	}
}

// ---------------------------------------------------------------------------
// 3. checkECRECSTask — ecr.md image-URI substring cross-ref.
//
// checkECRECSTask (core/aws/ecr_related_extra.go:66) already scans every
// ecs-task Fields value for ".dkr.ecr." + "/"+repoName, but the real
// FetchECSTasksPage (core/aws/ecs_task.go:127-141) never stores the
// container image URI (Task.Containers[].Image) in any Fields entry — this
// test drives the real fetch path with a task carrying a live container
// image and proves no Fields value contains it, so checkECSTaskECR
// (correct substring logic already in place) can never fire against real
// fetcher output.
// ---------------------------------------------------------------------------

type fakeECSListClustersOnly struct {
	clusterArns []string
}

func (f *fakeECSListClustersOnly) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: f.clusterArns}, nil
}

type fakeECSListTasksOnly struct {
	taskArns []string
}

func (f *fakeECSListTasksOnly) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: f.taskArns}, nil
}

type fakeECSDescribeTasksWithImage struct {
	tasks []ecstypes.Task
}

func (f *fakeECSDescribeTasksWithImage) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{Tasks: f.tasks}, nil
}

// ecsTaskRound2FullFake composes the three narrow ECS mocks above into one
// awsclient.ECSAPI value — the registered "ecs-task" paginated fetcher reads
// ListClusters/ListTasks/DescribeTasks off a single *ServiceClients.ECS
// field. DescribeClusters/ListServices/DescribeServices are stubbed since
// this test never touches them. DescribeTaskDefinition returns a
// ClientException ("does not exist"), matching the pre-refactor 4-arg
// FetchECSTasksPage contract, so Fields["task_def_join_error"] stays unset.
type ecsTaskRound2FullFake struct {
	*fakeECSListClustersOnly
	*fakeECSListTasksOnly
	*fakeECSDescribeTasksWithImage
}

func (f *ecsTaskRound2FullFake) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return &ecs.DescribeClustersOutput{}, nil
}

func (f *ecsTaskRound2FullFake) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return &ecs.ListServicesOutput{}, nil
}

func (f *ecsTaskRound2FullFake) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return &ecs.DescribeServicesOutput{}, nil
}

func (f *ecsTaskRound2FullFake) DescribeTaskDefinition(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "ClientException", Message: "task definition does not exist"}
}

func TestECR_Related_ECSTask_ResolvesViaRealFetcherOutput(t *testing.T) {
	imageURI := "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-repo:latest"
	task := ecstypes.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/task-abc123"),
		ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme:1"),
		LastStatus:        aws.String("RUNNING"),
		Containers: []ecstypes.Container{
			{Name: aws.String("acme"), Image: aws.String(imageURI)},
		},
	}

	listClustersAPI := &fakeECSListClustersOnly{clusterArns: []string{"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"}}
	listTasksAPI := &fakeECSListTasksOnly{taskArns: []string{*task.TaskArn}}
	describeTasksAPI := &fakeECSDescribeTasksWithImage{tasks: []ecstypes.Task{task}}

	fetcher := resource.GetPaginatedFetcher("ecs-task")
	ecsFull := &ecsTaskRound2FullFake{listClustersAPI, listTasksAPI, describeTasksAPI}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return fetcher(context.Background(), &awsclient.ServiceClients{ECS: ecsFull}, token)
	})
	if err != nil {
		t.Fatalf("FetchECSTasks returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}

	hasImageField := false
	for _, v := range resources[0].Fields {
		if v == imageURI {
			hasImageField = true
		}
	}
	if !hasImageField {
		t.Fatalf("no Fields value on the real FetchECSTasks output carries the container image URI %q — the fetcher must emit Task.Containers[].Image into Fields (spec ecr.md — image-URI substring cross-ref)", imageURI)
	}

	repo := ecrtypes.Repository{RepositoryName: aws.String("acme-repo")}
	repoRes := resource.Resource{ID: "acme-repo", Name: "acme-repo", RawStruct: repo}
	cache := resource.ResourceCache{"ecs-task": resource.ResourceCacheEntry{Resources: resources}}

	checker := checkerByTarget(t, "ecr", "ecs-task")
	result := checker(context.Background(), nil, repoRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (checkECRECSTask's existing substring match should resolve once the fetcher emits the image URI)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "task-abc123" {
		t.Fatalf("ResourceIDs = %v, want [task-abc123]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 4. checkKinesisLambda / checkMSKLambda — checker-owned ListEventSourceMappings
// call, not lambda-cache Fields["event_source_arn"].
//
// Coordinator decision (milestone/close): the fetcher-side
// FetchLambdaFunctionsPageWithEventSources wiring was reverted — it violated
// the pinned no-N+1-at-fetch contract in aws_lambda_n1_test.go. The correct
// mechanism per docs/resources/kinesis.md §lambda / docs/resources/msk.md
// §lambda: the CHECKER itself issues ONE lambda:ListEventSourceMappings call
// with the EventSourceArn filter set to the open stream/cluster ARN (budget
// rule 7 in docs/related-resources.md), then maps the returned
// FunctionArn/FunctionName entries against the already-loaded lambda cache.
//
// kinesis.md §lambda: "Lambda functions consuming records from this stream
// via event-source mappings... EventSourceMappingConfiguration.EventSourceArn
// == <this stream's StreamARN>."
// msk.md §lambda: "If the lambda list has not loaded its event-source
// mappings yet, call ListEventSourceMappings(EventSourceArn=<ClusterArn>)
// once per cluster."
//
// Each fake asserts it was called with the correct EventSourceArn filter —
// an unfiltered (nil FunctionName... i.e. nil EventSourceArn) call returns
// an error, so a checker that fails to pass the filter observably fails
// rather than silently succeeding against a lenient fake.
// ---------------------------------------------------------------------------

type fakeLambdaListEventSourceMappingsByArn struct {
	awsclient.LambdaAPI
	wantEventSourceArn string
	mappings           []lambdatypes.EventSourceMappingConfiguration
	err                error
	calls              int
}

func (f *fakeLambdaListEventSourceMappingsByArn) ListEventSourceMappings(_ context.Context, params *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	f.calls++
	if params.EventSourceArn == nil || *params.EventSourceArn == "" {
		return nil, fmt.Errorf("ListEventSourceMappings called without an EventSourceArn filter")
	}
	if *params.EventSourceArn != f.wantEventSourceArn {
		return nil, fmt.Errorf("ListEventSourceMappings called with EventSourceArn %q, want %q", *params.EventSourceArn, f.wantEventSourceArn)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: f.mappings}, nil
}

func TestKinesis_Related_Lambda_ResolvesViaListEventSourceMappingsFilteredByStreamArn(t *testing.T) {
	streamARN := "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	streamRes := resource.Resource{
		ID:     "clickstream-ingest",
		Name:   "clickstream-ingest",
		Fields: map[string]string{"stream_arn": streamARN},
	}

	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"
	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: streamARN,
		mappings: []lambdatypes.EventSourceMappingConfiguration{
			{FunctionArn: aws.String(fnArn)},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "data-pipeline-transform", Name: "data-pipeline-transform", Fields: map[string]string{"arn": fnArn}},
			},
		},
	}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec kinesis.md §lambda — one ListEventSourceMappings(EventSourceArn=<StreamARN>) call, mapped against the lambda cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "data-pipeline-transform" {
		t.Fatalf("ResourceIDs = %v, want [data-pipeline-transform]", result.ResourceIDs())
	}
	if fake.calls != 1 {
		t.Fatalf("ListEventSourceMappings called %d times, want exactly 1 (budget rule 7 — one call per open stream)", fake.calls)
	}
}

func TestKinesis_Related_Lambda_NoMappingsReturnsZero(t *testing.T) {
	streamARN := "arn:aws:kinesis:us-east-1:123456789012:stream/idle-stream"
	streamRes := resource.Resource{
		ID:     "idle-stream",
		Name:   "idle-stream",
		Fields: map[string]string{"stream_arn": streamARN},
	}

	fake := &fakeLambdaListEventSourceMappingsByArn{wantEventSourceArn: streamARN}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "unrelated-fn", Name: "unrelated-fn", Fields: map[string]string{"arn": "arn:aws:lambda:us-east-1:123456789012:function:unrelated-fn"}},
			},
		},
	}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, cache)

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 when ListEventSourceMappings returns no mappings for this stream", result.Count())
	}
}

func TestKinesis_Related_Lambda_APIErrorSetsErrAndNegativeCount(t *testing.T) {
	streamARN := "arn:aws:kinesis:us-east-1:123456789012:stream/broken-stream"
	streamRes := resource.Resource{
		ID:     "broken-stream",
		Name:   "broken-stream",
		Fields: map[string]string{"stream_arn": streamARN},
	}

	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: streamARN,
		err:                fmt.Errorf("AccessDeniedException: not authorized"),
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Fatalf("Count = %d, want -1 on ListEventSourceMappings API error (matches sibling checker error convention in this file, e.g. checkKinesisCFN/checkKinesisKMS)", result.Count())
	}
	if result.Err() == nil {
		t.Fatalf("Err = nil, want non-nil on ListEventSourceMappings API error")
	}
}

func TestMSK_Related_Lambda_ResolvesViaListEventSourceMappingsFilteredByClusterArn(t *testing.T) {
	clusterArn := "arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/aaaa-bbbb"
	clusterRes := resource.Resource{
		ID:        "acme-events-prod",
		Name:      "acme-events-prod",
		RawStruct: kafkatypes.Cluster{ClusterArn: aws.String(clusterArn)},
	}

	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"
	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: clusterArn,
		mappings: []lambdatypes.EventSourceMappingConfiguration{
			{FunctionArn: aws.String(fnArn)},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "data-pipeline-transform", Name: "data-pipeline-transform", Fields: map[string]string{"arn": fnArn}},
			},
		},
	}

	checker := checkerByTarget(t, "msk", "lambda")
	result := checker(context.Background(), clients, clusterRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec msk.md §lambda — call ListEventSourceMappings(EventSourceArn=<ClusterArn>) once per cluster, mapped against the lambda cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "data-pipeline-transform" {
		t.Fatalf("ResourceIDs = %v, want [data-pipeline-transform]", result.ResourceIDs())
	}
	if fake.calls != 1 {
		t.Fatalf("ListEventSourceMappings called %d times, want exactly 1 (budget rule 7 — one call per open cluster)", fake.calls)
	}
}

func TestMSK_Related_Lambda_NoMappingsReturnsZero(t *testing.T) {
	clusterArn := "arn:aws:kafka:us-east-1:123456789012:cluster/idle-cluster/cccc-dddd"
	clusterRes := resource.Resource{
		ID:        "idle-cluster",
		Name:      "idle-cluster",
		RawStruct: kafkatypes.Cluster{ClusterArn: aws.String(clusterArn)},
	}

	fake := &fakeLambdaListEventSourceMappingsByArn{wantEventSourceArn: clusterArn}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "unrelated-fn", Name: "unrelated-fn", Fields: map[string]string{"arn": "arn:aws:lambda:us-east-1:123456789012:function:unrelated-fn"}},
			},
		},
	}

	checker := checkerByTarget(t, "msk", "lambda")
	result := checker(context.Background(), clients, clusterRes, cache)

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 when ListEventSourceMappings returns no mappings for this cluster", result.Count())
	}
}

func TestMSK_Related_Lambda_APIErrorSetsErrAndNegativeCount(t *testing.T) {
	clusterArn := "arn:aws:kafka:us-east-1:123456789012:cluster/broken-cluster/eeee-ffff"
	clusterRes := resource.Resource{
		ID:        "broken-cluster",
		Name:      "broken-cluster",
		RawStruct: kafkatypes.Cluster{ClusterArn: aws.String(clusterArn)},
	}

	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: clusterArn,
		err:                fmt.Errorf("ThrottlingException: rate exceeded"),
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	checker := checkerByTarget(t, "msk", "lambda")
	result := checker(context.Background(), clients, clusterRes, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Fatalf("Count = %d, want -1 on ListEventSourceMappings API error (matches sibling checker error convention in this file, e.g. checkMSKCFN/checkMSKVPC)", result.Count())
	}
	if result.Err() == nil {
		t.Fatalf("Err = nil, want non-nil on ListEventSourceMappings API error")
	}
}

// ---------------------------------------------------------------------------
// 5. checkLogsECSTask — logs.md family-from-task_definition cross-ref.
//
// checkLogsECSTask (core/aws/logs_related.go:149-166) extracts a family
// substring from the log group name and checks it against cached ecs-task
// ID/Name — but FetchECSTasks always sets task ID/Name to the bare task UUID
// (core/aws/ecs_task.go:81-85), never the family. The correct mechanism
// extracts family:revision from Fields["task_definition"]
// (arn:...:task-definition/<family>:<rev>, core/aws/ecs_task.go:99/134)
// and matches on that instead of the bare UUID.
// ---------------------------------------------------------------------------

func TestLogs_Related_ECSTask_MatchesFamilyFromTaskDefinitionField(t *testing.T) {
	logRes := resource.Resource{ID: "/ecs/checkout/prod", Name: "/ecs/checkout/prod"}

	taskRes := resource.Resource{
		ID:   "9f8e7d6c-1234-4abc-9def-0123456789ab",
		Name: "9f8e7d6c-1234-4abc-9def-0123456789ab",
		Fields: map[string]string{
			"task_id":         "9f8e7d6c-1234-4abc-9def-0123456789ab",
			"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/checkout:7",
		},
	}

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}

	checker := checkerByTarget(t, "logs", "ecs-task")
	result := checker(context.Background(), nil, logRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (spec logs.md — family must come from Fields[task_definition], not the bare task UUID)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != taskRes.ID {
		t.Fatalf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), taskRes.ID)
	}
}

// ---------------------------------------------------------------------------
// 6. checkPipelineEbRule — pipeline.md ARN construction, no extra call.
//
// checkPipelineEbRule (core/aws/pipeline_related.go:300) already calls
// eventbridge:ListRuleNamesByTarget correctly, but reads res.Fields["arn"],
// a key FetchCodePipelinesPage (core/aws/pipeline.go:74-80) never
// populates (only name/pipeline_type/created/updated/version are set). The
// correct mechanism constructs the ARN
// (arn:aws:codepipeline:<region>:<account>:<name> — AWS CodePipeline
// resource ARNs have no "pipeline/" segment, unlike e.g. IAM policy ARNs)
// with no extra API call, since region+account are already known from the
// client context.
// ---------------------------------------------------------------------------

// The fake client for ListRuleNamesByTarget now lives in
// fakes_eventbridge_test.go (fakeEventBridgeAPI) — see that file's header
// for the one-fake-per-interface convention.

type fakeCodePipelineListPipelinesOnly struct {
	awsclient.CodePipelineAPI
	pipelines []cptypes.PipelineSummary
}

func (f *fakeCodePipelineListPipelinesOnly) ListPipelines(_ context.Context, _ *codepipeline.ListPipelinesInput, _ ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	return &codepipeline.ListPipelinesOutput{Pipelines: f.pipelines}, nil
}

func TestPipeline_Related_EbRule_ResolvesViaRealFetcherOutput(t *testing.T) {
	pipelineARN := "arn:aws:codepipeline:us-east-1:123456789012:checkout-deploy"

	listAPI := &fakeCodePipelineListPipelinesOnly{
		pipelines: []cptypes.PipelineSummary{{Name: aws.String("checkout-deploy")}},
	}

	identity := session.NewIdentityStore()
	identity.Set("123456789012", nil)
	fetchClients := &awsclient.ServiceClients{CodePipeline: listAPI, Region: "us-east-1"}
	fetchClients.SetIdentityStore(identity)

	fetchResult, err := awsclient.FetchCodePipelinesPageWithClients(context.Background(), fetchClients, "")
	if err != nil {
		t.Fatalf("FetchCodePipelinesPageWithClients returned error: %v", err)
	}
	if len(fetchResult.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(fetchResult.Resources))
	}
	if got := fetchResult.Resources[0].Fields["arn"]; got != pipelineARN {
		t.Fatalf("Fields[arn] = %q, want %q — the pipeline fetcher must construct the ARN with no \"pipeline/\" segment (region+account are already known from the client context; AWS CodePipeline resource ARN format)", got, pipelineARN)
	}

	fake := &fakeEventBridgeAPI{
		RuleNamesByTargetArn: map[string][]string{pipelineARN: {"checkout-deploy-trigger"}},
	}
	clients := &awsclient.ServiceClients{EventBridge: fake}

	checker := checkerByTarget(t, "pipeline", "eb-rule")
	result := checker(context.Background(), clients, fetchResult.Resources[0], resource.ResourceCache{})

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec pipeline.md — Fields[arn] must be populated so ListRuleNamesByTarget resolves)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "checkout-deploy-trigger" {
		t.Fatalf("ResourceIDs = %v, want [checkout-deploy-trigger]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 7. checkSQSKMS — sqs.md GetQueueAttributes KmsMasterKeyId, no extra call.
//
// FetchSQSQueuesPage (core/aws/sqs.go:64-71) already calls
// GetQueueAttributes with AttributeNameAll, which returns KmsMasterKeyId in
// the attrs map — but the fetcher (core/aws/sqs.go:93-100) never copies
// that value into Fields["kms_key_id"]. checkSQSKMS reads that missing key
// and permanently returns State: RelatedUnknown / Count:0. This test drives the real fetch path
// with a fake GetQueueAttributes response carrying KmsMasterKeyId and proves
// the field must reach Fields, then the kms cross-ref resolves.
// ---------------------------------------------------------------------------

type fakeSQSListQueuesOnly struct {
	urls []string
}

func (f *fakeSQSListQueuesOnly) ListQueues(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	return &sqs.ListQueuesOutput{QueueUrls: f.urls}, nil
}

type fakeSQSGetQueueAttributesWithKMS struct {
	byURL map[string]map[string]string
}

func (f *fakeSQSGetQueueAttributesWithKMS) GetQueueAttributes(_ context.Context, params *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	if params.QueueUrl == nil {
		return &sqs.GetQueueAttributesOutput{}, nil
	}
	attrs, ok := f.byURL[*params.QueueUrl]
	if !ok {
		return &sqs.GetQueueAttributesOutput{}, nil
	}
	return &sqs.GetQueueAttributesOutput{Attributes: attrs}, nil
}

func TestSQS_Related_KMS_ResolvesFromGetQueueAttributesKmsMasterKeyId(t *testing.T) {
	queueURL := "https://sqs.us-east-1.amazonaws.com/123456789012/checkout-events"
	keyID := "a1b2c3d4-5678-90ab-cdef-111111111111"

	listAPI := &fakeSQSListQueuesOnly{urls: []string{queueURL}}
	attrAPI := &fakeSQSGetQueueAttributesWithKMS{
		byURL: map[string]map[string]string{
			queueURL: {
				"QueueArn":       "arn:aws:sqs:us-east-1:123456789012:checkout-events",
				"KmsMasterKeyId": keyID,
			},
		},
	}

	result, err := awsclient.FetchSQSQueuesPage(context.Background(), listAPI, attrAPI, "")
	if err != nil {
		t.Fatalf("FetchSQSQueuesPage returned error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(result.Resources))
	}
	got := result.Resources[0].Fields["kms_key_id"]
	if got != keyID {
		t.Fatalf("Fields[kms_key_id] = %q, want %q (sqs.go must copy GetQueueAttributes' KmsMasterKeyId into Fields)", got, keyID)
	}

	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: keyID, Name: keyID}},
		},
	}
	checker := checkerByTarget(t, "sqs", "kms")
	checkResult := checker(context.Background(), nil, result.Resources[0], cache)
	if checkResult.Count() != 1 {
		t.Fatalf("checkSQSKMS Count = %d, want 1 once kms_key_id is populated", checkResult.Count())
	}
}

// ---------------------------------------------------------------------------
// 8. checkSubnetASG / checkSubnetEKS — reverse cross-ref field keys.
//
// checkSubnetASG (core/aws/subnet_related.go:236) reads
// asgRes.Fields["vpc_zone_identifier"] / ["subnets"], but
// FetchAutoScalingGroupsPage (core/aws/asg.go) never writes either key
// (only asg_name/min_size/.../suspended_processes are set) — the sibling
// forward checker checkASGSubnets reads RawStruct.VPCZoneIdentifier
// directly and works fine, but this reverse checker only looks at Fields.
// checkSubnetEKS (core/aws/subnet_related.go:283) has the identical bug
// for Fields["subnets"]/["subnet_ids"] against buildEKSResource
// (core/aws/eks.go), whose Fields never carry subnet IDs (they live on
// RawStruct.ResourcesVpcConfig.SubnetIds).
// ---------------------------------------------------------------------------

type fakeASGDescribeAutoScalingGroupsOnly struct {
	groups []asgtypes.AutoScalingGroup
}

func (f *fakeASGDescribeAutoScalingGroupsOnly) DescribeAutoScalingGroups(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: f.groups}, nil
}

func TestSubnet_Related_ASG_ResolvesViaRealFetcherOutput(t *testing.T) {
	subnetRes := resource.Resource{ID: "subnet-0abc123", Name: "subnet-0abc123"}

	asg := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("checkout-asg"),
		VPCZoneIdentifier:    aws.String("subnet-0abc123,subnet-0def456"),
	}
	listAPI := &fakeASGDescribeAutoScalingGroupsOnly{groups: []asgtypes.AutoScalingGroup{asg}}

	fetchResult, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), listAPI, "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage returned error: %v", err)
	}
	if len(fetchResult.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(fetchResult.Resources))
	}

	hasSubnetField := fetchResult.Resources[0].Fields["vpc_zone_identifier"] != "" || fetchResult.Resources[0].Fields["subnets"] != ""
	if !hasSubnetField {
		t.Fatalf("neither Fields[vpc_zone_identifier] nor Fields[subnets] is populated on real FetchAutoScalingGroupsPage output — checkSubnetASG (core/aws/subnet_related.go:236) can never resolve without one of them")
	}

	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: fetchResult.Resources},
	}

	checker := checkerByTarget(t, "subnet", "asg")
	result := checker(context.Background(), nil, subnetRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (spec — asg fetcher must populate Fields[vpc_zone_identifier] so the reverse subnet:asg checker resolves)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "checkout-asg" {
		t.Fatalf("ResourceIDs = %v, want [checkout-asg]", result.ResourceIDs())
	}
}

type fakeEKSAPIForSubnetTest struct {
	awsclient.EKSAPI
	clusters map[string]ekstypes.Cluster
}

func (f *fakeEKSAPIForSubnetTest) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	names := make([]string, 0, len(f.clusters))
	for name := range f.clusters {
		names = append(names, name)
	}
	return &eks.ListClustersOutput{Clusters: names}, nil
}

func (f *fakeEKSAPIForSubnetTest) DescribeCluster(_ context.Context, params *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	if params.Name == nil {
		return &eks.DescribeClusterOutput{}, nil
	}
	c, ok := f.clusters[*params.Name]
	if !ok {
		return &eks.DescribeClusterOutput{}, nil
	}
	return &eks.DescribeClusterOutput{Cluster: &c}, nil
}

func TestSubnet_Related_EKS_ResolvesViaRealFetcherOutput(t *testing.T) {
	subnetRes := resource.Resource{ID: "subnet-0abc123", Name: "subnet-0abc123"}

	cluster := ekstypes.Cluster{
		Name: aws.String("acme-eks-cluster"),
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			SubnetIds: []string{"subnet-0abc123", "subnet-0def456"},
		},
	}
	fake := &fakeEKSAPIForSubnetTest{clusters: map[string]ekstypes.Cluster{"acme-eks-cluster": cluster}}
	clients := &awsclient.ServiceClients{EKS: fake}

	fetchResult, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("FetchEKSClustersPage returned error: %v", err)
	}
	if len(fetchResult.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(fetchResult.Resources))
	}

	hasSubnetField := fetchResult.Resources[0].Fields["subnets"] != "" || fetchResult.Resources[0].Fields["subnet_ids"] != ""
	if !hasSubnetField {
		t.Fatalf("neither Fields[subnets] nor Fields[subnet_ids] is populated on real FetchEKSClustersPage output — checkSubnetEKS (core/aws/subnet_related.go:283) can never resolve without one of them")
	}

	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: fetchResult.Resources},
	}

	checker := checkerByTarget(t, "subnet", "eks")
	result := checker(context.Background(), nil, subnetRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (spec — eks fetcher must populate Fields[subnet_ids] so the reverse subnet:eks checker resolves)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "acme-eks-cluster" {
		t.Fatalf("ResourceIDs = %v, want [acme-eks-cluster]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 9. checkASGRole / checkECRRole — bare role name, not full ARN.
//
// checkASGRole (core/aws/asg_related.go:224-250) and checkECRRole ->
// ecrPolicyRoleARNs (core/aws/ecr_related_extra.go:183-211) both return
// full role ARNs verbatim, unlike every sibling role-pivot checker
// (checkEC2Role, core/aws/ec2_related.go:497-498) which strips to the
// bare name after the last "/". FetchRolesByIDs
// (core/aws/iam_roles.go:173) calls iam:GetRole(RoleName: id), which AWS
// requires to be a bare name — a full-ARN ResourceID 404s when drilled into.
// ---------------------------------------------------------------------------

func TestASG_Related_Role_ReturnsBareRoleName(t *testing.T) {
	asg := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("checkout-asg"),
		ServiceLinkedRoleARN: aws.String("arn:aws:iam::123456789012:role/aws-service-role/autoscaling.amazonaws.com/AWSServiceRoleForAutoScaling"),
	}
	asgRes := resource.Resource{ID: "checkout-asg", Name: "checkout-asg", RawStruct: asg}

	checker := checkerByTarget(t, "asg", "role")
	result := checker(context.Background(), nil, asgRes, resource.ResourceCache{})

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1", result.Count())
	}
	for _, id := range result.ResourceIDs() {
		if id != "AWSServiceRoleForAutoScaling" {
			t.Fatalf("ResourceIDs = %v, want bare role name [AWSServiceRoleForAutoScaling] (iam:GetRole requires RoleName, not an ARN, per FetchRolesByIDs / core/aws/iam_roles.go:173) — got full ARN %q", result.ResourceIDs(), id)
		}
	}
}

func TestECR_Related_Role_ReturnsBareRoleNameFromPolicy(t *testing.T) {
	repo := ecrtypes.Repository{RepositoryName: aws.String("checkout-service")}
	repoRes := resource.Resource{ID: "checkout-service", Name: "checkout-service", RawStruct: repo}

	policyText := `{"Statement":[{"Principal":{"AWS":"arn:aws:iam::123456789012:role/ci-deploy-role"}}]}`
	fake := &fakeECRGetRepositoryPolicy{policyText: policyText}
	clients := &awsclient.ServiceClients{ECR: fake}

	checker := checkerByTarget(t, "ecr", "role")
	result := checker(context.Background(), clients, repoRes, resource.ResourceCache{})

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1", result.Count())
	}
	for _, id := range result.ResourceIDs() {
		if id != "ci-deploy-role" {
			t.Fatalf("ResourceIDs = %v, want bare role name [ci-deploy-role] (iam:GetRole requires RoleName, not an ARN) — got full ARN %q", result.ResourceIDs(), id)
		}
	}
}

// TestECR_Related_Role_DropsCrossAccountPrincipal locks the cross-account
// filter: an ECR repository policy principal in a different account than the
// repository owner (RegistryId) is not fetchable via iam:GetRole here and must
// be dropped, leaving only the owner-account role as a bare name.
func TestECR_Related_Role_DropsCrossAccountPrincipal(t *testing.T) {
	repo := ecrtypes.Repository{
		RepositoryName: aws.String("checkout-service"),
		RegistryId:     aws.String("111111111111"),
	}
	repoRes := resource.Resource{ID: "checkout-service", Name: "checkout-service", RawStruct: repo}

	policyText := `{"Statement":[{"Principal":{"AWS":[` +
		`"arn:aws:iam::111111111111:role/local-ci-role",` +
		`"arn:aws:iam::999999999999:role/foreign-ci-role"` +
		`]}}]}`
	fake := &fakeECRGetRepositoryPolicy{policyText: policyText}
	clients := &awsclient.ServiceClients{ECR: fake}

	checker := checkerByTarget(t, "ecr", "role")
	result := checker(context.Background(), clients, repoRes, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (owner-account role only; cross-account dropped)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "local-ci-role" {
		t.Fatalf("ResourceIDs = %v, want [local-ci-role] (foreign-account foreign-ci-role excluded)", result.ResourceIDs())
	}
}

type fakeECRGetRepositoryPolicy struct {
	awsclient.ECRAPI
	policyText string
}

func (f *fakeECRGetRepositoryPolicy) GetRepositoryPolicy(_ context.Context, _ *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	return &ecr.GetRepositoryPolicyOutput{PolicyText: aws.String(f.policyText)}, nil
}

// ---------------------------------------------------------------------------
// 10. checkGlueCFN — region resolves from clients/config, not env; account
// comes from the session-scoped identity store, not a live STS call.
//
// checkGlueCFN resolves region from c.Region rather than os.Getenv, so it
// is immune to AWS_REGION/AWS_DEFAULT_REGION being unset in this test
// process. The session names the region here because a session that recorded
// none declines rather than falling back to the ambient config (aws5 row 2);
// the empty-Region form this test used pinned that fallback and must not be
// restored. The remaining dependency is accountIDFromClients, which reads
// c.IdentityStore() — this test seeds that store directly (the honest
// in-session path: STS GetCallerIdentity is memoized there for the
// lifetime of one Session) instead of relying on a live STS call or a
// production fallback constant.
// ---------------------------------------------------------------------------

type fakeGlueGetTags struct {
	awsclient.GlueAPI
	byArn map[string]map[string]string
}

func (f *fakeGlueGetTags) GetTags(_ context.Context, params *glue.GetTagsInput, _ ...func(*glue.Options)) (*glue.GetTagsOutput, error) {
	if params.ResourceArn == nil {
		return &glue.GetTagsOutput{}, nil
	}
	tags, ok := f.byArn[*params.ResourceArn]
	if !ok {
		return &glue.GetTagsOutput{}, nil
	}
	return &glue.GetTagsOutput{Tags: tags}, nil
}

func TestGlue_Related_CFN_ResolvesRegionWithoutEnvVar(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	jobRes := resource.Resource{ID: "etl-nightly-job", Name: "etl-nightly-job"}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "data-pipeline-stack", Name: "data-pipeline-stack", Fields: map[string]string{"stack_name": "data-pipeline-stack"}},
			},
		},
	}

	fake := &fakeGlueGetTags{
		byArn: map[string]map[string]string{
			"arn:aws:glue:us-east-1:123456789012:job/etl-nightly-job": {"aws:cloudformation:stack-name": "data-pipeline-stack"},
		},
	}
	clients := &awsclient.ServiceClients{Region: "us-east-1", Glue: fake}
	identity := session.NewIdentityStore()
	identity.Set("123456789012", nil)
	clients.SetIdentityStore(identity)

	checker := checkerByTarget(t, "glue", "cfn")
	result := checker(context.Background(), clients, jobRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (region must be resolved from clients/config, not AWS_REGION env var — got State: RelatedUnknown via regionFromEnv())", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "data-pipeline-stack" {
		t.Fatalf("ResourceIDs = %v, want [data-pipeline-stack]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 11. checkWAFCF — CLOUDFRONT-scope resolves; REGIONAL keeps returning 0.
//
// checkWAFCF (core/aws/waf_related.go:150-186) already guards on
// Fields["scope"] == CLOUDFRONT and calls
// cloudfront:ListDistributionsByWebACLId correctly — but
// FetchWAFWebACLsPage (core/aws/waf.go:36,85) hardcodes both the
// ListWebACLs request and every result's Fields["scope"] to
// wafv2types.ScopeRegional, so no WAF resource can ever carry
// scope=CLOUDFRONT today. This test constructs a CLOUDFRONT-scope resource
// directly (as the fixed two-scope fetcher will) to prove the existing
// checker mechanism resolves once such a fixture exists, and confirms the
// REGIONAL case is unaffected (still 0).
// ---------------------------------------------------------------------------

// The fake CloudFront client for these tests now lives in
// fakes_cloudfront_test.go (fakeCloudFrontAPI) — see that file's header for
// the one-fake-per-interface convention.

func TestWAF_Related_CF_CloudfrontScopeResolvesDistribution(t *testing.T) {
	webACLID := "acl-1234abcd"
	waf := resource.Resource{
		ID:     webACLID,
		Name:   "cdn-protection",
		Fields: map[string]string{"scope": string(wafv2types.ScopeCloudfront), "id": webACLID},
	}

	fake := &fakeCloudFrontAPI{
		DistByWebACLID: map[string][]string{webACLID: {"E1234567890ABC"}},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}

	checker := checkerByTarget(t, "waf", "cf")
	result := checker(context.Background(), clients, waf, resource.ResourceCache{})

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 for a CLOUDFRONT-scope Web ACL with a bound distribution", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "E1234567890ABC" {
		t.Fatalf("ResourceIDs = %v, want [E1234567890ABC]", result.ResourceIDs())
	}
}

func TestWAF_Related_CF_RegionalScopeStaysZero(t *testing.T) {
	waf := resource.Resource{
		ID:     "acl-regional",
		Name:   "api-protection",
		Fields: map[string]string{"scope": string(wafv2types.ScopeRegional), "id": "acl-regional"},
	}

	fake := &fakeCloudFrontAPI{
		DistByWebACLID: map[string][]string{"acl-regional": {"SHOULD-NOT-BE-RETURNED"}},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}

	checker := checkerByTarget(t, "waf", "cf")
	result := checker(context.Background(), clients, waf, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 for a REGIONAL-scope Web ACL (CloudFront can only bind CLOUDFRONT-scope ACLs)", result.Count())
	}
}

// TestWAF_Related_CF_PassesFullARNNotBareID pins the docs/resources/waf.md
// contract that checkWAFCF must call cloudfront:ListDistributionsByWebACLId
// with the Web ACL's full ARN (res.Fields["arn"]), not the bare WebACL Id
// (res.ID / res.Fields["id"]). The fake only answers to the exact ARN, so a
// regression that reverts to passing the bare ID observably returns Count:0
// here even though a real distribution is bound to the ACL.
func TestWAF_Related_CF_PassesFullARNNotBareID(t *testing.T) {
	const webACLID = "acl-abcdef01"
	const webACLArn = "arn:aws:wafv2:us-east-1:123456789012:global/webacl/cdn-protection/acl-abcdef01"

	waf := resource.Resource{
		ID:   webACLID,
		Name: "cdn-protection",
		Fields: map[string]string{
			"scope": string(wafv2types.ScopeCloudfront),
			"id":    webACLID,
			"arn":   webACLArn,
		},
	}

	fake := &fakeCloudFrontAPI{
		WantWebACLArn:  webACLArn,
		DistByWebACLID: map[string][]string{webACLArn: {"EARNMATCH0001"}},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}

	checker := checkerByTarget(t, "waf", "cf")
	result := checker(context.Background(), clients, waf, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 — checkWAFCF must pass the full WebACL ARN (Fields[\"arn\"]), not the bare ID, to ListDistributionsByWebACLId", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "EARNMATCH0001" {
		t.Fatalf("ResourceIDs = %v, want [EARNMATCH0001]", result.ResourceIDs())
	}
}

// silence unused-import guard for wafv2 (the checker test above only needs
// wafv2types, but the checker's own file imports the wafv2 client package —
// referenced here to keep the import list intentional if future tests in
// this file need to construct wafv2 API inputs directly).
var _ = wafv2.ListWebACLsInput{}

// ---------------------------------------------------------------------------
// 12. checkEIPECS / checkEIPECSSvc — zero-call ENI cross-ref.
//
// checkEIPECS/ECSSvc/ECSTask (core/aws/eip_related.go:196-220) are all
// hardcoded to return State: RelatedUnknown whenever res.ID != "" — each function's own
// comment claims resolving requires per-cluster DescribeTasks, "outside the
// 1-call budget." But the EIP's NetworkInterfaceId (already on
// ec2types.Address, used by checkEIPENI) can be cross-referenced against the
// already-loaded ecs-task cache's Attachments[].Details networkInterfaceId
// with zero extra API calls — the same reverse-scan-over-loaded-cache
// pattern used throughout this package (e.g. checkEFSSubnet).
// ---------------------------------------------------------------------------

func TestEIP_Related_ECSTask_MatchesViaNetworkInterfaceId(t *testing.T) {
	eip := ec2types.Address{
		AllocationId:       aws.String("eipalloc-0abc123"),
		NetworkInterfaceId: aws.String("eni-0a1b2c3d"),
	}
	eipRes := resource.Resource{ID: "eipalloc-0abc123", Name: "eipalloc-0abc123", RawStruct: eip}

	task := ecstypes.Task{
		TaskArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/jkl012"),
		ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-0a1b2c3d")},
				},
			},
		},
	}
	taskRes := resource.Resource{ID: "jkl012", Name: "jkl012", RawStruct: task}

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}

	checker := checkerByTarget(t, "eip", "ecs-task")
	result := checker(context.Background(), nil, eipRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (zero-call ENI cross-ref against the already-loaded ecs-task cache; checker is a hardcoded -1 stub today)", result.Count())
	}
}

func TestEIP_Related_ECS_MatchesViaTaskClusterArn(t *testing.T) {
	eip := ec2types.Address{
		AllocationId:       aws.String("eipalloc-0abc123"),
		NetworkInterfaceId: aws.String("eni-0a1b2c3d"),
	}
	eipRes := resource.Resource{ID: "eipalloc-0abc123", Name: "eipalloc-0abc123", RawStruct: eip}

	task := ecstypes.Task{
		TaskArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/jkl012"),
		ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-0a1b2c3d")},
				},
			},
		},
	}
	taskRes := resource.Resource{ID: "jkl012", Name: "jkl012", RawStruct: task}

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
		"ecs":      resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "prod-cluster", Name: "prod-cluster"}}},
	}

	checker := checkerByTarget(t, "eip", "ecs")
	result := checker(context.Background(), nil, eipRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (zero-call cross-ref via the matching task's ClusterArn; checker is a hardcoded -1 stub today)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// 13. checkR53Logs — ListQueryLoggingConfigs cross-ref.
//
// checkR53Logs (core/aws/r53_related.go:314-319) is hardcoded to return
// State: RelatedUnknown whenever res.ID != "" — its own comment states
// route53:ListQueryLoggingConfigs "is not in Route53API yet." The correct
// mechanism issues one ListQueryLoggingConfigs call per open zone and
// matches CloudWatchLogsLogGroupArn against the logs cache.
// ---------------------------------------------------------------------------

type fakeRoute53ListQueryLoggingConfigs struct {
	awsclient.Route53API
	byZoneID map[string]string
}

func (f *fakeRoute53ListQueryLoggingConfigs) ListQueryLoggingConfigs(_ context.Context, params *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	if params.HostedZoneId == nil {
		return &route53.ListQueryLoggingConfigsOutput{}, nil
	}
	arn, ok := f.byZoneID[*params.HostedZoneId]
	if !ok {
		return &route53.ListQueryLoggingConfigsOutput{}, nil
	}
	return &route53.ListQueryLoggingConfigsOutput{
		QueryLoggingConfigs: []route53types.QueryLoggingConfig{
			{HostedZoneId: params.HostedZoneId, CloudWatchLogsLogGroupArn: aws.String(arn)},
		},
	}, nil
}

func TestR53_Related_Logs_ResolvesViaListQueryLoggingConfigs(t *testing.T) {
	zoneID := "Z1234567890ABC"
	zoneRes := resource.Resource{ID: zoneID, Name: "example.com."}

	logGroupArn := "arn:aws:logs:us-east-1:123456789012:log-group:/aws/route53/example.com"
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "/aws/route53/example.com", Name: "/aws/route53/example.com", Fields: map[string]string{"arn": logGroupArn}},
			},
		},
	}

	fake := &fakeRoute53ListQueryLoggingConfigs{byZoneID: map[string]string{zoneID: logGroupArn}}
	clients := &awsclient.ServiceClients{Route53: fake}

	checker := checkerByTarget(t, "r53", "logs")
	result := checker(context.Background(), clients, zoneRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec — one ListQueryLoggingConfigs call per open zone, matching CloudWatchLogsLogGroupArn)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "/aws/route53/example.com" {
		t.Fatalf("ResourceIDs = %v, want [/aws/route53/example.com]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 14. checkVPCER53 — ListHostedZonesByVPC cross-ref.
//
// checkVPCER53 (core/aws/vpce_related.go:197-202) is hardcoded to
// return State: RelatedUnknown whenever res.ID != "" — its own comment states the
// associated-zones list lives on route53:ListHostedZonesByVPC, "not in the
// r53 hosted-zone cache." The correct mechanism issues one
// ListHostedZonesByVPC call per open endpoint and matches the returned
// zones against the r53 cache.
// ---------------------------------------------------------------------------

type fakeRoute53ListHostedZonesByVPC struct {
	awsclient.Route53API
	byVpcID map[string][]route53types.HostedZoneSummary
}

func (f *fakeRoute53ListHostedZonesByVPC) ListHostedZonesByVPC(_ context.Context, params *route53.ListHostedZonesByVPCInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error) {
	if params.VPCId == nil {
		return &route53.ListHostedZonesByVPCOutput{}, nil
	}
	return &route53.ListHostedZonesByVPCOutput{HostedZoneSummaries: f.byVpcID[*params.VPCId]}, nil
}

func TestVPCE_Related_R53_ResolvesViaListHostedZonesByVPC(t *testing.T) {
	vpcID := "vpc-0abc123"
	vpce := ec2types.VpcEndpoint{
		VpcEndpointId:     aws.String("vpce-0abc123"),
		VpcId:             aws.String(vpcID),
		PrivateDnsEnabled: aws.Bool(true),
	}
	vpceRes := resource.Resource{ID: "vpce-0abc123", Name: "vpce-0abc123", RawStruct: vpce}

	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "Z1234567890ABC", Name: "internal.acme.local."},
			},
		},
	}

	fake := &fakeRoute53ListHostedZonesByVPC{
		byVpcID: map[string][]route53types.HostedZoneSummary{
			vpcID: {{HostedZoneId: aws.String("Z1234567890ABC"), Name: aws.String("internal.acme.local.")}},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}

	checker := checkerByTarget(t, "vpce", "r53")
	result := checker(context.Background(), clients, vpceRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (spec — one ListHostedZonesByVPC call per open endpoint, matched against the r53 cache)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "Z1234567890ABC" {
		t.Fatalf("ResourceIDs = %v, want [Z1234567890ABC]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// 15. checkSubnetEFS — zero-call ENI cross-ref (reverse of checkEFSSubnet).
//
// checkSubnetEFS (core/aws/subnet_related.go:258-263) is hardcoded to
// return State: RelatedUnknown whenever res.ID != "" — its own comment claims mount
// targets require per-file-system DescribeMountTargets, "outside the 1-call
// budget." But checkEFSSubnet (core/aws/efs_related.go:141-177) already
// solves the exact same relationship in reverse with zero extra calls: scan
// the eni cache for mount-target ENIs whose Description contains the
// filesystem ID. This test mirrors that pattern for the subnet->efs
// direction.
// ---------------------------------------------------------------------------

func TestSubnet_Related_EFS_MatchesViaMountTargetENIScan(t *testing.T) {
	subnetRes := resource.Resource{ID: "subnet-0abc123", Name: "subnet-0abc123"}

	eni := ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-0mount01"),
		SubnetId:           aws.String("subnet-0abc123"),
		Description:        aws.String("EFS mount target for fs-0a1b2c3d"),
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "eni-0mount01", Name: "eni-0mount01", RawStruct: eni}}},
		"efs": resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "fs-0a1b2c3d", Name: "fs-0a1b2c3d"}}},
	}

	checker := checkerByTarget(t, "subnet", "efs")
	result := checker(context.Background(), nil, subnetRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (zero-call ENI-description scan mirroring checkEFSSubnet's reverse direction; checker is a hardcoded -1 stub today)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// 16. FetchCloudTrailTrails — trail.md §3.1/§3.2 Findings emission.
//
// docs/attention-signals.md L158 / docs/resources/trail.md:68-92 require
// r.Findings for: LogFileValidationEnabled==false (Warning),
// IsLogging==false (Broken), LatestDeliveryError non-empty (Broken).
// FetchCloudTrailTrails (core/aws/trail.go) computes all three signals
// into Fields but never appends to r.Findings — this is also why "trail" is
// pinned in knownIssueCoverageGaps (isIssueCapable sees a registered Wave-2
// enricher but InFetcherWave2Sentinel unconditionally returns empty
// Findings). The demo fixtures already carry both witnesses:
// security-audit-trail (IsLogging=false) and data-events-trail
// (LatestDeliveryError set) — core/demo/fixtures/cloudtrail.go.
// ---------------------------------------------------------------------------

func TestTrail_Fetcher_EmitsFindingsForDocumentedWave1And2Signals(t *testing.T) {
	clients := demo.NewServiceClients()

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), clients.CloudTrail)
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails returned error: %v", err)
	}

	byName := make(map[string]resource.Resource, len(resources))
	for _, r := range resources {
		byName[r.Name] = r
	}

	notLogging, ok := byName["security-audit-trail"]
	if !ok {
		t.Fatalf("demo fixtures missing security-audit-trail (IsLogging=false witness)")
	}
	if len(notLogging.Findings) == 0 {
		t.Fatalf("security-audit-trail (is_logging=false) has zero Findings — spec trail.md:80-83 requires a Broken finding")
	}
	foundBroken := false
	for _, f := range notLogging.Findings {
		if f.Severity == domain.SevBroken {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Fatalf("security-audit-trail Findings = %+v, want at least one SevBroken finding for is_logging=false", notLogging.Findings)
	}

	deliveryError, ok := byName["data-events-trail"]
	if !ok {
		t.Fatalf("demo fixtures missing data-events-trail (LatestDeliveryError witness)")
	}
	if len(deliveryError.Findings) == 0 {
		t.Fatalf("data-events-trail (latest_delivery_error set) has zero Findings — spec trail.md:85-88 requires a Broken finding")
	}

	logValidationDisabled, ok := byName["legacy-validation-disabled"]
	if !ok {
		t.Fatalf("demo fixtures missing legacy-validation-disabled (LogFileValidationEnabled=false witness)")
	}
	if len(logValidationDisabled.Findings) == 0 {
		t.Fatalf("legacy-validation-disabled (log_file_validation_enabled=false) has zero Findings — spec trail.md:72-74 requires a Warning finding")
	}
}

// TestDemoIssueCoverage_TrailNowHasAFlaggedFixture is the mirror-image
// assertion of a knownIssueCoverageGaps["trail"] allowlist entry: since
// FetchCloudTrailTrails emits Findings for the fixtures above, "trail" is
// issue-capable AND has a flagged demo fixture, so it must not be pinned as
// a known gap.
func TestDemoIssueCoverage_TrailNowHasAFlaggedFixture(t *testing.T) {
	clients := demo.NewServiceClients()
	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), clients.CloudTrail)
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails returned error: %v", err)
	}
	for _, r := range resources {
		if len(r.Findings) > 0 {
			return
		}
	}
	t.Fatalf("no trail demo fixture carries a non-empty Findings list — trail must have at least one flagged fixture now that the fetcher emits Findings")
}
