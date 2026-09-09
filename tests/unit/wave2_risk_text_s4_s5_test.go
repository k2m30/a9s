package unit

// wave2_risk_text_s4_s5_test.go — the Wave-2 "risk text" framework:
// domain.Finding.Detail (S5 operator sentence) + the list-view S4 override
// that shows the finding's concrete Phrase in the Status/lifecycle column
// instead of the raw AWS state.
//
// Contract (see docs/resources/<sn>.md §4 for each type):
//   - core/domain/finding.go: Finding carries `Detail string` (S5 sentence;
//     empty ⇒ render falls back to Phrase, no stray blank line).
//   - core/app/list_columns.go (listExtractCellValue) and
//     internal/tui/views/resourcelist.go (renderListDataRow): when a row's
//     enrichment-map Finding (delivered via SetEnrichmentState /
//     GetListEnrichmentFindings, see core/app/list_filter.go
//     applyEnrichmentState) is issue-severity (SevWarn/SevBroken), the
//     Status/lifecycle cell shows that Finding's Phrase — there is no glyph
//     prefix on the identity column.
//   - core/app/detail_body.go (buildAttentionSectionDetail): each Attention
//     entry renders both the short Phrase line (capitalized for display)
//     AND — on lines of its own below it — the full Detail sentence, wrapped
//     to the panel, falling back to Phrase alone when Detail == "".
//   - Tests #5-#7 drive the real Wave-2 enrichers (EnrichS3Posture,
//     EnrichEC2InstanceStatus, EnrichDBIMaintenance in core/aws/*.go), which
//     pass a Detail argument to setWave2Finding; ec2's phrase is the
//     §4-mandated "impaired: system checks failing", dbi's is "maintenance
//     scheduled".
//   - Test #4 drives the real Wave-1 fetcher FetchCloudWatchLogGroupsPage,
//     which classifies retention-nil into a domain.Finding and stores the
//     policy in words under Fields["retention"].

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdssvc "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Claims (3) and (4) above are not pinned in this file. The list-view Status
// override is pinned by list_ports_test.go's
// TestWave3ListStatusColumn_EnrichmentMapOnlyFinding_OverridesRawState, and the
// detail-view S5 sentence and its Detail=="" fallback by detail_ports_test.go's
// Test_DetailAttention_RendersFullDetailSentence_AlongsidePhrase and
// Test_DetailAttention_FallsBackToPhrase_WhenDetailEmpty. All three drive the
// live Controller. What remains below is the per-type finding text.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 4. logs retention-nil (docs/resources/logs.md §4).
// Pinned strings:
//   Phrase (S4): "retention: never expire"
//   Detail (S5): "Events in this group are kept forever and billed forever, and an unbounded audit log is also a growing pile of whatever the application logged. Set a retention period that matches how far back anyone actually looks."
// ---------------------------------------------------------------------------

// logsRetentionNilFake implements CWLogsDescribeLogGroupsAPI, returning a
// single log group with RetentionInDays == nil (never-expire retention) —
// the exact condition docs/resources/logs.md §4 row "retentionInDays nil"
// classifies as a Wave-1 Warning finding.
type logsRetentionNilFake struct {
	logGroupName string
}

func (f *logsRetentionNilFake) DescribeLogGroups(
	_ context.Context,
	_ *cloudwatchlogs.DescribeLogGroupsInput,
	_ ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: []cwlogstypes.LogGroup{
			{
				LogGroupName:    aws.String(f.logGroupName),
				RetentionInDays: nil, // never-expire — the signal under test
			},
		},
	}, nil
}

var _ awsclient.CWLogsDescribeLogGroupsAPI = (*logsRetentionNilFake)(nil)

