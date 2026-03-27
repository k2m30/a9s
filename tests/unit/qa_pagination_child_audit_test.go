package unit

// QA Stories B.3 (Load More for Specific Child Views) and M (Fetcher Pagination Audit).
//
// Section B.3: For each child fetcher, verifies the two-call Load More flow:
//   1. Call with continuationToken="" → gets page 1 with IsTruncated=true
//   2. Call with NextToken from page 1 → gets page 2 with IsTruncated=false
//   3. Verify total items = page1 + page2
//
// Section M: Verifies that all Phase 4b child fetchers are registered as
// PaginatedChildFetcher via resource.GetPaginatedChildFetcher(shortName).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// Section B.3: Load More for Specific Child Views
// ===========================================================================

// TestStory_B3_SFNExecutions_LoadMore verifies the two-call Load More flow
// for SFN executions. The mock returns enough items to exceed the max cap
// (200), forcing IsTruncated=true on call 1. Call 2 with the continuation
// token returns the remaining items.
func TestStory_B3_SFNExecutions_LoadMore(t *testing.T) {
	const maxCap = 200
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 5, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 items each (250 total). Cap is 200.
	// Page 1-4 each have NextToken, page 5 does not.
	var outputs []*sfn.ListExecutionsOutput
	for page := 0; page < 5; page++ {
		var executions []sfntypes.ExecutionListItem
		for i := 0; i < 50; i++ {
			executions = append(executions, sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String(fmt.Sprintf("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p%d-%d", page, i)),
				Name:            aws.String(fmt.Sprintf("exec-p%d-%d", page, i)),
				StartDate:       &startTs,
				StopDate:        &stopTs,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
				Status:          sfntypes.ExecutionStatusSucceeded,
			})
		}
		out := &sfn.ListExecutionsOutput{Executions: executions}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	// Call 1: continuationToken="" — should get 200 items with IsTruncated=true
	mock := &mockSFNListExecutionsClient{outputs: outputs}
	result1, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
		if result1.Pagination.NextToken == "" {
			t.Error("call 1: expected non-empty NextToken")
		}
	})

	// Call 2: use NextToken from call 1 — should get remaining items
	// Reset mock for the second call (the continuation mock delivers remaining pages)
	mock2 := &mockSFNListExecutionsClient{outputs: outputs[4:]} // page 5 only (50 items)
	result2, err := awsclient.FetchSFNExecutions(context.Background(), mock2, parentCtx, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_LogStreams_LoadMore verifies the two-call Load More flow for
// CloudWatch Log Streams (max cap 500).
func TestStory_B3_LogStreams_LoadMore(t *testing.T) {
	const maxCap = 500

	// Build mock: 12 pages of 50 streams each (600 total). Cap is 500.
	var outputs []*cloudwatchlogs.DescribeLogStreamsOutput
	for page := 0; page < 12; page++ {
		var streams []cwlogstypes.LogStream
		for i := 0; i < 50; i++ {
			streams = append(streams, cwlogstypes.LogStream{
				LogStreamName:      aws.String(fmt.Sprintf("stream-p%d-s%d", page, i)),
				LastEventTimestamp: aws.Int64(1711065600000),
			})
		}
		out := &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: streams}
		if page < 11 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	// Call 1: should get 500 items (10 pages consumed) with IsTruncated=true
	mock := &mockCWLogsDescribeLogStreamsClient{outputs: outputs}
	result1, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
		if result1.Pagination.NextToken == "" {
			t.Error("call 1: expected non-empty NextToken")
		}
	})

	// Call 2: use the continuation token — get remaining 100 items
	mock2 := &mockCWLogsDescribeLogStreamsClient{outputs: outputs[10:]}
	result2, err := awsclient.FetchLogStreams(context.Background(), mock2, "/aws/lambda/my-func", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 100 {
			t.Errorf("call 2: expected 100 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 600 {
			t.Errorf("expected total 600 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_AsgActivities_LoadMore verifies the two-call Load More flow
// for ASG scaling activities (max cap 200).
func TestStory_B3_AsgActivities_LoadMore(t *testing.T) {
	const maxCap = 200
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 activities each (250 total). Cap is 200.
	var outputs []*autoscaling.DescribeScalingActivitiesOutput
	for page := 0; page < 5; page++ {
		var activities []asgtypes.Activity
		for i := 0; i < 50; i++ {
			activities = append(activities, asgtypes.Activity{
				ActivityId:           aws.String(fmt.Sprintf("act-p%d-%d", page, i)),
				AutoScalingGroupName: aws.String("test-asg"),
				Cause:                aws.String("Scaling event"),
				StartTime:            &ts,
				StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
			})
		}
		out := &autoscaling.DescribeScalingActivitiesOutput{Activities: activities}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	parentCtx := map[string]string{"asg_name": "test-asg"}

	// Call 1
	mock := &mockASGDescribeScalingActivitiesClient{outputs: outputs}
	result1, err := awsclient.FetchAsgActivities(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockASGDescribeScalingActivitiesClient{outputs: outputs[4:]}
	result2, err := awsclient.FetchAsgActivities(context.Background(), mock2, parentCtx, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_CBBuilds_LoadMore verifies the two-call Load More flow for
// CodeBuild builds (max cap 200). This fetcher uses two APIs: ListBuildsForProject
// to get IDs, then BatchGetBuilds to get details.
func TestStory_B3_CBBuilds_LoadMore(t *testing.T) {
	const maxCap = 200
	startTs := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 build IDs each (250 total). Cap is 200.
	var listOutputs []*codebuild.ListBuildsForProjectOutput
	for page := 0; page < 5; page++ {
		var ids []string
		for i := 0; i < 50; i++ {
			ids = append(ids, fmt.Sprintf("my-project:build-p%d-%d", page, i))
		}
		out := &codebuild.ListBuildsForProjectOutput{Ids: ids}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		listOutputs = append(listOutputs, out)
	}

	// BatchGetBuilds mock: returns build details for each batch of IDs
	buildNum := int64(1)
	batchMock := &b3CBBatchGetBuildsMock{
		startTs:  &startTs,
		endTs:    &endTs,
		buildNum: &buildNum,
	}

	parentCtx := map[string]string{"project_name": "my-project"}

	// Call 1
	listMock := &mockCodeBuildListBuildsForProjectClient{outputs: listOutputs}
	result1, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, parentCtx, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2: use continuation token
	listMock2 := &mockCodeBuildListBuildsForProjectClient{outputs: listOutputs[4:]}
	batchMock2 := &b3CBBatchGetBuildsMock{
		startTs:  &startTs,
		endTs:    &endTs,
		buildNum: &buildNum,
	}
	result2, err := awsclient.FetchCBBuilds(context.Background(), listMock2, batchMock2, parentCtx, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// b3CBBatchGetBuildsMock returns build details for any IDs requested.
// It dynamically generates build objects matching the input IDs.
type b3CBBatchGetBuildsMock struct {
	startTs  *time.Time
	endTs    *time.Time
	buildNum *int64
}

func (m *b3CBBatchGetBuildsMock) BatchGetBuilds(ctx context.Context, params *codebuild.BatchGetBuildsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	var builds []cbtypes.Build
	for _, id := range params.Ids {
		num := *m.buildNum
		*m.buildNum++
		builds = append(builds, cbtypes.Build{
			Id:          aws.String(id),
			Arn:         aws.String(fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s", id)),
			BuildNumber: &num,
			BuildStatus: cbtypes.StatusTypeSucceeded,
			StartTime:   m.startTs,
			EndTime:     m.endTs,
			Initiator:   aws.String("codepipeline/my-pipeline"),
		})
	}
	return &codebuild.BatchGetBuildsOutput{Builds: builds}, nil
}

// TestStory_B3_GlueJobRuns_LoadMore verifies the two-call Load More flow
// for Glue job runs (max cap 200).
func TestStory_B3_GlueJobRuns_LoadMore(t *testing.T) {
	const maxCap = 200
	startTs := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 runs each (250 total). Cap is 200.
	var outputs []*glue.GetJobRunsOutput
	for page := 0; page < 5; page++ {
		var runs []gluetypes.JobRun
		for i := 0; i < 50; i++ {
			runs = append(runs, gluetypes.JobRun{
				Id:            aws.String(fmt.Sprintf("jr_p%d_%d", page, i)),
				JobName:       aws.String("test-etl-job"),
				JobRunState:   gluetypes.JobRunStateSucceeded,
				StartedOn:     &startTs,
				CompletedOn:   &endTs,
				ExecutionTime: 300,
			})
		}
		out := &glue.GetJobRunsOutput{JobRuns: runs}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	// Call 1
	mock := &mockGlueGetJobRunsClient{outputs: outputs}
	result1, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "test-etl-job", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockGlueGetJobRunsClient{outputs: outputs[4:]}
	result2, err := awsclient.FetchGlueJobRuns(context.Background(), mock2, "test-etl-job", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_AlarmHistory_LoadMore verifies the two-call Load More flow
// for CloudWatch alarm history (max cap 200).
func TestStory_B3_AlarmHistory_LoadMore(t *testing.T) {
	const maxCap = 200
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 items each (250 total). Cap is 200.
	var outputs []*cloudwatch.DescribeAlarmHistoryOutput
	for page := 0; page < 5; page++ {
		var items []cwtypes.AlarmHistoryItem
		for i := 0; i < 50; i++ {
			items = append(items, cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("test-alarm"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
				HistorySummary:  aws.String(fmt.Sprintf("Alarm transitioned event p%d-%d", page, i)),
				Timestamp:       &ts,
			})
		}
		out := &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: items}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	parentCtx := map[string]string{"alarm_name": "test-alarm"}

	// Call 1
	mock := &mockCloudWatchDescribeAlarmHistoryClient{outputs: outputs}
	result1, err := awsclient.FetchAlarmHistory(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockCloudWatchDescribeAlarmHistoryClient{outputs: outputs[4:]}
	result2, err := awsclient.FetchAlarmHistory(context.Background(), mock2, parentCtx, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_ECRImages_LoadMore verifies the two-call Load More flow
// for ECR images (max cap 500).
func TestStory_B3_ECRImages_LoadMore(t *testing.T) {
	const maxCap = 500
	pushTime := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Build mock: 12 pages of 50 images each (600 total). Cap is 500.
	var pages []*ecr.DescribeImagesOutput
	for page := 0; page < 12; page++ {
		var images []ecrtypes.ImageDetail
		for i := 0; i < 50; i++ {
			pt := pushTime.Add(time.Duration(-(page*50 + i)) * time.Second) // Descending push times
			images = append(images, ecrtypes.ImageDetail{
				ImageDigest:   aws.String(fmt.Sprintf("sha256:p%d-img%d", page, i)),
				ImageTags:     []string{fmt.Sprintf("v%d.%d", page, i)},
				ImagePushedAt: &pt,
				ImageSizeInBytes: aws.Int64(int64((page*50 + i + 1) * 1024)),
			})
		}
		out := &ecr.DescribeImagesOutput{ImageDetails: images}
		if page < 11 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		pages = append(pages, out)
	}

	parentCtx := map[string]string{
		"repository_name": "test-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/test-repo",
	}

	// Call 1
	mock := &mockECRDescribeImagesClient{pages: pages}
	result1, err := awsclient.FetchECRImages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockECRDescribeImagesClient{pages: pages[10:]}
	result2, err := awsclient.FetchECRImages(context.Background(), mock2, parentCtx, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 100 {
			t.Errorf("call 2: expected 100 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 600 {
			t.Errorf("expected total 600 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_EcsSvcLogs_LoadMore verifies the two-call Load More flow
// for ECS service logs (max cap 200). This is a cross-service fetcher
// that first calls DescribeTaskDefinition, then FilterLogEvents.
func TestStory_B3_EcsSvcLogs_LoadMore(t *testing.T) {
	const maxCap = 200

	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/web-service",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	// Build mock: 5 pages of 50 log events each (250 total). Cap is 200.
	var logOutputs []*cloudwatchlogs.FilterLogEventsOutput
	for page := 0; page < 5; page++ {
		var events []cwlogstypes.FilteredLogEvent
		for i := 0; i < 50; i++ {
			events = append(events, cwlogstypes.FilteredLogEvent{
				EventId:       aws.String(fmt.Sprintf("event-p%d-%d", page, i)),
				Timestamp:     aws.Int64(1711065600000 + int64(page*50+i)*1000),
				LogStreamName: aws.String(fmt.Sprintf("ecs/web/task-p%d", page)),
				Message:       aws.String(fmt.Sprintf("Log message p%d-%d", page, i)),
			})
		}
		out := &cloudwatchlogs.FilterLogEventsOutput{Events: events}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		logOutputs = append(logOutputs, out)
	}

	// Call 1
	logMock := &mockCWLogsFilterLogEventsClient{outputs: logOutputs}
	result1, err := awsclient.FetchEcsSvcLogs(
		context.Background(), taskDefMock, logMock,
		"my-cluster", "web-service", "arn:aws:ecs:us-east-1:123456789012:task-definition/web:1", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	logMock2 := &mockCWLogsFilterLogEventsClient{outputs: logOutputs[4:]}
	result2, err := awsclient.FetchEcsSvcLogs(
		context.Background(), taskDefMock, logMock2,
		"my-cluster", "web-service", "arn:aws:ecs:us-east-1:123456789012:task-definition/web:1",
		result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_RDSEvents_LoadMore verifies the two-call Load More flow
// for RDS instance events (max cap 200). RDS uses Marker instead of NextToken.
func TestStory_B3_RDSEvents_LoadMore(t *testing.T) {
	const maxCap = 200
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Build mock: 5 pages of 50 events each (250 total). Cap is 200.
	var outputs []*rds.DescribeEventsOutput
	for page := 0; page < 5; page++ {
		var events []rdstypes.Event
		for i := 0; i < 50; i++ {
			events = append(events, rdstypes.Event{
				SourceIdentifier: aws.String("my-db-instance"),
				SourceType:       rdstypes.SourceTypeDbInstance,
				Message:          aws.String(fmt.Sprintf("Event message p%d-%d", page, i)),
				Date:             &ts,
			})
		}
		out := &rds.DescribeEventsOutput{Events: events}
		if page < 4 {
			out.Marker = aws.String(fmt.Sprintf("marker-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	// Call 1
	mock := &mockRDSDescribeEventsClient{outputs: outputs}
	result1, err := awsclient.FetchRDSEvents(context.Background(), mock, "my-db-instance", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockRDSDescribeEventsClient{outputs: outputs[4:]}
	result2, err := awsclient.FetchRDSEvents(context.Background(), mock2, "my-db-instance", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_SNSTopicSubscriptions_LoadMore verifies the two-call Load More
// flow for SNS topic subscriptions (max cap 200).
func TestStory_B3_SNSTopicSubscriptions_LoadMore(t *testing.T) {
	const maxCap = 200
	topicArn := "arn:aws:sns:us-east-1:123456789012:test-topic"

	// Build mock: 5 pages of 50 subscriptions each (250 total). Cap is 200.
	var outputs []*sns.ListSubscriptionsByTopicOutput
	for page := 0; page < 5; page++ {
		var subs []snstypes.Subscription
		for i := 0; i < 50; i++ {
			subs = append(subs, snstypes.Subscription{
				SubscriptionArn: aws.String(fmt.Sprintf("arn:aws:sns:us-east-1:123456789012:test-topic:sub-p%d-%d", page, i)),
				TopicArn:        aws.String(topicArn),
				Protocol:        aws.String("email"),
				Endpoint:        aws.String(fmt.Sprintf("user-p%d-%d@example.com", page, i)),
				Owner:           aws.String("123456789012"),
			})
		}
		out := &sns.ListSubscriptionsByTopicOutput{Subscriptions: subs}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	// Call 1
	mock := &mockSNSListSubscriptionsByTopicClient{outputs: outputs}
	result1, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, topicArn, "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) != maxCap {
			t.Errorf("call 1: expected %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockSNSListSubscriptionsByTopicClient{outputs: outputs[4:]}
	result2, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock2, topicArn, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_count", func(t *testing.T) {
		if len(result2.Resources) != 50 {
			t.Errorf("call 2: expected 50 resources, got %d", len(result2.Resources))
		}
	})

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		if total != 250 {
			t.Errorf("expected total 250 items across 2 calls, got %d", total)
		}
	})
}

// TestStory_B3_LambdaInvocations_LoadMore verifies the two-call Load More flow
// for Lambda invocations (max cap 50). Uses REPORT lines from CW Logs.
func TestStory_B3_LambdaInvocations_LoadMore(t *testing.T) {
	const maxCap = 50

	// Build mock: 2 pages of 40 REPORT events each (80 total). Cap is 50.
	var outputs []*cloudwatchlogs.FilterLogEventsOutput
	for page := 0; page < 2; page++ {
		var events []cwlogstypes.FilteredLogEvent
		for i := 0; i < 40; i++ {
			events = append(events, cwlogstypes.FilteredLogEvent{
				EventId:   aws.String(fmt.Sprintf("evt-p%d-%d", page, i)),
				Timestamp: aws.Int64(1711065600000 + int64(page*40+i)*1000),
				Message: aws.String(fmt.Sprintf(
					"REPORT RequestId: req-p%d-%d\tDuration: %d.00 ms\tBilled Duration: %d ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB",
					page, i, 100+i, 100+i)),
			})
		}
		out := &cloudwatchlogs.FilterLogEventsOutput{Events: events}
		if page < 1 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	// Call 1
	mock := &mockCWLogsFilterLogEventsClient{outputs: outputs}
	result1, err := awsclient.FetchLambdaInvocations(
		context.Background(), mock, "my-function", "/aws/lambda/my-function", "")
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}

	t.Run("call1_count", func(t *testing.T) {
		if len(result1.Resources) < maxCap {
			t.Errorf("call 1: expected at least %d resources, got %d", maxCap, len(result1.Resources))
		}
	})

	t.Run("call1_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("call 1: Pagination is nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("call 1: expected IsTruncated=true")
		}
	})

	// Call 2
	mock2 := &mockCWLogsFilterLogEventsClient{outputs: outputs[1:]}
	result2, err := awsclient.FetchLambdaInvocations(
		context.Background(), mock2, "my-function", "/aws/lambda/my-function",
		result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}

	t.Run("call2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("call 2: Pagination is nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("call 2: expected IsTruncated=false")
		}
	})

	t.Run("total_items_both_calls", func(t *testing.T) {
		total := len(result1.Resources) + len(result2.Resources)
		// With 80 total REPORT events and cap=50, we should get around 80 total
		if total < 60 {
			t.Errorf("expected at least 60 total items across 2 calls, got %d", total)
		}
	})
}

// ===========================================================================
// Section M: Fetcher Pagination Audit — PaginatedChildFetcher Registration
// ===========================================================================

// TestStory_M_PaginatedChildFetcher_Registration verifies that all child
// fetchers migrated to PaginatedChildFetcher in Phase 4b are properly
// registered via resource.GetPaginatedChildFetcher(shortName).
func TestStory_M_PaginatedChildFetcher_Registration(t *testing.T) {
	// Phase 4b child fetchers
	phase4bFetchers := []struct {
		shortName   string
		displayName string
	}{
		{"cfn_events", "CFN Events"},
		{"cfn_resources", "CFN Resources"},
		{"elb_listeners", "ELB Listeners"},
		{"r53_records", "R53 Records"},
		{"role_policies", "Role Policies"},
		{"s3_objects", "S3 Objects"},
		{"sfn_execution_history", "SFN Execution History"},
	}

	for _, tc := range phase4bFetchers {
		t.Run(tc.displayName, func(t *testing.T) {
			fetcher := resource.GetPaginatedChildFetcher(tc.shortName)
			if fetcher == nil {
				t.Errorf("GetPaginatedChildFetcher(%q) returned nil — %s is not registered as PaginatedChildFetcher", tc.shortName, tc.displayName)
			}
		})
	}
}

// TestStory_M_AllB3ChildFetchers_AreRegistered verifies that all B.3 child
// fetchers are also registered as PaginatedChildFetcher.
func TestStory_M_AllB3ChildFetchers_AreRegistered(t *testing.T) {
	b3Fetchers := []struct {
		shortName   string
		displayName string
	}{
		{"sfn_executions", "SFN Executions"},
		{"log_streams", "Log Streams"},
		{"asg_activities", "ASG Activities"},
		{"cb_builds", "CodeBuild Builds"},
		{"glue_runs", "Glue Job Runs"},
		{"alarm_history", "Alarm History"},
		{"ecr_images", "ECR Images"},
		{"ecs_svc_logs", "ECS Service Logs"},
		{"dbi_events", "RDS Events"},
		{"sns_subscriptions", "SNS Topic Subscriptions"},
		{"lambda_invocations", "Lambda Invocations"},
	}

	for _, tc := range b3Fetchers {
		t.Run(tc.displayName, func(t *testing.T) {
			fetcher := resource.GetPaginatedChildFetcher(tc.shortName)
			if fetcher == nil {
				t.Errorf("GetPaginatedChildFetcher(%q) returned nil — %s is not registered as PaginatedChildFetcher", tc.shortName, tc.displayName)
			}
		})
	}
}

// TestStory_M_PaginatedChildFetcher_NotNilResult verifies that calling a
// paginated child fetcher with nil clients returns an error (not a panic).
func TestStory_M_PaginatedChildFetcher_NotNilResult(t *testing.T) {
	allFetchers := []string{
		"cfn_events", "cfn_resources", "elb_listeners", "r53_records",
		"role_policies", "s3_objects", "sfn_execution_history",
		"sfn_executions", "log_streams", "asg_activities", "cb_builds",
		"glue_runs", "alarm_history", "ecr_images", "ecs_svc_logs",
		"dbi_events", "sns_subscriptions", "lambda_invocations",
	}

	for _, shortName := range allFetchers {
		t.Run(shortName, func(t *testing.T) {
			fetcher := resource.GetPaginatedChildFetcher(shortName)
			if fetcher == nil {
				t.Fatalf("GetPaginatedChildFetcher(%q) returned nil", shortName)
			}

			// Calling with nil clients should return an error, not panic
			_, err := fetcher(context.Background(), nil, resource.ParentContext{}, "")
			if err == nil {
				t.Errorf("expected error when calling %q with nil clients, got nil", shortName)
			}
		})
	}
}
