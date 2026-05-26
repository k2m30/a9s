package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// AS-1393 W1.1 color-signal regression suite.
//
// When Wave-1 fetchers stopped writing Resource.Status, the per-row color
// signal disappeared for all child types whose ResourceTypeDef had no Color
// func (cb_builds, cfn_resources, glue_runs, log_events,
// lambda_invocation_logs, role_policies). The fix emits wave1 domain.Findings
// + adds Color: colorWave1OrHealthy to each catalog entry; the tests below
// pin both halves so a future regression fails loudly instead of silently
// rendering FAILED rows green.
// ---------------------------------------------------------------------------

func fetchOneCBBuild(t *testing.T, status cbtypes.StatusType) resource.Resource {
	t.Helper()
	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{{Ids: []string{"my-project:build-1"}}},
	}
	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{{
			Builds: []cbtypes.Build{{
				Id:          aws.String("my-project:build-1"),
				BuildNumber: aws.Int64(1),
				BuildStatus: status,
				ProjectName: aws.String("my-project"),
			}},
		}},
	}
	result, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, map[string]string{"project_name": "my-project"}, "")
	if err != nil {
		t.Fatalf("FetchCBBuilds: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 build, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func TestCBBuilds_ColorSignal(t *testing.T) {
	td := resource.GetChildType("cb_builds")
	if td == nil {
		t.Fatal("cb_builds child type not registered")
	}
	cases := []struct {
		name      string
		status    cbtypes.StatusType
		wantColor resource.Color
		wantSev   domain.Severity
		wantCode  domain.FindingCode
	}{
		{"FAILED", cbtypes.StatusTypeFailed, resource.ColorBroken, domain.SevBroken, awsclient.CodeCBBuildFailed},
		{"FAULT", cbtypes.StatusTypeFault, resource.ColorBroken, domain.SevBroken, awsclient.CodeCBBuildFault},
		{"TIMED_OUT", cbtypes.StatusTypeTimedOut, resource.ColorBroken, domain.SevBroken, awsclient.CodeCBBuildTimedOut},
		{"IN_PROGRESS", cbtypes.StatusTypeInProgress, resource.ColorWarning, domain.SevWarn, awsclient.CodeCBBuildInProgress},
		{"STOPPED", cbtypes.StatusTypeStopped, resource.ColorDim, domain.SevDim, awsclient.CodeCBBuildStopped},
		{"SUCCEEDED", cbtypes.StatusTypeSucceeded, resource.ColorHealthy, 0, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			r := fetchOneCBBuild(t, tc.status)
			if got := td.ResolveColor(r); got != tc.wantColor {
				t.Errorf("ResolveColor(%s) = %v, want %v (Findings=%+v, Fields[build_status]=%q)", tc.status, got, tc.wantColor, r.Findings, r.Fields["build_status"])
			}
			if tc.wantCode != "" {
				if len(r.Findings) == 0 {
					t.Fatalf("%s: expected wave1 finding, got none", tc.status)
				}
				f := r.Findings[0]
				if f.Source != "wave1" {
					t.Errorf("Findings[0].Source = %q, want wave1", f.Source)
				}
				if f.Severity != tc.wantSev {
					t.Errorf("Findings[0].Severity = %v, want %v", f.Severity, tc.wantSev)
				}
				if f.Code != tc.wantCode {
					t.Errorf("Findings[0].Code = %q, want %q", f.Code, tc.wantCode)
				}
			} else if len(r.Findings) != 0 {
				t.Errorf("%s: expected no findings, got %+v", tc.status, r.Findings)
			}
		})
	}
}

