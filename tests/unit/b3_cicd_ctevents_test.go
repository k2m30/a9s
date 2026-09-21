// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cbSortOrderFake records the ListBuildsForProject input so a test can assert
// what the request carried.
type cbSortOrderFake struct {
	awsclient.CodeBuildAPI
	lastList *codebuild.ListBuildsForProjectInput
}

func (f *cbSortOrderFake) ListBuildsForProject(
	_ context.Context,
	params *codebuild.ListBuildsForProjectInput,
	_ ...func(*codebuild.Options),
) (*codebuild.ListBuildsForProjectOutput, error) {
	f.lastList = params
	return &codebuild.ListBuildsForProjectOutput{}, nil
}

// TestCBListBuildsSendsNoSortOrder pins that the request omits SortOrder.
// CodeBuild rejects ListBuildsForProject with a sort order once a project has
// more than 100 builds, and descending is the order it answers in anyway.
func TestCBListBuildsSendsNoSortOrder(t *testing.T) {
	fake := &cbSortOrderFake{}
	_, err := awsclient.EnrichCodeBuildStatus(context.Background(),
		&awsclient.ServiceClients{CodeBuild: fake}, []resource.Resource{{ID: "busy-project"}}, nil)
	if err != nil {
		t.Fatalf("EnrichCodeBuildStatus: %v", err)
	}
	if fake.lastList == nil {
		t.Fatal("ListBuildsForProject was never called")
	}
	if fake.lastList.SortOrder != "" {
		t.Errorf("SortOrder = %q, want it unset", fake.lastList.SortOrder)
	}
}

// TestCBRunningBuildIsNotOK pins that a build still running, and one that was
// cancelled, report their own status instead of claiming success.
func TestCBRunningBuildIsNotOK(t *testing.T) {
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{"api": "api:1", "web": "web:1"},
		builds: map[string]cbtypes.Build{
			"api:1": {Id: aws.String("api:1"), BuildStatus: cbtypes.StatusTypeInProgress},
			"web:1": {Id: aws.String("web:1"), BuildStatus: cbtypes.StatusTypeStopped},
		},
	}
	res, err := awsclient.EnrichCodeBuildStatus(context.Background(),
		&awsclient.ServiceClients{CodeBuild: fake},
		[]resource.Resource{{ID: "api"}, {ID: "web"}}, nil)
	if err != nil {
		t.Fatalf("EnrichCodeBuildStatus: %v", err)
	}
	for id, want := range map[string]string{"api": "IN_PROGRESS", "web": "STOPPED"} {
		if got := res.FieldUpdates[id]["last_build"]; got != want {
			t.Errorf("%s last_build = %q, want %q", id, got, want)
		}
	}
}

// TestCBTimedOutBuildNamesTheBreakingPhase pins that a build ending on
// TIMED_OUT names the phase that ended it, and that the completion date is
// reported once.
func TestCBTimedOutBuildNamesTheBreakingPhase(t *testing.T) {
	end := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{"slow": "slow:1"},
		builds: map[string]cbtypes.Build{
			"slow:1": {
				Id:            aws.String("slow:1"),
				BuildStatus:   cbtypes.StatusTypeTimedOut,
				BuildComplete: true,
				EndTime:       &end,
				Phases: []cbtypes.BuildPhase{
					{PhaseType: cbtypes.BuildPhaseTypeDownloadSource, PhaseStatus: cbtypes.StatusTypeSucceeded},
					{PhaseType: cbtypes.BuildPhaseTypeBuild, PhaseStatus: cbtypes.StatusTypeTimedOut},
					{PhaseType: cbtypes.BuildPhaseTypeCompleted},
				},
			},
		},
	}
	res, err := awsclient.EnrichCodeBuildStatus(context.Background(),
		&awsclient.ServiceClients{CodeBuild: fake}, []resource.Resource{{ID: "slow"}}, nil)
	if err != nil {
		t.Fatalf("EnrichCodeBuildStatus: %v", err)
	}
	fs := res.Findings["slow"]
	if len(fs) == 0 {
		t.Fatal("a timed-out build produced no finding")
	}
	rows := res.AttentionDetails["slow"][fs[0].Code].Rows
	if !slices.ContainsFunc(rows, func(r domain.DetailRow) bool { return r.Label == "Phase" }) {
		t.Errorf("no Phase row naming what broke; got %v", rows)
	}
	ended := 0
	for _, r := range rows {
		if r.Label == "Ended" {
			ended++
		}
	}
	if ended != 1 {
		t.Errorf("the completion date is on %d rows, want 1: %v", ended, rows)
	}
}

