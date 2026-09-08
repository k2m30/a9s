package unit

// codex2_round5_test.go — the third instance of "a recorded failure never
// leaves its function", found by the gate in
// qa_failure_discharge_gate_test.go.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// codex2DBIRows builds the enricher input for one instance, matching what
// w2DBIEnrich feeds it.
func codex2DBIRows(t *testing.T, instances ...rdstypes.DBInstance) []resource.Resource {
	t.Helper()
	rs := make([]resource.Resource, 0, len(instances))
	for _, db := range instances {
		r := w2Res(aws.ToString(db.DBInstanceIdentifier), db)
		r.Fields["engine"] = aws.ToString(db.Engine)
		r.Fields["engine_version"] = aws.ToString(db.EngineVersion)
		rs = append(rs, r)
	}
	return rs
}

// TestEnrichDBI_RefusedEngineVersionLookupReachesTheOperator pins the third
// silent lane: enrichDBIEngineVersions recorded its failures into a local
// slice, used it only to raise the Truncated flag, and dropped the causes. The
// row rendered "?" with nothing anywhere saying the call was refused.
func TestEnrichDBI_RefusedEngineVersionLookupReachesTheOperator(t *testing.T) {
	old := w2DBIInstance("acme-legacy-db")
	old.Engine = aws.String("mysql")
	old.EngineVersion = aws.String("5.7.44")

	fake := &w2RDSEngineVersionsFake{
		status: map[string]string{},
		errs: map[string]error{
			"mysql|5.7.44": errors.New("AccessDenied: not authorized to perform: rds:DescribeDBEngineVersions"),
		},
	}

	res, err := w2Enricher(t, "dbi")(context.Background(),
		&awsclient.ServiceClients{RDS: fake}, codex2DBIRows(t, old), nil)

	if err == nil {
		t.Fatal("a refused DescribeDBEngineVersions must reach the operator through " +
			"the composite error, not only as a Truncated flag")
	}
	if !res.TruncatedIDs["acme-legacy-db"] {
		t.Error("the row must still render uninspected")
	}
	// The refusal says nothing about the engine, so no finding may be claimed.
	if len(res.Findings["acme-legacy-db"]) != 0 {
		t.Errorf("Findings = %+v, want none: a refused lookup is not a verdict",
			res.Findings["acme-legacy-db"])
	}
}

// TestEnrichDBI_HealthyRunStillReturnsNoError is the negative control: the
// aggregate must stay nil when nothing failed, or every clean run reports one.
func TestEnrichDBI_HealthyRunStillReturnsNoError(t *testing.T) {
	current := w2DBIInstance("acme-billing-db")
	current.Engine = aws.String("postgres")
	current.EngineVersion = aws.String("16.3")

	fake := &w2RDSEngineVersionsFake{status: map[string]string{"postgres|16.3": "available"}}
	res, err := w2Enricher(t, "dbi")(context.Background(),
		&awsclient.ServiceClients{RDS: fake}, codex2DBIRows(t, current), nil)
	if err != nil {
		t.Fatalf("clean run returned %v, want nil", err)
	}
	if len(res.TruncatedIDs) != 0 {
		t.Errorf("TruncatedIDs = %v, want empty on a clean run", res.TruncatedIDs)
	}
}
