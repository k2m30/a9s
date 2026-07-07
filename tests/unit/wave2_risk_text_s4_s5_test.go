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
//   - internal/domain/finding.go: Finding gains `Detail string` (S5 sentence;
//     empty ⇒ render falls back to Phrase, no stray blank line).
//   - internal/app/list_columns.go (listExtractCellValue) and/or
//     internal/tui/views/resourcelist.go (renderListDataRow): when a row's
//     enrichment-map Finding (delivered via SetEnrichmentState /
//     GetListEnrichmentFindings, see internal/app/list_filter.go
//     applyEnrichmentState) is issue-severity (SevWarn/SevBroken), the
//     Status/lifecycle cell must show that Finding's Phrase — not just a
//     glyph prefix on the identity column (today's ONLY effect of
//     SetEnrichmentState per resolveListDecoratorFull in
//     internal/app/list_columns.go).
//   - internal/tui/views/detail_fields.go (injectAttentionSection): each
//     Attention entry must render both the short Phrase line (already does,
//     via capitalizeFirst) AND — on its own additional line — the full
//     Detail sentence, falling back to Phrase alone when Detail == "".
//
// RED classification:
//   - Tests #2 and #3 (detail S5) are COMPILE-RED right now: domain.Finding
//     has no Detail field, so `domain.Finding{..., Detail: "..."}` fails to
//     compile until the coder adds the field.
//   - Test #1 (list S4) is LOGIC-RED: it compiles today (Finding literals use
//     only existing fields) but fails at assertion time because
//     SetEnrichmentState's findings map never reaches listExtractCellValue —
//     it only drives the "! "/"~ " glyph prefix on the identity column, per
//     resolveListDecoratorFull (internal/app/list_columns.go) and
//     buildMarkerModel-style harnesses (see qa_enrichment_marker_test.go).
//   - Tests #5-#7 (s3/ec2/dbi exemplars, driven through the real Wave-2
//     enrichers) are COMPILE-RED for the same reason as #2/#3 (they assert
//     on Finding.Detail) AND LOGIC-RED even post-compile-fix: none of
//     EnrichS3PublicAccessBlock, EnrichEC2InstanceStatus, EnrichDBIMaintenance
//     (internal/aws/*.go) call setWave2Finding with a Detail argument; ec2's
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
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdssvc "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// 1. S4 in list: a flagged row's Status column shows the concrete Phrase.
// ---------------------------------------------------------------------------

// wave2S4TypeDef mirrors markerTypeDef (qa_enrichment_marker_test.go) — a
// "name"+"state" column pair so the Status/lifecycle column is unambiguous.
func wave2S4TypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 30},
		},
	}
}

// TestWave2_ListStatusColumn_ShowsConcretePhrase_ForIssueFinding verifies bug
// (3): a resource carrying an issue-severity enrichment finding
// ("no automated backups") must show that concrete Phrase in the rendered
// Status/lifecycle column — not the raw AWS state ("available") the row was
// seeded with. This is the LIST S4 surface (docs/resources/*.md §4 "S4").
func TestWave2_ListStatusColumn_ShowsConcretePhrase_ForIssueFinding(t *testing.T) {
	tuitest.NoColor(t)

	td := wave2S4TypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{
				ID:     "i-flagged-1",
				Name:   "billing-db-01",
				Fields: map[string]string{"name": "billing-db-01", "state": "available"},
			},
		},
	})

	findings := map[string]domain.Finding{
		"i-flagged-1": {
			Code:     "dbi.no-backups",
			Phrase:   "no automated backups",
			Severity: domain.SevWarn,
			Source:   "wave2:dbi",
		},
	}
	m.SetEnrichmentState(0, false, findings, nil)

	rendered := m.View()
	plain := stripANSI(rendered)
	line := findLineContaining(plain, "billing-db-01")
	if line == "" {
		t.Fatal("could not find rendered row for billing-db-01")
	}
	if !strings.Contains(line, "no automated backups") {
		t.Errorf("flagged row's Status column must show the concrete cause %q; got row: %q", "no automated backups", line)
	}
	if strings.Contains(line, "available") {
		t.Errorf("flagged row must NOT still show the raw AWS state %q once an issue finding exists for it; got row: %q", "available", line)
	}
}

// ---------------------------------------------------------------------------
// 2. S5 in detail: full Detail sentence renders alongside the short Phrase.
// ---------------------------------------------------------------------------

