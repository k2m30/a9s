package unit_test

// phase03_containers_pr03c_test.go — regression-pin tests for Wave 1 finding
// migration, covering the 4 container types in PR-03c:
// ng, ecs, ecs-svc, ecs-task.
//
// Migration contract (PR-03c — NOT YET implemented; tests are intentionally RED):
//   - Fetchers STOP writing Resource.Status for lifecycle states.
//   - Fetchers EMIT canonical Finding entries (Source: "wave1") for non-healthy
//     states.
//   - Each type has a corresponding internal/aws/<svc>_codes.go with constants.
//   - Each type's Color func reads Findings first, then falls back to structural
//     fields.
//
// State → Severity mapping decisions:
//
//   ng:
//     ACTIVE       → no Finding (healthy)
//     CREATING     → SevWarn  (transitional)
//     UPDATING     → SevWarn  (transitional)
//     DELETING     → SevWarn  (transitional; emit-as-Warn consistent with ECS;
//                              DELETING is non-terminal from the operator's
//                              perspective — the nodegroup may re-appear)
//     CREATE_FAILED → SevBroken
//     DELETE_FAILED → SevBroken
//     DEGRADED      → SevBroken
//
//   ecs:
//     ACTIVE         → no Finding (healthy)
//     PROVISIONING   → SevWarn  (transitional)
//     DEPROVISIONING → SevWarn  (transitional)
//     FAILED         → SevBroken
//     INACTIVE       → SevBroken (cluster is dead — non-recoverable without
//                                recreation; Color func currently maps to
//                                ColorBroken)
//
//   ecs-svc:
//     ACTIVE   → no Finding (healthy)
//     DRAINING → SevWarn  (transitional)
//     INACTIVE → SevBroken (service has been deleted; non-recoverable)
//     running_count vs desired_count check stays structural (Wave 2-class).
//
//   ecs-task:
//     RUNNING      → no Finding (healthy)
//     STOPPED      → no Finding (lifecycle-terminal; stop_code carries the
//                    meaningful info; structural Color func reads stop_code)
//     PROVISIONING   → SevWarn  (transitional)
//     PENDING        → SevWarn  (transitional)
//     ACTIVATING     → SevWarn  (transitional)
//     DEACTIVATING   → SevWarn  (transitional)
//     STOPPING       → SevWarn  (transitional)
//     DEPROVISIONING → SevWarn  (transitional)
//     health_status and stop_code checks stay structural (Wave 2-class).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecssvc "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekssvc "github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// =============================================================================
// NG (EKS Node Groups)
// =============================================================================

// TestPR03c_NGCodes_ConstantsExist verifies that ng_codes.go declares the Wave 1
// finding constants. Fails to compile until the file is created.
func TestPR03c_NGCodes_ConstantsExist(t *testing.T) {
	var _ domain.FindingCode = awsclient.CodeNGStateCreating
	var _ domain.FindingCode = awsclient.CodeNGStateUpdating
	var _ domain.FindingCode = awsclient.CodeNGStateDeleting
	var _ domain.FindingCode = awsclient.CodeNGStateCreateFailed
	var _ domain.FindingCode = awsclient.CodeNGStateDeleteFailed
	var _ domain.FindingCode = awsclient.CodeNGStateDegraded
}

