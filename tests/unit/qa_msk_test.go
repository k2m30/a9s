package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// T-MSK01 - Test MSK ListClustersV2 fetch
// ---------------------------------------------------------------------------

func TestFetchMSKClusters_ParsesMultipleClusters(t *testing.T) {
	creationTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	listMock := &mockMSKListClustersV2Client{
		output: &kafka.ListClustersV2Output{
			ClusterInfoList: []kafkatypes.Cluster{
				{
					ClusterName:    aws.String("events-cluster"),
					ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/events-cluster/abc-123"),
					ClusterType:    kafkatypes.ClusterTypeProvisioned,
					State:          kafkatypes.ClusterStateActive,
					CurrentVersion: aws.String("2.8.1"),
					CreationTime:   &creationTime,
					Tags:           map[string]string{"env": "production"},
				},
				{
					ClusterName:    aws.String("logs-cluster"),
					ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/logs-cluster/def-456"),
					ClusterType:    kafkatypes.ClusterTypeServerless,
					State:          kafkatypes.ClusterStateCreating,
					CurrentVersion: aws.String("3.5.1"),
					CreationTime:   &creationTime,
				},
			},
		},
	}

	resources, err := awsclient.FetchMSKClusters(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields
	requiredFields := []string{"cluster_name", "cluster_type", "state", "version"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first cluster
	r0 := resources[0]
	if r0.ID != "events-cluster" {
		t.Errorf("resource[0].ID: expected %q, got %q", "events-cluster", r0.ID)
	}
	if r0.Name != "events-cluster" {
		t.Errorf("resource[0].Name: expected %q, got %q", "events-cluster", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["cluster_type"] != "PROVISIONED" {
		t.Errorf("resource[0].Fields[\"cluster_type\"]: expected %q, got %q", "PROVISIONED", r0.Fields["cluster_type"])
	}
	if r0.Fields["version"] != "2.8.1" {
		t.Errorf("resource[0].Fields[\"version\"]: expected %q, got %q", "2.8.1", r0.Fields["version"])
	}

	// Verify second cluster
	r1 := resources[1]
	if r1.Status != "CREATING" {
		t.Errorf("resource[1].Status: expected %q, got %q", "CREATING", r1.Status)
	}
	if r1.Fields["cluster_type"] != "SERVERLESS" {
		t.Errorf("resource[1].Fields[\"cluster_type\"]: expected %q, got %q", "SERVERLESS", r1.Fields["cluster_type"])
	}

	// Verify RawStruct is set
	if r0.RawStruct == nil {
		t.Error("resource[0].RawStruct should not be nil")
	}

}

func TestFetchMSKClusters_ListError(t *testing.T) {
	listMock := &mockMSKListClustersV2Client{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchMSKClusters(context.Background(), listMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchMSKClusters_EmptyResponse(t *testing.T) {
	listMock := &mockMSKListClustersV2Client{
		output: &kafka.ListClustersV2Output{
			ClusterInfoList: []kafkatypes.Cluster{},
		},
	}

	resources, err := awsclient.FetchMSKClusters(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-MSK02 - Resource type definition
// ---------------------------------------------------------------------------

func TestMSK_ResourceTypeDef(t *testing.T) {
	rt := resource.FindResourceType("msk")
	if rt == nil {
		t.Fatal("resource type 'msk' not found")
	}

	if rt.Name != "MSK Clusters" {
		t.Errorf("expected name %q, got %q", "MSK Clusters", rt.Name)
	}

	expected := []struct {
		title string
		key   string
		width int
	}{
		{"Cluster Name", "cluster_name", 28},
		{"Type", "cluster_type", 14},
		{"State", "state", 14},
		{"Version", "version", 14},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
	}
}

func TestMSK_Aliases(t *testing.T) {
	aliases := []string{"msk", "kafka"}
	for _, alias := range aliases {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("expected resource type for alias %q, got nil", alias)
		}
	}
}