// TestCBGitLabProjectGetsTheBuildspecFinding pins that a GitLab-backed project
// is covered by the contributor-controlled-buildspec signal: a pull-request
// author can change the file there exactly as on GitHub.
func TestCBGitLabProjectGetsTheBuildspecFinding(t *testing.T) {
	for _, src := range []cbtypes.SourceType{cbtypes.SourceTypeGitlab, cbtypes.SourceTypeGitlabSelfManaged} {
		t.Run(string(src), func(t *testing.T) {
			listFake := &fakeCodeBuildListProjects{
				Output: &codebuild.ListProjectsOutput{Projects: []string{"gl-project"}},
			}
			batchFake := &fakeCodeBuildBatchGetProjects{Output: &codebuild.BatchGetProjectsOutput{
				Projects: []cbtypes.Project{{
					Name:   aws.String("gl-project"),
					Source: &cbtypes.ProjectSource{Type: src, Location: aws.String("https://gitlab.com/acme/app.git")},
				}},
			}}
			out, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), listFake, batchFake, "")
			if err != nil {
				t.Fatalf("FetchCodeBuildProjectsPage: %v", err)
			}
			if len(out.Resources) != 1 {
				t.Fatalf("expected 1 row, got %d", len(out.Resources))
			}
			if len(out.Resources[0].Findings) == 0 {
				t.Errorf("a %s-backed project raised no buildspec finding", src)
			}
		})
	}
}

// TestCTOriginIsEmptyWhenNothingNamesIt pins that a record carrying no user
// agent leaves the ORIGIN cell blank rather than showing a glyph that reads
// as an origin of its own.
func TestCTOriginIsEmptyWhenNothingNamesIt(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","accountId":"123456789012"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"123456789012"}`
	event := cloudtrailtypes.Event{
		EventId:         aws.String("no-ua-01"),
		EventName:       aws.String("GetObject"),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(ctJSON),
	}
	res, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage: %v", err)
	}
	if got := res.Resources[0].Fields["_ct.origin"]; got != "" {
		t.Errorf("_ct.origin = %q, want it empty", got)
	}
}

// TestCTOriginReadsTheTopLevelConsoleFlag pins that CloudTrail's own
// placement of sessionCredentialFromConsole, at the top of the record, marks
// the call as coming from the console.
func TestCTOriginReadsTheTopLevelConsoleFlag(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"123456789012"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","userAgent":"aws-sdk-java/1.0","eventCategory":"Management"` +
		`,"eventType":"AwsApiCall","sessionCredentialFromConsole":"true"` +
		`,"recipientAccountId":"123456789012"}`
	event := cloudtrailtypes.Event{
		EventId:         aws.String("console-01"),
		EventName:       aws.String("GetObject"),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(ctJSON),
	}
	res, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage: %v", err)
	}
	if got := res.Resources[0].Fields["_ct.origin"]; got != "Console" {
		t.Errorf("_ct.origin = %q, want %q", got, "Console")
	}
}

// secretsBinaryFake answers GetSecretValue with a binary payload.
type secretsBinaryFake struct{}

func (secretsBinaryFake) GetSecretValue(
	_ context.Context,
	_ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	return &secretsmanager.GetSecretValueOutput{SecretBinary: []byte{0x00, 0x01, 0x02, 0x03}}, nil
}

// TestRevealBinarySecretSaysSo pins that revealing a binary secret says what
// it holds rather than rendering as an empty value.
func TestRevealBinarySecretSaysSo(t *testing.T) {
	got, err := awsclient.RevealSecret(context.Background(), secretsBinaryFake{}, "prod/keystore")
	if err != nil {
		t.Fatalf("RevealSecret: %v", err)
	}
	if got != "binary secret (4 bytes)" {
		t.Errorf("RevealSecret = %q, want %q", got, "binary secret (4 bytes)")
	}
}

// TestCBBuildLogTimestampsCarrySeconds pins that two events logged in the
// same minute are told apart on screen.
func TestCBBuildLogTimestampsCarrySeconds(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1718445600000), Message: aws.String("phase start")},
				{Timestamp: aws.Int64(1718445617000), Message: aws.String("phase end")},
			},
		},
	}
	out, err := awsclient.FetchCBBuildLogs(context.Background(), mock, "/aws/codebuild/p", "stream", "")
	if err != nil {
		t.Fatalf("FetchCBBuildLogs: %v", err)
	}
	first, second := out.Resources[0].Fields["timestamp"], out.Resources[1].Fields["timestamp"]
	if first == second {
		t.Errorf("two events 17 seconds apart share the timestamp %q", first)
	}
	if !strings.HasSuffix(first, ":00") || !strings.HasSuffix(second, ":17") {
		t.Errorf("timestamps = %q, %q; want them to carry seconds", first, second)
	}
}

// TestCBBuildLogsNeedAStream pins that a build with no log stream cannot be
// drilled into: GetLogEvents refuses an empty stream name.
func TestCBBuildLogsNeedAStream(t *testing.T) {
	rt := resource.GetChildType("cb_builds")
	if rt == nil {
		t.Fatal("cb_builds child type not found")
	}
	var drill func(domain.Resource) bool
	for i := range rt.Children {
		if rt.Children[i].ChildType == "cb_build_logs" {
			drill = rt.Children[i].DrillCondition
		}
	}
	if drill == nil {
		t.Fatal("the cb_build_logs child view is not registered with a drill condition")
	}
	noStream := domain.Resource{Fields: map[string]string{"log_group_name": "/aws/codebuild/p"}}
	if drill(noStream) {
		t.Error("a build with a log group but no stream was offered its logs")
	}
	withStream := domain.Resource{Fields: map[string]string{"log_group_name": "/aws/codebuild/p", "log_stream_name": "s"}}
	if !drill(withStream) {
		t.Error("a build with both a log group and a stream was refused its logs")
	}
}
