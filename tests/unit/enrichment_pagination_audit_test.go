package unit

// enrichment_pagination_audit_test.go — account-wide enricher pagination and
// a meta-test AST audit.
//
// Account-wide enrichers walk every page of their List/Describe API, capped
// at EnrichmentCap pages to avoid unbounded API calls. The structural
// meta-test (AST walk of *_issue_enrichment.go files) flags an enricher that
// calls a paginated API without a loop.
//
// # Covered enrichers
//
//   - EnrichBackupJobs            (backup.ListBackupJobs   — NextToken)
//   - EnrichEC2InstanceStatus     (ec2.DescribeInstanceStatus — NextToken)
//   - EnrichEBSVolumeStatus       (ec2.DescribeVolumeStatus  — NextToken)
//
// ASG / TGW / VPCFlowLogs use per-resource DescribeXxx calls keyed by resource
// ID and are capped at EnrichmentCap resources — not account-wide scans. They
// are out of scope for this file.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backupsdk "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	ec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Fake: backup client (ListBackupJobs with NextToken pagination)
// ---------------------------------------------------------------------------

// backupPaginatedFake implements BackupAPI, serving ordered pages for
// ListBackupJobs calls. Each successive call (regardless of NextToken value)
// returns the next entry from pages[].
type backupPaginatedFake struct {
	awsclient.BackupAPI

	pages     []*backupsdk.ListBackupJobsOutput
	callCount int
}

func newBackupPaginatedFake(pages ...*backupsdk.ListBackupJobsOutput) *backupPaginatedFake {
	return &backupPaginatedFake{pages: pages}
}

func (f *backupPaginatedFake) ListBackupJobs(
	_ context.Context,
	_ *backupsdk.ListBackupJobsInput,
	_ ...func(*backupsdk.Options),
) (*backupsdk.ListBackupJobsOutput, error) {
	idx := f.callCount
	f.callCount++
	if idx >= len(f.pages) {
		// Past the defined pages: return empty, no NextToken.
		return &backupsdk.ListBackupJobsOutput{}, nil
	}
	return f.pages[idx], nil
}

// Compile-time check.
var _ awsclient.BackupAPI = (*backupPaginatedFake)(nil)

// ---------------------------------------------------------------------------
// Fake: EC2 client (DescribeInstanceStatus + DescribeVolumeStatus with NextToken)
// ---------------------------------------------------------------------------

// ec2PaginatedFake implements EC2API, serving ordered pages for
// DescribeInstanceStatus and DescribeVolumeStatus calls independently.
type ec2PaginatedFake struct {
	awsclient.EC2API

	instanceStatusPages     []*ec2sdk.DescribeInstanceStatusOutput
	instanceStatusCallCount int

	volumeStatusPages     []*ec2sdk.DescribeVolumeStatusOutput
	volumeStatusCallCount int
}

func newEC2PaginatedFake() *ec2PaginatedFake {
	return &ec2PaginatedFake{}
}

func (f *ec2PaginatedFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2sdk.DescribeInstanceStatusInput,
	_ ...func(*ec2sdk.Options),
) (*ec2sdk.DescribeInstanceStatusOutput, error) {
	idx := f.instanceStatusCallCount
	f.instanceStatusCallCount++
	if idx >= len(f.instanceStatusPages) {
		return &ec2sdk.DescribeInstanceStatusOutput{}, nil
	}
	return f.instanceStatusPages[idx], nil
}

func (f *ec2PaginatedFake) DescribeVolumeStatus(
	_ context.Context,
	_ *ec2sdk.DescribeVolumeStatusInput,
	_ ...func(*ec2sdk.Options),
) (*ec2sdk.DescribeVolumeStatusOutput, error) {
	idx := f.volumeStatusCallCount
	f.volumeStatusCallCount++
	if idx >= len(f.volumeStatusPages) {
		return &ec2sdk.DescribeVolumeStatusOutput{}, nil
	}
	return f.volumeStatusPages[idx], nil
}

// Compile-time check.
var _ awsclient.EC2API = (*ec2PaginatedFake)(nil)

// ---------------------------------------------------------------------------
// Helpers — test resource builders
// ---------------------------------------------------------------------------

// backupResources builds minimal resource.Resource slices for backup tests.
// BackupJobs is account-wide, so resources is unused by the enricher — we pass
// an empty slice to satisfy the function signature.
func backupResources() []resource.Resource {
	return []resource.Resource{}
}

// ec2InstanceResources builds minimal EC2 instance resource stubs.
func ec2InstanceResources(ids ...string) []resource.Resource {
	rr := make([]resource.Resource, len(ids))
	for i, id := range ids {
		rr[i] = resource.Resource{ID: id, Name: id}
	}
	return rr
}

// ebsVolumeResources builds minimal EBS volume resource stubs.
func ebsVolumeResources(ids ...string) []resource.Resource {
	rr := make([]resource.Resource, len(ids))
	for i, id := range ids {
		rr[i] = resource.Resource{ID: id, Name: id}
	}
	return rr
}