// TestWave2_Logs_RetentionNil_PinsS4S5Strings drives the real
// FetchCloudWatchLogGroupsPage fetcher against a log group with
// RetentionInDays == nil and asserts the resulting resource carries a
// Finding with the exact §4-mandated Phrase and Detail. The fetcher
// (core/aws/cwlogs.go) classifies retention-nil into a domain.Finding and
// writes the policy in words ("never expire", "<N> days") under
// Fields["retention"].
func TestWave2_Logs_RetentionNil_PinsS4S5Strings(t *testing.T) {
	fake := &logsRetentionNilFake{logGroupName: "/aws/lambda/never-expire-fn"}

	result, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchCloudWatchLogGroupsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	if len(r.Findings) == 0 {
		t.Fatalf("expected a Finding for retention-nil log group %q; got zero Findings (Fields: %v)", r.ID, r.Fields)
	}
	f := r.Findings[0]
	if f.Phrase != "retention: never expire" {
		t.Errorf("logs retention-nil Phrase (S4) = %q, want %q (docs/resources/logs.md §4)", f.Phrase, "retention: never expire")
	}
	if f.Detail != "Events in this group are kept forever and billed forever, and an unbounded audit log is also a growing pile of whatever the application logged. Set a retention period that matches how far back anyone actually looks." {
		t.Errorf("logs retention-nil Detail (S5) = %q, want %q (docs/resources/logs.md §4)", f.Detail, "Events in this group are kept forever and billed forever, and an unbounded audit log is also a growing pile of whatever the application logged. Set a retention period that matches how far back anyone actually looks.")
	}
}

// ---------------------------------------------------------------------------
// 5. s3 PAB incomplete (docs/resources/s3.md §4), driven through the REAL
// enricher EnrichS3Posture.
// Pinned strings:
//   Phrase (S4): "public access block incomplete"
//   Detail (S5): "This bucket does not set all four public-access settings, so it relies on the account-level block to stop a future policy or ACL from making it public, and that block may not be set either. Turn on all four settings on the bucket itself so it is safe regardless of the account."
// ---------------------------------------------------------------------------

// TestWave2_S3_PABIncomplete_PinsS4S5Strings drives the real
// EnrichS3Posture enricher against a bucket with no PAB
// configuration (reusing s3PABFake from aws_s3_issue_enrichment_test.go, the
// existing sibling test's fake for this exact enricher) and asserts the exact
// §4-mandated Phrase and Detail.
func TestWave2_S3_PABIncomplete_PinsS4S5Strings(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"bucket-no-pab": nil, // nil value → NoSuchPublicAccessBlockConfiguration
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{
		pabResource("bucket-no-pab"),
	}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}
	findings, ok := result.Findings["bucket-no-pab"]
	if !ok {
		t.Fatalf("expected finding for bucket-no-pab")
	}
	finding := findings[0]
	if finding.Phrase != "public access block incomplete" {
		t.Errorf("s3 PAB Phrase (S4) = %q, want %q", finding.Phrase, "public access block incomplete")
	}
	if finding.Detail != "This bucket does not set all four public-access settings, so it relies on the account-level block to stop a future policy or ACL from making it public, and that block may not be set either. Turn on all four settings on the bucket itself so it is safe regardless of the account." {
		t.Errorf("s3 PAB Detail (S5) = %q, want %q", finding.Detail, "This bucket does not set all four public-access settings, so it relies on the account-level block to stop a future policy or ACL from making it public, and that block may not be set either. Turn on all four settings on the bucket itself so it is safe regardless of the account.")
	}
}

// ---------------------------------------------------------------------------
// 6. ec2 impaired (docs/resources/ec2.md §4), driven through the REAL
// enricher EnrichEC2InstanceStatus.
// Pinned strings (quoted verbatim from docs/resources/ec2.md §4 row
// "SystemStatus.Status == impaired"):
//   Phrase (S4): "impaired: system checks failing"
//   Detail (S5): "AWS's own checks of the host or the instance are failing, which means the problem is below your software: the hypervisor, the network path, or the instance's ability to boot. Stop and start the instance so it moves to different hardware, and read the system log first if you need the cause on record."
// ---------------------------------------------------------------------------

type ec2ImpairedFake struct {
	awsclient.EC2API
	statuses []ec2types.InstanceStatus
}

func (f *ec2ImpairedFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2svc.DescribeInstanceStatusInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeInstanceStatusOutput, error) {
	return &ec2svc.DescribeInstanceStatusOutput{InstanceStatuses: f.statuses}, nil
}

