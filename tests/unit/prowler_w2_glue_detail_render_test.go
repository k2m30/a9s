package unit_test

// prowler_w2_glue_detail_render_test.go — the rendered half of ruling J for
// glue.
//
// T-INV-2 now accepts a Detail sentence in place of rows, which is only safe
// if the detail view actually renders that sentence. Asserting on the
// IssueEnricherResult would prove the enricher set the field, not that an
// operator sees anything: the row is folded and the block is built by code
// this test does not own. So it drives the same path the app does and reads
// what comes out.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	glue "github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

type w2GlueRunFake struct {
	awsclient.GlueAPI

	state        gluetypes.JobRunState
	errorMessage *string
}

func (f *w2GlueRunFake) GetJobRuns(_ context.Context, in *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	return &glue.GetJobRunsOutput{
		JobRuns: []gluetypes.JobRun{{
			JobName:      in.JobName,
			JobRunState:  f.state,
			ErrorMessage: f.errorMessage,
		}},
	}, nil
}

// TestW2GlueFailedRunDetailReachesTheScreen pins that a failed Glue run gives
// the operator a line past the phrase whether or not AWS said why it failed.
//
// The message-less case is the one that matters: it is exactly the shape that
// had zero rows once the row restating the phrase was dropped, and the only
// thing standing between it and a bare entry is the Detail sentence being
// rendered.
func TestW2GlueFailedRunDetailReachesTheScreen(t *testing.T) {
	td := catalogTypeFor(t, "glue")

	for _, tc := range []struct {
		name    string
		errMsg  *string
		wantRow bool
	}{
		{"AWS reported a cause", aws.String("JobRunFailed: worker ran out of memory"), true},
		{"AWS reported no cause", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clients := &awsclient.ServiceClients{Glue: &w2GlueRunFake{
				state:        gluetypes.JobRunStateFailed,
				errorMessage: tc.errMsg,
			}}
			rows := []resource.Resource{{ID: "acme-etl-job", Name: "acme-etl-job", Fields: map[string]string{}}}

			res, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, rows, nil)
			if err != nil {
				t.Fatalf("EnrichGlueJobStatus: %v", err)
			}
			row := rows[0]
			runtime.ApplyWave2ToRow(&row, td, res.Findings, res.AttentionDetails)

			var finding domain.Finding
			for _, f := range row.Findings {
				if strings.HasPrefix(string(f.Code), "glue.") {
					finding = f
				}
			}
			if finding.Code == "" {
				t.Fatalf("no glue finding on the folded row; got %v", row.Findings)
			}
			if got := len(row.AttentionDetails[finding.Code].Rows) > 0; got != tc.wantRow {
				t.Errorf("has rows = %v, want %v", got, tc.wantRow)
			}

			lines := detailAttentionValuesFor(t, row, "glue")
			if len(lines) == 0 {
				t.Fatal("the Attention block rendered nothing; a failed run must say more than its phrase")
			}
			for _, l := range lines {
				if isVacuousPhrase(l) {
					t.Errorf("rendered attention line %q is a bare severity word", l)
				}
			}
			// The Attention sentence wraps to the panel and arrives as successive
			// lines; a single line holding the sentence is exactly what the panel
			// edge cuts. What must hold is that the whole sentence reaches the
			// screen.
			if joined := strings.Join(lines, " "); !strings.Contains(joined, strings.Join(strings.Fields(finding.Detail), " ")) {
				t.Errorf("the Detail sentence never reached the screen; rendered lines were %q", lines)
			}
		})
	}
}

func catalogTypeFor(t *testing.T, shortName string) resource.ResourceTypeDef {
	t.Helper()
	for _, td := range resource.AllResourceTypes() {
		if td.ShortName == shortName {
			return td
		}
	}
	t.Fatalf("no registered resource type %q", shortName)
	return resource.ResourceTypeDef{}
}
