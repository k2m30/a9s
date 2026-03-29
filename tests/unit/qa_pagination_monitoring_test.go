package unit

// qa_pagination_monitoring_test.go — pagination tests for monitoring/messaging fetchers:
// alarm, logs, ddb, sqs, sns

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: CloudWatch DescribeAlarms (paginated)
// ---------------------------------------------------------------------------

type mockCloudWatchDescribeAlarmsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*cloudwatch.DescribeAlarmsOutput, error)
}

func (m *mockCloudWatchDescribeAlarmsAPIPaginated) DescribeAlarms(_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCloudWatchAlarmsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCloudWatchAlarmsPage_FirstPage(t *testing.T) {
	threshold := 90.0
	mock := &mockCloudWatchDescribeAlarmsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmsOutput, error) {
			return &cloudwatch.DescribeAlarmsOutput{
				MetricAlarms: []cwtypes.MetricAlarm{
					{
						AlarmName:  aws.String("cpu-high-alarm"),
						StateValue: cwtypes.StateValueAlarm,
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String("AWS/EC2"),
						Threshold:  &threshold,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "cpu-high-alarm" {
		t.Errorf("resource ID: expected %q, got %q", "cpu-high-alarm", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCloudWatchAlarmsPage_Continuation(t *testing.T) {
	threshold := 50.0
	mock := &mockCloudWatchDescribeAlarmsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmsOutput, error) {
			return &cloudwatch.DescribeAlarmsOutput{
				MetricAlarms: []cwtypes.MetricAlarm{
					{
						AlarmName:  aws.String("memory-alarm"),
						StateValue: cwtypes.StateValueOk,
						MetricName: aws.String("MemoryUtilization"),
						Namespace:  aws.String("CWAgent"),
						Threshold:  &threshold,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchCloudWatchAlarmsPage_Empty(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmsOutput, error) {
			return &cloudwatch.DescribeAlarmsOutput{
				MetricAlarms: []cwtypes.MetricAlarm{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCloudWatchAlarmsPage_Error(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmsOutput, error) {
			return nil, errors.New("describe alarms failed")
		},
	}

	_, err := awsclient.FetchCloudWatchAlarmsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudWatchLogs DescribeLogGroups (paginated)
// ---------------------------------------------------------------------------

type mockCWLogsDescribeLogGroupsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

func (m *mockCWLogsDescribeLogGroupsAPIPaginated) DescribeLogGroups(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCloudWatchLogGroupsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCloudWatchLogGroupsPage_FirstPage(t *testing.T) {
	storedBytes := int64(1048576) // 1 MB
	retentionDays := int32(30)
	creationTime := int64(1700000000000)
	mock := &mockCWLogsDescribeLogGroupsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwlogstypes.LogGroup{
					{
						LogGroupName:    aws.String("/aws/lambda/my-handler"),
						StoredBytes:     &storedBytes,
						RetentionInDays: &retentionDays,
						CreationTime:    &creationTime,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "/aws/lambda/my-handler" {
		t.Errorf("resource ID: expected %q, got %q", "/aws/lambda/my-handler", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCloudWatchLogGroupsPage_Continuation(t *testing.T) {
	storedBytes := int64(2097152) // 2 MB
	creationTime := int64(1700100000000)
	mock := &mockCWLogsDescribeLogGroupsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwlogstypes.LogGroup{
					{
						LogGroupName: aws.String("/aws/apigateway/my-api"),
						StoredBytes:  &storedBytes,
						CreationTime: &creationTime,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchCloudWatchLogGroupsPage_Empty(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwlogstypes.LogGroup{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCloudWatchLogGroupsPage_Error(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsAPIPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return nil, errors.New("describe log groups failed")
		},
	}

	_, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: DynamoDB ListTables + DescribeTable (paginated)
// ---------------------------------------------------------------------------

type mockDDBListTablesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*dynamodb.ListTablesOutput, error)
}

func (m *mockDDBListTablesAPIPaginated) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

type mockDDBDescribeTableAPIPaginated struct {
	Calls      int
	DescribeFunc func(call int, tableName string) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDDBDescribeTableAPIPaginated) DescribeTable(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	m.Calls++
	name := ""
	if input.TableName != nil {
		name = *input.TableName
	}
	return m.DescribeFunc(m.Calls, name)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchDynamoDBTablesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchDynamoDBTablesPage_FirstPage(t *testing.T) {
	itemCount := int64(1000)
	tableBytes := int64(512000)
	listMock := &mockDDBListTablesAPIPaginated{
		PageFunc: func(_ int) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{
				TableNames:             []string{"orders"},
				LastEvaluatedTableName: aws.String("orders"),
			}, nil
		},
	}
	describeMock := &mockDDBDescribeTableAPIPaginated{
		DescribeFunc: func(_ int, tableName string) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					TableName:     aws.String(tableName),
					TableStatus:   ddbtypes.TableStatusActive,
					ItemCount:     &itemCount,
					TableSizeBytes: &tableBytes,
					BillingModeSummary: &ddbtypes.BillingModeSummary{
						BillingMode: ddbtypes.BillingModePayPerRequest,
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listMock, describeMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with LastEvaluatedTableName")
	}
	if result.Pagination.NextToken != "orders" {
		t.Errorf("NextToken: expected %q, got %q", "orders", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "orders" {
		t.Errorf("resource ID: expected %q, got %q", "orders", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchDynamoDBTablesPage_Continuation(t *testing.T) {
	itemCount := int64(500)
	tableBytes := int64(256000)
	listMock := &mockDDBListTablesAPIPaginated{
		PageFunc: func(_ int) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{
				TableNames:             []string{"products"},
				LastEvaluatedTableName: nil,
			}, nil
		},
	}
	describeMock := &mockDDBDescribeTableAPIPaginated{
		DescribeFunc: func(_ int, tableName string) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					TableName:      aws.String(tableName),
					TableStatus:    ddbtypes.TableStatusActive,
					ItemCount:      &itemCount,
					TableSizeBytes: &tableBytes,
				},
			}, nil
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listMock, describeMock, "orders")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (LastEvaluatedTableName=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchDynamoDBTablesPage_Empty(t *testing.T) {
	listMock := &mockDDBListTablesAPIPaginated{
		PageFunc: func(_ int) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{
				TableNames:             []string{},
				LastEvaluatedTableName: nil,
			}, nil
		},
	}
	describeMock := &mockDDBDescribeTableAPIPaginated{
		DescribeFunc: func(_ int, _ string) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{Table: &ddbtypes.TableDescription{}}, nil
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listMock, describeMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchDynamoDBTablesPage_Error(t *testing.T) {
	listMock := &mockDDBListTablesAPIPaginated{
		PageFunc: func(_ int) (*dynamodb.ListTablesOutput, error) {
			return nil, errors.New("list tables failed")
		},
	}
	describeMock := &mockDDBDescribeTableAPIPaginated{
		DescribeFunc: func(_ int, _ string) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{Table: &ddbtypes.TableDescription{}}, nil
		},
	}

	_, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listMock, describeMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SQS ListQueues + GetQueueAttributes (paginated)
// ---------------------------------------------------------------------------

type mockSQSListQueuesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*sqs.ListQueuesOutput, error)
}

func (m *mockSQSListQueuesAPIPaginated) ListQueues(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

type mockSQSGetQueueAttributesAPIPaginated struct {
	Calls    int
	AttrFunc func(call int, queueURL string) (*sqs.GetQueueAttributesOutput, error)
}

func (m *mockSQSGetQueueAttributesAPIPaginated) GetQueueAttributes(_ context.Context, input *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	m.Calls++
	url := ""
	if input.QueueUrl != nil {
		url = *input.QueueUrl
	}
	return m.AttrFunc(m.Calls, url)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSQSQueuesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSQSQueuesPage_FirstPage(t *testing.T) {
	listMock := &mockSQSListQueuesAPIPaginated{
		PageFunc: func(_ int) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{
				QueueUrls: []string{"https://sqs.us-east-1.amazonaws.com/111111111111/my-queue"},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}
	attrMock := &mockSQSGetQueueAttributesAPIPaginated{
		AttrFunc: func(_ int, _ string) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "42",
					"ApproximateNumberOfMessagesNotVisible": "5",
					"DelaySeconds":                          "0",
				},
			}, nil
		},
	}

	result, err := awsclient.FetchSQSQueuesPage(context.Background(), listMock, attrMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-queue" {
		t.Errorf("resource ID: expected %q, got %q", "my-queue", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSQSQueuesPage_Continuation(t *testing.T) {
	listMock := &mockSQSListQueuesAPIPaginated{
		PageFunc: func(_ int) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{
				QueueUrls: []string{"https://sqs.us-east-1.amazonaws.com/111111111111/another-queue"},
				NextToken: nil,
			}, nil
		},
	}
	attrMock := &mockSQSGetQueueAttributesAPIPaginated{
		AttrFunc: func(_ int, _ string) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "0",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"DelaySeconds":                          "10",
				},
			}, nil
		},
	}

	result, err := awsclient.FetchSQSQueuesPage(context.Background(), listMock, attrMock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchSQSQueuesPage_Empty(t *testing.T) {
	listMock := &mockSQSListQueuesAPIPaginated{
		PageFunc: func(_ int) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{
				QueueUrls: []string{},
				NextToken: nil,
			}, nil
		},
	}
	attrMock := &mockSQSGetQueueAttributesAPIPaginated{
		AttrFunc: func(_ int, _ string) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{}}, nil
		},
	}

	result, err := awsclient.FetchSQSQueuesPage(context.Background(), listMock, attrMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchSQSQueuesPage_Error(t *testing.T) {
	listMock := &mockSQSListQueuesAPIPaginated{
		PageFunc: func(_ int) (*sqs.ListQueuesOutput, error) {
			return nil, errors.New("list queues failed")
		},
	}
	attrMock := &mockSQSGetQueueAttributesAPIPaginated{
		AttrFunc: func(_ int, _ string) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{}}, nil
		},
	}

	_, err := awsclient.FetchSQSQueuesPage(context.Background(), listMock, attrMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SNS ListTopics (paginated)
// ---------------------------------------------------------------------------

type mockSNSListTopicsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*sns.ListTopicsOutput, error)
}

func (m *mockSNSListTopicsAPIPaginated) ListTopics(_ context.Context, _ *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSNSTopicsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSNSTopicsPage_FirstPage(t *testing.T) {
	mock := &mockSNSListTopicsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListTopicsOutput, error) {
			return &sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:aws:sns:us-east-1:111111111111:my-alerts")},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSNSTopicsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	// ID is the full ARN
	if result.Resources[0].ID != "arn:aws:sns:us-east-1:111111111111:my-alerts" {
		t.Errorf("resource ID: expected %q, got %q", "arn:aws:sns:us-east-1:111111111111:my-alerts", result.Resources[0].ID)
	}
	// Name is extracted from the last segment of the ARN
	if result.Resources[0].Name != "my-alerts" {
		t.Errorf("resource Name: expected %q, got %q", "my-alerts", result.Resources[0].Name)
	}
}

func TestQA_Pagination_FetchSNSTopicsPage_Continuation(t *testing.T) {
	mock := &mockSNSListTopicsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListTopicsOutput, error) {
			return &sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:aws:sns:us-east-1:111111111111:order-events")},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSNSTopicsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchSNSTopicsPage_Empty(t *testing.T) {
	mock := &mockSNSListTopicsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListTopicsOutput, error) {
			return &sns.ListTopicsOutput{
				Topics:    []snstypes.Topic{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSNSTopicsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchSNSTopicsPage_Error(t *testing.T) {
	mock := &mockSNSListTopicsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListTopicsOutput, error) {
			return nil, errors.New("list topics failed")
		},
	}

	_, err := awsclient.FetchSNSTopicsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