// TestWave2_DetailAttention_RendersFullDetailSentence_AlongsidePhrase verifies
// bug (4): the detail view's Attention section must render the full S5
// operator sentence (Finding.Detail) on its own line, in addition to the
// short S4 Phrase — not the Phrase alone.
//
// COMPILE-RED: domain.Finding has no Detail field yet.
func TestWave2_DetailAttention_RendersFullDetailSentence_AlongsidePhrase(t *testing.T) {
	// PlainContent() renders plain text regardless of NO_COLOR — no color
	// reset needed here (mirrors TestViews_DetailAttention_PrefersFindingsPhraseOverIssues
	// in phase03_view_reads_test.go, which also skips it for PlainContent-only assertions).

	const code domain.FindingCode = "dbi.pending-maintenance"
	r := resource.Resource{
		ID:   "db-maint-1",
		Name: "prod-maint-db",
		Fields: map[string]string{
			"status": "available",
		},
		Findings: []domain.Finding{
			{
				Code:     code,
				Phrase:   "maintenance scheduled",
				Detail:   "Pending maintenance action overdue: system-update.",
				Severity: domain.SevWarn,
				Source:   "wave2:dbi",
			},
		},
	}

	k := keys.Default()
	m := views.NewDetail(r, "dbi", nil, k)
	m.SetSize(200, 100)

	out := m.PlainContent()

	if !strings.Contains(out, "Pending maintenance action overdue: system-update.") {
		t.Errorf("detail Attention section must render the full S5 Detail sentence; got:\n%s", out)
	}
	if !strings.Contains(out, "maintenance scheduled") {
		t.Errorf("detail Attention section must still render the short S4 Phrase alongside Detail; got:\n%s", out)
	}
	// The two must appear as genuinely separate lines, not concatenated —
	// otherwise the "short phrase for triage, full sentence for follow-up"
	// contract collapses into one run-on string.
	foundPhraseLine := false
	foundDetailLine := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "maintenance scheduled") && !strings.Contains(ln, "Pending maintenance action overdue") {
			foundPhraseLine = true
		}
		if strings.Contains(ln, "Pending maintenance action overdue: system-update.") {
			foundDetailLine = true
		}
	}
	if !foundPhraseLine {
		t.Errorf("expected a line containing only the short Phrase (no Detail text on it); got:\n%s", out)
	}
	if !foundDetailLine {
		t.Errorf("expected a separate line containing the full Detail sentence; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 3. S5 fallback: Detail=="" still renders Phrase, no stray empty line.
// ---------------------------------------------------------------------------

// TestWave2_DetailAttention_FallsBackToPhrase_WhenDetailEmpty verifies that
// when Finding.Detail is the empty string, the Attention section still
// renders the Phrase line and does not emit an extra blank line where the
// Detail sentence would have gone.
//
// COMPILE-RED: domain.Finding has no Detail field yet.
func TestWave2_DetailAttention_FallsBackToPhrase_WhenDetailEmpty(t *testing.T) {
	// PlainContent() renders plain text regardless of NO_COLOR — no color
	// reset needed here (see comment in the sibling test above).

	const code domain.FindingCode = "ec2.instance-status-impaired"
	r := resource.Resource{
		ID:   "i-nodep-1",
		Name: "worker-nodetail",
		Fields: map[string]string{
			"state": "running",
		},
		Findings: []domain.Finding{
			{
				Code:     code,
				Phrase:   "impaired: system checks failing",
				Detail:   "",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2",
			},
		},
	}

	k := keys.Default()
	m := views.NewDetail(r, "ec2", nil, k)
	m.SetSize(200, 100)

	out := m.PlainContent()

	if !strings.Contains(out, "impaired: system checks failing") {
		t.Errorf("Attention section must render Phrase when Detail is empty; got:\n%s", out)
	}

	// No stray blank line immediately following the Attention phrase entry —
	// find the phrase line, then assert the very next non-empty content line
	// is NOT an empty string masquerading as a Detail placeholder. We check
	// this by ensuring there is no line that is purely whitespace sitting
	// directly between the Attention section header and the next real field,
	// beyond the single pre-existing separator blank line the section always
	// emits after all entries (see injectAttentionSection's trailing Spacer).
	lines := strings.Split(out, "\n")
	phraseIdx := -1
	for i, ln := range lines {
		if strings.Contains(ln, "impaired: system checks failing") {
			phraseIdx = i
			break
		}
	}
	if phraseIdx == -1 {
		t.Fatal("phrase line not found")
	}
	if phraseIdx+1 < len(lines) && strings.TrimSpace(lines[phraseIdx+1]) == "" {
		t.Errorf("expected no stray empty line directly after the Phrase entry when Detail is empty (fallback must not add blank lines); line after phrase: %q", lines[phraseIdx+1])
	}
}

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
// (internal/aws/cwlogs.go) does not classify retention-nil into a
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
// enricher EnrichS3PublicAccessBlock.
// Pinned strings:
//   Phrase (S4): "public access block incomplete"
//   Detail (S5): "Bucket-level public access block is missing or partial — account-level PAB may still apply."
// ---------------------------------------------------------------------------

// TestWave2_S3_PABIncomplete_PinsS4S5Strings drives the real
// EnrichS3PublicAccessBlock enricher against a bucket with no PAB
// configuration (reusing s3PABFake from aws_s3_issue_enrichment_test.go, the
// existing sibling test's fake for this exact enricher) and asserts the exact
// §4-mandated Phrase and Detail.
//
// COMPILE-RED: domain.Finding has no Detail field yet, and
// EnrichS3PublicAccessBlock's setWave2Finding call site has no Detail
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
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
	wantDetail := "Pending maintenance action overdue: system-update (New minor engine patch 16.2.3)."
	if finding.Detail != wantDetail {
		t.Errorf("dbi pending-maintenance Detail (S5) = %q, want %q (docs/resources/dbi.md §4)", finding.Detail, wantDetail)
	}
}
