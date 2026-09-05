package unit

// prowler_w2_dbi_snap_test.go — dbi-snap row 18 of the w2 Prowler batch:
// a DB snapshot shared with every AWS account.
//
// The restore attribute is the only place this is visible; DescribeDBSnapshots
// does not carry it. The enricher reached here is the one wired on the
// dbi-snap catalog literal, so the test also proves the new call landed in the
// registered cross-ref enricher rather than in a second, unregistered one.

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const w2DBISnapCodePublic = "dbi-snap.public"

// w2DBISnapAttrFake answers DescribeDBSnapshotAttributes per snapshot.
// Snapshots absent from sharedWith are private: the restore attribute exists
// but its value list is empty, which is what RDS returns for an unshared
// snapshot.
type w2DBISnapAttrFake struct {
	awsclient.RDSAPI

	sharedWith map[string][]string
	errs       map[string]error
}

func (f *w2DBISnapAttrFake) DescribeDBSnapshotAttributes(_ context.Context, in *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	id := aws.ToString(in.DBSnapshotIdentifier)
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	return &rds.DescribeDBSnapshotAttributesOutput{
		DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
			DBSnapshotIdentifier: in.DBSnapshotIdentifier,
			DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{
				{AttributeName: aws.String("restore"), AttributeValues: f.sharedWith[id]},
			},
		},
	}, nil
}

func w2DBISnapshot(id string) rdstypes.DBSnapshot {
	return rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(id),
		DBSnapshotArn:        aws.String("arn:aws:rds:eu-central-1:123456789012:snapshot:" + id),
		DBInstanceIdentifier: aws.String("acme-orders-db"),
		Status:               aws.String("available"),
		SnapshotType:         aws.String("manual"),
		Engine:               aws.String("postgres"),
		Encrypted:            aws.Bool(true),
	}
}

func w2DBISnapRun(t *testing.T, fake *w2DBISnapAttrFake, ids ...string) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		rs = append(rs, w2Res(id, w2DBISnapshot(id)))
	}
	res, err := w2Enricher(t, "dbi-snap")(context.Background(), &awsclient.ServiceClients{RDS: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

func w2DBISnapEnrich(t *testing.T, fake *w2DBISnapAttrFake, ids ...string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2DBISnapRun(t, fake, ids...)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

func TestW2DBISnapPublic(t *testing.T) {
	fake := &w2DBISnapAttrFake{sharedWith: map[string][]string{
		"acme-orders-db-2026-02-01": {"all"},
	}}
	res := w2DBISnapEnrich(t, fake, "acme-orders-db-2026-02-01", "acme-orders-db-2026-02-02")

	w2AssertFinding(t, res.Findings["acme-orders-db-2026-02-01"], w2DBISnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2:dbi-snap")
	w2AssertRow(t, w2Rows(t, res, "acme-orders-db-2026-02-01", w2DBISnapCodePublic), "Restore", "all")
	w2AssertFindingDef(t, "dbi-snap", w2DBISnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2")

	w2AssertNoCode(t, res.Findings["acme-orders-db-2026-02-02"], w2DBISnapCodePublic)
}

// A snapshot shared with named accounts is a deliberate cross-account grant,
// not a public one. Only the literal `all` group means anyone can restore it.
func TestW2DBISnapSharedWithNamedAccountsIsNotPublic(t *testing.T) {
	fake := &w2DBISnapAttrFake{sharedWith: map[string][]string{
		"acme-orders-db-2026-02-01": {"210987654321", "333333333333"},
	}}
	res := w2DBISnapEnrich(t, fake, "acme-orders-db-2026-02-01")
	w2AssertNoCode(t, res.Findings["acme-orders-db-2026-02-01"], w2DBISnapCodePublic)
}

func TestW2DBISnapPrivateSnapshotIsClean(t *testing.T) {
	res := w2DBISnapEnrich(t, &w2DBISnapAttrFake{}, "acme-orders-db-2026-02-02")
	w2AssertNoCode(t, res.Findings["acme-orders-db-2026-02-02"], w2DBISnapCodePublic)
	if res.TruncatedIDs["acme-orders-db-2026-02-02"] {
		t.Error("a private snapshot was marked unknown; the empty restore list is a definite answer")
	}
}

// A snapshot deleted between the list call and the attribute call answers
// DBSnapshotNotFound. That is an expected race, not a failure of the run: the
// row goes unknown and the rest of the batch is still evaluated.
func TestW2DBISnapNotFoundIsTruncatedNotFailure(t *testing.T) {
	fake := &w2DBISnapAttrFake{
		sharedWith: map[string][]string{"acme-orders-db-2026-02-01": {"all"}},
		errs: map[string]error{
			"acme-orders-db-gone": &rdstypes.DBSnapshotNotFoundFault{Message: aws.String("not found")},
		},
	}
	res, err := w2DBISnapRun(t, fake, "acme-orders-db-gone", "acme-orders-db-2026-02-01")
	if err != nil {
		t.Errorf("a snapshot that vanished between the list call and the attribute call was reported as a run failure: %v", err)
	}

	if !res.TruncatedIDs["acme-orders-db-gone"] {
		t.Error("vanished snapshot not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["acme-orders-db-gone"], w2DBISnapCodePublic)
	w2AssertFinding(t, res.Findings["acme-orders-db-2026-02-01"], w2DBISnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2:dbi-snap")
}

// An AccessDenied on one snapshot must leave that row unknown and the rest of
// the batch answered — a partial answer must never render as a complete one.
func TestW2DBISnapDeniedOnOneItemKeepsTheRestEvaluated(t *testing.T) {
	fake := &w2DBISnapAttrFake{
		sharedWith: map[string][]string{"acme-orders-db-2026-02-01": {"all"}},
		errs: map[string]error{
			"acme-orders-db-denied": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2DBISnapRun(t, fake, "acme-orders-db-denied", "acme-orders-db-2026-02-01")
	if err == nil {
		t.Error("a denied attribute read was not folded into the composite error")
	}

	if !res.TruncatedIDs["acme-orders-db-denied"] {
		t.Error("denied snapshot not marked in TruncatedIDs")
	}
	w2AssertFinding(t, res.Findings["acme-orders-db-2026-02-01"], w2DBISnapCodePublic, "shared with all AWS accounts", domain.SevBroken, "wave2:dbi-snap")
}

func TestW2DBISnapNilClientIsSafe(t *testing.T) {
	rs := []resource.Resource{w2Res("acme-orders-db-2026-02-01", w2DBISnapshot("acme-orders-db-2026-02-01"))}
	res, err := w2Enricher(t, "dbi-snap")(context.Background(), &awsclient.ServiceClients{}, rs, nil)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings["acme-orders-db-2026-02-01"], w2DBISnapCodePublic)
}

func TestW2DBISnapCapBoundary(t *testing.T) {
	ids := make([]string, 0, awsclient.EnrichmentCap+1)
	shared := map[string][]string{}
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		id := "acme-orders-db-snap-" + strconv.Itoa(i)
		ids = append(ids, id)
		shared[id] = []string{"all"}
	}

	atCap := w2DBISnapEnrich(t, &w2DBISnapAttrFake{sharedWith: shared}, ids[:awsclient.EnrichmentCap]...)
	if atCap.Truncated {
		t.Error("exactly EnrichmentCap snapshots reported Truncated")
	}

	overCap := w2DBISnapEnrich(t, &w2DBISnapAttrFake{sharedWith: shared}, ids...)
	if !overCap.Truncated {
		t.Error("EnrichmentCap+1 snapshots did not report Truncated")
	}
}
