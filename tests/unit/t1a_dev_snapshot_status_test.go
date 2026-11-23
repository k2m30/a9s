package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t1aDevDBISnaps struct{ status string }

func (m t1aDevDBISnaps) DescribeDBSnapshots(context.Context, *rds.DescribeDBSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	return &rds.DescribeDBSnapshotsOutput{DBSnapshots: []rdstypes.DBSnapshot{{
		DBSnapshotIdentifier: aws.String("snap-a"), Status: aws.String(m.status), Encrypted: aws.Bool(true),
	}}}, nil
}

type t1aDevRDSClusterSnaps struct{ status string }

func (m t1aDevRDSClusterSnaps) DescribeDBClusterSnapshots(context.Context, *rds.DescribeDBClusterSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []rdstypes.DBClusterSnapshot{{
		DBClusterSnapshotIdentifier: aws.String("snap-a"), Engine: aws.String("aurora-postgresql"), Status: aws.String(m.status), StorageEncrypted: aws.Bool(true),
	}}}, nil
}

type t1aDevDocDBClusterSnaps struct{ status string }

func (m t1aDevDocDBClusterSnaps) DescribeDBClusterSnapshots(context.Context, *docdb.DescribeDBClusterSnapshotsInput, ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{{
		DBClusterSnapshotIdentifier: aws.String("snap-a"), Engine: aws.String("docdb"), Status: aws.String(m.status), StorageEncrypted: aws.Bool(true),
	}}}, nil
}

// The RDS API reference gives no value list for a DB snapshot's or a DB
// cluster snapshot's Status, so these types have no state table; a status
// a9s has no word for still reaches the Status cell with the value AWS sent,
// and the row is a warning, never clean.
func TestT1aDev_SnapshotStatusOutsideTheKnownSetIsNotClean(t *testing.T) {
	const unknown = "storage-optimizing"
	fetches := map[string]func() (resource.FetchResult, error){
		"dbi-snap": func() (resource.FetchResult, error) {
			return awsclient.FetchDBISnapshotsPage(context.Background(), t1aDevDBISnaps{unknown}, "")
		},
		"dbc-snap (rds)": func() (resource.FetchResult, error) {
			return awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), t1aDevRDSClusterSnaps{unknown}, "")
		},
		"dbc-snap (docdb)": func() (resource.FetchResult, error) {
			return awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), t1aDevDocDBClusterSnaps{unknown}, "")
		},
	}
	for name, fetch := range fetches {
		page, err := fetch()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(page.Resources) != 1 {
			t.Fatalf("%s: %d rows, want 1", name, len(page.Resources))
		}
		findings := page.Resources[0].Findings
		top, ok := domain.TopFinding(findings)
		if !ok || top.Severity != domain.SevWarn || !strings.Contains(domain.StatusPhrase(findings), unknown) {
			t.Errorf("%s: status %q renders %q from %v, want a warning naming the value", name, unknown, domain.StatusPhrase(findings), findings)
		}
	}
}