// makeBackupJob creates a BackupJob with the given plan ID, job ID, state and
// a CreationDate recent enough to be within the 24-hour finding window.
func makeBackupJob(planID, jobID string, state backuptypes.BackupJobState) backuptypes.BackupJob {
	now := time.Now()
	job := backuptypes.BackupJob{
		BackupJobId:  aws.String(jobID),
		State:        state,
		CreationDate: &now,
	}
	if planID != "" {
		job.CreatedBy = &backuptypes.RecoveryPointCreator{
			BackupPlanId: aws.String(planID),
		}
	}
	return job
}

// makeInstanceStatus creates an EC2 InstanceStatus with the given instance ID
// and a non-ok instance status to trigger a finding.
func makeInstanceStatus(instanceID string, statusVal ec2types.SummaryStatus) ec2types.InstanceStatus {
	return ec2types.InstanceStatus{
		InstanceId: aws.String(instanceID),
		InstanceStatus: &ec2types.InstanceStatusSummary{
			Status: statusVal,
		},
		SystemStatus: &ec2types.InstanceStatusSummary{
			Status: ec2types.SummaryStatusOk,
		},
	}
}

// makeVolumeStatus creates an EC2 VolumeStatus with the given volume ID
// and a non-ok I/O status to trigger a finding.
func makeVolumeStatus(volumeID string, statusVal string) ec2types.VolumeStatusItem {
	return ec2types.VolumeStatusItem{
		VolumeId: aws.String(volumeID),
		VolumeStatus: &ec2types.VolumeStatusInfo{
			Status: ec2types.VolumeStatusInfoStatus(statusVal),
		},
	}
}

// ---------------------------------------------------------------------------
// TestEnrichBackupJobs_PaginatesListBackupJobs
// ---------------------------------------------------------------------------