func fetchOneCfnResource(t *testing.T, status cfntypes.ResourceStatus) resource.Resource {
	t.Helper()
	mock := &mockCFNListStackResourcesClient{
		outputs: []*cloudformation.ListStackResourcesOutput{{
			StackResourceSummaries: []cfntypes.StackResourceSummary{{
				LogicalResourceId: aws.String("MyResource"),
				ResourceType:      aws.String("AWS::S3::Bucket"),
				ResourceStatus:    status,
			}},
		}},
	}
	result, err := awsclient.FetchCfnResources(context.Background(), mock, "my-stack", "")
	if err != nil {
		t.Fatalf("FetchCfnResources: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func TestCfnResources_ColorSignal(t *testing.T) {
	td := resource.GetChildType("cfn_resources")
	if td == nil {
		t.Fatal("cfn_resources child type not registered")
	}
	cases := []struct {
		name      string
		status    cfntypes.ResourceStatus
		wantColor resource.Color
		wantCode  domain.FindingCode
	}{
		{"CREATE_FAILED", cfntypes.ResourceStatusCreateFailed, resource.ColorBroken, awsclient.CodeCfnResourceFailed},
		{"UPDATE_FAILED", cfntypes.ResourceStatusUpdateFailed, resource.ColorBroken, awsclient.CodeCfnResourceFailed},
		{"DELETE_FAILED", cfntypes.ResourceStatusDeleteFailed, resource.ColorBroken, awsclient.CodeCfnResourceFailed},
		{"CREATE_IN_PROGRESS", cfntypes.ResourceStatusCreateInProgress, resource.ColorWarning, awsclient.CodeCfnResourceInProgress},
		{"DELETE_COMPLETE", cfntypes.ResourceStatusDeleteComplete, resource.ColorDim, awsclient.CodeCfnResourceDeleted},
		{"CREATE_COMPLETE", cfntypes.ResourceStatusCreateComplete, resource.ColorHealthy, ""},
		{"UPDATE_COMPLETE", cfntypes.ResourceStatusUpdateComplete, resource.ColorHealthy, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			r := fetchOneCfnResource(t, tc.status)
			if got := td.ResolveColor(r); got != tc.wantColor {
				t.Errorf("ResolveColor(%s) = %v, want %v", tc.status, got, tc.wantColor)
			}
			if tc.wantCode != "" {
				if len(r.Findings) == 0 || r.Findings[0].Code != tc.wantCode {
					t.Errorf("expected Findings[0].Code=%q, got %+v", tc.wantCode, r.Findings)
				}
			} else if len(r.Findings) != 0 {
				t.Errorf("expected no findings for %s, got %+v", tc.status, r.Findings)
			}
		})
	}
}

func fetchOneGlueRun(t *testing.T, state gluetypes.JobRunState) resource.Resource {
	t.Helper()
	mock := &mockGlueGetJobRunsClient{
		outputs: []*glue.GetJobRunsOutput{{
			JobRuns: []gluetypes.JobRun{{
				Id:          aws.String("jr-12345678"),
				JobRunState: state,
				JobName:     aws.String("my-job"),
			}},
		}},
	}
	result, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "my-job", "")
	if err != nil {
		t.Fatalf("FetchGlueJobRuns: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 run, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func TestGlueRuns_ColorSignal(t *testing.T) {
	td := resource.GetChildType("glue_runs")
	if td == nil {
		t.Fatal("glue_runs child type not registered")
	}
	cases := []struct {
		state     gluetypes.JobRunState
		wantColor resource.Color
		wantCode  domain.FindingCode
	}{
		{gluetypes.JobRunStateFailed, resource.ColorBroken, awsclient.CodeGlueRunFailed},
		{gluetypes.JobRunStateTimeout, resource.ColorBroken, awsclient.CodeGlueRunTimeout},
		{gluetypes.JobRunStateError, resource.ColorBroken, awsclient.CodeGlueRunError},
		{gluetypes.JobRunStateExpired, resource.ColorBroken, awsclient.CodeGlueRunExpired},
		{gluetypes.JobRunStateRunning, resource.ColorWarning, awsclient.CodeGlueRunRunning},
		{gluetypes.JobRunStateStarting, resource.ColorWarning, awsclient.CodeGlueRunStarting},
		{gluetypes.JobRunStateStopping, resource.ColorWarning, awsclient.CodeGlueRunStopping},
		{gluetypes.JobRunStateWaiting, resource.ColorWarning, awsclient.CodeGlueRunWaiting},
		{gluetypes.JobRunStateStopped, resource.ColorDim, awsclient.CodeGlueRunStopped},
		{gluetypes.JobRunStateSucceeded, resource.ColorHealthy, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.state), func(t *testing.T) {
			r := fetchOneGlueRun(t, tc.state)
			if got := td.ResolveColor(r); got != tc.wantColor {
				t.Errorf("ResolveColor(%s) = %v, want %v", tc.state, got, tc.wantColor)
			}
			if tc.wantCode != "" {
				if len(r.Findings) == 0 || r.Findings[0].Code != tc.wantCode {
					t.Errorf("expected Findings[0].Code=%q, got %+v", tc.wantCode, r.Findings)
				}
			} else if len(r.Findings) != 0 {
				t.Errorf("expected no findings for %s, got %+v", tc.state, r.Findings)
			}
		})
	}
}

func fetchOneLogEvent(t *testing.T, message string) resource.Resource {
	t.Helper()
	now := time.Now().UnixMilli()
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwltypes.OutputLogEvent{{
				Message:   aws.String(message),
				Timestamp: aws.Int64(now),
			}},
		},
	}
	result, err := awsclient.FetchLogEvents(context.Background(), mock, "lg", "ls", "")
	if err != nil {
		t.Fatalf("FetchLogEvents: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func TestLogEvents_ColorSignal(t *testing.T) {
	td := resource.GetChildType("log_events")
	if td == nil {
		t.Fatal("log_events child type not registered")
	}
	cases := []struct {
		name      string
		message   string
		wantColor resource.Color
		wantCode  domain.FindingCode
	}{
		{"ERROR_message", "ERROR: bad happened", resource.ColorBroken, awsclient.CodeCWLogError},
		{"FATAL_message", "FATAL crash", resource.ColorBroken, awsclient.CodeCWLogError},
		{"Exception_message", "Exception: NullPointer", resource.ColorBroken, awsclient.CodeCWLogError},
		{"WARN_message", "WARN: deprecated path", resource.ColorWarning, awsclient.CodeCWLogWarn},
		{"REPORT_message", "REPORT RequestId: 1", resource.ColorHealthy, ""},
		{"plain_message", "hello world", resource.ColorHealthy, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := fetchOneLogEvent(t, tc.message)
			if got := td.ResolveColor(r); got != tc.wantColor {
				t.Errorf("ResolveColor(%q) = %v, want %v", tc.message, got, tc.wantColor)
			}
			if tc.wantCode != "" {
				if len(r.Findings) == 0 || r.Findings[0].Code != tc.wantCode {
					t.Errorf("expected Findings[0].Code=%q, got %+v", tc.wantCode, r.Findings)
				}
			} else if len(r.Findings) != 0 {
				t.Errorf("expected no findings for %q, got %+v", tc.message, r.Findings)
			}
		})
	}
}

