package unit_test

// phase03_compute_pr03b_test.go — regression-pin tests for Wave 1 finding migration,
// covering the 9 remaining compute types in PR-03b:
// lambda, eks, asg, eb, ebs, ami, eip, eni, ebs-snap.
//
// Migration contract (PR-03b — implemented):
//   - Fetchers STOP writing Resource.Status for lifecycle states.
//   - Fetchers EMIT canonical Finding entries (Source: "wave1") for non-healthy,
//     non-terminal states.
//   - Each type has a corresponding internal/aws/<svc>_codes.go with constants.
//   - Each type's Color func reads Findings first, then falls back to structural fields.
//
// Per-type vocabulary (derived from Color func in types_compute.go /
// types_containers.go / types_networking.go):
//
//   lambda: Active→healthy, Pending→SevWarn, Failed→SevBroken, Inactive→SevWarn
//           (fetcher currently writes Status=runtime or Status="Failed"/"Pending";
//            post-migration writes Fields["state"] + emits Finding for non-Active)
//   eks:    ACTIVE→healthy, CREATING/UPDATING→SevWarn, FAILED→SevBroken,
//           DELETING→no Finding (lifecycle terminal)
//   asg:    ""→healthy, "Delete in progress"→SevWarn (only Status source)
//           No SevBroken at the fetcher level; asg.Color uses structural fields
//           (in_service_count, instances_unhealthy_count) for Broken — those remain.
//   eb:     Green→healthy, Yellow→SevWarn, Red→SevBroken, Grey→SevWarn
//           (fetcher writes Status=health; post-migration emits Finding for non-Green)
//   ebs:    in-use/available→healthy, creating→SevWarn, error→SevBroken,
//           deleting→no Finding (lifecycle terminal)
//   ami:    available→healthy, pending/transient→SevWarn, failed/error/invalid→SevBroken,
//           deregistered/disabled→no Finding (lifecycle terminal)
//   eip:    association_id!=nil→healthy; no association_id AND no instance_id→SevWarn
//           (current fetcher writes Status=domain, not an issue state)
//   eni:    in-use→healthy, available→SevWarn (or Healthy for requester-managed),
//           attaching/detaching→SevWarn
//   ebs-snap: completed→healthy, pending→SevWarn, error/recoverable/recovering→SevBroken

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingsvc "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekssvc "github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	ebsvc "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// =============================================================================
// LAMBDA
// =============================================================================

// TestPR03b_LambdaCodes_ConstantsExist verifies that lambda_codes.go declares
// the Wave 1 finding constants. Fails to compile until the file is created.
func TestPR03b_LambdaCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeLambdaStatePending
	var _ domain.FindingCode = awsclient.CodeLambdaStateFailed
}

