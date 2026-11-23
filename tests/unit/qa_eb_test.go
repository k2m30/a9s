package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestFetchEBEnvironments_ParsesMultipleEnvironments(t *testing.T) {
	now := time.Now()
	mock := &mockEBClient{
		output: &elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentName:   aws.String("my-env-prod"),
					EnvironmentId:     aws.String("e-abc123"),
					ApplicationName:   aws.String("my-app"),
					Status:            ebtypes.EnvironmentStatusReady,
					Health:            ebtypes.EnvironmentHealthGreen,
					VersionLabel:      aws.String("v1.2.3"),
					SolutionStackName: aws.String("64bit Amazon Linux 2 v5.8.0 running Node.js 18"),
					PlatformArn:       aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Node.js"),
					EndpointURL:       aws.String("my-env.us-east-1.elasticbeanstalk.com"),
					DateCreated:       &now,
					DateUpdated:       &now,
					EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/my-app/my-env-prod"),
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

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEBEnvironmentsPage(context.Background(), mock, token)
	})
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
	// A Green environment carries no finding.
	if len(r.Findings) != 0 {
		t.Errorf("expected 0 Findings for Green environment (health is structural, not wave1), got %d", len(r.Findings))
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

	// An Updating environment with Yellow health carries its Updating state
	// finding and, beside it, CodeEBHealthYellow/SevWarn: health is a second
	// axis that stands while the environment is live.
	r2 := resources[1]
	if len(r2.Findings) != 2 {
		t.Fatalf("expected the state and the health finding for an Updating Yellow environment, got %v", r2.Findings)
	}
	if _, want := catalog.LifecycleOf("eb", "Updating"); r2.Findings[0].Code != want {
		t.Errorf("r2 Findings[0].Code = %q, want the Updating state finding", r2.Findings[0].Code)
	}
	if r2.Findings[1].Code != awsclient.CodeEBHealthYellow || r2.Findings[1].Severity != domain.SevWarn {
		t.Errorf("r2 Findings[1] = %s/%v, want %s/%v", r2.Findings[1].Code, r2.Findings[1].Severity, awsclient.CodeEBHealthYellow, domain.SevWarn)
	}
}

func TestFetchEBEnvironments_EmptyResponse(t *testing.T) {
	mock := &mockEBClient{
		output: &elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEBEnvironmentsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchEBEnvironments_APIError(t *testing.T) {
	mock := &mockEBClient{
		err: &MockAPIError{Code: "InvalidParameterValue", Message: "invalid"},
	}

	_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEBEnvironmentsPage(context.Background(), mock, token)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
