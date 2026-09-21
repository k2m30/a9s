package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// b2EBTargetFake records the ListTargetsByRule input it was handed.
type b2EBTargetFake struct {
	awsclient.EventBridgeAPI
	last *eventbridge.ListTargetsByRuleInput
}

func (f *b2EBTargetFake) ListTargetsByRule(
	_ context.Context,
	in *eventbridge.ListTargetsByRuleInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	f.last = in
	return &eventbridge.ListTargetsByRuleOutput{}, nil
}

// EventBusName has a minimum length of one, so a rule on the default bus is
// read by omitting the field rather than sending it empty.
func TestEBRuleTargets_DefaultBusOmitsEventBusName(t *testing.T) {
	fake := &b2EBTargetFake{}
	if _, err := awsclient.FetchEventBridgeRuleTargets(context.Background(), fake,
		map[string]string{"rule_name": "acme-nightly", "event_bus": ""}, ""); err != nil {
		t.Fatalf("FetchEventBridgeRuleTargets: %v", err)
	}
	if fake.last.EventBusName != nil {
		t.Errorf("EventBusName = %q, want it unset for the default bus", *fake.last.EventBusName)
	}

	if _, err := awsclient.FetchEventBridgeRuleTargets(context.Background(), fake,
		map[string]string{"rule_name": "acme-nightly", "event_bus": "acme-bus"}, ""); err != nil {
		t.Fatalf("FetchEventBridgeRuleTargets: %v", err)
	}
	if fake.last.EventBusName == nil || *fake.last.EventBusName != "acme-bus" {
		t.Errorf("EventBusName = %v, want %q", fake.last.EventBusName, "acme-bus")
	}
}

// ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS is a second enabled state, and
// a rule in it delivers to its targets like any other.
func TestEBRule_CloudTrailEnabledStateCountsAsEnabled(t *testing.T) {
	const rule = "acme-audit"
	fake := newEBPaginatedFake()

	res, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(),
		&awsclient.ServiceClients{EventBridge: fake},
		[]resource.Resource{{
			ID:   rule,
			Name: rule,
			Fields: map[string]string{
				"name":  rule,
				"state": string(eventbridgetypes.RuleStateEnabledWithAllCloudtrailManagementEvents),
			},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichEventBridgeRuleTargets: %v", err)
	}
	if len(res.Findings[rule]) == 0 {
		t.Error("a rule in the CloudTrail-management enabled state with no targets produced no finding")
	}
}

// Glue 5.0 streams executor output without the argument, so only 4.0 and
// earlier have anything to turn on.
func TestGlue_ContinuousLogging_OnlyBefore5(t *testing.T) {
	for _, tc := range []struct {
		version  string
		wantFind bool
	}{
		{"0.9", true},
		{"4.0", true},
		{"5.0", false},
		{"", true},
	} {
		t.Run("glue "+tc.version, func(t *testing.T) {
			job := w6bGlueJob("acme-etl")
			job.GlueVersion = aws.String(tc.version)
			if tc.version == "" {
				job.GlueVersion = nil
			}
			delete(job.DefaultArguments, w6bGlueContinuousLogArg)

			got := b2HasCode(w6bFetchGlue(t, job)[0].Findings, w6bGlueCodeLoggingOff)
			if got != tc.wantFind {
				t.Errorf("continuous-logging-off finding = %v, want %v", got, tc.wantFind)
			}
		})
	}
}

// A run that expired never produced its output, which the job-runs child view
// already calls broken; the job that owns it reads the same way.
func TestGlue_ExpiredRunIsNotHealthy(t *testing.T) {
	const job = "acme-etl"
	fake := &glueJobFake{jobRuns: map[string]gluetypes.JobRunState{job: gluetypes.JobRunStateExpired}}

	res, err := awsclient.EnrichGlueJobStatus(context.Background(),
		&awsclient.ServiceClients{Glue: fake}, []resource.Resource{{Name: job}}, nil)
	if err != nil {
		t.Fatalf("EnrichGlueJobStatus: %v", err)
	}
	if !b2HasCode(res.Findings[job], domain.FindingCode("glue.latest-run-failed")) {
		t.Errorf("an EXPIRED latest run produced %v, want glue.latest-run-failed", res.Findings)
	}
	if got := res.FieldUpdates[job]["last_run"]; got != string(gluetypes.JobRunStateExpired) {
		t.Errorf("last_run = %q, want %q", got, gluetypes.JobRunStateExpired)
	}
}

// A pipeline with no failed stage has no stage to name: healthy, never run
// and mid-execution all leave the column blank rather than claiming health.
func TestPipeline_NoFailedStageLeavesStatusBlank(t *testing.T) {
	const name = "acme-deploy"
	fake := &cpGetPipelineStateFake{results: map[string]*codepipeline.GetPipelineStateOutput{
		name: {
			PipelineName: aws.String(name),
			StageStates: []cptypes.StageState{{
				StageName:       aws.String("Build"),
				LatestExecution: &cptypes.StageExecution{Status: cptypes.StageExecutionStatusSucceeded},
			}},
		},
	}}

	res, err := awsclient.EnrichCodePipelineStatus(context.Background(),
		&awsclient.ServiceClients{CodePipeline: fake},
		[]resource.Resource{{ID: name, Name: name, Fields: map[string]string{}}}, nil)
	if err != nil {
		t.Fatalf("EnrichCodePipelineStatus: %v", err)
	}
	if got := res.FieldUpdates[name]["last_status"]; got != "" {
		t.Errorf("last_status = %q, want empty", got)
	}
}

// Results in Athena owned storage are encrypted whatever the workgroup's S3
// result configuration says, so the S3-side setting decides nothing there.
func TestAthena_ManagedResultsAreNotUnencrypted(t *testing.T) {
	const wg = "acme-analytics"
	for _, tc := range []struct {
		name     string
		managed  *athenatypes.ManagedQueryResultsConfiguration
		wantFind bool
	}{
		{"results in the caller's bucket, unencrypted", nil, true},
		{"managed results turned off", &athenatypes.ManagedQueryResultsConfiguration{Enabled: false}, true},
		{"results in Athena owned storage", &athenatypes.ManagedQueryResultsConfiguration{Enabled: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &athenaGetWorkGroupFake{results: map[string]*athenatypes.WorkGroup{
				wg: {
					Name: aws.String(wg),
					Configuration: &athenatypes.WorkGroupConfiguration{
						EnforceWorkGroupConfiguration:    aws.Bool(true),
						ManagedQueryResultsConfiguration: tc.managed,
						ResultConfiguration: &athenatypes.ResultConfiguration{
							OutputLocation: aws.String("s3://acme-athena-results/"),
						},
					},
				},
			}}
			res, err := awsclient.EnrichAthenaWorkGroup(context.Background(),
				&awsclient.ServiceClients{Athena: fake}, athenaWorkGroupResources(wg), nil)
			if err != nil {
				t.Fatalf("EnrichAthenaWorkGroup: %v", err)
			}
			got := b2HasCode(res.Findings[wg], domain.FindingCode("athena.results-unencrypted"))
			if got != tc.wantFind {
				t.Errorf("results-unencrypted finding = %v, want %v", got, tc.wantFind)
			}
		})
	}
}
