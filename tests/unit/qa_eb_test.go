package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-EB-001 - Test Elastic Beanstalk Environments response parsing
// ---------------------------------------------------------------------------

func TestFetchEBEnvironments_ParsesMultipleEnvironments(t *testing.T) {
	now := time.Now()
	mock := &mockEBClient{
		output: &elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentName: aws.String("my-env-prod"),
					EnvironmentId:   aws.String("e-abc123"),
					ApplicationName: aws.String("my-app"),
					Status:          ebtypes.EnvironmentStatusReady,
					Health:          ebtypes.EnvironmentHealthGreen,
					VersionLabel:    aws.String("v1.2.3"),
					SolutionStackName: aws.String("64bit Amazon Linux 2 v5.8.0 running Node.js 18"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Node.js"),
					EndpointURL:     aws.String("my-env.us-east-1.elasticbeanstalk.com"),
					DateCreated:     &now,
					DateUpdated:     &now,
					EnvironmentArn:  aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/my-app/my-env-prod"),
				},
				{
					EnvironmentName: aws.String("my-env-staging"),
					EnvironmentId:   aws.String("e-def456"),
					ApplicationName: aws.String("my-app"),
					Status:          ebtypes.EnvironmentStatusUpdating,
					Health:          ebtypes.EnvironmentHealthYellow,
				},
			},
		},
	}

	resources, err := awsclient.FetchEBEnvironments(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "my-env-prod" {
		t.Errorf("expected Name 'my-env-prod', got %q", r.Name)
	}
	if r.ID != "e-abc123" {
		t.Errorf("expected ID 'e-abc123', got %q", r.ID)
	}
	if r.Status != "Ready" {
		t.Errorf("expected Status 'Ready', got %q", r.Status)
	}
	if r.Fields["environment_name"] != "my-env-prod" {
		t.Errorf("expected Fields[environment_name] 'my-env-prod', got %q", r.Fields["environment_name"])
	}
	if r.Fields["application_name"] != "my-app" {
		t.Errorf("expected Fields[application_name] 'my-app', got %q", r.Fields["application_name"])
	}
	if r.Fields["status"] != "Ready" {
		t.Errorf("expected Fields[status] 'Ready', got %q", r.Fields["status"])
	}
	if r.Fields["health"] != "Green" {
		t.Errorf("expected Fields[health] 'Green', got %q", r.Fields["health"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second env
	r2 := resources[1]
	if r2.Status != "Updating" {
		t.Errorf("expected Status 'Updating', got %q", r2.Status)
	}
}

func TestFetchEBEnvironments_EmptyResponse(t *testing.T) {
	mock := &mockEBClient{
		output: &elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{},
		},
	}

	resources, err := awsclient.FetchEBEnvironments(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchEBEnvironments_APIError(t *testing.T) {
	mock := &mockEBClient{
		err: &mockAPIError{code: "InvalidParameterValue", message: "invalid"},
	}

	_, err := awsclient.FetchEBEnvironments(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
