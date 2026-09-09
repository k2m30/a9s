package unit

// prowler_w2_dbc_snap_test.go — dbc-snap row 19 of the w2 Prowler batch:
// a cluster snapshot shared with every AWS account.
//
// The dbc-snap list merges DocumentDB and RDS cluster snapshots, and each row
// keeps the SDK struct it arrived as. Both clients expose the same
// DescribeDBClusterSnapshotAttributes shape, so a snapshot shared with `all`
// must produce the identical finding whichever client answered — the
// architectural point of this row.

import (
	"context"
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

const w2DBCSnapCodePublic = "dbc-snap.public"

type w2DocDBSnapAttrFake struct {
	awsclient.DocDBAPI

	sharedWith map[string][]string
	errs       map[string]error
}

func (f *w2DocDBSnapAttrFake) DescribeDBClusterSnapshotAttributes(_ context.Context, in *docdb.DescribeDBClusterSnapshotAttributesInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotAttributesOutput, error) {
	id := aws.ToString(in.DBClusterSnapshotIdentifier)
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	return &docdb.DescribeDBClusterSnapshotAttributesOutput{
		DBClusterSnapshotAttributesResult: &docdbtypes.DBClusterSnapshotAttributesResult{
			DBClusterSnapshotIdentifier: in.DBClusterSnapshotIdentifier,
			DBClusterSnapshotAttributes: []docdbtypes.DBClusterSnapshotAttribute{
				{AttributeName: aws.String("restore"), AttributeValues: f.sharedWith[id]},
			},
		},
	}, nil
}

type w2RDSSnapAttrFake struct {
	awsclient.RDSAPI

	sharedWith map[string][]string
	errs       map[string]error
}

func (f *w2RDSSnapAttrFake) DescribeDBClusterSnapshotAttributes(_ context.Context, in *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	id := aws.ToString(in.DBClusterSnapshotIdentifier)
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	return &rds.DescribeDBClusterSnapshotAttributesOutput{
		DBClusterSnapshotAttributesResult: &rdstypes.DBClusterSnapshotAttributesResult{
			DBClusterSnapshotIdentifier: in.DBClusterSnapshotIdentifier,
			DBClusterSnapshotAttributes: []rdstypes.DBClusterSnapshotAttribute{
				{AttributeName: aws.String("restore"), AttributeValues: f.sharedWith[id]},
			},
		},
	}, nil
}

func w2DocDBClusterSnapshot(id string) docdbtypes.DBClusterSnapshot {
	return docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(id),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:eu-central-1:123456789012:cluster-snapshot:" + id),
		DBClusterIdentifier:         aws.String("acme-docs-cluster"),
		Status:                      aws.String("available"),
		SnapshotType:                aws.String("manual"),
		Engine:                      aws.String("docdb"),
		StorageEncrypted:            aws.Bool(true),
	}
}

func w2RDSClusterSnapshot(id string) rdstypes.DBClusterSnapshot {
	return rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(id),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:eu-central-1:123456789012:cluster-snapshot:" + id),
		DBClusterIdentifier:         aws.String("acme-orders-cluster"),
		Status:                      aws.String("available"),
		SnapshotType:                aws.String("manual"),
		Engine:                      aws.String("aurora-postgresql"),
		StorageEncrypted:            aws.Bool(true),
	}
}

