package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-ECS01 - Test ECS Clusters two-step fetch (ListClusters + DescribeClusters)
// ---------------------------------------------------------------------------

func TestFetchECSClusters_ParsesMultipleClusters(t *testing.T) {
	listMock := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{
				"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
				"arn:aws:ecs:us-east-1:123456789012:cluster/staging-cluster",
			},
		},
	}

	describeMock := &mockECSDescribeClustersClient{
		output: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{
				{
					ClusterName:                       aws.String("prod-cluster"),
					Status:                            aws.String("ACTIVE"),
					RunningTasksCount:                 12,
					PendingTasksCount:                 2,
					ActiveServicesCount:               5,
					RegisteredContainerInstancesCount: 3,
					ClusterArn:                        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
				},
				{
					ClusterName:                       aws.String("staging-cluster"),
					Status:                            aws.String("ACTIVE"),
					RunningTasksCount:                 4,
					PendingTasksCount:                 0,
					ActiveServicesCount:               2,
					RegisteredContainerInstancesCount: 1,
					ClusterArn:                        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/staging-cluster"),
				},
			},
		},
	}

	resources, err := awsclient.FetchECSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"cluster_name", "status", "running_tasks", "pending_tasks", "services_count"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first cluster
	r0 := resources[0]
	if r0.ID != "prod-cluster" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-cluster", r0.ID)
	}
	if r0.Name != "prod-cluster" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-cluster", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["cluster_name"] != "prod-cluster" {
		t.Errorf("resource[0].Fields[\"cluster_name\"]: expected %q, got %q", "prod-cluster", r0.Fields["cluster_name"])
	}
	if r0.Fields["status"] != "ACTIVE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ACTIVE", r0.Fields["status"])
	}
	if r0.Fields["running_tasks"] != "12" {
		t.Errorf("resource[0].Fields[\"running_tasks\"]: expected %q, got %q", "12", r0.Fields["running_tasks"])
	}
	if r0.Fields["pending_tasks"] != "2" {
		t.Errorf("resource[0].Fields[\"pending_tasks\"]: expected %q, got %q", "2", r0.Fields["pending_tasks"])
	}
	if r0.Fields["services_count"] != "5" {
		t.Errorf("resource[0].Fields[\"services_count\"]: expected %q, got %q", "5", r0.Fields["services_count"])
	}

	// Verify second cluster
	r1 := resources[1]
	if r1.ID != "staging-cluster" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging-cluster", r1.ID)
	}
	if r1.Fields["running_tasks"] != "4" {
		t.Errorf("resource[1].Fields[\"running_tasks\"]: expected %q, got %q", "4", r1.Fields["running_tasks"])
	}
	if r1.Fields["pending_tasks"] != "0" {
		t.Errorf("resource[1].Fields[\"pending_tasks\"]: expected %q, got %q", "0", r1.Fields["pending_tasks"])
	}
	if r1.Fields["services_count"] != "2" {
		t.Errorf("resource[1].Fields[\"services_count\"]: expected %q, got %q", "2", r1.Fields["services_count"])
	}
}

func TestFetchECSClusters_ListClustersError(t *testing.T) {
	listMock := &mockECSListClustersClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	describeMock := &mockECSDescribeClustersClient{}

	resources, err := awsclient.FetchECSClusters(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchECSClusters_EmptyResponse(t *testing.T) {
	listMock := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{},
		},
	}
	describeMock := &mockECSDescribeClustersClient{}

	resources, err := awsclient.FetchECSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
