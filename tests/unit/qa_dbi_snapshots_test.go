package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestQA_DBISnapshots_FetchSuccess(t *testing.T) {
	now := time.Now()
	mock := &mockRDSDescribeDBSnapshotsClient{
		output: &rds.DescribeDBSnapshotsOutput{
			DBSnapshots: []rdstypes.DBSnapshot{
				{
					DBSnapshotIdentifier: aws.String("dbi-snap-auto-001"),
					DBInstanceIdentifier: aws.String("my-db-instance"),
					Status:               aws.String("available"),
					Engine:               aws.String("mysql"),
					SnapshotType:         aws.String("automated"),
					SnapshotCreateTime:   &now,
				},
				{
					DBSnapshotIdentifier: aws.String("dbi-snap-manual-001"),
					DBInstanceIdentifier: aws.String("my-db-instance"),
					Status:               aws.String("creating"),
					Engine:               aws.String("postgres"),
					SnapshotType:         aws.String("manual"),
					SnapshotCreateTime:   &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchDBISnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "dbi-snap-auto-001" {
		t.Errorf("expected ID 'dbi-snap-auto-001', got %q", r.ID)
	}
	if r.Name != "dbi-snap-auto-001" {
		t.Errorf("expected Name 'dbi-snap-auto-001', got %q", r.Name)
	}
	// §4 phrase design: r.Status holds the §4 phrase, not the raw AWS status.
	// Healthy snapshots (available + nil-encrypted treated as no unencrypted signal)
	// produce r.Status == "" (healthy silence).
	if r.Status != "" {
		t.Errorf("expected Status '' (healthy silence), got %q", r.Status)
	}
	if r.Fields["snapshot_id"] != "dbi-snap-auto-001" {
		t.Errorf("expected snapshot_id 'dbi-snap-auto-001', got %q", r.Fields["snapshot_id"])
	}
	if r.Fields["db_instance"] != "my-db-instance" {
		t.Errorf("expected db_instance 'my-db-instance', got %q", r.Fields["db_instance"])
	}
	if r.Fields["engine"] != "mysql" {
		t.Errorf("expected engine 'mysql', got %q", r.Fields["engine"])
	}
	if r.Fields["snapshot_type"] != "automated" {
		t.Errorf("expected snapshot_type 'automated', got %q", r.Fields["snapshot_type"])
	}
	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_DBISnapshots_FetchEmpty(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsClient{
		output: &rds.DescribeDBSnapshotsOutput{
			DBSnapshots: []rdstypes.DBSnapshot{},
		},
	}

	resources, err := awsclient.FetchDBISnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_DBISnapshots_FetchError(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchDBISnapshots(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_DBISnapshots_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("dbi-snap")
	if rt == nil {
		t.Fatal("resource type 'dbi-snap' not found")
	}
	if rt.Name != "DB Instance Snapshots" {
		t.Errorf("expected Name 'DB Instance Snapshots', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"snapshot_id", "Snapshot ID"},
		{"db_instance", "DB Instance"},
		{"status", "Status"},
		{"engine", "Engine"},
		{"snapshot_type", "Type"},
		{"created", "Created"},
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