func w2DBCSnapRun(t *testing.T, clients *awsclient.ServiceClients, rs ...resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	res, err := w2Enricher(t, "dbc-snap")(context.Background(), clients, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

func w2DBCSnapEnrich(t *testing.T, clients *awsclient.ServiceClients, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2DBCSnapRun(t, clients, rs...)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

// The same shared-with-all snapshot must read identically whichever SDK
// produced the row. A per-client divergence here would make the same exposure
// visible on Aurora rows and invisible on DocumentDB rows.
func TestW2DBCSnapPublicIdenticalAcrossBothClients(t *testing.T) {
	docID := "acme-docs-cluster-2026-02-01"
	rdsID := "acme-orders-cluster-2026-02-01"

	clients := &awsclient.ServiceClients{
		DocDB: &w2DocDBSnapAttrFake{sharedWith: map[string][]string{docID: {"all"}}},
		RDS:   &w2RDSSnapAttrFake{sharedWith: map[string][]string{rdsID: {"all"}}},
	}
	res := w2DBCSnapEnrich(t, clients,
		w2Res(docID, w2DocDBClusterSnapshot(docID)),
		w2Res(rdsID, w2RDSClusterSnapshot(rdsID)),
	)

	docF := w2AssertFinding(t, res.Findings[docID], w2DBCSnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2")
	rdsF := w2AssertFinding(t, res.Findings[rdsID], w2DBCSnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2")
	if docF.Detail != rdsF.Detail {
		t.Errorf("Detail differs by client: docdb %q vs rds %q", docF.Detail, rdsF.Detail)
	}

	w2AssertRow(t, w2Rows(t, res, docID, w2DBCSnapCodePublic), "Restore", "all")
	w2AssertRow(t, w2Rows(t, res, rdsID, w2DBCSnapCodePublic), "Restore", "all")

	w2AssertFindingDef(t, "dbc-snap", w2DBCSnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2")
}

func TestW2DBCSnapPrivateSnapshotsAreCleanOnBothClients(t *testing.T) {
	docID := "acme-docs-cluster-2026-02-02"
	rdsID := "acme-orders-cluster-2026-02-02"

	clients := &awsclient.ServiceClients{
		DocDB: &w2DocDBSnapAttrFake{},
		RDS:   &w2RDSSnapAttrFake{},
	}
	res := w2DBCSnapEnrich(t, clients,
		w2Res(docID, w2DocDBClusterSnapshot(docID)),
		w2Res(rdsID, w2RDSClusterSnapshot(rdsID)),
	)

	w2AssertNoCode(t, res.Findings[docID], w2DBCSnapCodePublic)
	w2AssertNoCode(t, res.Findings[rdsID], w2DBCSnapCodePublic)
}

// Named-account sharing is a deliberate grant; only `all` is public.
func TestW2DBCSnapNamedAccountsIsNotPublic(t *testing.T) {
	docID := "acme-docs-cluster-2026-02-01"
	clients := &awsclient.ServiceClients{
		DocDB: &w2DocDBSnapAttrFake{sharedWith: map[string][]string{docID: {"210987654321"}}},
		RDS:   &w2RDSSnapAttrFake{},
	}
	res := w2DBCSnapEnrich(t, clients, w2Res(docID, w2DocDBClusterSnapshot(docID)))
	w2AssertNoCode(t, res.Findings[docID], w2DBCSnapCodePublic)
}

// A failure on the DocumentDB side must not blank the Aurora rows in the same
// mixed batch, and vice versa.
func TestW2DBCSnapErrorOnOneClientLeavesTheOtherAnswered(t *testing.T) {
	docID := "acme-docs-cluster-gone"
	rdsID := "acme-orders-cluster-2026-02-01"

	clients := &awsclient.ServiceClients{
		DocDB: &w2DocDBSnapAttrFake{errs: map[string]error{
			docID: &docdbtypes.DBClusterSnapshotNotFoundFault{Message: aws.String("not found")},
		}},
		RDS: &w2RDSSnapAttrFake{sharedWith: map[string][]string{rdsID: {"all"}}},
	}
	res, err := w2DBCSnapRun(t, clients,
		w2Res(docID, w2DocDBClusterSnapshot(docID)),
		w2Res(rdsID, w2RDSClusterSnapshot(rdsID)),
	)
	if err != nil {
		t.Errorf("a snapshot that vanished between the list call and the attribute call was reported as a run failure: %v", err)
	}

	if _, marked := res.TruncatedIDs[docID]; !marked {
		t.Error("vanished DocumentDB snapshot not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings[docID], w2DBCSnapCodePublic)
	w2AssertFinding(t, res.Findings[rdsID], w2DBCSnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2")
}

func TestW2DBCSnapNilClientsAreSafe(t *testing.T) {
	docID := "acme-docs-cluster-2026-02-01"
	res, err := w2Enricher(t, "dbc-snap")(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w2Res(docID, w2DocDBClusterSnapshot(docID))}, nil)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[docID], w2DBCSnapCodePublic)
}