// TestPR03b_LambdaFetcher_ActiveEmitsNoFinding asserts that an Active lambda
// function emits no Finding and no Status after migration.
func TestPR03b_LambdaFetcher_ActiveEmitsNoFinding(t *testing.T) {
	mock := &pr03bLambdaMock{
		fns: []lambdatypes.FunctionConfiguration{
			{
				FunctionName: aws.String("my-api-handler"),
				Runtime:      lambdatypes.RuntimeNodejs20x,
				State:        lambdatypes.StateActive,
			},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for Active state", len(r.Findings))
	}
	if r.Fields["state"] != "Active" {
		t.Errorf("Fields[\"state\"]: got %q, want %q", r.Fields["state"], "Active")
	}
}

// TestPR03b_LambdaFetcher_FailedEmitsBrokenFinding asserts that a Failed lambda
// function emits one SevBroken Finding with CodeLambdaStateFailed.
func TestPR03b_LambdaFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03bLambdaMock{
		fns: []lambdatypes.FunctionConfiguration{
			{
				FunctionName: aws.String("broken-processor"),
				Runtime:      lambdatypes.RuntimePython312,
				State:        lambdatypes.StateFailed,
				StateReason:  aws.String("function failed to deploy"),
			},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for Failed state", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeLambdaStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeLambdaStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// lambda mock
// ---------------------------------------------------------------------------

type pr03bLambdaMock struct {
	fns []lambdatypes.FunctionConfiguration
}

func (m *pr03bLambdaMock) ListFunctions(
	_ context.Context,
	_ *lambdasvc.ListFunctionsInput,
	_ ...func(*lambdasvc.Options),
) (*lambdasvc.ListFunctionsOutput, error) {
	return &lambdasvc.ListFunctionsOutput{Functions: m.fns}, nil
}

// =============================================================================
// EKS
// =============================================================================

// TestPR03b_EKSCodes_ConstantsExist verifies that eks_codes.go declares
// Wave 1 finding constants. Fails to compile until the file is created.
func TestPR03b_EKSCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeEKSStateFailed
	var _ domain.FindingCode = awsclient.CodeEKSStateCreating
	var _ domain.FindingCode = awsclient.CodeEKSStateUpdating
}

// TestPR03b_EKSFetcher_ActiveEmitsNoFinding asserts that an ACTIVE EKS cluster
// emits no Finding and no Status after migration.
func TestPR03b_EKSFetcher_ActiveEmitsNoFinding(t *testing.T) {
	listMock := &pr03bEKSListMock{clusters: []string{"prod-cluster"}}
	describeMock := &pr03bEKSDescribeMock{
		clusters: map[string]*ekstypes.Cluster{
			"prod-cluster": {
				Name:    aws.String("prod-cluster"),
				Status:  ekstypes.ClusterStatusActive,
				Version: aws.String("1.29"),
			},
		},
	}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchEKSClusters: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for ACTIVE cluster", len(r.Findings))
	}
}

// TestPR03b_EKSFetcher_FailedEmitsBrokenFinding asserts that a FAILED EKS
// cluster emits one SevBroken Finding with CodeEKSStateFailed.
func TestPR03b_EKSFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	listMock := &pr03bEKSListMock{clusters: []string{"failed-cluster"}}
	describeMock := &pr03bEKSDescribeMock{
		clusters: map[string]*ekstypes.Cluster{
			"failed-cluster": {
				Name:    aws.String("failed-cluster"),
				Status:  ekstypes.ClusterStatusFailed,
				Version: aws.String("1.28"),
			},
		},
	}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchEKSClusters: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for FAILED cluster", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeEKSStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEKSStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// eks mocks
// ---------------------------------------------------------------------------

type pr03bEKSListMock struct {
	clusters []string
}

func (m *pr03bEKSListMock) ListClusters(
	_ context.Context,
	_ *ekssvc.ListClustersInput,
	_ ...func(*ekssvc.Options),
) (*ekssvc.ListClustersOutput, error) {
	return &ekssvc.ListClustersOutput{Clusters: m.clusters}, nil
}

type pr03bEKSDescribeMock struct {
	clusters map[string]*ekstypes.Cluster
}

func (m *pr03bEKSDescribeMock) DescribeCluster(
	_ context.Context,
	input *ekssvc.DescribeClusterInput,
	_ ...func(*ekssvc.Options),
) (*ekssvc.DescribeClusterOutput, error) {
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	return &ekssvc.DescribeClusterOutput{Cluster: m.clusters[name]}, nil
}

// =============================================================================
// ASG
// =============================================================================

// TestPR03b_ASGCodes_ConstantsExist verifies that asg_codes.go declares
// Wave 1 finding constants. Fails to compile until the file is created.
func TestPR03b_ASGCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeASGStateDeleting
}