// TestEnrichBackupJobs_PaginatesListBackupJobs verifies that EnrichBackupJobs
// follows NextToken across two pages of ListBackupJobs results.
//
// Contract:
//   - Page 1: 10 jobs (5 COMPLETED, 5 FAILED) with NextToken="t1"
//   - Page 2: 5 jobs (all COMPLETED) with NextToken=nil
//   - ListBackupJobs called exactly twice
//   - Findings from page 2 are not dropped (no duplicated plan IDs so all
//     failed jobs on page 1 produce findings)
//   - result.Truncated == false (both pages consumed)
func TestEnrichBackupJobs_PaginatesListBackupJobs(t *testing.T) {
	// Build page 1: 5 failed jobs (distinct plan IDs) + 5 completed jobs.
	p1Jobs := make([]backuptypes.BackupJob, 0, 10)
	for i := range 5 {
		p1Jobs = append(p1Jobs,
			makeBackupJob(fmt.Sprintf("plan-failed-%d", i), fmt.Sprintf("job-f%d", i), backuptypes.BackupJobStateFailed),
		)
	}
	for i := range 5 {
		p1Jobs = append(p1Jobs,
			makeBackupJob(fmt.Sprintf("plan-ok-%d", i), fmt.Sprintf("job-ok%d", i), backuptypes.BackupJobStateCompleted),
		)
	}

	// Build page 2: 5 completed jobs with distinct plan IDs (so they are new).
	p2Jobs := make([]backuptypes.BackupJob, 0, 5)
	for i := range 5 {
		p2Jobs = append(p2Jobs,
			makeBackupJob(fmt.Sprintf("plan-p2-%d", i), fmt.Sprintf("job-p2-%d", i), backuptypes.BackupJobStateCompleted),
		)
	}

	fake := newBackupPaginatedFake(
		&backupsdk.ListBackupJobsOutput{
			BackupJobs: p1Jobs,
			NextToken:  aws.String("t1"),
		},
		&backupsdk.ListBackupJobsOutput{
			BackupJobs: p2Jobs,
			NextToken:  nil,
		},
	)

	clients := &awsclient.ServiceClients{Backup: fake}
	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, backupResources(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ListBackupJobs must have been called twice.
	if fake.callCount != 2 {
		t.Errorf("ListBackupJobs called %d times, want 2 (one per page)", fake.callCount)
	}

	// Truncated must be false — both pages were fully consumed.
	if result.Truncated {
		t.Errorf("result.Truncated = true, want false (both pages consumed)")
	}

	// 5 failed jobs on page 1 → 5 findings (each distinct plan-failed-N key).
	wantFindings := 5
	if len(result.Findings) != wantFindings {
		t.Errorf("len(result.Findings) = %d, want %d", len(result.Findings), wantFindings)
	}

	// Each failed plan must have a finding.
	for i := range 5 {
		key := fmt.Sprintf("plan-failed-%d", i)
		if _, ok := result.Findings[key]; !ok {
			t.Errorf("missing finding for key %q (page 1 failed job)", key)
		}
	}

	// Page-2 plan keys must NOT appear in findings (all COMPLETED → no finding).
	for i := range 5 {
		key := fmt.Sprintf("plan-p2-%d", i)
		if _, ok := result.Findings[key]; ok {
			t.Errorf("unexpected finding for key %q (completed job should not produce finding)", key)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEnrichBackupJobs_CapsAtEnrichmentCap
// ---------------------------------------------------------------------------

// TestEnrichBackupJobs_CapsAtEnrichmentCap verifies that when ListBackupJobs
// always returns NextToken (simulating an enormous account), the enricher
// stops after EnrichmentCap pages and sets result.Truncated = true.
func TestEnrichBackupJobs_CapsAtEnrichmentCap(t *testing.T) {
	// Build EnrichmentCap+2 pages, each with NextToken always set.
	pages := make([]*backupsdk.ListBackupJobsOutput, awsclient.EnrichmentCap+2)
	for i := range pages {
		// Use a unique plan ID per page so jobs don't de-duplicate.
		job := makeBackupJob(
			fmt.Sprintf("plan-cap-%d", i),
			fmt.Sprintf("job-cap-%d", i),
			backuptypes.BackupJobStateCompleted,
		)
		pages[i] = &backupsdk.ListBackupJobsOutput{
			BackupJobs: []backuptypes.BackupJob{job},
			NextToken:  aws.String(fmt.Sprintf("tok-%d", i+1)),
		}
	}

	fake := newBackupPaginatedFake(pages...)

	clients := &awsclient.ServiceClients{Backup: fake}
	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, backupResources(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must stop at EnrichmentCap pages — not go on indefinitely.
	if fake.callCount > awsclient.EnrichmentCap {
		t.Errorf("ListBackupJobs called %d times, want at most %d (EnrichmentCap)", fake.callCount, awsclient.EnrichmentCap)
	}

	// result.Truncated must signal that the walk was cut short.
	if !result.Truncated {
		t.Errorf("result.Truncated = false, want true (walk capped at EnrichmentCap pages)")
	}
}

// ---------------------------------------------------------------------------
// TestEnrichEC2InstanceStatus_PaginatesDescribeInstanceStatus
// ---------------------------------------------------------------------------

// TestEnrichEC2InstanceStatus_PaginatesDescribeInstanceStatus verifies that
// EnrichEC2InstanceStatus follows NextToken across two pages and processes
// all instance statuses (including those only on page 2).
//
// Contract:
//   - Page 1: 3 instances (2 impaired, 1 ok-ish) with NextToken="p1"
//   - Page 2: 2 instances (1 impaired, 1 ok) with NextToken=nil
//   - DescribeInstanceStatus called exactly twice
//   - Findings from page 2 are not dropped
func TestEnrichEC2InstanceStatus_PaginatesDescribeInstanceStatus(t *testing.T) {
	// Instance IDs spread across two pages.
	p1Impaired := []string{"i-aaa001", "i-aaa002"}
	p2Impaired := []string{"i-bbb001"}

	p1Statuses := []ec2types.InstanceStatus{
		makeInstanceStatus("i-aaa001", ec2types.SummaryStatusImpaired),
		makeInstanceStatus("i-aaa002", ec2types.SummaryStatusImpaired),
		makeInstanceStatus("i-aaa003", ec2types.SummaryStatusOk), // ok → no finding
	}
	p2Statuses := []ec2types.InstanceStatus{
		makeInstanceStatus("i-bbb001", ec2types.SummaryStatusImpaired),
		makeInstanceStatus("i-bbb002", ec2types.SummaryStatusOk), // ok → no finding
	}

	fake := newEC2PaginatedFake()
	fake.instanceStatusPages = []*ec2sdk.DescribeInstanceStatusOutput{
		{InstanceStatuses: p1Statuses, NextToken: aws.String("p1")},
		{InstanceStatuses: p2Statuses, NextToken: nil},
	}

	// Pass all instance IDs as known resources so none are treated as unmatched.
	allIDs := append(p1Impaired, p2Impaired...)
	allIDs = append(allIDs, "i-aaa003", "i-bbb002")
	clients := &awsclient.ServiceClients{EC2: fake}
	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, ec2InstanceResources(allIDs...), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DescribeInstanceStatus must have been called twice.
	if fake.instanceStatusCallCount != 2 {
		t.Errorf("DescribeInstanceStatus called %d times, want 2 (one per page)", fake.instanceStatusCallCount)
	}

	// Truncated must be false — both pages fully consumed.
	if result.Truncated {
		t.Errorf("result.Truncated = true, want false (both pages consumed)")
	}

	// All impaired instances (page 1 + page 2) must have findings.
	wantFindings := len(p1Impaired) + len(p2Impaired)
	if len(result.Findings) != wantFindings {
		t.Errorf("len(result.Findings) = %d, want %d (impaired from both pages)", len(result.Findings), wantFindings)
	}

	for _, id := range p1Impaired {
		if _, ok := result.Findings[id]; !ok {
			t.Errorf("missing finding for %q (page 1 impaired instance)", id)
		}
	}
	for _, id := range p2Impaired {
		if _, ok := result.Findings[id]; !ok {
			t.Errorf("missing finding for %q (page 2 impaired instance, would be dropped without pagination)", id)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEnrichEC2InstanceStatus_CapsAtEnrichmentCap
// ---------------------------------------------------------------------------

// TestEnrichEC2InstanceStatus_CapsAtEnrichmentCap verifies that when
// DescribeInstanceStatus always returns NextToken the enricher stops after
// EnrichmentCap pages and sets result.Truncated = true.
func TestEnrichEC2InstanceStatus_CapsAtEnrichmentCap(t *testing.T) {
	pages := make([]*ec2sdk.DescribeInstanceStatusOutput, awsclient.EnrichmentCap+2)
	for i := range pages {
		pages[i] = &ec2sdk.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{},
			NextToken:        aws.String(fmt.Sprintf("ec2-tok-%d", i+1)),
		}
	}

	fake := newEC2PaginatedFake()
	fake.instanceStatusPages = pages

	clients := &awsclient.ServiceClients{EC2: fake}
	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, ec2InstanceResources(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.instanceStatusCallCount > awsclient.EnrichmentCap {
		t.Errorf("DescribeInstanceStatus called %d times, want at most %d (EnrichmentCap)",
			fake.instanceStatusCallCount, awsclient.EnrichmentCap)
	}

	if !result.Truncated {
		t.Errorf("result.Truncated = false, want true (walk capped at EnrichmentCap pages)")
	}
}

// ---------------------------------------------------------------------------
// TestEnrichEBSVolumeStatus_PaginatesDescribeVolumeStatus
// ---------------------------------------------------------------------------

// TestEnrichEBSVolumeStatus_PaginatesDescribeVolumeStatus verifies that
// EnrichEBSVolumeStatus follows NextToken across two pages and processes
// all volume statuses (including those only on page 2).
//
// Contract:
//   - Page 1: 3 volumes (2 degraded, 1 ok) with NextToken="v1"
//   - Page 2: 2 volumes (1 degraded, 1 ok) with NextToken=nil
//   - DescribeVolumeStatus called exactly twice
//   - Findings from page 2 are not dropped
func TestEnrichEBSVolumeStatus_PaginatesDescribeVolumeStatus(t *testing.T) {
	p1Degraded := []string{"vol-aaa001", "vol-aaa002"}
	p2Degraded := []string{"vol-bbb001"}

	p1Vols := []ec2types.VolumeStatusItem{
		makeVolumeStatus("vol-aaa001", "impaired"),
		makeVolumeStatus("vol-aaa002", "impaired"),
		makeVolumeStatus("vol-aaa003", "ok"), // ok → no finding
	}
	p2Vols := []ec2types.VolumeStatusItem{
		makeVolumeStatus("vol-bbb001", "impaired"),
		makeVolumeStatus("vol-bbb002", "ok"), // ok → no finding
	}

	fake := newEC2PaginatedFake()
	fake.volumeStatusPages = []*ec2sdk.DescribeVolumeStatusOutput{
		{VolumeStatuses: p1Vols, NextToken: aws.String("v1")},
		{VolumeStatuses: p2Vols, NextToken: nil},
	}

	allIDs := append(p1Degraded, p2Degraded...)
	allIDs = append(allIDs, "vol-aaa003", "vol-bbb002")
	clients := &awsclient.ServiceClients{EC2: fake}
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, ebsVolumeResources(allIDs...), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DescribeVolumeStatus must have been called twice.
	if fake.volumeStatusCallCount != 2 {
		t.Errorf("DescribeVolumeStatus called %d times, want 2 (one per page)", fake.volumeStatusCallCount)
	}

	// Truncated must be false — both pages fully consumed.
	if result.Truncated {
		t.Errorf("result.Truncated = true, want false (both pages consumed)")
	}

	// All degraded volumes (page 1 + page 2) must have findings.
	wantFindings := len(p1Degraded) + len(p2Degraded)
	if len(result.Findings) != wantFindings {
		t.Errorf("len(result.Findings) = %d, want %d (degraded from both pages)", len(result.Findings), wantFindings)
	}

	for _, id := range p1Degraded {
		if _, ok := result.Findings[id]; !ok {
			t.Errorf("missing finding for %q (page 1 degraded volume)", id)
		}
	}
	for _, id := range p2Degraded {
		if _, ok := result.Findings[id]; !ok {
			t.Errorf("missing finding for %q (page 2 degraded volume, would be dropped without pagination)", id)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEnrichEBSVolumeStatus_CapsAtEnrichmentCap
// ---------------------------------------------------------------------------

// TestEnrichEBSVolumeStatus_CapsAtEnrichmentCap verifies that when
// DescribeVolumeStatus always returns NextToken the enricher stops after
// EnrichmentCap pages and sets result.Truncated = true.
func TestEnrichEBSVolumeStatus_CapsAtEnrichmentCap(t *testing.T) {
	pages := make([]*ec2sdk.DescribeVolumeStatusOutput, awsclient.EnrichmentCap+2)
	for i := range pages {
		pages[i] = &ec2sdk.DescribeVolumeStatusOutput{
			VolumeStatuses: []ec2types.VolumeStatusItem{},
			NextToken:      aws.String(fmt.Sprintf("ebs-tok-%d", i+1)),
		}
	}

	fake := newEC2PaginatedFake()
	fake.volumeStatusPages = pages

	clients := &awsclient.ServiceClients{EC2: fake}
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, ebsVolumeResources(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.volumeStatusCallCount > awsclient.EnrichmentCap {
		t.Errorf("DescribeVolumeStatus called %d times, want at most %d (EnrichmentCap)",
			fake.volumeStatusCallCount, awsclient.EnrichmentCap)
	}

	if !result.Truncated {
		t.Errorf("result.Truncated = false, want true (walk capped at EnrichmentCap pages)")
	}
}

// ---------------------------------------------------------------------------
// TestNoSingleCallListAPIEnrichers (meta-test / structural audit)
// ---------------------------------------------------------------------------

// nonPaginatedAPIs is the allowlist of AWS API calls that are legitimately
// non-paginated (account-wide singletons, GetXxx operations, etc.).
// Each entry is matched against the called method name (selector.Sel.Name).
// Add ONLY with a justification comment.
var nonPaginatedAPIs = []string{
	// DescribeLoggingStatus — returns one cluster's logging status, not a
	// list. redshift.DescribeLoggingStatusOutput carries no Marker or
	// NextToken field, so there is nothing to page.
	"DescribeLoggingStatus",
	// SES account-level singleton — no pages, single account object returned.
	"GetSendQuota",
	// SESv2 GetAccount — account-wide singleton, no NextToken.
	"GetAccount",
	// SESv2 GetEmailIdentity — single-resource per call (not a list op).
	"GetEmailIdentity",
	// GetRegistrationStatus is a single-item health check, not a list.
	"GetRegistrationStatus",
	// GetBucketNotificationConfiguration — single bucket config object.
	"GetBucketNotificationConfiguration",
	// GetPublicAccessBlock — single bucket PAB object.
	"GetPublicAccessBlock",
	// GetKeyRotationStatus — returns a single key's rotation config.
	"GetKeyRotationStatus",
	// GetRepositoryPermissionsPolicy — one repository's resource policy
	// document. codeartifact.GetRepositoryPermissionsPolicyOutput carries a
	// single Policy and no token field. The audit judges each call rather than
	// each function: its enricher paginating a different call does not exempt
	// this one.
	"GetRepositoryPermissionsPolicy",
	// DescribeEnvironmentHealth — single environment health object.
	"DescribeEnvironmentHealth",
	// DescribeLoadBalancerAttributes — single LB's attributes.
	"DescribeLoadBalancerAttributes",
	// GetQueueAttributes — returns a map of attributes for one queue.
	"GetQueueAttributes",
	// GetWorkGroup — single workgroup config.
	"GetWorkGroup",
	// GetHostedZone — single zone record.
	"GetHostedZone",
	// GetDistributionConfig — single CloudFront distribution config.
	"GetDistributionConfig",
	// DescribeClusterV2 — MSK single-cluster describe.
	"DescribeClusterV2",
	// DescribeCertificate — single ACM certificate details.
	"DescribeCertificate",
	// BatchGetBuilds — batch fetch by IDs, not a paginated list.
	"BatchGetBuilds",
	// ListBuildsForProject — returns a single page of build IDs (enricher
	// reads only the most recent one; deliberately not paginated).
	"ListBuildsForProject",
	// GetContinuousBackupsDescription → DescribeContinuousBackups: single-table check.
	"DescribeContinuousBackups",
	// GetRole — single IAM role details.
	"GetRole",
	// GetPipelineState — single pipeline state object.
	"GetPipelineState",
	// DescribeStateMachine — single SFN state machine details.
	"DescribeStateMachine",
	// GetJobRuns — per-job; enricher fetches max:1 record intentionally.
	"GetJobRuns",
	// DescribeReplicationGroups — used per-Redis cluster (not account-wide).
	"DescribeReplicationGroups",
	// DescribeScalingActivities — per-ASG, MaxRecords=1, intentionally single-call.
	"DescribeScalingActivities",
	// DescribeMountTargets — per-EFS filesystem.
	"DescribeMountTargets",
	// DescribeTransitGatewayAttachments — per-TGW resource, capped per-resource.
	"DescribeTransitGatewayAttachments",
	// DescribeFlowLogs — per-VPC resource, capped per-resource.
	"DescribeFlowLogs",
	// DescribeStacks — per-CFN stack; single describe for drift check.
	"DescribeStacks",
	// DescribeStackEvents — per-CFN stack; single page of recent events.
	"DescribeStackEvents",
	// DescribeEnvironmentResources — single EB environment resources.
	"DescribeEnvironmentResources",
	// GetTargetGroupAttributes → DescribeTargetHealth — per-TG.
	"DescribeTargetHealth",
	// GetTopicAttributes — single SNS topic attributes.
	"GetTopicAttributes",
	// GetFunction — single Lambda function config.
	"GetFunction",
	// ListExecutions — called with MaxResults=1 to fetch the single most-recent
	// execution per state machine; pagination is intentionally bypassed.
	"ListExecutions",
	// DescribeServices — ECS batch-describe (takes ARN list, max 10 per call);
	// the API does not return NextToken; pagination is not applicable.
	"DescribeServices",
	// DescribeClusters — ECS batch-describe (takes ARN list); no NextToken.
	"DescribeClusters",
	// DescribeTasks — ECS batch-describe (takes ARN list); no NextToken.
	"DescribeTasks",
	// GetLoggingConfiguration — WAFv2 single-item get; returns one config object.
	"GetLoggingConfiguration",
	// ListResourcesForWebACL — WAFv2 returns all associated resource ARNs in
	// a single response (no NextToken in output); not a paginated operation.
	"ListResourcesForWebACL",
	// The seven below were surfaced once the audit began walking the
	// unexported helpers the enrichers delegate to, not only the exported
	// Enrich* entry points. Each output struct was read off the SDK version
	// this module pins.
	//
	// DescribeDBClusterSnapshotAttributes — one snapshot's attribute list.
	// rds.DescribeDBClusterSnapshotAttributesOutput carries only
	// DBClusterSnapshotAttributesResult; there is no token field.
	"DescribeDBClusterSnapshotAttributes",
	// DescribeDBSnapshotAttributes — same shape for the instance snapshot.
	"DescribeDBSnapshotAttributes",
	// GetResourcePolicy — one table's resource policy document.
	// dynamodb.GetResourcePolicyOutput carries Policy and RevisionId only.
	"GetResourcePolicy",
	// DescribeTaskDefinition — one task definition revision.
	// ecs.DescribeTaskDefinitionOutput carries TaskDefinition and Tags only.
	"DescribeTaskDefinition",
	// DescribeFileSystemPolicy — one file system's policy document.
	"DescribeFileSystemPolicy",
	// DescribeBackupPolicy — one file system's backup policy.
	"DescribeBackupPolicy",
	// DescribeDBEngineVersions — the enricher filters to a single
	// Engine + EngineVersion pair, which identifies one version, so the
	// Marker the output carries is never set. Paginating it would loop over a
	// one-row answer.
	"DescribeDBEngineVersions",
}

// paginationBurnDown is the list of call sites the per-call-site audit already
// finds unpaginated. They are carried rather than failing, so this gate goes
// red only on a NEW one; each entry is a finding routed to the batch that owns
// the enricher. Deleting an entry that is no longer found is enforced below,
// so the list cannot outlive the work.
//
// Key shape: "<file>:<Enrich func>:<SDK operation>".
var paginationBurnDown = map[string]bool{}

// TestNoSingleCallListAPIEnrichers walks core/aws/*_issue_enrichment.go via
// go/ast and flags any call expression that:
//
//  1. Is a 3-level selector call (clients.X.Op(...)) to an AWS SDK
//     list/describe operation that is NOT in the nonPaginatedAPIs allowlist, AND
//  2. Sits in no enclosing loop that drives a cursor — a NextToken, Marker or
//     ContinuationToken, a Start… field of a multi-field cursor, the
//     IsTruncated flag those answer with, or an SDK paginator's HasMorePages.
//
// Every function in the file is walked, not only the exported Enrich* ones.
// An enricher that hands the call to an unexported helper has the same defect
// as one that makes it inline, and the helpers hold a third of the call sites.
//
// The test passes when zero such calls are found, meaning every
// paginated-capable API either has a loop guard or is explicitly allowlisted.
//
// This is the structural regression pin: adding a new single-call enricher
// targeting a paginated API must fail this test, forcing the author to either
// implement pagination or justify the skip-list addition.
func TestNoSingleCallListAPIEnrichers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	// thisFile = .../tests/unit/enrichment_pagination_audit_test.go
	// two levels up -> repo root
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	matches, err := filepath.Glob(filepath.Join(repoRoot, "core", "aws", "*_issue_enrichment.go"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("filepath.Glob returned zero matches for core/aws/*_issue_enrichment.go — check repo layout")
	}

	// Build a skip-set from nonPaginatedAPIs for O(1) lookup.
	skipSet := make(map[string]bool, len(nonPaginatedAPIs))
	for _, op := range nonPaginatedAPIs {
		skipSet[op] = true
	}

	// Share one FileSet across all files so fset.Position(pos).Line gives the
	// right per-file line number.
	fset := token.NewFileSet()

	var violations []string
	var carried []string

	for _, filePath := range matches {
		// Defensive: skip any _test.go files that the glob might pick up.
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		src, parseErr := parser.ParseFile(fset, filePath, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse error in %s: %v", filePath, parseErr)
		}

		baseName := filepath.Base(filePath)

		for _, decl := range src.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name == nil || fn.Body == nil {
				continue
			}
			funcName := fn.Name.Name

			// Collect all 3-level selector calls: clients.Service.Op(...)
			// A 3-level selector is: SelectorExpr{ X: SelectorExpr{ X: Ident("clients") } }
			calls := collectThreeLevelCalls(fn.Body, "clients")

			for _, callInfo := range calls {
				opName := callInfo.opName
				line := fset.Position(callInfo.pos).Line

				// This call drives a pagination token itself.
				if callInfo.paginated {
					continue
				}

				// If the op is on the allowlist, it's legitimately non-paginated.
				if skipSet[opName] {
					continue
				}

				// We only flag operations that look like list/describe calls —
				// these are the ones likely to paginate.
				if !looksLikeListOrDescribe(opName) {
					continue
				}

				key := fmt.Sprintf("%s:%s:%s", baseName, funcName, opName)
				if paginationBurnDown[key] {
					carried = append(carried, fmt.Sprintf("%s:%d: %s calls %s", baseName, line, funcName, opName))
					continue
				}

				violations = append(violations, fmt.Sprintf(
					"%s:%d: %s calls %s without pagination (NextToken/Marker absent); add to skip-list with justification or paginate",
					baseName, line, funcName, opName,
				))
			}
		}
	}

	if len(carried) > 0 {
		t.Logf("burn-down: %d known unpaginated call site(s) still open:\n  %s",
			len(carried), strings.Join(carried, "\n  "))
	}
	for key := range paginationBurnDown {
		if !seenBurnDownKey(carried, key) {
			t.Errorf("paginationBurnDown lists %q, which the audit no longer finds — delete the entry so the list stays the real burn-down", key)
		}
	}

	if len(violations) > 0 {
		t.Errorf(
			"found %d enricher(s) calling a potentially-paginated API without a pagination loop or skip-list entry:\n\n  %s\n\n"+
				"Either:\n"+
				"  (a) implement NextToken/Marker pagination in the enricher, or\n"+
				"  (b) add the operation to nonPaginatedAPIs in enrichment_pagination_audit_test.go with a justification comment.",
			len(violations),
			strings.Join(violations, "\n  "),
		)
	}
}

// seenBurnDownKey reports whether the audit produced a carried entry for key,
// whose shape is "<file>:<func>:<op>" against carried's "<file>:<line>: <func>
// calls <op>".
func seenBurnDownKey(carried []string, key string) bool {
	parts := strings.Split(key, ":")
	if len(parts) != 3 {
		return false
	}
	for _, c := range carried {
		if strings.HasPrefix(c, parts[0]+":") && strings.Contains(c, parts[1]+" calls "+parts[2]) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// AST helpers for TestNoSingleCallListAPIEnrichers
// ---------------------------------------------------------------------------

// callSite records the operation name and source position of a detected call,
// and whether that call is the one being paginated.
type callSite struct {
	opName    string
	pos       token.Pos
	paginated bool
}

// collectThreeLevelCalls finds all call expressions of the form root.X.Op(...)
// within the given AST node. Returns one callSite per distinct call.
//
// paginated is decided per call, not per function: the call must sit inside a
// loop whose own subtree references a pagination token. A function that
// paginates one API and single-shots another mentions NextToken either way, so
// a function-wide check calls the second one paginated and never reports it.
func collectThreeLevelCalls(body ast.Node, rootIdent string) []callSite {
	var sites []callSite
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// sel.X should itself be a SelectorExpr (the "clients.Service" part).
		innerSel, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// innerSel.X should be an Ident with the root name (e.g. "clients").
		ident, ok := innerSel.X.(*ast.Ident)
		if !ok || ident.Name != rootIdent {
			return true
		}
		sites = append(sites, callSite{
			opName:    sel.Sel.Name,
			pos:       call.Pos(),
			paginated: inPaginatedWalk(stack),
		})
		return true
	})
	return sites
}

// inPaginatedWalk reports whether the call currently on top of stack is part
// of a pagination walk: either a loop enclosing it drives a cursor, or it is
// the page reader handed to walkAccountPages.
//
// The cursor has to live in the loop that contains the call, because a second
// loop elsewhere in the same function says nothing about this call.
//
// A cursor is not always one token named NextToken. Route 53 pages
// ListResourceRecordSets on three Start… input fields and an IsTruncated
// boolean, and the SDK's own paginators expose HasMorePages and nothing else.
// Recognising only the three token names read those loops as unpaginated,
// which is a false alarm the next author answers with an allowlist entry —
// and an allowlist entry then hides the real single-call regression the audit
// exists to catch.
//
// The walkAccountPages arm: an account-wide enricher does not write its own
// loop, because the loop and the marking of the rows past the last walked
// page are one rule (one page-cap helper owns the walk bound). A call inside
// that helper's page reader is paginated AND bounded AND accounted for,
// which is strictly more than a loop-only form would demand.
func inPaginatedWalk(stack []ast.Node) bool {
	for _, n := range stack {
		switch node := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			if bodyContainsAny(n, cursorNames...) || bodyReadsTruncatedOffACallResult(n) {
				return true
			}
		case *ast.CallExpr:
			if fn, ok := node.Fun.(*ast.Ident); ok && fn.Name == "walkAccountPages" {
				return true
			}
		}
	}
	return false
}

// cursorNames are the identifiers a paginating loop drives its cursor with:
// the three single-token names, the Start… fields of a multi-field cursor,
// and the SDK paginator's own condition.
//
// IsTruncated is deliberately NOT here. It is a field name a cached list
// entry carries too, so a loop over cache entries that reads
// entry.IsTruncated and single-shots an API inside would read as paginated —
// the very regression this audit exists to catch. It is recognised by
// bodyReadsTruncatedOffACallResult instead, which requires it be read off a
// value the loop itself got back from a call.
var cursorNames = []string{
	"NextToken", "Marker", "ContinuationToken",
	"HasMorePages",
	"StartRecordName", "StartRecordType", "StartRecordIdentifier",
}

// bodyReadsTruncatedOffACallResult reports whether body reads .IsTruncated off
// a variable the body itself assigned from a call — the Route 53 shape, where
// the loop's own page response answers whether another page is due.
func bodyReadsTruncatedOffACallResult(body ast.Node) bool {
	fromCall := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		if _, isCall := assign.Rhs[0].(*ast.CallExpr); !isCall {
			return true
		}
		for _, lhs := range assign.Lhs {
			if id, ok := lhs.(*ast.Ident); ok {
				fromCall[id.Name] = true
			}
		}
		return true
	})
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsTruncated" {
			return true
		}
		if base, ok := sel.X.(*ast.Ident); ok && fromCall[base.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

// bodyContainsAny reports whether any of the given identifier names appear
// anywhere within the AST body node.
func bodyContainsAny(body ast.Node, names ...string) bool {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if ok && nameSet[ident.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

// looksLikeListOrDescribe returns true if the operation name starts with a
// prefix associated with paginated AWS list/describe APIs. GetXxx are
// generally single-item lookups and are excluded here; they belong in the
// nonPaginatedAPIs skip-list only if they happen to share a List/Describe
// prefix.
func looksLikeListOrDescribe(op string) bool {
	for _, prefix := range []string{"List", "Describe", "Get"} {
		if strings.HasPrefix(op, prefix) {
			return true
		}
	}
	return false
}

// TestPaginationAudit_JudgesEachCallNotEachFunction is the audit's own test.
// An enricher that pages one API and single-shots another mentions NextToken
// either way, so a function-wide check exempts the second call and reports
// nothing. This pins that the second call is still seen.
func TestPaginationAudit_JudgesEachCallNotEachFunction(t *testing.T) {
	const src = `package aws

func EnrichThing(ctx any, clients *ServiceClients) {
	token := ""
	for {
		out, _ := clients.Backup.ListBackupJobs(ctx, &In{NextToken: &token})
		if out.NextToken == nil {
			break
		}
		token = *out.NextToken
	}
	clients.EC2.DescribeVolumeStatus(ctx, &In{})
	for _, id := range ids {
		clients.EC2.DescribeInstanceStatus(ctx, &In{})
	}
	input := &In{}
	for range PerParentPageCap {
		out, _ := clients.Route53.ListResourceRecordSets(ctx, input)
		if !out.IsTruncated {
			break
		}
		input.StartRecordName = out.NextRecordName
		input.StartRecordType = out.NextRecordType
	}
	pager := NewListThingsPaginator(clients.Athena, &In{})
	for pager.HasMorePages() {
		pager.NextPage(ctx)
		clients.Athena.ListDataCatalogs(ctx, &In{})
	}
	for _, name := range []string{"eip", "ec2"} {
		entry := cache[name]
		if entry.IsTruncated {
			continue
		}
		clients.EC2.DescribeAddresses(ctx, &In{})
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := file.Decls[0].(*ast.FuncDecl)

	got := map[string]bool{}
	for _, c := range collectThreeLevelCalls(fn.Body, "clients") {
		got[c.opName] = c.paginated
	}

	want := map[string]bool{
		// Inside the loop that advances the token.
		"ListBackupJobs": true,
		// Outside any loop.
		"DescribeVolumeStatus": false,
		// Inside a loop, but one that iterates resource IDs and drives no
		// token — the shape a function-wide check cannot tell from the first.
		"DescribeInstanceStatus": false,
		// Route 53's cursor is three input fields and a boolean on the output,
		// and it spells none of them NextToken. A loop this correct read as
		// unpaginated, which is a false alarm the next author answers with an
		// allowlist entry that then hides a real one.
		"ListResourceRecordSets": true,
		// The SDK's own paginator drives the cursor; the loop condition is
		// the paginator's, not a token the enricher names.
		"ListDataCatalogs": true,
		// IsTruncated read off a CACHE ENTRY, not off this loop's own call
		// output. The loop drives no cursor and the call inside it is a
		// single shot, which is exactly the regression the audit exists to
		// catch — recognising the field wherever it appears would exempt it.
		"DescribeAddresses": false,
	}
	for op, wantPaginated := range want {
		gotPaginated, seen := got[op]
		if !seen {
			t.Errorf("%s was not collected at all", op)
			continue
		}
		if gotPaginated != wantPaginated {
			t.Errorf("%s paginated = %v, want %v", op, gotPaginated, wantPaginated)
		}
	}
	if len(got) != len(want) {
		t.Errorf("collected %d calls, want %d: %v", len(got), len(want), got)
	}
}
