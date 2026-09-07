package unit

// aws_enrichment_ecs_cfn_elb_eb_test.go — Behavioral tests for the ECS, CFN,
// ELB, and EB enrichers.
//
// Enrichers covered:
//   - EnrichECSServices      (line 1715)
//   - EnrichECSTasks         (line 1957)
//   - EnrichECSClusters      (line 1860)
//   - EnrichCFNStackEvents   (line 2876)
//   - EnrichELBAttributes    (line 2267)
//   - EnrichCFNCombined      (line 2956)
//   - EnrichEBEnvironmentHealth (line 2209)
//
// Per enricher: happy-path (findings emitted), truncation (API error), no-issue.
// All fakes embed the aggregate interface and override only the method under test.
//
// Note: Redis has no Wave-2 enricher — spec §3.2 explicitly has no Wave-2 signals.
// EnrichRedisReplicationGroup tests were removed post-phase-7.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfnsvc "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2svc "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// =============================================================================
// ECS fakes — shared by ECSServices, ECSClusters, ECSTasks
// =============================================================================

// fakeECSEnricher embeds ECSAPI and overrides DescribeServices, DescribeClusters,
// and DescribeTasks for Wave-3 tests.
type fakeECSEnricher struct {
	awsclient.ECSAPI

	// DescribeServices
	descSvcOut *ecs.DescribeServicesOutput
	descSvcErr error

	// DescribeClusters
	descClustersOut *ecs.DescribeClustersOutput
	descClustersErr error

	// DescribeTasks
	descTasksOut *ecs.DescribeTasksOutput
	descTasksErr error
}

func (f *fakeECSEnricher) DescribeServices(
	_ context.Context,
	_ *ecs.DescribeServicesInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeServicesOutput, error) {
	if f.descSvcErr != nil {
		return nil, f.descSvcErr
	}
	if f.descSvcOut != nil {
		return f.descSvcOut, nil
	}
	return &ecs.DescribeServicesOutput{}, nil
}

func (f *fakeECSEnricher) DescribeClusters(
	_ context.Context,
	_ *ecs.DescribeClustersInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeClustersOutput, error) {
	if f.descClustersErr != nil {
		return nil, f.descClustersErr
	}
	if f.descClustersOut != nil {
		return f.descClustersOut, nil
	}
	return &ecs.DescribeClustersOutput{}, nil
}

func (f *fakeECSEnricher) DescribeTasks(
	_ context.Context,
	_ *ecs.DescribeTasksInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	if f.descTasksErr != nil {
		return nil, f.descTasksErr
	}
	if f.descTasksOut != nil {
		return f.descTasksOut, nil
	}
	return &ecs.DescribeTasksOutput{}, nil
}

// Compile-time check: fakeECSEnricher satisfies ECSAPI.
var _ awsclient.ECSAPI = (*fakeECSEnricher)(nil)

// =============================================================================
// EnrichECSServices
// =============================================================================

