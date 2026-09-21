package unit

// b1_finding_guards_test.go — the conditions under which a posture rule has
// nothing to report: a setting that makes the rule moot, a lifecycle state in
// which the shape it flags is the expected one, an AWS verdict that is not a
// verdict yet.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// b1PhrasesOf joins every finding phrase on a row for one assertion message.
func b1PhrasesOf(fs []domain.Finding) string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Phrase)
	}
	return strings.Join(out, " | ")
}

func b1HasPhrase(fs []domain.Finding, substr string) bool {
	for _, f := range fs {
		if strings.Contains(f.Phrase, substr) {
			return true
		}
	}
	return false
}

// b1ASGGroups runs one page of groups through the fetcher.
func b1ASGGroups(t *testing.T, groups ...asgtypes.AutoScalingGroup) []resource.Resource {
	t.Helper()
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: groups},
	}
	res, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage: %v", err)
	}
	return res.Resources
}

func TestB1ASG_HealthCheckTypeList_CountsELB(t *testing.T) {
	rows := b1ASGGroups(t, asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("prod-web-asg"),
		MinSize:              aws.Int32(2),
		MaxSize:              aws.Int32(6),
		DesiredCapacity:      aws.Int32(3),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/web/abc"},
		HealthCheckType:      aws.String("ELB,EBS"),
		DefaultCooldown:      aws.Int32(300),
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if b1HasPhrase(rows[0].Findings, "health check") {
		t.Errorf("a group set to \"ELB,EBS\" does check load-balancer health; findings: %s", b1PhrasesOf(rows[0].Findings))
	}
}

func TestB1ASG_HealthCheckTypeWithoutELB_StillFlagged(t *testing.T) {
	rows := b1ASGGroups(t, asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("prod-web-asg"),
		MinSize:              aws.Int32(2),
		MaxSize:              aws.Int32(6),
		DesiredCapacity:      aws.Int32(3),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/web/abc"},
		HealthCheckType:      aws.String("EC2,EBS"),
		DefaultCooldown:      aws.Int32(300),
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if !b1HasPhrase(rows[0].Findings, "health check") {
		t.Errorf("a group behind a target group with no ELB health check must be flagged; findings: %s", b1PhrasesOf(rows[0].Findings))
	}
}

func TestB1ASGLaunchConfig_MetadataEndpointDisabled_NoIMDSv1Finding(t *testing.T) {
	fake := &asgLaunchConfigPagesFake{
		pages: []*autoscaling.DescribeLaunchConfigurationsOutput{{
			LaunchConfigurations: []asgtypes.LaunchConfiguration{{
				LaunchConfigurationName: aws.String("lc-locked"),
				MetadataOptions: &asgtypes.InstanceMetadataOptions{
					HttpEndpoint: asgtypes.InstanceMetadataEndpointStateDisabled,
					HttpTokens:   asgtypes.InstanceMetadataHttpTokensStateOptional,
				},
			}},
		}},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, []resource.Resource{asgGroupOn("asg-locked", "lc-locked")}, nil)
	if err != nil {
		t.Fatalf("EnrichASGScalingActivities: %v", err)
	}
	if fs := result.Findings["asg-locked"]; len(fs) != 0 {
		t.Errorf("metadata endpoint disabled leaves nothing for IMDSv1 to permit; findings: %s", b1PhrasesOf(fs))
	}
}

func b1EC2Instance(inst ec2types.Instance) []resource.Resource {
	mock := &mockEC2Client{output: &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{inst}}},
	}}
	res, _ := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	return res.Resources
}

