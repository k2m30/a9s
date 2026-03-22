package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T-ECSSVC01 - Test ECS Services three-step fetch
// (ListClusters -> ListServices -> DescribeServices)
// ---------------------------------------------------------------------------

func TestFetchECSServices_ParsesMultipleServices(t *testing.T) {
	clusterArn := "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"
	createdAt := time.Now()

	listClustersMock := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{clusterArn},
		},
	}

	listServicesMock := &mockECSListServicesClient{
		outputs: map[string]*ecs.ListServicesOutput{
			clusterArn: {
				ServiceArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/web-service",
					"arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service",
				},
			},
		},
	}

	describeServicesMock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName:    aws.String("web-service"),
					ClusterArn:     aws.String(clusterArn),
					Status:         aws.String("ACTIVE"),
					DesiredCount:   3,
					RunningCount:   3,
					LaunchType:     ecstypes.LaunchTypeFargate,
					ServiceArn:     aws.String("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/web-service"),
					TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-task:5"),
					RoleArn:        aws.String("arn:aws:iam::123456789012:role/ecsServiceRole"),
					CreatedAt:      &createdAt,
				},
				{
					ServiceName:    aws.String("api-service"),
					ClusterArn:     aws.String(clusterArn),
					Status:         aws.String("ACTIVE"),
					DesiredCount:   2,
					RunningCount:   1,
					LaunchType:     ecstypes.LaunchTypeEc2,
					ServiceArn:     aws.String("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
					TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:3"),
					CreatedAt:      &createdAt,
				},
			},
		},
	}

	resources, err := awsclient.FetchECSServices(context.Background(), listClustersMock, listServicesMock, describeServicesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"service_name", "cluster", "status", "desired_count", "running_count", "launch_type"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first service
	r0 := resources[0]
	if r0.ID != "web-service" {
		t.Errorf("resource[0].ID: expected %q, got %q", "web-service", r0.ID)
	}
	if r0.Name != "web-service" {
		t.Errorf("resource[0].Name: expected %q, got %q", "web-service", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["service_name"] != "web-service" {
		t.Errorf("resource[0].Fields[\"service_name\"]: expected %q, got %q", "web-service", r0.Fields["service_name"])
	}
	if r0.Fields["cluster"] != clusterArn {
		t.Errorf("resource[0].Fields[\"cluster\"]: expected %q, got %q", clusterArn, r0.Fields["cluster"])
	}
	if r0.Fields["status"] != "ACTIVE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ACTIVE", r0.Fields["status"])
	}
	if r0.Fields["desired_count"] != "3" {
		t.Errorf("resource[0].Fields[\"desired_count\"]: expected %q, got %q", "3", r0.Fields["desired_count"])
	}
	if r0.Fields["running_count"] != "3" {
		t.Errorf("resource[0].Fields[\"running_count\"]: expected %q, got %q", "3", r0.Fields["running_count"])
	}
	if r0.Fields["launch_type"] != "FARGATE" {
		t.Errorf("resource[0].Fields[\"launch_type\"]: expected %q, got %q", "FARGATE", r0.Fields["launch_type"])
	}

	// Verify second service
	r1 := resources[1]
	if r1.ID != "api-service" {
		t.Errorf("resource[1].ID: expected %q, got %q", "api-service", r1.ID)
	}
	if r1.Fields["desired_count"] != "2" {
		t.Errorf("resource[1].Fields[\"desired_count\"]: expected %q, got %q", "2", r1.Fields["desired_count"])
	}
	if r1.Fields["running_count"] != "1" {
		t.Errorf("resource[1].Fields[\"running_count\"]: expected %q, got %q", "1", r1.Fields["running_count"])
	}
	if r1.Fields["launch_type"] != "EC2" {
		t.Errorf("resource[1].Fields[\"launch_type\"]: expected %q, got %q", "EC2", r1.Fields["launch_type"])
	}
}

func TestFetchECSServices_ListClustersError(t *testing.T) {
	listClustersMock := &mockECSListClustersClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	listServicesMock := &mockECSListServicesClient{}
	describeServicesMock := &mockECSDescribeServicesClient{}

	resources, err := awsclient.FetchECSServices(context.Background(), listClustersMock, listServicesMock, describeServicesMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchECSServices_EmptyResponse(t *testing.T) {
	listClustersMock := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{},
		},
	}
	listServicesMock := &mockECSListServicesClient{}
	describeServicesMock := &mockECSDescribeServicesClient{}

	resources, err := awsclient.FetchECSServices(context.Background(), listClustersMock, listServicesMock, describeServicesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
