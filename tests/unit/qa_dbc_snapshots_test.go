package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestQA_DBCSnapshots_FetchSuccess(t *testing.T) {
	now := time.Now()
	mock := &mockDocDBDescribeSnapshotsClient{
		output: &docdb.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
				{
					DBClusterSnapshotIdentifier: aws.String("dbc-snap-auto-001"),
					DBClusterIdentifier:         aws.String("my-docdb-cluster"),
					Status:                      aws.String("available"),
					Engine:                      aws.String("docdb"),
					SnapshotType:                aws.String("automated"),
					SnapshotCreateTime:          &now,
					StorageType:                 aws.String("standard"),
				},
				{
					DBClusterSnapshotIdentifier: aws.String("dbc-snap-manual-001"),
					DBClusterIdentifier:         aws.String("my-docdb-cluster"),
					Status:                      aws.String("creating"),
					Engine:                      aws.String("docdb"),
					SnapshotType:                aws.String("manual"),
					SnapshotCreateTime:          &now,
					StorageType:                 aws.String("iopt1"),
				},
			},
		},
	}

	resources, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "dbc-snap-auto-001" {
		t.Errorf("expected ID 'dbc-snap-auto-001', got %q", r.ID)
	}
	if r.Name != "dbc-snap-auto-001" {
		t.Errorf("expected Name 'dbc-snap-auto-001', got %q", r.Name)
	}
	// Per spec §4 (docs/resources/dbc-snap.md): Healthy snapshots render
	// blank in the §4 column. The fetcher's computeDBCSnapPhrase maps the
	// raw AWS keyword "available" → "" so the Status column stays blank.
	if r.Fields["snapshot_id"] != "dbc-snap-auto-001" {
		t.Errorf("expected snapshot_id 'dbc-snap-auto-001', got %q", r.Fields["snapshot_id"])
	}
	if r.Fields["cluster_id"] != "my-docdb-cluster" {
		t.Errorf("expected cluster_id 'my-docdb-cluster', got %q", r.Fields["cluster_id"])
	}
	if r.Fields["engine"] != "docdb" {
		t.Errorf("expected engine 'docdb', got %q", r.Fields["engine"])
	}
	if r.Fields["snapshot_type"] != "automated" {
		t.Errorf("expected snapshot_type 'automated', got %q", r.Fields["snapshot_type"])
	}
	if r.Fields["storage_type"] != "standard" {
		t.Errorf("expected storage_type 'standard', got %q", r.Fields["storage_type"])
	}

	r2 := resources[1]
	// Per Phase-03 PR-03e: fetcher no longer writes Resource.Status; the
	// display phrase lives in Fields["status"], findings carry severity.
	if r2.Fields["status"] != "creating" {
		t.Errorf("expected Fields[\"status\"] 'creating', got %q", r2.Fields["status"])
	}
	if r2.Fields["snapshot_type"] != "manual" {
		t.Errorf("expected snapshot_type 'manual', got %q", r2.Fields["snapshot_type"])
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_DBCSnapshots_FetchEmpty(t *testing.T) {
	mock := &mockDocDBDescribeSnapshotsClient{
		output: &docdb.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{},
		},
	}

	resources, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_DBCSnapshots_FetchError(t *testing.T) {
	mock := &mockDocDBDescribeSnapshotsClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_DBCSnapshots_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("dbc-snap")
	if rt == nil {
		t.Fatal("resource type 'dbc-snap' not found")
	}
	if rt.Name != "DB Cluster Snapshots" {
		t.Errorf("expected Name 'DB Cluster Snapshots', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"snapshot_id", "Snapshot ID"},
		{"cluster_id", "Cluster ID"},
		{"status", "Status"},
		{"engine", "Engine"},
		{"snapshot_type", "Type"},
		{"snapshot_create_time", "Created"},
		{"storage_type", "Storage"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
		if rt.Columns[i].Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, rt.Columns[i].Title)
		}
	}
}