func TestB1EC2_MetadataEndpointDisabled_NoIMDSv1Finding(t *testing.T) {
	rows := b1EC2Instance(ec2types.Instance{
		InstanceId:   aws.String("i-0aaaa1111bbbb2222"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpTokens:   ec2types.HttpTokensStateOptional,
			HttpEndpoint: ec2types.InstanceMetadataEndpointStateDisabled,
		},
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if b1HasPhrase(rows[0].Findings, "IMDSv1") {
		t.Errorf("metadata endpoint disabled leaves nothing for IMDSv1 to permit; findings: %s", b1PhrasesOf(rows[0].Findings))
	}
}

func TestB1EC2_MetadataEndpointEnabled_IMDSv1StillFlagged(t *testing.T) {
	rows := b1EC2Instance(ec2types.Instance{
		InstanceId:   aws.String("i-0aaaa1111bbbb2222"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpTokens:   ec2types.HttpTokensStateOptional,
			HttpEndpoint: ec2types.InstanceMetadataEndpointStateEnabled,
		},
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if !b1HasPhrase(rows[0].Findings, "IMDSv1") {
		t.Errorf("a reachable metadata service with optional tokens must be flagged; findings: %s", b1PhrasesOf(rows[0].Findings))
	}
}

func TestB1EC2_CompletedScheduledEvent_NoFinding(t *testing.T) {
	soon := time.Now().Add(24 * time.Hour)
	fake := &ec2InstanceStatusFake{statuses: []ec2types.InstanceStatus{{
		InstanceId: aws.String("i-0aaaa1111bbbb2222"),
		Events: []ec2types.InstanceStatusEvent{{
			Code:        ec2types.EventCodeSystemReboot,
			Description: aws.String("[Completed] The instance was rebooted"),
			NotBefore:   &soon,
		}},
	}}}
	clients := &awsclient.ServiceClients{EC2: fake}
	rs := []resource.Resource{{ID: "i-0aaaa1111bbbb2222", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	if fs := result.Findings["i-0aaaa1111bbbb2222"]; len(fs) != 0 {
		t.Errorf("a completed event leaves nothing to plan around; findings: %s", b1PhrasesOf(fs))
	}
}

func TestB1EC2_PendingScheduledEvent_StillFlagged(t *testing.T) {
	soon := time.Now().Add(24 * time.Hour)
	fake := &ec2InstanceStatusFake{statuses: []ec2types.InstanceStatus{{
		InstanceId: aws.String("i-0aaaa1111bbbb2222"),
		Events: []ec2types.InstanceStatusEvent{{
			Code:        ec2types.EventCodeSystemReboot,
			Description: aws.String("The instance is scheduled for a reboot"),
			NotBefore:   &soon,
		}},
	}}}
	clients := &awsclient.ServiceClients{EC2: fake}
	rs := []resource.Resource{{ID: "i-0aaaa1111bbbb2222", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	if fs := result.Findings["i-0aaaa1111bbbb2222"]; len(fs) == 0 {
		t.Error("an event still ahead of the instance must be flagged")
	}
}

func b1EBSVolumeStatus(t *testing.T, status ec2types.VolumeStatusInfoStatus) []domain.Finding {
	t.Helper()
	out := &ec2.DescribeVolumeStatusOutput{VolumeStatuses: []ec2types.VolumeStatusItem{{
		VolumeId:     aws.String("vol-0aaaa1111bbbb2222"),
		VolumeStatus: &ec2types.VolumeStatusInfo{Status: status},
	}}}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichEBSVolumeStatus: %v", err)
	}
	return result.Findings["vol-0aaaa1111bbbb2222"]
}

func TestB1EBS_InsufficientData_IsNotDegraded(t *testing.T) {
	if fs := b1EBSVolumeStatus(t, ec2types.VolumeStatusInfoStatusInsufficientData); len(fs) != 0 {
		t.Errorf("insufficient-data means the checks have not finished, not that the volume is broken; findings: %s", b1PhrasesOf(fs))
	}
}

func TestB1EBS_ImpairedAndWarning_AreDegraded(t *testing.T) {
	for _, status := range []ec2types.VolumeStatusInfoStatus{
		ec2types.VolumeStatusInfoStatusImpaired,
		ec2types.VolumeStatusInfoStatusWarning,
	} {
		if fs := b1EBSVolumeStatus(t, status); len(fs) == 0 {
			t.Errorf("%s must raise the degraded finding", status)
		}
	}
}

// b1EBSSnapshotCoverage runs the volume enricher against one snapshot of the
// volume in the given state and reports whether the volume was told it has no
// snapshot.
func b1EBSSnapshotCoverage(t *testing.T, state ec2types.SnapshotState) []domain.Finding {
	t.Helper()
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{}}
	volumes := []resource.Resource{{
		ID:     "vol-0aaaa1111bbbb2222",
		Fields: map[string]string{"volume_id": "vol-0aaaa1111bbbb2222", "state": string(ec2types.VolumeStateInUse)},
	}}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID:     "snap-0aaaa1111bbbb2222",
			Fields: map[string]string{"snapshot_id": "snap-0aaaa1111bbbb2222", "volume_id": "vol-0aaaa1111bbbb2222", "state": string(state)},
		}}},
	}
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, volumes, cache)
	if err != nil {
		t.Fatalf("EnrichEBSVolumeStatus: %v", err)
	}
	return result.Findings["vol-0aaaa1111bbbb2222"]
}

func TestB1EBS_SnapshotInErrorIsNotCoverage(t *testing.T) {
	if !b1HasPhrase(b1EBSSnapshotCoverage(t, ec2types.SnapshotStateError), "snapshot") {
		t.Error("a snapshot in error cannot be restored from, so the volume still has no snapshot behind it")
	}
}

func TestB1EBS_CompletedSnapshotIsCoverage(t *testing.T) {
	if fs := b1EBSSnapshotCoverage(t, ec2types.SnapshotStateCompleted); b1HasPhrase(fs, "snapshot") {
		t.Errorf("a completed snapshot covers the volume; findings: %s", b1PhrasesOf(fs))
	}
}

func b1ECSClusterFindings(t *testing.T, status string) []domain.Finding {
	t.Helper()
	fake := &fakeECSEnricher{descClustersOut: &ecs.DescribeClustersOutput{
		Clusters: []ecstypes.Cluster{{
			ClusterName:                       aws.String("acme-cluster"),
			Status:                            aws.String(status),
			PendingTasksCount:                 5,
			RunningTasksCount:                 0,
			RegisteredContainerInstancesCount: 3,
		}},
	}}
	clients := &awsclient.ServiceClients{ECS: fake}
	rs := []resource.Resource{{ID: "acme-cluster", Name: "acme-cluster", Fields: map[string]string{"cluster_name": "acme-cluster"}}}
	result, err := awsclient.EnrichECSClusters(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichECSClusters: %v", err)
	}
	return result.Findings["acme-cluster"]
}

func TestB1ECS_NonActiveClusterHasNoTaskCountFinding(t *testing.T) {
	for _, status := range []string{"PROVISIONING", "DEPROVISIONING", "INACTIVE"} {
		if fs := b1ECSClusterFindings(t, status); len(fs) != 0 {
			t.Errorf("%s cluster: task counts describe a running cluster; findings: %s", status, b1PhrasesOf(fs))
		}
	}
}

func TestB1ECS_ActiveClusterStillFlagged(t *testing.T) {
	if fs := b1ECSClusterFindings(t, "ACTIVE"); len(fs) == 0 {
		t.Error("an ACTIVE cluster with pending tasks and none running must be flagged")
	}
}

func TestB1ECSTask_WindowsContainerHasNoWritableRootFinding(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4444"
	c := pw1HealthyContainer("app")
	c.ReadonlyRootFilesystem = nil
	def := pw1TaskDef(pw1TaskDefARN, c)
	def.NetworkMode = ecstypes.NetworkModeAwsvpc
	def.RuntimePlatform = &ecstypes.RuntimePlatform{
		OperatingSystemFamily: ecstypes.OSFamilyWindowsServer2022Core,
		CpuArchitecture:       ecstypes.CPUArchitectureX8664,
	}

	result := pw1RunOneTask(t, id, def)
	if b1HasPhrase(result.Findings[id], "root filesystem") {
		t.Errorf("readonlyRootFilesystem is unsupported on Windows, so the definition has no setting to change; findings: %s", b1PhrasesOf(result.Findings[id]))
	}
}

func TestB1ECSTask_LinuxContainerStillFlagged(t *testing.T) {
	const id = "0aaaa1111bbbb2222cccc3333dddd4444"
	c := pw1HealthyContainer("app")
	c.ReadonlyRootFilesystem = nil
	def := pw1TaskDef(pw1TaskDefARN, c)
	def.RuntimePlatform = &ecstypes.RuntimePlatform{OperatingSystemFamily: ecstypes.OSFamilyLinux}

	result := pw1RunOneTask(t, id, def)
	if !b1HasPhrase(result.Findings[id], "root filesystem") {
		t.Errorf("a Linux container with a writable root must be flagged; findings: %s", b1PhrasesOf(result.Findings[id]))
	}
}

// b1TransferCertFake answers DescribeCertificate with a certificate that goes
// inactive in half a day; every other call goes to the demo fake.
type b1TransferCertFake struct {
	*fakes.TransferFake
}

func (f *b1TransferCertFake) DescribeCertificate(_ context.Context, _ *transfer.DescribeCertificateInput, _ ...func(*transfer.Options)) (*transfer.DescribeCertificateOutput, error) {
	inactive := time.Now().Add(12 * time.Hour)
	return &transfer.DescribeCertificateOutput{Certificate: &transfertypes.DescribedCertificate{
		CertificateId: aws.String("cert-b1"),
		Status:        transfertypes.CertificateStatusTypeActive,
		InactiveDate:  &inactive,
	}}, nil
}

func TestB1TransferCertificate_LastHoursReadAsOneDay(t *testing.T) {
	childShortName := transferAgreementsChildShortName(t)
	enrich := resource.GetDetailEnricher(childShortName)
	if enrich == nil {
		t.Fatalf("no DetailEnricher registered for %q", childShortName)
	}
	agreement := mustFindTransferResource(t, fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID), fixtures.AgreementProdPartnerID)

	clients := &awsclient.ServiceClients{Transfer: &b1TransferCertFake{TransferFake: fakes.NewTransfer()}}
	enriched, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, agreement)
	if err != nil {
		t.Fatalf("DetailEnrich: %v", err)
	}
	if b1HasPhrase(enriched.Findings, "expires in 0d") {
		t.Errorf("a certificate with hours left still has a day on it; findings: %s", b1PhrasesOf(enriched.Findings))
	}
	if !b1HasPhrase(enriched.Findings, "expires in 1d") {
		t.Errorf("want \"expires in 1d\"; findings: %s", b1PhrasesOf(enriched.Findings))
	}
}
