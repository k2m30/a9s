package unit

// wave2_risk_text_s4_s5_test.go — RED (TDD) tests for the Wave-2 "risk text"
// framework: domain.Finding.Detail (S5 operator sentence) + the list-view S4
// override that shows the finding's concrete Phrase in the Status/lifecycle
// column instead of the raw AWS state.
//
// Bugs pinned by this file (see docs/resources/<sn>.md §4 for each type):
//   (3) A flagged LIST row shows no cause text — the Status column keeps the
//       raw AWS state/lifecycle value even when an issue-severity Finding
//       exists for that row's enrichment-map entry.
//   (4) domain.Finding exposes only Phrase (a short S4 cause) — there is no
//       S5 "concrete operator sentence" field, so the detail Attention
//       section can only ever show the short phrase, never the full remedy
//       sentence the spec calls for.
//
// Expected fix (implemented by a9s-coder, NOT in this file):
//   - core/domain/finding.go: Finding gains `Detail string` (S5 sentence;
//     empty ⇒ render falls back to Phrase, no stray blank line).
//   - core/app/list_columns.go (listExtractCellValue) and/or
//     internal/tui/views/resourcelist.go (renderListDataRow): when a row's
//     enrichment-map Finding (delivered via SetEnrichmentState /
//     GetListEnrichmentFindings, see core/app/list_filter.go
//     applyEnrichmentState) is issue-severity (SevWarn/SevBroken), the
//     Status/lifecycle cell must show that Finding's Phrase — not just a
//     glyph prefix on the identity column (no glyph prefix is produced any more).
//   - core/app/detail_body.go (injectAttentionSectionDetail): each Attention
//     entry must render both the short Phrase line (capitalized for display)
//     AND — on lines of its own below it — the full Detail sentence, wrapped
//     to the panel, falling back to Phrase alone when Detail == "".
//
// RED classification:
//   - Tests #2 and #3 (detail S5) are COMPILE-RED right now: domain.Finding
//     has no Detail field, so `domain.Finding{..., Detail: "..."}` fails to
//     compile until the coder adds the field.
//   - Test #1 (list S4) is LOGIC-RED: it compiles today (Finding literals use
//     only existing fields) but fails at assertion time because
//     SetEnrichmentState's findings map never reaches listExtractCellValue —
//     it only drives the "! "/"~ " glyph prefix on the identity column, per
//     resolveListRowSeverity (core/app/list_columns.go) and
//     buildMarkerModel-style harnesses (see qa_enrichment_marker_test.go).
//   - Tests #5-#7 (s3/ec2/dbi exemplars, driven through the real Wave-2
//     enrichers) are COMPILE-RED for the same reason as #2/#3 (they assert
//     on Finding.Detail) AND LOGIC-RED even post-compile-fix: none of
//     EnrichS3Posture, EnrichEC2InstanceStatus, EnrichDBIMaintenance
//     (core/aws/*.go) call setWave2Finding with a Detail argument; ec2's
//     enricher additionally emits "system status: impaired" (built from
//     strings.ToLower(row.Label)+": "+row.Value) instead of the §4-mandated
//     "impaired: system checks failing", and dbi's enricher emits the
//     buzzword Phrase "pending maintenance" instead of the §4-mandated
//     "maintenance scheduled".
//   - Test #4 (logs, driven through the real Wave-1 fetcher
//     FetchCloudWatchLogGroupsPage) is LOGIC-RED independent of the Detail
//     field: the fetcher does not classify retention-nil into a
//     domain.Finding at all today — it only stores the raw retention_days
//     field string — so the resulting resource has zero Findings.

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
// 1 (S4 in list: a flagged row's Status column shows the concrete Phrase) was
// TestWave2_ListStatusColumn_ShowsConcretePhrase_ForIssueFinding, a legacy
// views.ResourceListModel.SetEnrichmentState()/.View()-driven pin. Ported to
// list_ports_test.go's TestWave3ListStatusColumn_
// EnrichmentMapOnlyFinding_OverridesRawState, which drives the same
// enrichment-map-only status-cell-override branch (list_body.go) through the
// live Controller.ApplyEnrichmentState seam.
//
// 2 and 3 (S5-in-detail full sentence, and Detail=="" fallback) were legacy
// views.DetailModel.PlainContent()-driven pins for the same claim
// Test_DetailAttention_RendersFullDetailSentence_AlongsidePhrase /
// Test_DetailAttention_FallsBackToPhrase_WhenDetailEmpty
// (tests/unit/detail_ports_test.go) now pin on the live
// injectAttentionSectionDetail path — removed here, wave3 detail-family
// cleanup (specs/022-codebase-cleanup).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 4. logs retention-nil (docs/resources/logs.md §4).
// Pinned strings:
//   Phrase (S4): "retention: never expire"
//   Detail (S5): "No retention policy set — events kept forever, billed indefinitely."
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
// Finding with the exact §4-mandated Phrase and Detail.
//
// LOGIC-RED today regardless of the Detail field: FetchCloudWatchLogGroupsPage
// (core/aws/cwlogs.go) does not classify retention-nil into a
// domain.Finding at all — it only stores the raw retention_days field string.
// This test fails today with zero Findings on the resource, not merely a
// missing Detail string; the coder must add Wave-1 classification for this
// signal, not just extend the struct.
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
	if f.Detail != "No retention policy set — events kept forever, billed indefinitely." {
		t.Errorf("logs retention-nil Detail (S5) = %q, want %q (docs/resources/logs.md §4)", f.Detail, "No retention policy set — events kept forever, billed indefinitely.")
	}
}