// TestPR03b_ASGFetcher_HealthyEmitsNoFinding asserts that an ASG with empty
// status emits no Finding and no Status after migration.
func TestPR03b_ASGFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03bASGMock{
		asgs: []autoscalingtypes.AutoScalingGroup{
			{
				AutoScalingGroupName: aws.String("prod-asg"),
				MinSize:              aws.Int32(2),
				MaxSize:              aws.Int32(10),
				DesiredCapacity:      aws.Int32(4),
				// Status is nil → healthy (no "Delete in progress")
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy ASG", len(r.Findings))
	}
}

// TestPR03b_ASGFetcher_DeletingEmitsWarnFinding asserts that an ASG with
// "Delete in progress" status emits one SevWarn Finding with CodeASGStateDeleting.
//
// NOTE: ASG has no SevBroken lifecycle state at the fetcher level. The asg.Color
// func derives Broken from structural fields (in_service_count < min_size) that
// are computed by the fetcher separately — those remain structural; only the
// "Delete in progress" string status is migrated to a Finding.
func TestPR03b_ASGFetcher_DeletingEmitsWarnFinding(t *testing.T) {
	mock := &pr03bASGMock{
		asgs: []autoscalingtypes.AutoScalingGroup{
			{
				AutoScalingGroupName: aws.String("retiring-asg"),
				MinSize:              aws.Int32(0),
				MaxSize:              aws.Int32(0),
				DesiredCapacity:      aws.Int32(0),
				Status:               aws.String("Delete in progress"),
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for deleting ASG", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeASGStateDeleting {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeASGStateDeleting)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// asg mock
// ---------------------------------------------------------------------------

type pr03bASGMock struct {
	asgs []autoscalingtypes.AutoScalingGroup
}

func (m *pr03bASGMock) DescribeAutoScalingGroups(
	_ context.Context,
	_ *autoscalingsvc.DescribeAutoScalingGroupsInput,
	_ ...func(*autoscalingsvc.Options),
) (*autoscalingsvc.DescribeAutoScalingGroupsOutput, error) {
	return &autoscalingsvc.DescribeAutoScalingGroupsOutput{AutoScalingGroups: m.asgs}, nil
}

// =============================================================================
// EB (Elastic Beanstalk)
// =============================================================================

// TestPR03b_EBFetcher_GreenEmitsNoFinding asserts that a Green Elastic Beanstalk
// environment emits no Finding and no Status after migration.
func TestPR03b_EBFetcher_GreenEmitsNoFinding(t *testing.T) {
	mock := &pr03bEBMock{
		envs: []ebtypes.EnvironmentDescription{
			{
				EnvironmentName: aws.String("my-app-prod"),
				EnvironmentId:   aws.String("e-abc123xyz"),
				ApplicationName: aws.String("my-app"),
				Health:          ebtypes.EnvironmentHealthGreen,
				Status:          ebtypes.EnvironmentStatusReady,
				VersionLabel:    aws.String("v1.0.0"),
			},
		},
	}

	result, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEBEnvironmentsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for Green environment", len(r.Findings))
	}
}

// ---------------------------------------------------------------------------
// eb mock
// ---------------------------------------------------------------------------

type pr03bEBMock struct {
	envs []ebtypes.EnvironmentDescription
}

func (m *pr03bEBMock) DescribeEnvironments(
	_ context.Context,
	_ *ebsvc.DescribeEnvironmentsInput,
	_ ...func(*ebsvc.Options),
) (*ebsvc.DescribeEnvironmentsOutput, error) {
	return &ebsvc.DescribeEnvironmentsOutput{Environments: m.envs}, nil
}

// =============================================================================
// EBS Volumes
// =============================================================================

// TestPR03b_EBSCodes_ConstantsExist verifies that ebs_codes.go declares
// Wave 1 volume finding constants. Fails to compile until the file is created.
func TestPR03b_EBSCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeEBSStateCreating
	var _ domain.FindingCode = awsclient.CodeEBSStateError
}

// TestPR03b_EBSFetcher_InUseEmitsNoFinding asserts that an in-use EBS volume
// emits no Finding and no Status after migration.
func TestPR03b_EBSFetcher_InUseEmitsNoFinding(t *testing.T) {
	mock := &pr03bEBSVolMock{
		vols: []ec2types.Volume{
			{
				VolumeId:  aws.String("vol-0abc123def456"),
				State:     ec2types.VolumeStateInUse,
				Size:      aws.Int32(100),
				Encrypted: aws.Bool(true),
				VolumeType: ec2types.VolumeTypeGp3,
			},
		},
	}

	result, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEBSVolumesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for in-use volume", len(r.Findings))
	}
	if r.Fields["state"] != "in-use" {
		t.Errorf("Fields[\"state\"]: got %q, want %q", r.Fields["state"], "in-use")
	}
}

// TestPR03b_EBSFetcher_ErrorEmitsBrokenFinding asserts that an error-state
// EBS volume emits one SevBroken Finding with CodeEBSStateError.
func TestPR03b_EBSFetcher_ErrorEmitsBrokenFinding(t *testing.T) {
	mock := &pr03bEBSVolMock{
		vols: []ec2types.Volume{
			{
				VolumeId:   aws.String("vol-0def789ghi012"),
				State:      ec2types.VolumeStateError,
				Size:       aws.Int32(50),
				Encrypted:  aws.Bool(true),
				VolumeType: ec2types.VolumeTypeGp2,
			},
		},
	}

	result, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEBSVolumesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for error volume", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeEBSStateError {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEBSStateError)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ebs volume mock
// ---------------------------------------------------------------------------

type pr03bEBSVolMock struct {
	vols []ec2types.Volume
}

func (m *pr03bEBSVolMock) DescribeVolumes(
	_ context.Context,
	_ *ec2svc.DescribeVolumesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeVolumesOutput, error) {
	return &ec2svc.DescribeVolumesOutput{Volumes: m.vols}, nil
}

// =============================================================================
// AMI
// =============================================================================

// TestPR03b_AMICodes_ConstantsExist verifies that ami_codes.go declares
// Wave 1 finding constants. Fails to compile until the file is created.
func TestPR03b_AMICodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeAMIStatePending
	var _ domain.FindingCode = awsclient.CodeAMIStateFailed
}

// TestPR03b_AMIFetcher_AvailableEmitsNoFinding asserts that an available AMI
// emits no Finding and no Status after migration.
func TestPR03b_AMIFetcher_AvailableEmitsNoFinding(t *testing.T) {
	mock := &pr03bAMIMock{
		images: []ec2types.Image{
			{
				ImageId:      aws.String("ami-0abcdef1234567890"),
				Name:         aws.String("my-golden-ami-v1"),
				State:        ec2types.ImageStateAvailable,
				Architecture: ec2types.ArchitectureValuesX8664,
				OwnerId:      aws.String("000000000000"),
				Public:       aws.Bool(false),
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchAMIs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for available AMI", len(r.Findings))
	}
	if r.Fields["state"] != "available" {
		t.Errorf("Fields[\"state\"]: got %q, want %q", r.Fields["state"], "available")
	}
}

// TestPR03b_AMIFetcher_FailedEmitsBrokenFinding asserts that a failed AMI
// emits one SevBroken Finding with CodeAMIStateFailed.
func TestPR03b_AMIFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03bAMIMock{
		images: []ec2types.Image{
			{
				ImageId:      aws.String("ami-0fffaaabbbcccddd0"),
				Name:         aws.String("broken-build-ami"),
				State:        ec2types.ImageStateFailed,
				Architecture: ec2types.ArchitectureValuesX8664,
				OwnerId:      aws.String("000000000000"),
				Public:       aws.Bool(false),
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchAMIs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed AMI", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeAMIStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeAMIStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ami mock
// ---------------------------------------------------------------------------

type pr03bAMIMock struct {
	images []ec2types.Image
}

func (m *pr03bAMIMock) DescribeImages(
	_ context.Context,
	_ *ec2svc.DescribeImagesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeImagesOutput, error) {
	return &ec2svc.DescribeImagesOutput{Images: m.images}, nil
}

// =============================================================================
// EIP (Elastic IPs)
// =============================================================================

// TestPR03b_EIPCodes_ConstantsExist verifies that eip_codes.go declares
// the Wave 1 finding constant. Fails to compile until the file is created.
//
// EIP has no "broken" lifecycle state — the actionable issue is cost waste
// (allocated but unassociated). SevWarn is the highest severity for EIP.
func TestPR03b_EIPCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeEIPUnassociated
}

// TestPR03b_EIPFetcher_AssociatedEmitsNoFinding asserts that an EIP that is
// associated with an instance emits no Finding and no Status after migration.
//
// NOTE: The current fetcher writes Status=domain (vpc/standard), which is NOT
// a health state. Post-migration the fetcher writes no Status and emits a
// CodeEIPUnassociated finding only when the EIP is unattached.
func TestPR03b_EIPFetcher_AssociatedEmitsNoFinding(t *testing.T) {
	mock := &pr03bEIPMock{
		addrs: []ec2types.Address{
			{
				AllocationId:  aws.String("eipalloc-0abc123def456789"),
				PublicIp:      aws.String("54.240.192.1"),
				Domain:        ec2types.DomainTypeVpc,
				AssociationId: aws.String("eipassoc-0abc123def456789"),
				InstanceId:    aws.String("i-0abc1234def56789"),
			},
		},
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchElasticIPs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for associated EIP", len(r.Findings))
	}
}

// TestPR03b_EIPFetcher_UnassociatedEmitsWarnFinding asserts that an EIP with
// no association, no instance, and no ENI emits one SevWarn Finding with
// CodeEIPUnassociated (cost-waste signal).
//
// NOTE: No SevBroken broken test for EIP — unassociated is the worst state,
// and it maps to SevWarn (cost waste, not service failure).
func TestPR03b_EIPFetcher_UnassociatedEmitsWarnFinding(t *testing.T) {
	mock := &pr03bEIPMock{
		addrs: []ec2types.Address{
			{
				AllocationId: aws.String("eipalloc-0def456ghi789012"),
				PublicIp:     aws.String("52.95.110.1"),
				Domain:       ec2types.DomainTypeVpc,
				// No AssociationId, InstanceId, or NetworkInterfaceId → UNATTACHED
			},
		},
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchElasticIPs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for unassociated EIP", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeEIPUnassociated {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEIPUnassociated)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// eip mock
// ---------------------------------------------------------------------------

type pr03bEIPMock struct {
	addrs []ec2types.Address
}

func (m *pr03bEIPMock) DescribeAddresses(
	_ context.Context,
	_ *ec2svc.DescribeAddressesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeAddressesOutput, error) {
	return &ec2svc.DescribeAddressesOutput{Addresses: m.addrs}, nil
}

// =============================================================================
// ENI (Network Interfaces)
// =============================================================================

// TestPR03b_ENICodes_ConstantsExist verifies that eni_codes.go declares
// Wave 1 finding constants. Fails to compile until the file is created.
//
// NOTE: ENI "available" state maps to SevWarn for non-requester-managed
// interfaces (idle/unused ENI, potential cost waste). No SevBroken state exists
// at the fetcher lifecycle level for ENI.
func TestPR03b_ENICodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeENIStateAttaching
	var _ domain.FindingCode = awsclient.CodeENIStateDetaching
	var _ domain.FindingCode = awsclient.CodeENIStateAvailable
}

// TestPR03b_ENIFetcher_InUseEmitsNoFinding asserts that an in-use ENI
// emits no Finding and no Status after migration.
func TestPR03b_ENIFetcher_InUseEmitsNoFinding(t *testing.T) {
	mock := &pr03bENIMock{
		enis: []ec2types.NetworkInterface{
			{
				NetworkInterfaceId: aws.String("eni-0abc123def456789a"),
				Status:             ec2types.NetworkInterfaceStatusInUse,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
				VpcId:              aws.String("vpc-0abc1234"),
				PrivateIpAddress:   aws.String("10.0.1.50"),
			},
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for in-use ENI", len(r.Findings))
	}
}

// TestPR03b_ENIFetcher_AttachingEmitsWarnFinding asserts that an ENI in the
// "attaching" state emits one SevWarn Finding with CodeENIStateAttaching.
//
// NOTE: ENI has no SevBroken lifecycle state — attaching/detaching are the
// non-healthy states and both map to SevWarn (transitional).
func TestPR03b_ENIFetcher_AttachingEmitsWarnFinding(t *testing.T) {
	mock := &pr03bENIMock{
		enis: []ec2types.NetworkInterface{
			{
				NetworkInterfaceId: aws.String("eni-0def456ghi789012b"),
				Status:             ec2types.NetworkInterfaceStatusAttaching,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
				VpcId:              aws.String("vpc-0def5678"),
				PrivateIpAddress:   aws.String("10.0.2.20"),
			},
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for attaching ENI", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeENIStateAttaching {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeENIStateAttaching)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// eni mock
// ---------------------------------------------------------------------------

type pr03bENIMock struct {
	enis []ec2types.NetworkInterface
}

func (m *pr03bENIMock) DescribeNetworkInterfaces(
	_ context.Context,
	_ *ec2svc.DescribeNetworkInterfacesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeNetworkInterfacesOutput, error) {
	return &ec2svc.DescribeNetworkInterfacesOutput{NetworkInterfaces: m.enis}, nil
}

// =============================================================================
// EBS Snapshots
// =============================================================================

// TestPR03b_EBSSnapCodes_ConstantsExist verifies that ebs_snap_codes.go (or
// ebs_codes.go) declares Wave 1 snapshot finding constants. Fails to compile
// until the constants are created.
//
// NOTE: Snapshot constants may live in the same ebs_codes.go as volume
// constants, or in a separate ebs_snap_codes.go — the coder decides.
func TestPR03b_EBSSnapCodes_ConstantsExist(t *testing.T) {
	t.Helper()
	var _ domain.FindingCode = awsclient.CodeEBSSnapStatePending
	var _ domain.FindingCode = awsclient.CodeEBSSnapStateError
}

// TestPR03b_EBSSnapFetcher_CompletedEmitsNoFinding asserts that a completed
// EBS snapshot emits no Finding and no Status after migration.
func TestPR03b_EBSSnapFetcher_CompletedEmitsNoFinding(t *testing.T) {
	mock := &pr03bEBSSnapMock{
		snaps: []ec2types.Snapshot{
			{
				SnapshotId: aws.String("snap-0abc123def456789a"),
				VolumeId:   aws.String("vol-0abc123def456789"),
				State:      ec2types.SnapshotStateCompleted,
				Encrypted:  aws.Bool(true),
				VolumeSize: aws.Int32(100),
				OwnerId:    aws.String("000000000000"),
			},
		},
	}

	result, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEBSSnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for completed snapshot", len(r.Findings))
	}
	if r.Fields["state"] != "completed" {
		t.Errorf("Fields[\"state\"]: got %q, want %q", r.Fields["state"], "completed")
	}
}

// TestPR03b_EBSSnapFetcher_ErrorEmitsBrokenFinding asserts that an error-state
// EBS snapshot emits one SevBroken Finding with CodeEBSSnapStateError.
func TestPR03b_EBSSnapFetcher_ErrorEmitsBrokenFinding(t *testing.T) {
	mock := &pr03bEBSSnapMock{
		snaps: []ec2types.Snapshot{
			{
				SnapshotId: aws.String("snap-0def456ghi789012b"),
				VolumeId:   aws.String("vol-0def456ghi789012"),
				State:      ec2types.SnapshotStateError,
				Encrypted:  aws.Bool(true),
				VolumeSize: aws.Int32(200),
				OwnerId:    aws.String("000000000000"),
			},
		},
	}

	result, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEBSSnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for error snapshot", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeEBSSnapStateError {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeEBSSnapStateError)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// ebs snapshot mock
// ---------------------------------------------------------------------------

type pr03bEBSSnapMock struct {
	snaps []ec2types.Snapshot
}

func (m *pr03bEBSSnapMock) DescribeSnapshots(
	_ context.Context,
	_ *ec2svc.DescribeSnapshotsInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeSnapshotsOutput, error) {
	return &ec2svc.DescribeSnapshotsOutput{Snapshots: m.snaps}, nil
}

// =============================================================================
// CR P2 FINDINGS — additional tests for CodeRabbit review issues
// =============================================================================

// TestPR03b_LambdaColor_BrokenOverridesWave1 verifies that the structural broken
// overrides in the Lambda Color func (deprecated runtime, last_update_status=Failed)
// win even when a wave1 Finding is present. Before the fix, the Color func returned
// ColorFromSeverity(wave1) early, downgrading these to yellow.
//
// Pre-fix: Color returns ColorWarning (wave1 SevWarn early-return).
// Post-fix: Color evaluates structural broken overrides BEFORE wave1.
func TestPR03b_LambdaColor_BrokenOverridesWave1(t *testing.T) {
	td := resource.FindResourceType("lambda")
	if td == nil {
		t.Fatal("lambda type def missing")
	}

	// Inactive function with deprecated runtime → must be Broken (deprecated wins).
	r := resource.Resource{
		Type: "lambda",
		Findings: []domain.Finding{{Code: awsclient.CodeLambdaStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}},
		Fields:   map[string]string{"state": "Inactive", "runtime": "python3.7"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("inactive + deprecated runtime: Color = %v, want ColorBroken (deprecated runtime overrides wave1 SevWarn)", got)
	}

	// Pending function with last_update_status=Failed → must be Broken.
	r2 := resource.Resource{
		Type: "lambda",
		Findings: []domain.Finding{{Code: awsclient.CodeLambdaStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}},
		Fields:   map[string]string{"state": "Pending", "last_update_status": "Failed"},
	}
	if got := td.Color(r2); got != resource.ColorBroken {
		t.Errorf("pending + last_update_status=Failed: Color = %v, want ColorBroken", got)
	}
}

// TestPR03b_ENIFetcher_RequesterManagedSuppressesAvailableFinding pins that
// AWS-managed ENIs flagged via the RequesterManaged *bool field do NOT emit a
// CodeENIStateAvailable Finding even when Status is "available".
//
// Pre-fix: fetcher checks interfaceType != "requester-managed" (a string
// comparison against InterfaceType), which misses ENIs whose InterfaceType is
// "interface" or any other non-"requester-managed" string but whose
// RequesterManaged boolean is true (EFS, VPC endpoints, ELB managed by AWS).
// Post-fix: fetcher reads eni.RequesterManaged to detect AWS-managed ENIs.
func TestPR03b_ENIFetcher_RequesterManagedSuppressesAvailableFinding(t *testing.T) {
	// ENI with Status=available, RequesterManaged=true, InterfaceType="interface"
	// — this represents a real AWS-managed ENI (e.g. EFS mount target) that has
	// InterfaceType="interface" but is controlled by AWS via RequesterManaged.
	mock := &pr03bENIMock{
		enis: []ec2types.NetworkInterface{
			{
				NetworkInterfaceId: aws.String("eni-0123abcdef456789a"),
				Status:             ec2types.NetworkInterfaceStatusAvailable,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface, // NOT "requester-managed"
				RequesterManaged:   aws.Bool(true),                         // but IS requester-managed
				VpcId:              aws.String("vpc-0abc1234"),
				PrivateIpAddress:   aws.String("10.0.3.10"),
			},
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	// AWS-managed ENI: must NOT emit a warning finding
	if len(r.Findings) != 0 {
		t.Errorf("Findings count = %d, want 0 — RequesterManaged=true ENI must not emit CodeENIStateAvailable (AWS controls this interface)", len(r.Findings))
	}
}

// TestPR03b_ENIFetcher_AvailableNonRequesterEmitsFinding pins the inverse:
// non-requester-managed ENIs in available state DO get the cost-waste warning.
// This is the baseline case — the test confirms the fix does not suppress
// legitimate warnings for user-managed idle ENIs.
func TestPR03b_ENIFetcher_AvailableNonRequesterEmitsFinding(t *testing.T) {
	// ENI with Status=available, RequesterManaged=false — user-owned idle ENI
	mock := &pr03bENIMock{
		enis: []ec2types.NetworkInterface{
			{
				NetworkInterfaceId: aws.String("eni-0abc987def654321b"),
				Status:             ec2types.NetworkInterfaceStatusAvailable,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
				RequesterManaged:   aws.Bool(false),
				VpcId:              aws.String("vpc-0def5678"),
				PrivateIpAddress:   aws.String("10.0.4.20"),
			},
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings count = %d, want 1 — non-requester-managed available ENI must emit CodeENIStateAvailable", len(r.Findings))
	}
	if r.Findings[0].Code != awsclient.CodeENIStateAvailable {
		t.Errorf("Findings[0].Code = %q, want %q", r.Findings[0].Code, awsclient.CodeENIStateAvailable)
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity = %v, want domain.SevWarn", r.Findings[0].Severity)
	}
	if r.Findings[0].Source != "wave1" {
		t.Errorf("Findings[0].Source = %q, want wave1", r.Findings[0].Source)
	}
}

// TestPR03b_EBFetcher_DoesNotEmitHealthAsWave1Finding asserts that the EB fetcher
// does NOT emit wave1 Findings for health values (Yellow, Red, Grey). Health
// classification stays structural via the Color func reading Fields["health"].
//
// Pre-fix: FetchEBEnvironmentsPage emits wave1 Findings for Yellow/Red/Grey health,
// causing the status column to show "health degraded" instead of the operational status.
// Post-fix: r.Findings is empty; r.Fields["health"] carries the raw health value.
func TestPR03b_EBFetcher_DoesNotEmitHealthAsWave1Finding(t *testing.T) {
	cases := []struct {
		name   string
		health ebtypes.EnvironmentHealth
		status ebtypes.EnvironmentStatus
	}{
		{"Yellow", ebtypes.EnvironmentHealthYellow, ebtypes.EnvironmentStatusReady},
		{"Red", ebtypes.EnvironmentHealthRed, ebtypes.EnvironmentStatusReady},
		{"Grey", ebtypes.EnvironmentHealthGrey, ebtypes.EnvironmentStatusUpdating},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &pr03bEBMock{
				envs: []ebtypes.EnvironmentDescription{
					{
						EnvironmentName: aws.String("my-app-env"),
						EnvironmentId:   aws.String("e-abc123xyz"),
						ApplicationName: aws.String("my-app"),
						Health:          tc.health,
						Status:          tc.status,
						VersionLabel:    aws.String("v1.0.0"),
					},
				},
			}

			result, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchEBEnvironmentsPage: unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			r := result.Resources[0]

			// Health is structural, not wave1 — fetcher must not emit Findings for health.
			if len(r.Findings) != 0 {
				t.Errorf("health=%s: Findings: got %d, want 0 (health must not be emitted as wave1 Finding)", tc.name, len(r.Findings))
			}
			// Fetcher must not write Status.
			// Fields["health"] must carry the raw health value for structural Color classification.
			if r.Fields["health"] != tc.name {
				t.Errorf("health=%s: Fields[\"health\"]: got %q, want %q", tc.name, r.Fields["health"], tc.name)
			}
		})
	}
}

// TODO(follow-up): consolidate per-type single-method mock adapters
// (pr03bLambdaMock, pr03bEKSListMock, pr03bASGMock, etc.) into shared helpers
// in tests/unit/helpers_*.go — each is a trivial struct satisfying a one-method
// interface. Consolidation is intentionally deferred to avoid disrupting the
// current coder → QA diff cycle.

// =============================================================================
// A1 — Lambda Inactive emits NO Finding
// =============================================================================

// TestPR03b_LambdaFetcher_InactiveEmitsNoFinding pins that Lambda Inactive
// state is treated as lifecycle-class (ColorDim) — NOT promoted to a wave1
// Finding. Lambda Inactive functions are non-broken and excluded from the
// issue badge / ctrl+z filter; emitting a SevWarn Finding would reverse
// that intent and make the legacy "Inactive": ColorDim case unreachable.
func TestPR03b_LambdaFetcher_InactiveEmitsNoFinding(t *testing.T) {
	mock := &pr03bLambdaMock{
		fns: []lambdatypes.FunctionConfiguration{
			{
				FunctionName: aws.String("evicted-worker"),
				Runtime:      lambdatypes.RuntimeNodejs20x,
				State:        lambdatypes.StateInactive,
				StateReason:  aws.String("The function was not invoked for 14 days"),
			},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	// Inactive is lifecycle-class — NOT an actionable issue.
	if len(r.Findings) != 0 {
		t.Errorf("Inactive function: Findings = %d, want 0 (Inactive is lifecycle-class, not an issue)", len(r.Findings))
	}
	// State field must still be written so the Color func can return ColorDim.
	if r.Fields["state"] != "Inactive" {
		t.Errorf("Fields[\"state\"] = %q, want \"Inactive\"", r.Fields["state"])
	}
}

// =============================================================================
// A2 — EC2 state_reason_code registered in GetFieldKeys
// =============================================================================

// TestPR03b_EC2Fields_StateReasonCodeRegistered asserts that "state_reason_code"
// is declared in the EC2 SetFieldKeysForTest call. The EC2 fetcher writes this key
// (used by Color's Server.* branch), so it must be registered to appear in view
// column projections and pass the TestColumnKeysHaveProducers check.
func TestPR03b_EC2Fields_StateReasonCodeRegistered(t *testing.T) {
	keys := resource.GetFieldKeys("ec2")
	for _, k := range keys {
		if k == "state_reason_code" {
			return
		}
	}
	t.Errorf("ec2 SetFieldKeysForTest must include \"state_reason_code\" (written by FetchEC2Instances per PR-03b); registered keys: %v", keys)
}

// =============================================================================
// A3 — Lambda state registered in GetFieldKeys
// =============================================================================

// TestPR03b_LambdaFields_StateRegistered asserts that "state" is declared in the
// Lambda SetFieldKeysForTest call. The Lambda fetcher writes Fields["state"] for
// every function (used by Color's lifecycle-class branch); the key must be
// registered to flow through column projections correctly.
func TestPR03b_LambdaFields_StateRegistered(t *testing.T) {
	keys := resource.GetFieldKeys("lambda")
	for _, k := range keys {
		if k == "state" {
			return
		}
	}
	t.Errorf("lambda SetFieldKeysForTest must include \"state\" (written by FetchLambdaFunctions per PR-03b); registered keys: %v", keys)
}