// TestPR03c_NGFetcher_ActiveEmitsNoFinding asserts that an ACTIVE node group
// emits no Finding and no Status after migration.
func TestPR03c_NGFetcher_ActiveEmitsNoFinding(t *testing.T) {
	listClustersMock := &pr03cEKSListMock{clusters: []string{"prod-cluster"}}
	listNGMock := &pr03cEKSListNodegroupsMock{
		nodegroups: map[string][]string{
			"prod-cluster": {"prod-ng"},
		},
	}
	describeNGMock := &pr03cEKSDescribeNodegroupMock{
		nodegroups: map[string]*ekstypes.Nodegroup{
			"prod-cluster/prod-ng": {
				NodegroupName: aws.String("prod-ng"),
				ClusterName:   aws.String("prod-cluster"),
				Status:        ekstypes.NodegroupStatusActive,
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: aws.Int32(3),
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("FetchNodeGroups: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for ACTIVE node group", len(r.Findings))
	}
}

// TestPR03c_NGFetcher_BrokenEmitsFinding asserts that a CREATE_FAILED node group
// emits one SevBroken Finding with CodeNGStateCreateFailed.
func TestPR03c_NGFetcher_BrokenEmitsFinding(t *testing.T) {
	listClustersMock := &pr03cEKSListMock{clusters: []string{"prod-cluster"}}
	listNGMock := &pr03cEKSListNodegroupsMock{
		nodegroups: map[string][]string{
			"prod-cluster": {"broken-ng"},
		},
	}
	describeNGMock := &pr03cEKSDescribeNodegroupMock{
		nodegroups: map[string]*ekstypes.Nodegroup{
			"prod-cluster/broken-ng": {
				NodegroupName: aws.String("broken-ng"),
				ClusterName:   aws.String("prod-cluster"),
				Status:        ekstypes.NodegroupStatusCreateFailed,
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("FetchNodeGroups: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for CREATE_FAILED node group", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeNGStateCreateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeNGStateCreateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03c_NGFetcher_TransitionalEmitsWarn asserts that a CREATING node group
// emits one SevWarn Finding with CodeNGStateCreating.
func TestPR03c_NGFetcher_TransitionalEmitsWarn(t *testing.T) {
	listClustersMock := &pr03cEKSListMock{clusters: []string{"dev-cluster"}}
	listNGMock := &pr03cEKSListNodegroupsMock{
		nodegroups: map[string][]string{
			"dev-cluster": {"new-ng"},
		},
	}
	describeNGMock := &pr03cEKSDescribeNodegroupMock{
		nodegroups: map[string]*ekstypes.Nodegroup{
			"dev-cluster/new-ng": {
				NodegroupName: aws.String("new-ng"),
				ClusterName:   aws.String("dev-cluster"),
				Status:        ekstypes.NodegroupStatusCreating,
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: aws.Int32(2),
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("FetchNodeGroups: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for CREATING node group", len(r.Findings))
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", r.Findings[0].Severity)
	}
	if r.Findings[0].Code != awsclient.CodeNGStateCreating {
		t.Errorf("Findings[0].Code: got %q, want %q", r.Findings[0].Code, awsclient.CodeNGStateCreating)
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ng mocks
// ---------------------------------------------------------------------------

type pr03cEKSListMock struct {
	clusters []string
}

func (m *pr03cEKSListMock) ListClusters(
	_ context.Context,
	_ *ekssvc.ListClustersInput,
	_ ...func(*ekssvc.Options),
) (*ekssvc.ListClustersOutput, error) {
	return &ekssvc.ListClustersOutput{Clusters: m.clusters}, nil
}

type pr03cEKSListNodegroupsMock struct {
	// keyed by cluster name
	nodegroups map[string][]string
}

func (m *pr03cEKSListNodegroupsMock) ListNodegroups(
	_ context.Context,
	input *ekssvc.ListNodegroupsInput,
	_ ...func(*ekssvc.Options),
) (*ekssvc.ListNodegroupsOutput, error) {
	clusterName := ""
	if input.ClusterName != nil {
		clusterName = *input.ClusterName
	}
	return &ekssvc.ListNodegroupsOutput{Nodegroups: m.nodegroups[clusterName]}, nil
}

type pr03cEKSDescribeNodegroupMock struct {
	// keyed by "<clusterName>/<nodegroupName>"
	nodegroups map[string]*ekstypes.Nodegroup
}

func (m *pr03cEKSDescribeNodegroupMock) DescribeNodegroup(
	_ context.Context,
	input *ekssvc.DescribeNodegroupInput,
	_ ...func(*ekssvc.Options),
) (*ekssvc.DescribeNodegroupOutput, error) {
	key := ""
	if input.ClusterName != nil && input.NodegroupName != nil {
		key = *input.ClusterName + "/" + *input.NodegroupName
	}
	return &ekssvc.DescribeNodegroupOutput{Nodegroup: m.nodegroups[key]}, nil
}

// =============================================================================
// ECS (ECS Clusters)
// =============================================================================

// TestPR03c_ECSCodes_ConstantsExist verifies that ecs_codes.go declares the Wave 1
// finding constants. Fails to compile until the file is created.
func TestPR03c_ECSCodes_ConstantsExist(t *testing.T) {
	var _ domain.FindingCode = awsclient.CodeECSStateProvisioning
	var _ domain.FindingCode = awsclient.CodeECSStateDeprovisioning
	var _ domain.FindingCode = awsclient.CodeECSStateFailed
	var _ domain.FindingCode = awsclient.CodeECSStateInactive
}

// TestPR03c_ECSFetcher_ActiveEmitsNoFinding asserts that an ACTIVE ECS cluster
// emits no Finding and no Status after migration.
func TestPR03c_ECSFetcher_ActiveEmitsNoFinding(t *testing.T) {
	listMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"},
	}
	describeMock := &pr03cECSDescribeClustersMock{
		clusters: []ecstypes.Cluster{
			{
				ClusterName:         aws.String("prod-cluster"),
				Status:              aws.String("ACTIVE"),
				RunningTasksCount:   5,
				PendingTasksCount:   0,
				ActiveServicesCount: 3,
			},
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
	if err != nil {
		t.Fatalf("FetchECSClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for ACTIVE cluster", len(r.Findings))
	}
}

// TestPR03c_ECSFetcher_BrokenEmitsFinding asserts that a FAILED ECS cluster
// emits one SevBroken Finding with CodeECSStateFailed.
func TestPR03c_ECSFetcher_BrokenEmitsFinding(t *testing.T) {
	listMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/failed-cluster"},
	}
	describeMock := &pr03cECSDescribeClustersMock{
		clusters: []ecstypes.Cluster{
			{
				ClusterName: aws.String("failed-cluster"),
				Status:      aws.String("FAILED"),
			},
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
	if err != nil {
		t.Fatalf("FetchECSClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for FAILED cluster", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeECSStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeECSStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03c_ECSFetcher_TransitionalEmitsWarn asserts that a PROVISIONING ECS
// cluster emits one SevWarn Finding with CodeECSStateProvisioning.
func TestPR03c_ECSFetcher_TransitionalEmitsWarn(t *testing.T) {
	listMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/new-cluster"},
	}
	describeMock := &pr03cECSDescribeClustersMock{
		clusters: []ecstypes.Cluster{
			{
				ClusterName: aws.String("new-cluster"),
				Status:      aws.String("PROVISIONING"),
			},
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
	if err != nil {
		t.Fatalf("FetchECSClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for PROVISIONING cluster", len(r.Findings))
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", r.Findings[0].Severity)
	}
	if r.Findings[0].Code != awsclient.CodeECSStateProvisioning {
		t.Errorf("Findings[0].Code: got %q, want %q", r.Findings[0].Code, awsclient.CodeECSStateProvisioning)
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ecs mocks
// ---------------------------------------------------------------------------

type pr03cECSListClustersMock struct {
	arns []string
}

func (m *pr03cECSListClustersMock) ListClusters(
	_ context.Context,
	_ *ecssvc.ListClustersInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.ListClustersOutput, error) {
	return &ecssvc.ListClustersOutput{ClusterArns: m.arns}, nil
}

type pr03cECSDescribeClustersMock struct {
	clusters []ecstypes.Cluster
}

func (m *pr03cECSDescribeClustersMock) DescribeClusters(
	_ context.Context,
	_ *ecssvc.DescribeClustersInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.DescribeClustersOutput, error) {
	return &ecssvc.DescribeClustersOutput{Clusters: m.clusters}, nil
}

// =============================================================================
// ECS-SVC (ECS Services)
// =============================================================================

// TestPR03c_ECSSvcCodes_ConstantsExist verifies that ecs_svc_codes.go declares
// the Wave 1 finding constants. Fails to compile until the file is created.
func TestPR03c_ECSSvcCodes_ConstantsExist(t *testing.T) {
	var _ domain.FindingCode = awsclient.CodeECSSvcStateInactive
	var _ domain.FindingCode = awsclient.CodeECSSvcStateDraining
}

// TestPR03c_ECSSvcFetcher_ActiveEmitsNoFinding asserts that an ACTIVE ECS service
// emits no Finding and no Status after migration.
func TestPR03c_ECSSvcFetcher_ActiveEmitsNoFinding(t *testing.T) {
	listClustersMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"},
	}
	listSvcMock := &pr03cECSListServicesMock{
		serviceArns: []string{"arn:aws:ecs:us-east-1:000000000000:service/prod-cluster/api-svc"},
	}
	describeSvcMock := &pr03cECSDescribeServicesMock{
		services: []ecstypes.Service{
			{
				ServiceName:    aws.String("api-svc"),
				ClusterArn:     aws.String("arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"),
				Status:         aws.String("ACTIVE"),
				DesiredCount:   3,
				RunningCount:   3,
				LaunchType:     ecstypes.LaunchTypeFargate,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:000000000000:task-definition/api:5"),
			},
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listSvcMock, describeSvcMock, "")
	if err != nil {
		t.Fatalf("FetchECSServicesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for ACTIVE service", len(r.Findings))
	}
}

// TestPR03c_ECSSvcFetcher_BrokenEmitsFinding asserts that an INACTIVE ECS service
// emits one SevBroken Finding with CodeECSSvcStateInactive.
//
// INACTIVE is the terminal state for a deleted service — non-recoverable without
// recreation. The Color func currently maps INACTIVE → ColorBroken.
func TestPR03c_ECSSvcFetcher_BrokenEmitsFinding(t *testing.T) {
	listClustersMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"},
	}
	listSvcMock := &pr03cECSListServicesMock{
		serviceArns: []string{"arn:aws:ecs:us-east-1:000000000000:service/prod-cluster/dead-svc"},
	}
	describeSvcMock := &pr03cECSDescribeServicesMock{
		services: []ecstypes.Service{
			{
				ServiceName:  aws.String("dead-svc"),
				ClusterArn:   aws.String("arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"),
				Status:       aws.String("INACTIVE"),
				DesiredCount: 0,
				RunningCount: 0,
			},
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listSvcMock, describeSvcMock, "")
	if err != nil {
		t.Fatalf("FetchECSServicesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for INACTIVE service", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeECSSvcStateInactive {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeECSSvcStateInactive)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03c_ECSSvcFetcher_TransitionalEmitsWarn asserts that a DRAINING ECS
// service emits one SevWarn Finding with CodeECSSvcStateDraining.
func TestPR03c_ECSSvcFetcher_TransitionalEmitsWarn(t *testing.T) {
	listClustersMock := &pr03cECSListClustersMock{
		arns: []string{"arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"},
	}
	listSvcMock := &pr03cECSListServicesMock{
		serviceArns: []string{"arn:aws:ecs:us-east-1:000000000000:service/prod-cluster/draining-svc"},
	}
	describeSvcMock := &pr03cECSDescribeServicesMock{
		services: []ecstypes.Service{
			{
				ServiceName:  aws.String("draining-svc"),
				ClusterArn:   aws.String("arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"),
				Status:       aws.String("DRAINING"),
				DesiredCount: 0,
				RunningCount: 1,
			},
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listSvcMock, describeSvcMock, "")
	if err != nil {
		t.Fatalf("FetchECSServicesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for DRAINING service", len(r.Findings))
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", r.Findings[0].Severity)
	}
	if r.Findings[0].Code != awsclient.CodeECSSvcStateDraining {
		t.Errorf("Findings[0].Code: got %q, want %q", r.Findings[0].Code, awsclient.CodeECSSvcStateDraining)
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ecs-svc mocks
// ---------------------------------------------------------------------------

type pr03cECSListServicesMock struct {
	serviceArns []string
}

func (m *pr03cECSListServicesMock) ListServices(
	_ context.Context,
	_ *ecssvc.ListServicesInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.ListServicesOutput, error) {
	return &ecssvc.ListServicesOutput{ServiceArns: m.serviceArns}, nil
}

type pr03cECSDescribeServicesMock struct {
	services []ecstypes.Service
}

func (m *pr03cECSDescribeServicesMock) DescribeServices(
	_ context.Context,
	_ *ecssvc.DescribeServicesInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.DescribeServicesOutput, error) {
	return &ecssvc.DescribeServicesOutput{Services: m.services}, nil
}

// =============================================================================
// ECS-TASK (ECS Tasks)
// =============================================================================

// TestPR03c_ECSTaskCodes_ConstantsExist verifies that ecs_task_codes.go declares
// the Wave 1 finding constants. Fails to compile until the file is created.
//
// NOTE: CodeECSTaskStateStopped is intentionally excluded. STOPPED is a
// lifecycle-terminal state where the stop_code carries the meaningful information.
// The structural Color func reads stop_code directly; emitting a wave1 Finding for
// STOPPED would interfere with that override (same precedent as Lambda Inactive).
func TestPR03c_ECSTaskCodes_ConstantsExist(t *testing.T) {
	var _ domain.FindingCode = awsclient.CodeECSTaskStateProvisioning
	var _ domain.FindingCode = awsclient.CodeECSTaskStatePending
	var _ domain.FindingCode = awsclient.CodeECSTaskStateActivating
	var _ domain.FindingCode = awsclient.CodeECSTaskStateDeactivating
	var _ domain.FindingCode = awsclient.CodeECSTaskStateStopping
	var _ domain.FindingCode = awsclient.CodeECSTaskStateDeprovisioning
}

// TestPR03c_ECSTaskFetcher_RunningEmitsNoFinding asserts that a RUNNING ECS task
// emits no Finding and no Status after migration.
func TestPR03c_ECSTaskFetcher_RunningEmitsNoFinding(t *testing.T) {
	taskARN := "arn:aws:ecs:us-east-1:000000000000:task/prod-cluster/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	clusterARN := "arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"
	taskDefARN := "arn:aws:ecs:us-east-1:000000000000:task-definition/api:5"

	listClustersMock := &pr03cECSListClustersMock{arns: []string{clusterARN}}
	listTasksMock := &pr03cECSListTasksMock{taskArns: []string{taskARN}}
	describeTasksMock := &pr03cECSDescribeTasksMock{
		tasks: []ecstypes.Task{
			{
				TaskArn:           aws.String(taskARN),
				ClusterArn:        aws.String(clusterARN),
				LastStatus:        aws.String("RUNNING"),
				TaskDefinitionArn: aws.String(taskDefARN),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
				HealthStatus:      ecstypes.HealthStatusHealthy,
			},
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
	if err != nil {
		t.Fatalf("FetchECSTasksPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for RUNNING task", len(r.Findings))
	}
}

// TestPR03c_ECSTaskFetcher_StoppedEmitsNoFinding asserts that a STOPPED ECS task
// emits no Finding (lifecycle-terminal; structural Color reads stop_code directly).
//
// This is the same pattern as Lambda Inactive and EC2 shutting-down:
// lifecycle-terminal states are NOT promoted to wave1 Findings.
func TestPR03c_ECSTaskFetcher_StoppedEmitsNoFinding(t *testing.T) {
	taskARN := "arn:aws:ecs:us-east-1:000000000000:task/prod-cluster/b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5"
	clusterARN := "arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"
	taskDefARN := "arn:aws:ecs:us-east-1:000000000000:task-definition/api:5"

	listClustersMock := &pr03cECSListClustersMock{arns: []string{clusterARN}}
	listTasksMock := &pr03cECSListTasksMock{taskArns: []string{taskARN}}
	describeTasksMock := &pr03cECSDescribeTasksMock{
		tasks: []ecstypes.Task{
			{
				TaskArn:           aws.String(taskARN),
				ClusterArn:        aws.String(clusterARN),
				LastStatus:        aws.String("STOPPED"),
				StopCode:          ecstypes.TaskStopCodeUserInitiated,
				TaskDefinitionArn: aws.String(taskDefARN),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
			},
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
	if err != nil {
		t.Fatalf("FetchECSTasksPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	// STOPPED is lifecycle-terminal — no wave1 Finding (stop_code handled structurally).
	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for STOPPED task (lifecycle-terminal; stop_code is structural)", len(r.Findings))
	}
}

// TestPR03c_ECSTaskFetcher_TransitionalEmitsWarn asserts that a PENDING ECS task
// emits one SevWarn Finding with CodeECSTaskStatePending.
func TestPR03c_ECSTaskFetcher_TransitionalEmitsWarn(t *testing.T) {
	taskARN := "arn:aws:ecs:us-east-1:000000000000:task/prod-cluster/c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6"
	clusterARN := "arn:aws:ecs:us-east-1:000000000000:cluster/prod-cluster"
	taskDefARN := "arn:aws:ecs:us-east-1:000000000000:task-definition/api:5"

	listClustersMock := &pr03cECSListClustersMock{arns: []string{clusterARN}}
	listTasksMock := &pr03cECSListTasksMock{taskArns: []string{taskARN}}
	describeTasksMock := &pr03cECSDescribeTasksMock{
		tasks: []ecstypes.Task{
			{
				TaskArn:           aws.String(taskARN),
				ClusterArn:        aws.String(clusterARN),
				LastStatus:        aws.String("PENDING"),
				TaskDefinitionArn: aws.String(taskDefARN),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
			},
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
	if err != nil {
		t.Fatalf("FetchECSTasksPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for PENDING task", len(r.Findings))
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", r.Findings[0].Severity)
	}
	if r.Findings[0].Code != awsclient.CodeECSTaskStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", r.Findings[0].Code, awsclient.CodeECSTaskStatePending)
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", r.Findings[0].Source, "wave1")
	}
}

// NOTE: ECS Tasks have no separate SevBroken lifecycle state at the fetcher
// level. The broken classification (stop_code != UserInitiated, health_status ==
// UNHEALTHY) is structural — handled by the Color func reading Fields directly.
// No TestPR03c_ECSTaskFetcher_BrokenEmitsFinding is needed.

// ---------------------------------------------------------------------------
// ecs-task mocks
// ---------------------------------------------------------------------------

type pr03cECSListTasksMock struct {
	taskArns []string
}

func (m *pr03cECSListTasksMock) ListTasks(
	_ context.Context,
	_ *ecssvc.ListTasksInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.ListTasksOutput, error) {
	return &ecssvc.ListTasksOutput{TaskArns: m.taskArns}, nil
}

type pr03cECSDescribeTasksMock struct {
	tasks []ecstypes.Task
}

func (m *pr03cECSDescribeTasksMock) DescribeTasks(
	_ context.Context,
	_ *ecssvc.DescribeTasksInput,
	_ ...func(*ecssvc.Options),
) (*ecssvc.DescribeTasksOutput, error) {
	return &ecssvc.DescribeTasksOutput{Tasks: m.tasks}, nil
}

// =============================================================================
// ECS-TASKS child type (per-service task list)
// =============================================================================

// TestPR03c_EcsTasksChildType_ColorReadsFindings asserts that the "ecs_tasks"
// child type has a Color func that (a) reads wave1 Findings first and (b) applies
// structural overrides for UNHEALTHY health status and STOPPED + non-UserInitiated
// stop_code. Without the Color func the child type falls back to fallbackColor,
// which returns ColorHealthy for every status — masking PROVISIONING/STOPPING/
// DEPROVISIONING tasks in the per-service task list view.
func TestPR03c_EcsTasksChildType_ColorReadsFindings(t *testing.T) {
	td := resource.GetChildType("ecs_tasks")
	if td == nil {
		t.Fatal("ecs_tasks child type not registered")
	}
	if td.Color == nil {
		t.Fatal("ecs_tasks child type Color func not set — CR P2 regression")
	}

	// PROVISIONING via wave1 Finding → ColorWarning
	r := resource.Resource{
		Type: "ecs_tasks",
		Findings: []domain.Finding{
			{Code: awsclient.CodeECSTaskStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": "PROVISIONING"},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("PROVISIONING task: Color = %v, want ColorWarning", got)
	}

	// STOPPED + non-UserInitiated stop_code → ColorBroken (structural override).
	r2 := resource.Resource{
		Type:     "ecs_tasks",
		Findings: nil,
		Fields: map[string]string{
			"status":    "STOPPED",
			"stop_code": "EssentialContainerExited",
		},
	}
	if got := td.Color(r2); got != resource.ColorBroken {
		t.Errorf("STOPPED + non-UserInitiated stop_code: Color = %v, want ColorBroken", got)
	}

	// RUNNING → ColorHealthy
	r3 := resource.Resource{
		Type:     "ecs_tasks",
		Findings: nil,
		Fields:   map[string]string{"status": "RUNNING"},
	}
	if got := td.Color(r3); got != resource.ColorHealthy {
		t.Errorf("RUNNING task: Color = %v, want ColorHealthy", got)
	}

	// health == UNHEALTHY → ColorBroken (structural override).
	r4 := resource.Resource{
		Type:     "ecs_tasks",
		Findings: nil,
		Fields: map[string]string{
			"status": "RUNNING",
			"health": "UNHEALTHY",
		},
	}
	if got := td.Color(r4); got != resource.ColorBroken {
		t.Errorf("UNHEALTHY task: Color = %v, want ColorBroken", got)
	}
}