// ---------------------------------------------------------------------------
// 5. s3 PAB incomplete (docs/resources/s3.md §4), driven through the REAL
// enricher EnrichS3Posture.
// Pinned strings:
//   Phrase (S4): "public access block incomplete"
//   Detail (S5): "Bucket-level public access block is missing or partial — account-level PAB may still apply."
// ---------------------------------------------------------------------------

// TestWave2_S3_PABIncomplete_PinsS4S5Strings drives the real
// EnrichS3Posture enricher against a bucket with no PAB
// configuration (reusing s3PABFake from aws_s3_issue_enrichment_test.go, the
// existing sibling test's fake for this exact enricher) and asserts the exact
// §4-mandated Phrase and Detail.
//
// COMPILE-RED: domain.Finding has no Detail field yet, and
// EnrichS3Posture's setWave2Finding call site has no Detail
// argument to populate one even after the field exists.
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
	if finding.Detail != "Bucket-level public access block is missing or partial — account-level PAB may still apply." {
		t.Errorf("s3 PAB Detail (S5) = %q, want %q", finding.Detail, "Bucket-level public access block is missing or partial — account-level PAB may still apply.")
	}
}

// ---------------------------------------------------------------------------
// 6. ec2 impaired (docs/resources/ec2.md §4), driven through the REAL
// enricher EnrichEC2InstanceStatus.
// Pinned strings (quoted verbatim from docs/resources/ec2.md §4 row
// "SystemStatus.Status == impaired"):
//   Phrase (S4): "impaired: system checks failing"
//   Detail (S5): "AWS reports this instance is impaired — system or instance status checks are failing."
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
//
// COMPILE-RED: domain.Finding has no Detail field yet, and
// EnrichEC2InstanceStatus's setWave2Finding call site has no Detail argument
// to populate one even after the field exists.
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
	if finding.Detail != "AWS reports this instance is impaired — system or instance status checks are failing." {
		t.Errorf("ec2 impaired Detail (S5) = %q, want %q (docs/resources/ec2.md §4)", finding.Detail, "AWS reports this instance is impaired — system or instance status checks are failing.")
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
//
// NOTE: today's EnrichDBIMaintenance emits the buzzword Phrase
// "pending maintenance" (see dbi_issue_enrichment.go setWave2Finding call),
// not the §4-mandated "maintenance scheduled" — this is bug (4) concretely
// pinned for dbi.
//
// COMPILE-RED: domain.Finding has no Detail field yet.
// LOGIC-RED (post-compile-fix): the enricher's Phrase text and missing
// Detail argument both need coder changes.
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
