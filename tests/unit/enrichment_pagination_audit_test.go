package unit

// enrichment_pagination_audit_test.go — TDD tests for account-wide enricher
// pagination (Gap S7) and a meta-test AST audit.
//
// # Background
//
// Three account-wide enrichers currently call their respective AWS List/Describe
// API exactly once, reading only the first page. This file:
//
//  1. Pins the required multi-page contract so the coder can implement it (tests
//     fail before the fix, pass after).
//  2. Caps the walk at EnrichmentCap pages to avoid unbounded API calls.
//  3. Provides a structural meta-test (AST walk of *_issue_enrichment.go files) that flags any
//     future regression: a new enricher that calls a paginated API without a loop.
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

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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
	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, backupResources())
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

	// IssueCount must match the 5 severity-"!" findings.
	if result.IssueCount != wantFindings {
		t.Errorf("result.IssueCount = %d, want %d", result.IssueCount, wantFindings)
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
	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, backupResources())
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
	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, ec2InstanceResources(allIDs...))
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
	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, ec2InstanceResources())
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
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, ebsVolumeResources(allIDs...))
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
	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, ebsVolumeResources())
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
}

// TestNoSingleCallListAPIEnrichers walks internal/aws/*_issue_enrichment.go via
// go/ast and flags any Enrich* function that:
//
//  1. Contains a 3-level selector call (clients.X.Op(...)) to an AWS SDK
//     list/describe operation that is NOT in the nonPaginatedAPIs allowlist, AND
//  2. Has no identifier reference to NextToken, Marker, or ContinuationToken
//     anywhere in its function body.
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

	matches, err := filepath.Glob(filepath.Join(repoRoot, "internal", "aws", "*_issue_enrichment.go"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("filepath.Glob returned zero matches for internal/aws/*_issue_enrichment.go — check repo layout")
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
			if !strings.HasPrefix(fn.Name.Name, "Enrich") {
				continue
			}

			funcName := fn.Name.Name

			// Check whether the function body contains any pagination identifier.
			hasPaginationRef := bodyContainsAny(fn.Body, "NextToken", "Marker", "ContinuationToken")

			// Collect all 3-level selector calls: clients.Service.Op(...)
			// A 3-level selector is: SelectorExpr{ X: SelectorExpr{ X: Ident("clients") } }
			calls := collectThreeLevelCalls(fn.Body, "clients")

			for _, callInfo := range calls {
				opName := callInfo.opName
				line := fset.Position(callInfo.pos).Line

				// If the function already references a pagination token anywhere,
				// we assume the author intends to paginate and don't flag it.
				if hasPaginationRef {
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

				violations = append(violations, fmt.Sprintf(
					"%s:%d: %s calls %s without pagination (NextToken/Marker absent); add to skip-list with justification or paginate",
					baseName, line, funcName, opName,
				))
			}
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

// ---------------------------------------------------------------------------
// AST helpers for TestNoSingleCallListAPIEnrichers
// ---------------------------------------------------------------------------

// callSite records the operation name and source position of a detected call.
type callSite struct {
	opName string
	pos    token.Pos
}

// collectThreeLevelCalls finds all call expressions of the form root.X.Op(...)
// within the given AST node. Returns one callSite per distinct call.
func collectThreeLevelCalls(body ast.Node, rootIdent string) []callSite {
	var sites []callSite
	ast.Inspect(body, func(n ast.Node) bool {
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
		sites = append(sites, callSite{opName: sel.Sel.Name, pos: call.Pos()})
		return true
	})
	return sites
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