func fetchOneLambdaInvocationLog(t *testing.T, message string) resource.Resource {
	t.Helper()
	now := time.Now().UnixMilli()
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{{
			Events: []cwltypes.FilteredLogEvent{{
				EventId:   aws.String("evt-1"),
				Message:   aws.String(message),
				Timestamp: aws.Int64(now),
			}},
		}},
	}
	result, err := awsclient.FetchLambdaInvocationLogs(context.Background(), mock, "lg", "req-1", "")
	if err != nil {
		t.Fatalf("FetchLambdaInvocationLogs: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func TestLambdaInvocationLogs_ColorSignal(t *testing.T) {
	td := resource.GetChildType("lambda_invocation_logs")
	if td == nil {
		t.Fatal("lambda_invocation_logs child type not registered")
	}
	cases := []struct {
		name      string
		message   string
		wantColor resource.Color
		wantCode  domain.FindingCode
	}{
		{"ERROR_message", "ERROR: handler failed", resource.ColorBroken, awsclient.CodeCWLogError},
		{"Traceback_message", "Traceback (most recent call last)", resource.ColorBroken, awsclient.CodeCWLogError},
		{"WARN_message", "WARN: cold start", resource.ColorWarning, awsclient.CodeCWLogWarn},
		{"REPORT_message", "REPORT RequestId: ...", resource.ColorHealthy, ""},
		{"plain_message", "init complete", resource.ColorHealthy, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := fetchOneLambdaInvocationLog(t, tc.message)
			if got := td.ResolveColor(r); got != tc.wantColor {
				t.Errorf("ResolveColor(%q) = %v, want %v", tc.message, got, tc.wantColor)
			}
			if tc.wantCode != "" {
				if len(r.Findings) == 0 || r.Findings[0].Code != tc.wantCode {
					t.Errorf("expected Findings[0].Code=%q, got %+v", tc.wantCode, r.Findings)
				}
			} else if len(r.Findings) != 0 {
				t.Errorf("expected no findings for %q, got %+v", tc.message, r.Findings)
			}
		})
	}
}

