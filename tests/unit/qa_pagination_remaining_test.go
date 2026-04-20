package unit

// qa_pagination_remaining_test.go — pagination tests for remaining fetchers:
// eventbridge, kinesis, msk, sfn, sns-sub, glue, athena, redshift, backup, ses, waf

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: EventBridge ListRules (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockEventBridgeListRulesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*eventbridge.ListRulesOutput, error)
	lastInput *eventbridge.ListRulesInput
}

func (m *mockEventBridgeListRulesAPIPaginated) ListRules(_ context.Context, in *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEventBridgeRulesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEventBridgeRulesPage_FirstPage(t *testing.T) {
	mock := &mockEventBridgeListRulesAPIPaginated{
		PageFunc: func(_ int) (*eventbridge.ListRulesOutput, error) {
			return &eventbridge.ListRulesOutput{
				Rules: []ebtypes.Rule{
					{
						Name:         aws.String("my-eb-rule"),
						State:        ebtypes.RuleStateEnabled,
						EventBusName: aws.String("default"),
					},
				},
				NextToken: aws.String("token-eb-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEventBridgeRulesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-eb-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-eb-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-eb-rule" {
		t.Errorf("resource ID: expected %q, got %q", "my-eb-rule", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEventBridgeRulesPage_Continuation(t *testing.T) {
	mock := &mockEventBridgeListRulesAPIPaginated{
		PageFunc: func(_ int) (*eventbridge.ListRulesOutput, error) {
			return &eventbridge.ListRulesOutput{
				Rules: []ebtypes.Rule{
					{
						Name:  aws.String("last-eb-rule"),
						State: ebtypes.RuleStateDisabled,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEventBridgeRulesPage(context.Background(), mock, "token-eb-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-eb-rule" {
		t.Errorf("resource ID: expected %q, got %q", "last-eb-rule", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-eb-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-eb-page-2")
	}
}

func TestQA_Pagination_FetchEventBridgeRulesPage_Empty(t *testing.T) {
	mock := &mockEventBridgeListRulesAPIPaginated{
		PageFunc: func(_ int) (*eventbridge.ListRulesOutput, error) {
			return &eventbridge.ListRulesOutput{
				Rules:     []ebtypes.Rule{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEventBridgeRulesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchEventBridgeRulesPage_Error(t *testing.T) {
	mock := &mockEventBridgeListRulesAPIPaginated{
		PageFunc: func(_ int) (*eventbridge.ListRulesOutput, error) {
			return nil, errors.New("eventbridge: access denied")
		},
	}

	_, err := awsclient.FetchEventBridgeRulesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Kinesis ListStreams (paginated, HasMoreStreams + NextToken)
// ---------------------------------------------------------------------------

type mockKinesisListStreamsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*kinesis.ListStreamsOutput, error)
	lastInput *kinesis.ListStreamsInput
}

func (m *mockKinesisListStreamsAPIPaginated) ListStreams(_ context.Context, in *kinesis.ListStreamsInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchKinesisStreamsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchKinesisStreamsPage_FirstPage(t *testing.T) {
	mock := &mockKinesisListStreamsAPIPaginated{
		PageFunc: func(_ int) (*kinesis.ListStreamsOutput, error) {
			return &kinesis.ListStreamsOutput{
				StreamSummaries: []kinesistypes.StreamSummary{
					{
						StreamName:   aws.String("my-kinesis-stream"),
						StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-kinesis-stream"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
				},
				HasMoreStreams: aws.Bool(true),
				NextToken:      aws.String("token-kinesis-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with HasMoreStreams=true")
	}
	if result.Pagination.NextToken != "token-kinesis-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-kinesis-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-kinesis-stream" {
		t.Errorf("resource ID: expected %q, got %q", "my-kinesis-stream", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchKinesisStreamsPage_Continuation(t *testing.T) {
	mock := &mockKinesisListStreamsAPIPaginated{
		PageFunc: func(_ int) (*kinesis.ListStreamsOutput, error) {
			return &kinesis.ListStreamsOutput{
				StreamSummaries: []kinesistypes.StreamSummary{
					{
						StreamName:   aws.String("last-kinesis-stream"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
				},
				HasMoreStreams: aws.Bool(false),
				NextToken:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "token-kinesis-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-kinesis-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-kinesis-page-2")
	}
	if result.Resources[0].ID != "last-kinesis-stream" {
		t.Errorf("resource ID: expected %q, got %q", "last-kinesis-stream", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchKinesisStreamsPage_Empty(t *testing.T) {
	mock := &mockKinesisListStreamsAPIPaginated{
		PageFunc: func(_ int) (*kinesis.ListStreamsOutput, error) {
			return &kinesis.ListStreamsOutput{
				StreamSummaries: []kinesistypes.StreamSummary{},
				HasMoreStreams:  aws.Bool(false),
				NextToken:       nil,
			}, nil
		},
	}

	result, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchKinesisStreamsPage_Error(t *testing.T) {
	mock := &mockKinesisListStreamsAPIPaginated{
		PageFunc: func(_ int) (*kinesis.ListStreamsOutput, error) {
			return nil, errors.New("kinesis: throughput exceeded")
		},
	}

	_, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: MSK ListClustersV2 (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockMSKListClustersV2APIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*kafka.ListClustersV2Output, error)
	lastInput *kafka.ListClustersV2Input
}

func (m *mockMSKListClustersV2APIPaginated) ListClustersV2(_ context.Context, in *kafka.ListClustersV2Input, _ ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchMSKClustersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchMSKClustersPage_FirstPage(t *testing.T) {
	mock := &mockMSKListClustersV2APIPaginated{
		PageFunc: func(_ int) (*kafka.ListClustersV2Output, error) {
			return &kafka.ListClustersV2Output{
				ClusterInfoList: []kafkatypes.Cluster{
					{
						ClusterName: aws.String("my-msk-cluster"),
						ClusterType: kafkatypes.ClusterTypeProvisioned,
						State:       kafkatypes.ClusterStateActive,
					},
				},
				NextToken: aws.String("token-msk-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-msk-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-msk-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-msk-cluster" {
		t.Errorf("resource ID: expected %q, got %q", "my-msk-cluster", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchMSKClustersPage_Continuation(t *testing.T) {
	mock := &mockMSKListClustersV2APIPaginated{
		PageFunc: func(_ int) (*kafka.ListClustersV2Output, error) {
			return &kafka.ListClustersV2Output{
				ClusterInfoList: []kafkatypes.Cluster{
					{
						ClusterName: aws.String("last-msk-cluster"),
						State:       kafkatypes.ClusterStateActive,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "token-msk-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-msk-cluster" {
		t.Errorf("resource ID: expected %q, got %q", "last-msk-cluster", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-msk-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-msk-page-2")
	}
}

func TestQA_Pagination_FetchMSKClustersPage_Empty(t *testing.T) {
	mock := &mockMSKListClustersV2APIPaginated{
		PageFunc: func(_ int) (*kafka.ListClustersV2Output, error) {
			return &kafka.ListClustersV2Output{
				ClusterInfoList: []kafkatypes.Cluster{},
				NextToken:       nil,
			}, nil
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchMSKClustersPage_Error(t *testing.T) {
	mock := &mockMSKListClustersV2APIPaginated{
		PageFunc: func(_ int) (*kafka.ListClustersV2Output, error) {
			return nil, errors.New("msk: cluster not found")
		},
	}

	_, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SFN ListStateMachines (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockSFNListStateMachinesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*sfn.ListStateMachinesOutput, error)
	lastInput *sfn.ListStateMachinesInput
}

func (m *mockSFNListStateMachinesAPIPaginated) ListStateMachines(_ context.Context, in *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchStepFunctionsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchStepFunctionsPage_FirstPage(t *testing.T) {
	mock := &mockSFNListStateMachinesAPIPaginated{
		PageFunc: func(_ int) (*sfn.ListStateMachinesOutput, error) {
			return &sfn.ListStateMachinesOutput{
				StateMachines: []sfntypes.StateMachineListItem{
					{
						Name:            aws.String("my-state-machine"),
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine"),
						Type:            sfntypes.StateMachineTypeStandard,
					},
				},
				NextToken: aws.String("token-sfn-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchStepFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-sfn-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-sfn-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-state-machine" {
		t.Errorf("resource ID: expected %q, got %q", "my-state-machine", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchStepFunctionsPage_Continuation(t *testing.T) {
	mock := &mockSFNListStateMachinesAPIPaginated{
		PageFunc: func(_ int) (*sfn.ListStateMachinesOutput, error) {
			return &sfn.ListStateMachinesOutput{
				StateMachines: []sfntypes.StateMachineListItem{
					{
						Name: aws.String("last-state-machine"),
						Type: sfntypes.StateMachineTypeExpress,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchStepFunctionsPage(context.Background(), mock, "token-sfn-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-state-machine" {
		t.Errorf("resource ID: expected %q, got %q", "last-state-machine", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-sfn-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-sfn-page-2")
	}
}

func TestQA_Pagination_FetchStepFunctionsPage_Empty(t *testing.T) {
	mock := &mockSFNListStateMachinesAPIPaginated{
		PageFunc: func(_ int) (*sfn.ListStateMachinesOutput, error) {
			return &sfn.ListStateMachinesOutput{
				StateMachines: []sfntypes.StateMachineListItem{},
				NextToken:     nil,
			}, nil
		},
	}

	result, err := awsclient.FetchStepFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchStepFunctionsPage_Error(t *testing.T) {
	mock := &mockSFNListStateMachinesAPIPaginated{
		PageFunc: func(_ int) (*sfn.ListStateMachinesOutput, error) {
			return nil, errors.New("sfn: state machine not found")
		},
	}

	_, err := awsclient.FetchStepFunctionsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SNS ListSubscriptions (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockSNSListSubscriptionsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*sns.ListSubscriptionsOutput, error)
	lastInput *sns.ListSubscriptionsInput
}

func (m *mockSNSListSubscriptionsAPIPaginated) ListSubscriptions(_ context.Context, in *sns.ListSubscriptionsInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSNSSubscriptionsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSNSSubscriptionsPage_FirstPage(t *testing.T) {
	mock := &mockSNSListSubscriptionsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsOutput, error) {
			return &sns.ListSubscriptionsOutput{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:my-topic:abc-123"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:my-topic"),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("user@example.com"),
					},
				},
				NextToken: aws.String("token-sns-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-sns-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-sns-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "arn:aws:sns:us-east-1:123456789012:my-topic:abc-123" {
		t.Errorf("resource ID: expected %q, got %q", "arn:aws:sns:us-east-1:123456789012:my-topic:abc-123", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSNSSubscriptionsPage_Continuation(t *testing.T) {
	mock := &mockSNSListSubscriptionsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsOutput, error) {
			return &sns.ListSubscriptionsOutput{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:last-topic:def-456"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:last-topic"),
						Protocol:        aws.String("sqs"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), mock, "token-sns-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "arn:aws:sns:us-east-1:123456789012:last-topic:def-456" {
		t.Errorf("resource ID: expected %q, got %q", "arn:aws:sns:us-east-1:123456789012:last-topic:def-456", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-sns-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-sns-page-2")
	}
}

func TestQA_Pagination_FetchSNSSubscriptionsPage_Empty(t *testing.T) {
	mock := &mockSNSListSubscriptionsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsOutput, error) {
			return &sns.ListSubscriptionsOutput{
				Subscriptions: []snstypes.Subscription{},
				NextToken:     nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchSNSSubscriptionsPage_Error(t *testing.T) {
	mock := &mockSNSListSubscriptionsAPIPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsOutput, error) {
			return nil, errors.New("sns: subscription not found")
		},
	}

	_, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Glue GetJobs (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockGlueGetJobsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*glue.GetJobsOutput, error)
	lastInput *glue.GetJobsInput
}

func (m *mockGlueGetJobsAPIPaginated) GetJobs(_ context.Context, in *glue.GetJobsInput, _ ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchGlueJobsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchGlueJobsPage_FirstPage(t *testing.T) {
	mock := &mockGlueGetJobsAPIPaginated{
		PageFunc: func(_ int) (*glue.GetJobsOutput, error) {
			return &glue.GetJobsOutput{
				Jobs: []gluetypes.Job{
					{
						Name:        aws.String("my-glue-job"),
						Role:        aws.String("arn:aws:iam::123456789012:role/GlueRole"),
						GlueVersion: aws.String("4.0"),
					},
				},
				NextToken: aws.String("token-glue-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchGlueJobsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-glue-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-glue-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-glue-job" {
		t.Errorf("resource ID: expected %q, got %q", "my-glue-job", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchGlueJobsPage_Continuation(t *testing.T) {
	mock := &mockGlueGetJobsAPIPaginated{
		PageFunc: func(_ int) (*glue.GetJobsOutput, error) {
			return &glue.GetJobsOutput{
				Jobs: []gluetypes.Job{
					{
						Name:        aws.String("last-glue-job"),
						GlueVersion: aws.String("3.0"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchGlueJobsPage(context.Background(), mock, "token-glue-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-glue-job" {
		t.Errorf("resource ID: expected %q, got %q", "last-glue-job", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-glue-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-glue-page-2")
	}
}

func TestQA_Pagination_FetchGlueJobsPage_Empty(t *testing.T) {
	mock := &mockGlueGetJobsAPIPaginated{
		PageFunc: func(_ int) (*glue.GetJobsOutput, error) {
			return &glue.GetJobsOutput{
				Jobs:      []gluetypes.Job{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchGlueJobsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchGlueJobsPage_Error(t *testing.T) {
	mock := &mockGlueGetJobsAPIPaginated{
		PageFunc: func(_ int) (*glue.GetJobsOutput, error) {
			return nil, errors.New("glue: access denied")
		},
	}

	_, err := awsclient.FetchGlueJobsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Athena ListWorkGroups (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockAthenaListWorkGroupsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*athena.ListWorkGroupsOutput, error)
	lastInput *athena.ListWorkGroupsInput
}

func (m *mockAthenaListWorkGroupsAPIPaginated) ListWorkGroups(_ context.Context, in *athena.ListWorkGroupsInput, _ ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchAthenaWorkgroupsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchAthenaWorkgroupsPage_FirstPage(t *testing.T) {
	mock := &mockAthenaListWorkGroupsAPIPaginated{
		PageFunc: func(_ int) (*athena.ListWorkGroupsOutput, error) {
			return &athena.ListWorkGroupsOutput{
				WorkGroups: []athenatypes.WorkGroupSummary{
					{
						Name:  aws.String("my-workgroup"),
						State: athenatypes.WorkGroupStateEnabled,
					},
				},
				NextToken: aws.String("token-athena-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchAthenaWorkgroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-athena-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-athena-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-workgroup" {
		t.Errorf("resource ID: expected %q, got %q", "my-workgroup", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchAthenaWorkgroupsPage_Continuation(t *testing.T) {
	mock := &mockAthenaListWorkGroupsAPIPaginated{
		PageFunc: func(_ int) (*athena.ListWorkGroupsOutput, error) {
			return &athena.ListWorkGroupsOutput{
				WorkGroups: []athenatypes.WorkGroupSummary{
					{
						Name:  aws.String("last-workgroup"),
						State: athenatypes.WorkGroupStateDisabled,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAthenaWorkgroupsPage(context.Background(), mock, "token-athena-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-workgroup" {
		t.Errorf("resource ID: expected %q, got %q", "last-workgroup", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-athena-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-athena-page-2")
	}
}

func TestQA_Pagination_FetchAthenaWorkgroupsPage_Empty(t *testing.T) {
	mock := &mockAthenaListWorkGroupsAPIPaginated{
		PageFunc: func(_ int) (*athena.ListWorkGroupsOutput, error) {
			return &athena.ListWorkGroupsOutput{
				WorkGroups: []athenatypes.WorkGroupSummary{},
				NextToken:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAthenaWorkgroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchAthenaWorkgroupsPage_Error(t *testing.T) {
	mock := &mockAthenaListWorkGroupsAPIPaginated{
		PageFunc: func(_ int) (*athena.ListWorkGroupsOutput, error) {
			return nil, errors.New("athena: workgroup not found")
		},
	}

	_, err := awsclient.FetchAthenaWorkgroupsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Redshift DescribeClusters (paginated, Marker)
// ---------------------------------------------------------------------------

type mockRedshiftDescribeClustersAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*redshift.DescribeClustersOutput, error)
	lastInput *redshift.DescribeClustersInput
}

func (m *mockRedshiftDescribeClustersAPIPaginated) DescribeClusters(_ context.Context, in *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchRedshiftClustersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchRedshiftClustersPage_FirstPage(t *testing.T) {
	mock := &mockRedshiftDescribeClustersAPIPaginated{
		PageFunc: func(_ int) (*redshift.DescribeClustersOutput, error) {
			return &redshift.DescribeClustersOutput{
				Clusters: []redshifttypes.Cluster{
					{
						ClusterIdentifier: aws.String("my-redshift-cluster"),
						ClusterStatus:     aws.String("available"),
						NodeType:          aws.String("dc2.large"),
					},
				},
				Marker: aws.String("marker-redshift-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-redshift-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-redshift-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-redshift-cluster" {
		t.Errorf("resource ID: expected %q, got %q", "my-redshift-cluster", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchRedshiftClustersPage_Continuation(t *testing.T) {
	mock := &mockRedshiftDescribeClustersAPIPaginated{
		PageFunc: func(_ int) (*redshift.DescribeClustersOutput, error) {
			return &redshift.DescribeClustersOutput{
				Clusters: []redshifttypes.Cluster{
					{
						ClusterIdentifier: aws.String("last-redshift-cluster"),
						ClusterStatus:     aws.String("available"),
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "marker-redshift-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "last-redshift-cluster" {
		t.Errorf("resource ID: expected %q, got %q", "last-redshift-cluster", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-redshift-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-redshift-page-2")
	}
}

func TestQA_Pagination_FetchRedshiftClustersPage_Empty(t *testing.T) {
	mock := &mockRedshiftDescribeClustersAPIPaginated{
		PageFunc: func(_ int) (*redshift.DescribeClustersOutput, error) {
			return &redshift.DescribeClustersOutput{
				Clusters: []redshifttypes.Cluster{},
				Marker:   nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchRedshiftClustersPage_Error(t *testing.T) {
	mock := &mockRedshiftDescribeClustersAPIPaginated{
		PageFunc: func(_ int) (*redshift.DescribeClustersOutput, error) {
			return nil, errors.New("redshift: cluster not found")
		},
	}

	_, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Backup ListBackupPlans (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockBackupListBackupPlansAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*backup.ListBackupPlansOutput, error)
	lastInput *backup.ListBackupPlansInput
}

func (m *mockBackupListBackupPlansAPIPaginated) ListBackupPlans(_ context.Context, in *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchBackupPlansPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchBackupPlansPage_FirstPage(t *testing.T) {
	mock := &mockBackupListBackupPlansAPIPaginated{
		PageFunc: func(_ int) (*backup.ListBackupPlansOutput, error) {
			return &backup.ListBackupPlansOutput{
				BackupPlansList: []backuptypes.BackupPlansListMember{
					{
						BackupPlanId:   aws.String("plan-id-abc123"),
						BackupPlanName: aws.String("MyBackupPlan"),
					},
				},
				NextToken: aws.String("token-backup-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchBackupPlansPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-backup-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-backup-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "plan-id-abc123" {
		t.Errorf("resource ID: expected %q, got %q", "plan-id-abc123", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchBackupPlansPage_Continuation(t *testing.T) {
	mock := &mockBackupListBackupPlansAPIPaginated{
		PageFunc: func(_ int) (*backup.ListBackupPlansOutput, error) {
			return &backup.ListBackupPlansOutput{
				BackupPlansList: []backuptypes.BackupPlansListMember{
					{
						BackupPlanId:   aws.String("plan-id-def456"),
						BackupPlanName: aws.String("LastBackupPlan"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchBackupPlansPage(context.Background(), mock, "token-backup-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "plan-id-def456" {
		t.Errorf("resource ID: expected %q, got %q", "plan-id-def456", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-backup-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-backup-page-2")
	}
}

func TestQA_Pagination_FetchBackupPlansPage_Empty(t *testing.T) {
	mock := &mockBackupListBackupPlansAPIPaginated{
		PageFunc: func(_ int) (*backup.ListBackupPlansOutput, error) {
			return &backup.ListBackupPlansOutput{
				BackupPlansList: []backuptypes.BackupPlansListMember{},
				NextToken:       nil,
			}, nil
		},
	}

	result, err := awsclient.FetchBackupPlansPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchBackupPlansPage_Error(t *testing.T) {
	mock := &mockBackupListBackupPlansAPIPaginated{
		PageFunc: func(_ int) (*backup.ListBackupPlansOutput, error) {
			return nil, errors.New("backup: plan not found")
		},
	}

	_, err := awsclient.FetchBackupPlansPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SESv2 ListEmailIdentities (paginated, NextToken)
// ---------------------------------------------------------------------------

type mockSESv2ListEmailIdentitiesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*sesv2.ListEmailIdentitiesOutput, error)
	lastInput *sesv2.ListEmailIdentitiesInput
}

func (m *mockSESv2ListEmailIdentitiesAPIPaginated) ListEmailIdentities(_ context.Context, in *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSESIdentitiesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSESIdentitiesPage_FirstPage(t *testing.T) {
	mock := &mockSESv2ListEmailIdentitiesAPIPaginated{
		PageFunc: func(_ int) (*sesv2.ListEmailIdentitiesOutput, error) {
			return &sesv2.ListEmailIdentitiesOutput{
				EmailIdentities: []sesv2types.IdentityInfo{
					{
						IdentityName:       aws.String("example.com"),
						IdentityType:       sesv2types.IdentityTypeDomain,
						SendingEnabled:     true,
						VerificationStatus: sesv2types.VerificationStatusSuccess,
					},
				},
				NextToken: aws.String("token-ses-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-ses-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-ses-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "example.com" {
		t.Errorf("resource ID: expected %q, got %q", "example.com", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSESIdentitiesPage_Continuation(t *testing.T) {
	mock := &mockSESv2ListEmailIdentitiesAPIPaginated{
		PageFunc: func(_ int) (*sesv2.ListEmailIdentitiesOutput, error) {
			return &sesv2.ListEmailIdentitiesOutput{
				EmailIdentities: []sesv2types.IdentityInfo{
					{
						IdentityName:       aws.String("user@example.com"),
						IdentityType:       sesv2types.IdentityTypeEmailAddress,
						SendingEnabled:     true,
						VerificationStatus: sesv2types.VerificationStatusSuccess,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "token-ses-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "user@example.com" {
		t.Errorf("resource ID: expected %q, got %q", "user@example.com", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-ses-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-ses-page-2")
	}
}

func TestQA_Pagination_FetchSESIdentitiesPage_Empty(t *testing.T) {
	mock := &mockSESv2ListEmailIdentitiesAPIPaginated{
		PageFunc: func(_ int) (*sesv2.ListEmailIdentitiesOutput, error) {
			return &sesv2.ListEmailIdentitiesOutput{
				EmailIdentities: []sesv2types.IdentityInfo{},
				NextToken:       nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchSESIdentitiesPage_Error(t *testing.T) {
	mock := &mockSESv2ListEmailIdentitiesAPIPaginated{
		PageFunc: func(_ int) (*sesv2.ListEmailIdentitiesOutput, error) {
			return nil, errors.New("sesv2: identity not found")
		},
	}

	_, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: WAFv2 ListWebACLs (paginated, NextMarker)
// ---------------------------------------------------------------------------

type mockWAFv2ListWebACLsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*wafv2.ListWebACLsOutput, error)
	lastInput *wafv2.ListWebACLsInput
}

func (m *mockWAFv2ListWebACLsAPIPaginated) ListWebACLs(_ context.Context, in *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchWAFWebACLsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchWAFWebACLsPage_FirstPage(t *testing.T) {
	mock := &mockWAFv2ListWebACLsAPIPaginated{
		PageFunc: func(_ int) (*wafv2.ListWebACLsOutput, error) {
			return &wafv2.ListWebACLsOutput{
				WebACLs: []wafv2types.WebACLSummary{
					{
						Id:          aws.String("acl-id-abc123"),
						Name:        aws.String("MyWebACL"),
						ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/MyWebACL/acl-id-abc123"),
						Description: aws.String("Production WAF"),
						LockToken:   aws.String("lock-token-1"),
					},
				},
				NextMarker: aws.String("marker-waf-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchWAFWebACLsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextMarker")
	}
	if result.Pagination.NextToken != "marker-waf-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-waf-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "acl-id-abc123" {
		t.Errorf("resource ID: expected %q, got %q", "acl-id-abc123", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchWAFWebACLsPage_Continuation(t *testing.T) {
	mock := &mockWAFv2ListWebACLsAPIPaginated{
		PageFunc: func(_ int) (*wafv2.ListWebACLsOutput, error) {
			return &wafv2.ListWebACLsOutput{
				WebACLs: []wafv2types.WebACLSummary{
					{
						Id:   aws.String("acl-id-def456"),
						Name: aws.String("LastWebACL"),
					},
				},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchWAFWebACLsPage(context.Background(), mock, "marker-waf-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "acl-id-def456" {
		t.Errorf("resource ID: expected %q, got %q", "acl-id-def456", result.Resources[0].ID)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextMarker == nil || *mock.lastInput.NextMarker != "marker-waf-page-2" {
		t.Errorf("NextMarker not forwarded: got %v, want %q", mock.lastInput.NextMarker, "marker-waf-page-2")
	}
}

func TestQA_Pagination_FetchWAFWebACLsPage_Empty(t *testing.T) {
	mock := &mockWAFv2ListWebACLsAPIPaginated{
		PageFunc: func(_ int) (*wafv2.ListWebACLsOutput, error) {
			return &wafv2.ListWebACLsOutput{
				WebACLs:    []wafv2types.WebACLSummary{},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchWAFWebACLsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchWAFWebACLsPage_Error(t *testing.T) {
	mock := &mockWAFv2ListWebACLsAPIPaginated{
		PageFunc: func(_ int) (*wafv2.ListWebACLsOutput, error) {
			return nil, errors.New("wafv2: web ACL not found")
		},
	}

	_, err := awsclient.FetchWAFWebACLsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