var _ awsclient.EC2API = (*ec2ImpairedFake)(nil)

// TestWave2_EC2_SystemStatusImpaired_PinsS4S5Strings drives the real
// EnrichEC2InstanceStatus enricher against an instance with
// SystemStatus.Status == impaired and asserts the exact §4-mandated Phrase
// and Detail.
func TestWave2_EC2_SystemStatusImpaired_PinsS4S5Strings(t *testing.T) {
	fake := &ec2ImpairedFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-impaired-pin-1"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-impaired-pin-1", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus error: %v", err)
	}
	findings, ok := result.Findings["i-impaired-pin-1"]
	if !ok {
		t.Fatalf("expected finding for i-impaired-pin-1")
	}
	finding := findings[0]
	if finding.Phrase != "impaired: system checks failing" {
		t.Errorf("ec2 impaired Phrase (S4) = %q, want %q (docs/resources/ec2.md §4)", finding.Phrase, "impaired: system checks failing")
	}
	if finding.Detail != "AWS's own checks of the host or the instance are failing, which means the problem is below your software: the hypervisor, the network path, or the instance's ability to boot. Stop and start the instance so it moves to different hardware, and read the system log first if you need the cause on record." {
		t.Errorf("ec2 impaired Detail (S5) = %q, want %q (docs/resources/ec2.md §4)", finding.Detail, "AWS's own checks of the host or the instance are failing, which means the problem is below your software: the hypervisor, the network path, or the instance's ability to boot. Stop and start the instance so it moves to different hardware, and read the system log first if you need the cause on record.")
	}
}

// ---------------------------------------------------------------------------
// 7. dbi pending-maintenance (docs/resources/dbi.md §4), driven through the
// REAL enricher EnrichDBIMaintenance.
//
// Signal chosen: "Pending maintenance overdue" — the only signal actually
// emitted by EnrichDBIMaintenance (storage-full is a Wave-1/fetcher signal,
// not produced by this enricher).
//
// Pinned strings (quoted verbatim from docs/resources/dbi.md §4 row
// "Pending maintenance overdue"):
//   Phrase (S4): "maintenance scheduled"
//   Detail (S5) shape: "Pending maintenance action overdue: <ActionType> (<Description>)."
//     — concretized here with ActionType="system-update",
//     Description="New minor engine patch 16.2.3" (matching the existing
//     fixtures.NewDBIFixtures() PendingMaintenanceActions fixture used by
//     aws_dbi_issue_enrichment_test.go's TestDBI_Enrich_MaintenancePending_HealthyRow):
//     "Pending maintenance action overdue: system-update (New minor engine patch 16.2.3)."
// ---------------------------------------------------------------------------

type dbiMaintOverdueFake struct {
	awsclient.RDSAPI
	actions []rdstypes.ResourcePendingMaintenanceActions
}

func (f *dbiMaintOverdueFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rdssvc.DescribePendingMaintenanceActionsInput,
	_ ...func(*rdssvc.Options),
) (*rdssvc.DescribePendingMaintenanceActionsOutput, error) {
	return &rdssvc.DescribePendingMaintenanceActionsOutput{PendingMaintenanceActions: f.actions}, nil
}

func TestWave2_DBI_PendingMaintenanceOverdue_PinsS4S5Strings(t *testing.T) {
	const resourceID = "db-maint-overdue-pin"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintOverdueFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String(arn),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{
						Action:      aws.String("system-update"),
						Description: aws.String("New minor engine patch 16.2.3"),
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: resourceID, Name: resourceID, Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	findings, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q", resourceID)
	}
	finding := findings[0]
	if finding.Phrase != "maintenance scheduled" {
		t.Errorf("dbi pending-maintenance Phrase (S4) = %q, want %q (docs/resources/dbi.md §4 — currently a buzzword bug)", finding.Phrase, "maintenance scheduled")
	}
	wantDetail := "AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you."
	if finding.Detail != wantDetail {
		t.Errorf("dbi pending-maintenance Detail (S5) = %q, want %q (the one static sentence FindingDef declares, no longer keyed by Action/Description)", finding.Detail, wantDetail)
	}
}
