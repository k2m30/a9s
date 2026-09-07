package unit_test

// finding_row_cap_render_test.go — the closing "… +K more" row is a row, so
// the detail view renders it with no special case. This lives in the external
// test package because the real detail-render drive (detailAttentionValuesFor)
// does; the sink-level assertions on the same cap are in finding_row_cap_test.go.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backupsdk "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
)

func TestFindingRowCap_OverflowRowRendersInTheDetailAttentionSection(t *testing.T) {
	const arn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/render-tg/1d4b7f0a3c6e2958"
	hidden := 5
	targets := awsclient.FindingRowCap + hidden

	descs := make([]elbtypes.TargetHealthDescription, 0, targets)
	for i := range targets {
		descs = append(descs, elbtypes.TargetHealthDescription{
			Target: &elbtypes.TargetDescription{
				Id:   aws.String(fmt.Sprintf("i-0c3d2e1f0a9b%04d", i)),
				Port: aws.Int32(8080),
			},
			TargetHealth: &elbtypes.TargetHealth{
				State:  elbtypes.TargetHealthStateEnumUnhealthy,
				Reason: elbtypes.TargetHealthReasonEnumFailedHealthChecks,
			},
		})
	}
	fake := &fakeELBv2CR{describeTargetHealthOutput: &elbv2.DescribeTargetHealthOutput{
		TargetHealthDescriptions: descs,
	}}

	rows := []resource.Resource{{
		ID:     "render-tg",
		Name:   "render-tg",
		Fields: map[string]string{"target_group_arn": arn},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichTargetGroupHealth: unexpected error: %v", err)
	}

	var td resource.ResourceTypeDef
	for _, d := range resource.AllResourceTypes() {
		if d.ShortName == "tg" {
			td = d
			break
		}
	}
	if td.ShortName == "" {
		t.Fatalf("no registered resource type \"tg\"")
	}

	row := rows[0]
	a9sruntime.ApplyWave2ToRow(&row, td, result.Findings, result.AttentionDetails)

	want := fmt.Sprintf("… +%d more", hidden)
	values := detailAttentionValuesFor(t, row, "tg")
	for _, v := range values {
		if strings.Contains(v, want) {
			return
		}
	}
	t.Errorf("detail Attention section never rendered the overflow row %q; values=%q", want, values)
}

// backupJobsFake serves one page of ListBackupJobs. Embedding the interface
// leaves every other backup call unimplemented, which is what a wave-2 pin
// wants: a call this enricher must not make panics rather than passing.
type backupJobsFake struct {
	awsclient.BackupAPI
	jobs []backuptypes.BackupJob
}

func (f *backupJobsFake) ListBackupJobs(_ context.Context, _ *backupsdk.ListBackupJobsInput, _ ...func(*backupsdk.Options)) (*backupsdk.ListBackupJobsOutput, error) {
	return &backupsdk.ListBackupJobsOutput{BackupJobs: f.jobs}, nil
}

// TestFindingRowCap_BackupFailedJobRowsCloseWithTheOverflowRow drives more
// failed jobs through the backup enricher than one finding shows.
//
// The rows are not one list: the per-job State rows are unbounded, while
// "Most recent" and "Partial jobs" are single facts about the whole bucket.
// Dropping the per-job rows silently loses evidence, and dropping the two
// facts loses the summary — so the facts lead and the list follows the cap,
// which is why this asserts both the closing row and the surviving fact.
func TestFindingRowCap_BackupFailedJobRowsCloseWithTheOverflowRow(t *testing.T) {
	const planID = "acme-render-plan-0000-1111-2222-333333333333"
	const failed = 14

	now := time.Now()
	jobs := make([]backuptypes.BackupJob, 0, failed)
	for i := range failed {
		when := now.Add(-time.Duration(i) * time.Minute)
		jobs = append(jobs, backuptypes.BackupJob{
			BackupJobId:  aws.String(fmt.Sprintf("job-%04d", i)),
			State:        backuptypes.BackupJobStateFailed,
			CreationDate: &when,
			CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
		})
	}

	rows := []resource.Resource{{ID: planID, Name: "acme-render-plan", Type: "backup"}}
	result, err := awsclient.EnrichBackupJobs(context.Background(),
		&awsclient.ServiceClients{Backup: &backupJobsFake{jobs: jobs}}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs: unexpected error: %v", err)
	}

	var td resource.ResourceTypeDef
	for _, d := range resource.AllResourceTypes() {
		if d.ShortName == "backup" {
			td = d
			break
		}
	}
	if td.ShortName == "" {
		t.Fatalf("no registered resource type \"backup\"")
	}

	row := rows[0]
	a9sruntime.ApplyWave2ToRow(&row, td, result.Findings, result.AttentionDetails)
	values := detailAttentionValuesFor(t, row, "backup")
	joined := strings.Join(values, "\n")

	// One "Most recent" row joins the 14 per-job rows, so the cap hides
	// everything past the tenth of the 15.
	want := fmt.Sprintf("… +%d more", failed+1-awsclient.FindingRowCap)
	if !strings.Contains(joined, want) {
		t.Errorf("detail Attention section never rendered the overflow row %q; values=%q", want, values)
	}
	if !strings.Contains(joined, now.UTC().Format("2006-01-02 15:04 UTC")) {
		t.Errorf("the \"Most recent\" fact was pushed out by the per-job rows; values=%q", values)
	}
}

// TestFindingRowCap_ClosingRowRendersWithNoLabel reads the painted backup
// detail, because the label column is a rendering decision and the row model
// alone cannot show it.
//
// The closing row is not a supporting row; it is the statement that there are
// more of them. Wearing the label of the row above turns it into one, so the
// detail ended with "State: … +5 more" — a failed job whose state is that
// text. Blanking domain.DetailRow.Label is half the fix: the projection then
// has to ask for a value-only line, or the renderer paints a bare ":" where
// the label was.
func TestFindingRowCap_ClosingRowRendersWithNoLabel(t *testing.T) {
	const planID = "acme-label-plan-0000-1111-2222-333333333333"
	const failed = 14

	now := time.Now()
	jobs := make([]backuptypes.BackupJob, 0, failed)
	for i := range failed {
		when := now.Add(-time.Duration(i) * time.Minute)
		jobs = append(jobs, backuptypes.BackupJob{
			BackupJobId:  aws.String(fmt.Sprintf("job-%04d", i)),
			State:        backuptypes.BackupJobStateFailed,
			CreationDate: &when,
			CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
		})
	}

	rows := []resource.Resource{{ID: planID, Name: "acme-label-plan", Type: "backup"}}
	result, err := awsclient.EnrichBackupJobs(context.Background(),
		&awsclient.ServiceClients{Backup: &backupJobsFake{jobs: jobs}}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs: unexpected error: %v", err)
	}

	var td resource.ResourceTypeDef
	for _, d := range resource.AllResourceTypes() {
		if d.ShortName == "backup" {
			td = d
			break
		}
	}
	if td.ShortName == "" {
		t.Fatalf("no registered resource type \"backup\"")
	}

	row := rows[0]
	a9sruntime.ApplyWave2ToRow(&row, td, result.Findings, result.AttentionDetails)

	want := fmt.Sprintf("… +%d more", failed+1-awsclient.FindingRowCap)
	painted := stripAnsi(wave3RenderDetailFor(t, row, "backup", 120, 60))

	var closing string
	for _, line := range strings.Split(painted, "\n") {
		if strings.Contains(line, want) {
			closing = line
			break
		}
	}
	if closing == "" {
		t.Fatalf("the closing row %q never painted; detail was:\n%s", want, painted)
	}
	if strings.Contains(closing, "State: "+want) {
		t.Errorf("the closing row paints under the label of the row above it, so it reads as one more failed job:\n%s", closing)
	}
	if strings.Contains(closing, ": "+want) {
		t.Errorf("the closing row paints a bare label separator in front of the count:\n%s", closing)
	}
	// The State rows above it are still labelled — only the closing row is not.
	if !strings.Contains(painted, "State: failed") {
		t.Errorf("the per-job rows lost their label along with the closing row; detail was:\n%s", painted)
	}
}