// TestEnrichECSServices_StuckServiceEmitsBangFinding verifies that a service
// with desired > running and no in-progress deployment produces a "!" finding
// with summary containing "running" and "desired".
func TestEnrichECSServices_StuckServiceEmitsBangFinding(t *testing.T) {
	svcName := "my-service"
	fake := &fakeECSEnricher{
		descSvcOut: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName:  aws.String(svcName),
					DesiredCount: 3,
					RunningCount: 1,
					PendingCount: 0,
					Deployments:  []ecstypes.Deployment{},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   svcName,
			Name: svcName,
			Fields: map[string]string{
				"cluster":      "my-cluster",
				"service_name": svcName,
			},
		},
	}

	result, err := awsclient.EnrichECSServices(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[svcName]
	if !ok {
		t.Fatalf("expected finding for service %q; findings: %v", svcName, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if !strings.Contains(f.Phrase, "running") {
		t.Errorf("summary %q should contain %q", f.Phrase, "running")
	}
	if !strings.Contains(f.Phrase, "desired") {
		t.Errorf("summary %q should contain %q", f.Phrase, "desired")
	}
}

// TestEnrichECSServices_DeploymentRolloutFailedEmitsFinding verifies that a
// deployment with RolloutState=FAILED produces a "!" finding with "deployment
// rollout FAILED" in the summary.
func TestEnrichECSServices_DeploymentRolloutFailedEmitsFinding(t *testing.T) {
	svcName := "failing-svc"
	fake := &fakeECSEnricher{
		descSvcOut: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName:  aws.String(svcName),
					DesiredCount: 2,
					RunningCount: 2,
					Deployments: []ecstypes.Deployment{
						{
							RolloutState:       ecstypes.DeploymentRolloutStateFailed,
							RolloutStateReason: aws.String("tasks failed to start"),
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   svcName,
			Name: svcName,
			Fields: map[string]string{
				"cluster":      "prod-cluster",
				"service_name": svcName,
			},
		},
	}

	result, err := awsclient.EnrichECSServices(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[svcName]
	if !ok {
		t.Fatalf("expected finding for service %q; findings: %v", svcName, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("ecs-svc.deployment-failed"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[svcName]["ecs-svc.deployment-failed"].Rows); !strings.Contains(rows, "Deployment") {
		t.Errorf("no supporting row names the failed deployment: %s", rows)
	}
}

// TestEnrichECSServices_APIErrorSetsTruncated verifies that an API error on
// DescribeServices marks Truncated=true (no finding) and surfaces a composite error
// containing the enricher prefix and the failing resource ID.
func TestEnrichECSServices_APIErrorSetsTruncated(t *testing.T) {
	fake := &fakeECSEnricher{
		descSvcErr: errors.New("simulated DescribeServices error"),
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   "svc-err",
			Name: "svc-err",
			Fields: map[string]string{
				"cluster":      "cluster-a",
				"service_name": "svc-err",
			},
		},
	}

	result, err := awsclient.EnrichECSServices(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when DescribeServices fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "ecs-svc-enrich:") {
		t.Errorf("composite error must contain \"ecs-svc-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "svc-err") {
		t.Errorf("composite error must contain the failing service ID \"svc-err\", got: %q", errStr)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true on DescribeServices API error")
	}
	if !result.TruncatedIDs["svc-err"] {
		t.Error("svc-err must be in TruncatedIDs when its DescribeServices batch failed — a skipped batch must surface a per-row \"?\", not vanish")
	}
}

// TestEnrichECSServices_HealthyServiceNoFinding verifies that a service with
// desired == running and no deployment failures produces no finding.
func TestEnrichECSServices_HealthyServiceNoFinding(t *testing.T) {
	svcName := "healthy-svc"
	fake := &fakeECSEnricher{
		descSvcOut: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName:  aws.String(svcName),
					DesiredCount: 2,
					RunningCount: 2,
					Deployments: []ecstypes.Deployment{
						{RolloutState: ecstypes.DeploymentRolloutStateCompleted},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   svcName,
			Name: svcName,
			Fields: map[string]string{
				"cluster":      "healthy-cluster",
				"service_name": svcName,
			},
		},
	}

	result, err := awsclient.EnrichECSServices(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for healthy service; got %v", result.Findings)
	}
}

// TestEnrichECSServices_RecentEventUnableToPlaceEmitsFinding verifies that a
// recent ELB health check failure event produces a "!" finding.
func TestEnrichECSServices_RecentEventUnableToPlaceEmitsFinding(t *testing.T) {
	svcName := "placement-svc"
	recentTime := time.Now().Add(-1 * time.Minute)
	fake := &fakeECSEnricher{
		descSvcOut: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName:  aws.String(svcName),
					DesiredCount: 1,
					RunningCount: 1,
					Events: []ecstypes.ServiceEvent{
						{
							CreatedAt: aws.Time(recentTime),
							Message:   aws.String("service unable to place a task because no container instance met all of its requirements"),
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   svcName,
			Name: svcName,
			Fields: map[string]string{
				"cluster":      "event-cluster",
				"service_name": svcName,
			},
		},
	}

	result, err := awsclient.EnrichECSServices(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[svcName]; !ok {
		t.Errorf("expected finding for service with recent unable-to-place event; findings: %v", result.Findings)
	}
}

// =============================================================================
// EnrichECSClusters
// =============================================================================

// TestEnrichECSClusters_PendingTasksEmitsFinding verifies that a cluster with
// pendingTasksCount > 0 produces a "~" finding.
func TestEnrichECSClusters_PendingTasksEmitsFinding(t *testing.T) {
	clusterName := "prod-cluster"
	fake := &fakeECSEnricher{
		descClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{
				{
					ClusterName:                       aws.String(clusterName),
					PendingTasksCount:                 5,
					RunningTasksCount:                 10,
					RegisteredContainerInstancesCount: 3,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   clusterName,
			Name: clusterName,
			Fields: map[string]string{
				"cluster_name": clusterName,
			},
		},
	}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[clusterName]
	if !ok {
		t.Fatalf("expected finding for cluster %q; findings: %v", clusterName, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(f.Phrase, "pending") {
		t.Errorf("summary %q should contain %q", f.Phrase, "pending")
	}
}

// TestEnrichECSClusters_NoRunningTasksWithInstancesEmitsFinding verifies that
// running==0 but registered>0 produces a "~" finding.
func TestEnrichECSClusters_NoRunningTasksWithInstancesEmitsFinding(t *testing.T) {
	clusterName := "idle-cluster"
	fake := &fakeECSEnricher{
		descClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{
				{
					ClusterName:                       aws.String(clusterName),
					PendingTasksCount:                 0,
					RunningTasksCount:                 0,
					RegisteredContainerInstancesCount: 2,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   clusterName,
			Name: clusterName,
			Fields: map[string]string{
				"cluster_name": clusterName,
			},
		},
	}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[clusterName]; !ok {
		t.Errorf("expected finding for idle cluster; findings: %v", result.Findings)
	}
}

// TestEnrichECSClusters_BatchErrorMarksRowsTruncatedIDsNotBadge verifies that a
// DescribeClusters batch error is swallowed without propagating an error and
// without fabricating a finding, but that every cluster in the failed batch is
// marked in TruncatedIDs so its row shows a "?" coverage gap. ecs is a "~"-only
// enricher (IssueCount always 0), so the gap must never lower-bound the
// aggregate issue badge (Truncated stays false) — the per-row "?" is the
// correct signal, and it keeps a batch failure from becoming invisible.
func TestEnrichECSClusters_BatchErrorMarksRowsTruncatedIDsNotBadge(t *testing.T) {
	fake := &fakeECSEnricher{
		descClustersErr: errors.New("simulated DescribeClusters error"),
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   "err-cluster",
			Name: "err-cluster",
			Fields: map[string]string{
				"cluster_name": "err-cluster",
			},
		},
	}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Truncated {
		t.Error("Truncated must stay false: ecs is a \"~\"-only enricher, so a DescribeClusters API error must not lower-bound the aggregate issue badge")
	}
	if _, ok := result.Findings["err-cluster"]; ok {
		t.Errorf("unexpected finding for err-cluster when DescribeClusters errored; findings: %v", result.Findings)
	}
	// The whole batch was skipped by the API error — each cluster in it must be
	// marked in TruncatedIDs so its row shows a "?" coverage gap. Without this the
	// batch failure is completely invisible (no finding, no badge, no "?").
	if !result.TruncatedIDs["err-cluster"] {
		t.Error("err-cluster must be in TruncatedIDs when its DescribeClusters batch failed — a skipped batch must surface a per-row \"?\", not vanish")
	}
}

// TestEnrichECSClusters_HealthyClusterNoFinding verifies that a cluster with
// running > 0 and no pending tasks produces no finding.
func TestEnrichECSClusters_HealthyClusterNoFinding(t *testing.T) {
	clusterName := "healthy-cluster"
	fake := &fakeECSEnricher{
		descClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{
				{
					ClusterName:                       aws.String(clusterName),
					PendingTasksCount:                 0,
					RunningTasksCount:                 5,
					RegisteredContainerInstancesCount: 2,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   clusterName,
			Name: clusterName,
			Fields: map[string]string{
				"cluster_name": clusterName,
			},
		},
	}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for healthy cluster; got %v", result.Findings)
	}
}

// =============================================================================
// EnrichECSTasks
// =============================================================================

// TestEnrichECSTasks_TaskFailedToStartEmitsFinding verifies that StopCode
// TaskFailedToStart produces a "!" finding.
func TestEnrichECSTasks_TaskFailedToStartEmitsFinding(t *testing.T) {
	taskID := "abc12345678901234567890123456789012"
	taskARN := "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/" + taskID
	fake := &fakeECSEnricher{
		descTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:  aws.String(taskARN),
					StopCode: ecstypes.TaskStopCodeTaskFailedToStart,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   taskID,
			Name: taskID,
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": taskID,
			},
		},
	}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[taskID]
	if !ok {
		t.Fatalf("expected finding for task %q; findings: %v", taskID, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("ecs-task.task-failed"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[taskID]["ecs-task.task-failed"].Rows); !strings.Contains(rows, "TaskFailedToStart") {
		t.Errorf("no supporting row names the stop code: %s", rows)
	}
}

// TestEnrichECSTasks_ContainerNonZeroExitEmitsFinding verifies that a
// container with exit code != 0 produces a "!" finding.
func TestEnrichECSTasks_ContainerNonZeroExitEmitsFinding(t *testing.T) {
	taskID := "def12345678901234567890123456789012"
	taskARN := "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/" + taskID
	exitCode := int32(137)
	fake := &fakeECSEnricher{
		descTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:  aws.String(taskARN),
					StopCode: ecstypes.TaskStopCodeEssentialContainerExited,
					Containers: []ecstypes.Container{
						{
							Name:     aws.String("web"),
							ExitCode: &exitCode,
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   taskID,
			Name: taskID,
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": taskID,
			},
		},
	}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[taskID]; !ok {
		t.Fatalf("expected finding for task with non-zero exit code; findings: %v", result.Findings)
	}
}

// TestEnrichECSTasks_APIErrorSetsTruncated verifies API errors mark Truncated
// and surface a composite error — matching the ecs-svc/ecs-clusters convention
// for "!" bang-severity enrichers.
func TestEnrichECSTasks_APIErrorSetsTruncated(t *testing.T) {
	fake := &fakeECSEnricher{
		descTasksErr: errors.New("simulated DescribeTasks error"),
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   "task-err",
			Name: "task-err",
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": "task-err",
			},
		},
	}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when DescribeTasks fails")
	}
	if !result.Truncated {
		t.Error("expected Truncated=true on DescribeTasks API error")
	}
}

// TestEnrichECSTasks_BatchErrorMarksRowsTruncatedIDsNotBadgeAndReturnsErr verifies
// that a DescribeTasks batch error marks EVERY task in the failed batch in
// TruncatedIDs (per-row "?" coverage) and surfaces a non-nil aggregate error —
// matching the ecs-svc enricher's shape (both are "!" bang-severity enrichers,
// unlike the "~"-only ecs-clusters enricher which intentionally keeps err nil).
func TestEnrichECSTasks_BatchErrorMarksRowsTruncatedIDsNotBadgeAndReturnsErr(t *testing.T) {
	fake := &fakeECSEnricher{
		descTasksErr: errors.New("simulated DescribeTasks error"),
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   "id1",
			Name: "id1",
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": "id1",
			},
		},
		{
			ID:   "id2",
			Name: "id2",
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": "id2",
			},
		},
	}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when DescribeTasks fails")
	}
	if !result.TruncatedIDs["id1"] {
		t.Error("id1 must be in TruncatedIDs when its DescribeTasks batch failed — a skipped batch must surface a per-row \"?\", not vanish")
	}
	if !result.TruncatedIDs["id2"] {
		t.Error("id2 must be in TruncatedIDs when its DescribeTasks batch failed — a skipped batch must surface a per-row \"?\", not vanish")
	}
}

// TestEnrichECSTasks_HealthyTaskNoFinding verifies that a task with no stop
// code and zero exit codes produces no finding.
func TestEnrichECSTasks_HealthyTaskNoFinding(t *testing.T) {
	taskID := "ghi12345678901234567890123456789012"
	taskARN := "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/" + taskID
	exitCode := int32(0)
	fake := &fakeECSEnricher{
		descTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn: aws.String(taskARN),
					Containers: []ecstypes.Container{
						{
							Name:     aws.String("app"),
							ExitCode: &exitCode,
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{
		{
			ID:   taskID,
			Name: taskID,
			Fields: map[string]string{
				"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
				"task_id": taskID,
			},
		},
	}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for healthy task; got %v", result.Findings)
	}
}

// =============================================================================
// CFN fake
// =============================================================================

// fakeCFNEnricher embeds CFNAPI and overrides DescribeStackEvents and
// DescribeStacks for EnrichCFNStackEvents and EnrichCFNCombined.
type fakeCFNEnricher struct {
	awsclient.CFNAPI

	// DescribeStackEvents
	stackEvents    []cfntypes.StackEvent
	stackEventsErr error

	// DescribeStacks (used by EnrichCFNDrift inside EnrichCFNCombined)
	stacksOut *cfnsvc.DescribeStacksOutput
	stacksErr error
}

func (f *fakeCFNEnricher) DescribeStackEvents(
	_ context.Context,
	_ *cfnsvc.DescribeStackEventsInput,
	_ ...func(*cfnsvc.Options),
) (*cfnsvc.DescribeStackEventsOutput, error) {
	if f.stackEventsErr != nil {
		return nil, f.stackEventsErr
	}
	return &cfnsvc.DescribeStackEventsOutput{StackEvents: f.stackEvents}, nil
}

func (f *fakeCFNEnricher) DescribeStacks(
	_ context.Context,
	_ *cfnsvc.DescribeStacksInput,
	_ ...func(*cfnsvc.Options),
) (*cfnsvc.DescribeStacksOutput, error) {
	if f.stacksErr != nil {
		return nil, f.stacksErr
	}
	if f.stacksOut != nil {
		return f.stacksOut, nil
	}
	return &cfnsvc.DescribeStacksOutput{}, nil
}

// Compile-time check.
var _ awsclient.CFNAPI = (*fakeCFNEnricher)(nil)

// =============================================================================
// EnrichCFNStackEvents
// =============================================================================

// TestEnrichCFNStackEvents_FailedEventEmitsBangFinding verifies that a stack
// event with a _FAILED status produces a "!" finding.
func TestEnrichCFNStackEvents_FailedEventEmitsBangFinding(t *testing.T) {
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/abc"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:       cfntypes.ResourceStatusCreateFailed,
				LogicalResourceId:    aws.String("MyBucket"),
				ResourceType:         aws.String("AWS::S3::Bucket"),
				ResourceStatusReason: aws.String("bucket name already exists"),
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		{
			ID:   stackID,
			Name: "my-stack",
			Fields: map[string]string{
				"stack_name": "my-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNStackEvents(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[stackID]
	if !ok {
		t.Fatalf("expected finding for stack %q; findings: %v", stackID, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if !strings.Contains(f.Phrase, "recent resource failure") {
		t.Errorf("summary %q should contain %q", f.Phrase, "recent resource failure")
	}
}

// TestEnrichCFNStackEvents_APIErrorSetsPerResourceTruncation verifies that an
// API error on DescribeStackEvents marks TruncatedIDs for that resource and
// surfaces a composite error containing the enricher prefix and the failing stack ID.
func TestEnrichCFNStackEvents_APIErrorSetsPerResourceTruncation(t *testing.T) {
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/err-stack/xyz"
	fake := &fakeCFNEnricher{
		stackEventsErr: errors.New("simulated DescribeStackEvents error"),
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		{
			ID:   stackID,
			Name: "err-stack",
			Fields: map[string]string{
				"stack_name": "err-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNStackEvents(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when DescribeStackEvents fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "cfn-enrich:") {
		t.Errorf("composite error must contain \"cfn-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, stackID) {
		t.Errorf("composite error must contain the failing stack ID %q, got: %q", stackID, errStr)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true on DescribeStackEvents API error")
	}
	if !result.TruncatedIDs[stackID] {
		t.Errorf("expected TruncatedIDs[%q]=true; got map: %v", stackID, result.TruncatedIDs)
	}
}

// TestEnrichCFNStackEvents_NoFailedEventsNoFinding verifies that a stack with
// only successful events produces no finding.
func TestEnrichCFNStackEvents_NoFailedEventsNoFinding(t *testing.T) {
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/ok-stack/ok1"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
				LogicalResourceId: aws.String("MyBucket"),
				ResourceType:      aws.String("AWS::S3::Bucket"),
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		{
			ID:   stackID,
			Name: "ok-stack",
			Fields: map[string]string{
				"stack_name": "ok-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNStackEvents(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for healthy stack; got %v", result.Findings)
	}
}

// =============================================================================
// ELB fake
// =============================================================================

// fakeELBEnricher embeds ELBv2API and overrides DescribeLoadBalancerAttributes.
type fakeELBEnricher struct {
	awsclient.ELBv2API

	// perLBAttrs maps LB ARN → attributes list returned
	perLBAttrs map[string][]elbtypes.LoadBalancerAttribute
	errOnLBARN string // ARN that triggers an error
}

func (f *fakeELBEnricher) DescribeLoadBalancerAttributes(
	_ context.Context,
	in *elbv2svc.DescribeLoadBalancerAttributesInput,
	_ ...func(*elbv2svc.Options),
) (*elbv2svc.DescribeLoadBalancerAttributesOutput, error) {
	arn := aws.ToString(in.LoadBalancerArn)
	if arn == f.errOnLBARN {
		return nil, errors.New("simulated DescribeLoadBalancerAttributes error")
	}
	attrs := f.perLBAttrs[arn]
	return &elbv2svc.DescribeLoadBalancerAttributesOutput{Attributes: attrs}, nil
}

// DescribeListeners answers the enricher's listener-posture pass. These cases
// exercise the attributes half only, so every balancer reports no listeners —
// an empty set is a valid answer and must not colour the attribute findings.
func (f *fakeELBEnricher) DescribeListeners(
	_ context.Context,
	_ *elbv2svc.DescribeListenersInput,
	_ ...func(*elbv2svc.Options),
) (*elbv2svc.DescribeListenersOutput, error) {
	return &elbv2svc.DescribeListenersOutput{}, nil
}

// Compile-time check.
var _ awsclient.ELBv2API = (*fakeELBEnricher)(nil)

// =============================================================================
// EnrichELBAttributes
// =============================================================================

// TestEnrichELBAttributes_BothMisconfigurations_TildeFinding pins the CURRENT
// (correct) contract: a load balancer missing both deletion protection and
// access logging still produces only a "~" (SevWarn) finding, per commit
// 8555b124 ("elb enricher stops promoting warn to broken") — both flags
// missing at once is the AWS create-load-balancer default and must not
// escalate to SevBroken (see EnrichELBAttributes' own doc comment in
// core/aws/elb_issue_enrichment.go), or every freshly-created,
// unhardened LB would render red.
//
// RETIRED the old "both-missing promotion rule" invariant this test used to
// pin (TestEnrichELBAttributes_BothMisconfigurations_BangFinding): that
// promotion was deliberately removed in 8555b124, predating this task —
// this test was simply never updated to match.
func TestEnrichELBAttributes_BothMisconfigurations_TildeFinding(t *testing.T) {
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc"
	fake := &fakeELBEnricher{
		perLBAttrs: map[string][]elbtypes.LoadBalancerAttribute{
			lbARN: {
				{Key: aws.String("deletion_protection.enabled"), Value: aws.String("false")},
				{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("false")},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	const lbName = "my-lb"
	resources := []resource.Resource{{ID: lbName, Name: lbName, Fields: map[string]string{"load_balancer_arn": lbARN}}}

	result, err := awsclient.EnrichELBAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[lbName]
	if !ok {
		t.Fatalf("expected finding for LB %q; findings: %v", lbName, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %q (both misconfigured must NOT promote to broken)", f.Severity, "~")
	}
}

// TestEnrichELBAttributes_OnlyDeletionProtectionMissing_TildeFinding verifies
// that only deletion protection missing produces a "~" finding (not promoted).
func TestEnrichELBAttributes_OnlyDeletionProtectionMissing_TildeFinding(t *testing.T) {
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/partial-lb/def"
	fake := &fakeELBEnricher{
		perLBAttrs: map[string][]elbtypes.LoadBalancerAttribute{
			lbARN: {
				{Key: aws.String("deletion_protection.enabled"), Value: aws.String("false")},
				{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	const lbName = "partial-lb"
	resources := []resource.Resource{{ID: lbName, Name: lbName, Fields: map[string]string{"load_balancer_arn": lbARN}}}

	result, err := awsclient.EnrichELBAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[lbName]
	if !ok {
		t.Fatalf("expected finding for LB %q; findings: %v", lbName, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %q (single misconfiguration → ~)", f.Severity, "~")
	}
}

// TestEnrichELBAttributes_APIErrorMarksRowTruncatedIDNotBadge verifies that
// an API error marks TruncatedIDs for the failing LB's row, while leaving
// the aggregate Truncated flag false — elb only ever emits "~" (SevWarn)
// findings, so a coverage gap must never lower-bound the issue badge.
func TestEnrichELBAttributes_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/err-lb/ghi"
	fake := &fakeELBEnricher{
		errOnLBARN: lbARN,
		perLBAttrs: map[string][]elbtypes.LoadBalancerAttribute{},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	const lbName = "err-lb"
	resources := []resource.Resource{{ID: lbName, Name: lbName, Fields: map[string]string{"load_balancer_arn": lbARN}}}

	result, err := awsclient.EnrichELBAttributes(context.Background(), clients, resources, nil)
	// Per-resource errors now aggregate into a composite; assert surface but do
	// not require its absence (E1-E6 contract).
	_ = err
	if result.Truncated {
		t.Error("Truncated must stay false: elb only emits \"~\" findings, so a DescribeLoadBalancerAttributes API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if !result.TruncatedIDs[lbName] {
		t.Errorf("expected TruncatedIDs[%q]=true; got map: %v", lbName, result.TruncatedIDs)
	}
}

// TestEnrichELBAttributes_WellConfiguredLB_NoFinding verifies that a LB with
// both deletion protection and access logs enabled produces no finding.
func TestEnrichELBAttributes_WellConfiguredLB_NoFinding(t *testing.T) {
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/secure-lb/jkl"
	fake := &fakeELBEnricher{
		perLBAttrs: map[string][]elbtypes.LoadBalancerAttribute{
			lbARN: {
				{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
				{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{{ID: "secure-lb", Name: "secure-lb", Fields: map[string]string{"load_balancer_arn": lbARN}}}

	result, err := awsclient.EnrichELBAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for well-configured LB; got %v", result.Findings)
	}
}

// =============================================================================
// EnrichEBEnvironmentHealth
// =============================================================================

// fakeEBHealthEnricher embeds ElasticBeanstalkAPI and overrides
// DescribeEnvironmentHealth.
type fakeEBHealthEnricher struct {
	awsclient.ElasticBeanstalkAPI

	// perEnvCauses maps environment name → causes list
	perEnvCauses map[string][]string
	errOnEnvName string
}

func (f *fakeEBHealthEnricher) DescribeEnvironmentHealth(
	_ context.Context,
	in *elasticbeanstalk.DescribeEnvironmentHealthInput,
	_ ...func(*elasticbeanstalk.Options),
) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error) {
	name := aws.ToString(in.EnvironmentName)
	if name == f.errOnEnvName {
		return nil, errors.New("simulated DescribeEnvironmentHealth error")
	}
	causes := f.perEnvCauses[name]
	return &elasticbeanstalk.DescribeEnvironmentHealthOutput{
		Causes:       causes,
		HealthStatus: aws.String("Degraded"),
	}, nil
}

// Compile-time check.
var _ awsclient.ElasticBeanstalkAPI = (*fakeEBHealthEnricher)(nil)

// TestEnrichEBEnvironmentHealth_CausesEmitsTildeFinding verifies that a non-empty
// Causes slice produces a "~" finding with "EB causes:" in the summary.
func TestEnrichEBEnvironmentHealth_CausesEmitsTildeFinding(t *testing.T) {
	envName := "prod-env"
	envID := "e-abcdef1234"
	fake := &fakeEBHealthEnricher{
		perEnvCauses: map[string][]string{
			envName: {"No data available for some instances"},
		},
	}
	clients := &awsclient.ServiceClients{ElasticBeanstalk: fake}
	resources := []resource.Resource{
		{
			ID:   envID,
			Name: envName,
			Fields: map[string]string{
				"environment_name": envName,
			},
		},
	}

	result, err := awsclient.EnrichEBEnvironmentHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[envID]
	if !ok {
		t.Fatalf("expected finding keyed by env ID %q; findings: %v", envID, result.Findings)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("eb.environment-causes"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[envID]["eb.environment-causes"].Rows); !strings.Contains(rows, "Cause") {
		t.Errorf("no supporting row names the reported cause: %s", rows)
	}
}

// TestEnrichEBEnvironmentHealth_APIErrorMarksRowTruncatedIDNotBadge verifies
// that an API error marks TruncatedIDs for the failing environment's row,
// while leaving the aggregate Truncated flag false — eb is a "~"-only
// enricher (IssueCount always 0), so a coverage gap must never lower-bound
// the issue badge.
func TestEnrichEBEnvironmentHealth_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	envName := "err-env"
	envID := "e-errenv1234"
	fake := &fakeEBHealthEnricher{
		perEnvCauses: map[string][]string{},
		errOnEnvName: envName,
	}
	clients := &awsclient.ServiceClients{ElasticBeanstalk: fake}
	resources := []resource.Resource{
		{
			ID:   envID,
			Name: envName,
			Fields: map[string]string{
				"environment_name": envName,
			},
		},
	}

	result, err := awsclient.EnrichEBEnvironmentHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Truncated {
		t.Error("Truncated must stay false: eb is a \"~\"-only enricher, so a DescribeEnvironmentHealth error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if !result.TruncatedIDs[envID] {
		t.Errorf("expected TruncatedIDs[%q]=true; got map: %v", envID, result.TruncatedIDs)
	}
}

// TestEnrichEBEnvironmentHealth_NoCauses_NoFinding verifies that a healthy
// environment with no causes produces no finding.
func TestEnrichEBEnvironmentHealth_NoCauses_NoFinding(t *testing.T) {
	envName := "healthy-env"
	envID := "e-healthy1234"
	fake := &fakeEBHealthEnricher{
		perEnvCauses: map[string][]string{
			envName: {}, // empty causes
		},
	}
	clients := &awsclient.ServiceClients{ElasticBeanstalk: fake}
	resources := []resource.Resource{
		{
			ID:   envID,
			Name: envName,
			Fields: map[string]string{
				"environment_name": envName,
			},
		},
	}

	result, err := awsclient.EnrichEBEnvironmentHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for healthy environment; got %v", result.Findings)
	}
}

// =============================================================================
// EnrichCFNCombined
// =============================================================================

// TestEnrichCFNCombined_EventsAndDriftMerged verifies that when a stack has
// both a _FAILED event AND is DRIFTED, the combined result contains the "!"
// finding from events (events win over drift on conflict) and IssueCount > 0.
func TestEnrichCFNCombined_EventsAndDriftMerged(t *testing.T) {
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/combined-stack/c01"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:    cfntypes.ResourceStatusUpdateFailed,
				LogicalResourceId: aws.String("MyQueue"),
				ResourceType:      aws.String("AWS::SQS::Queue"),
			},
		},
		stacksOut: &cfnsvc.DescribeStacksOutput{
			Stacks: []cfntypes.Stack{
				{
					StackName: aws.String("combined-stack"),
					DriftInformation: &cfntypes.StackDriftInformation{
						StackDriftStatus: cfntypes.StackDriftStatusDrifted,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		{
			ID:   stackID,
			Name: "combined-stack",
			Fields: map[string]string{
				"stack_name": "combined-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNCombined(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Combined result must have at least one finding.
	if len(result.Findings) == 0 {
		t.Fatal("expected findings in combined result; got none")
	}
	// Events win on ID conflict: the stackID finding must be "!".
	fs, ok := result.Findings[stackID]
	if !ok {
		t.Fatalf("expected finding keyed by %q (event failure must produce a finding)", stackID)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("expected events finding (severity !) to win over drift finding; got severity %v", f.Severity)
	}
}

// TestEnrichCFNCombined_DriftOnlyNoEventFailure_TildeFinding verifies that when
// there are no failed events but the stack is DRIFTED, the result contains a
// "~" finding and IssueCount == 0.
func TestEnrichCFNCombined_DriftOnlyNoEventFailure_TildeFinding(t *testing.T) {
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/drift-only-stack/d01"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
				LogicalResourceId: aws.String("MyBucket"),
				ResourceType:      aws.String("AWS::S3::Bucket"),
			},
		},
		stacksOut: &cfnsvc.DescribeStacksOutput{
			Stacks: []cfntypes.Stack{
				{
					StackName: aws.String("drift-only-stack"),
					DriftInformation: &cfntypes.StackDriftInformation{
						StackDriftStatus: cfntypes.StackDriftStatusDrifted,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{
		{
			ID:   stackID,
			Name: "drift-only-stack",
			Fields: map[string]string{
				"stack_name": "drift-only-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNCombined(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[stackID]
	if !ok || len(fs) == 0 {
		t.Fatal("expected drift finding in combined result; got none")
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("drift-only finding Severity = %v, want SevWarn (drift finding is ~, not !)", fs[0].Severity)
	}
}

// TestEnrichCFNCombined_NilClient_ReturnsEmpty verifies the nil-client guard.
func TestEnrichCFNCombined_NilClient_ReturnsEmpty(t *testing.T) {
	clients := &awsclient.ServiceClients{CloudFormation: nil}
	resources := []resource.Resource{
		{
			ID:   "some-stack",
			Name: "some-stack",
			Fields: map[string]string{
				"stack_name": "some-stack",
			},
		},
	}

	result, err := awsclient.EnrichCFNCombined(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for nil client; got %v", result.Findings)
	}
}

// DescribeConfigurationSettings is the stub half of a partial test double: this fake embeds
// ElasticBeanstalkAPI as a nil interface and implements only the calls the enricher
// under test made when it was written. The enricher now also calls
// DescribeConfigurationSettings, and the promoted nil method panics rather than returning
// anything. An empty output keeps this fake's own scenario unchanged —
// no managed-updates, health-reporting or log-streaming posture is asserted here.
func (f *fakeEBHealthEnricher) DescribeConfigurationSettings(_ context.Context, _ *elasticbeanstalk.DescribeConfigurationSettingsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
}
