package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T072 - Test CloudWatch Logs DescribeLogGroups response parsing
// ---------------------------------------------------------------------------

func TestFetchCloudWatchLogGroups_ParsesMultipleLogGroups(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsClient{
		output: &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []cwlogstypes.LogGroup{
				{
					LogGroupName:         aws.String("/aws/lambda/prod-processor"),
					StoredBytes:          int64Ptr(1048576),
					RetentionInDays:      aws.Int32(30),
					CreationTime:         int64Ptr(1700000000000),
					Arn:                  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/prod-processor:*"),
					KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
					DataProtectionStatus: cwlogstypes.DataProtectionStatusActivated,
				},
				{
					LogGroupName:         aws.String("/aws/ecs/staging-service"),
					StoredBytes:          int64Ptr(524288),
					RetentionInDays:      aws.Int32(7),
					CreationTime:         int64Ptr(1710000000000),
					Arn:                  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/ecs/staging-service:*"),
					DataProtectionStatus: cwlogstypes.DataProtectionStatusDisabled,
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudWatchLogGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"log_group_name", "stored_bytes", "retention_days", "creation_time"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first log group
	r0 := resources[0]
	if r0.ID != "/aws/lambda/prod-processor" {
		t.Errorf("resource[0].ID: expected %q, got %q", "/aws/lambda/prod-processor", r0.ID)
	}
	if r0.Name != "/aws/lambda/prod-processor" {
		t.Errorf("resource[0].Name: expected %q, got %q", "/aws/lambda/prod-processor", r0.Name)
	}
	if r0.Fields["log_group_name"] != "/aws/lambda/prod-processor" {
		t.Errorf("resource[0].Fields[\"log_group_name\"]: expected %q, got %q", "/aws/lambda/prod-processor", r0.Fields["log_group_name"])
	}
	if r0.Fields["stored_bytes"] != "1048576" {
		t.Errorf("resource[0].Fields[\"stored_bytes\"]: expected %q, got %q", "1048576", r0.Fields["stored_bytes"])
	}
	if r0.Fields["retention_days"] != "30" {
		t.Errorf("resource[0].Fields[\"retention_days\"]: expected %q, got %q", "30", r0.Fields["retention_days"])
	}
	if r0.Fields["creation_time"] == "" {
		t.Error("resource[0].Fields[\"creation_time\"] should not be empty")
	}

	// Verify second log group
	r1 := resources[1]
	if r1.ID != "/aws/ecs/staging-service" {
		t.Errorf("resource[1].ID: expected %q, got %q", "/aws/ecs/staging-service", r1.ID)
	}
	if r1.Fields["stored_bytes"] != "524288" {
		t.Errorf("resource[1].Fields[\"stored_bytes\"]: expected %q, got %q", "524288", r1.Fields["stored_bytes"])
	}
	if r1.Fields["retention_days"] != "7" {
		t.Errorf("resource[1].Fields[\"retention_days\"]: expected %q, got %q", "7", r1.Fields["retention_days"])
	}
}

func TestFetchCloudWatchLogGroups_ErrorResponse(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCloudWatchLogGroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchCloudWatchLogGroups_EmptyResponse(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsClient{
		output: &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []cwlogstypes.LogGroup{},
		},
	}

	resources, err := awsclient.FetchCloudWatchLogGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