func fetchRolePoliciesForColor(t *testing.T, managed []string, inline []string) []resource.Resource {
	t.Helper()
	attached := make([]iamtypes.AttachedPolicy, 0, len(managed))
	for _, name := range managed {
		attached = append(attached, iamtypes.AttachedPolicy{
			PolicyName: aws.String(name),
			PolicyArn:  aws.String("arn:aws:iam::aws:policy/" + name),
		})
	}
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{{AttachedPolicies: attached}},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{{PolicyNames: inline}},
	}
	result, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, map[string]string{"role_name": "r"}, "")
	if err != nil {
		t.Fatalf("FetchRolePolicies: %v", err)
	}
	return result.Resources
}

func TestRolePolicies_ColorSignal(t *testing.T) {
	td := resource.GetChildType("role_policies")
	if td == nil {
		t.Fatal("role_policies child type not registered")
	}

	t.Run("AdministratorAccess_renders_broken_red", func(t *testing.T) {
		rs := fetchRolePoliciesForColor(t, []string{"AdministratorAccess"}, nil)
		if got := td.ResolveColor(rs[0]); got != resource.ColorBroken {
			t.Errorf("AdministratorAccess ResolveColor = %v, want ColorBroken", got)
		}
		if len(rs[0].Findings) == 0 || rs[0].Findings[0].Code != awsclient.CodeRolePolicyOverPrivileged {
			t.Errorf("expected over-privileged finding, got %+v", rs[0].Findings)
		}
	})
	t.Run("PowerUserAccess_renders_broken_red", func(t *testing.T) {
		rs := fetchRolePoliciesForColor(t, []string{"PowerUserAccess"}, nil)
		if got := td.ResolveColor(rs[0]); got != resource.ColorBroken {
			t.Errorf("PowerUserAccess ResolveColor = %v, want ColorBroken", got)
		}
	})
	t.Run("ReadOnlyAccess_renders_healthy", func(t *testing.T) {
		rs := fetchRolePoliciesForColor(t, []string{"ReadOnlyAccess"}, nil)
		if got := td.ResolveColor(rs[0]); got != resource.ColorHealthy {
			t.Errorf("ReadOnlyAccess ResolveColor = %v, want ColorHealthy", got)
		}
		if len(rs[0].Findings) != 0 {
			t.Errorf("expected no findings for ReadOnlyAccess, got %+v", rs[0].Findings)
		}
	})
	t.Run("Inline_policy_renders_dim", func(t *testing.T) {
		rs := fetchRolePoliciesForColor(t, nil, []string{"trust-policy"})
		if got := td.ResolveColor(rs[0]); got != resource.ColorDim {
			t.Errorf("inline policy ResolveColor = %v, want ColorDim", got)
		}
		if len(rs[0].Findings) == 0 || rs[0].Findings[0].Code != awsclient.CodeRolePolicyInline {
			t.Errorf("expected inline finding, got %+v", rs[0].Findings)
		}
	})
}
