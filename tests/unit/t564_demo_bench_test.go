package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
)

const t564EncRecoverable = "inaccessible-encryption-credentials-recoverable"

// The demo endpoint DocumentDB, Neptune and RDS share answers an unfiltered
// DocumentDB DescribeDBClusters with every engine's clusters, a Neptune one
// among them, and an engine=docdb one with DocumentDB clusters only — so the
// demo cannot hide a DB Clusters list that takes clusters of other engines
// from the DocumentDB call.
func TestT564Demo_DocDBEndpointAnswersEveryEngineUnlessFiltered(t *testing.T) {
	api := demo.NewServiceClients().DocDB
	engines := func(in *docdb.DescribeDBClustersInput) []string {
		out, err := api.DescribeDBClusters(context.Background(), in)
		if err != nil {
			t.Fatalf("demo DescribeDBClusters: %v", err)
		}
		var es []string
		for _, c := range out.DBClusters {
			es = append(es, aws.ToString(c.Engine))
		}
		return es
	}
	all := engines(&docdb.DescribeDBClustersInput{})
	if !slices.Contains(all, "neptune") || !slices.ContainsFunc(all, func(e string) bool { return e != "docdb" && e != "neptune" }) {
		t.Errorf("unfiltered demo DescribeDBClusters answers engines %v, want Neptune and RDS engines beside DocumentDB", all)
	}
	scoped := engines(&docdb.DescribeDBClustersInput{Filters: []docdbtypes.Filter{{Name: aws.String("engine"), Values: []string{"docdb"}}}})
	if len(scoped) == 0 || slices.ContainsFunc(scoped, func(e string) bool { return e != "docdb" }) {
		t.Errorf("engine=docdb demo DescribeDBClusters answers engines %v, want DocumentDB only", scoped)
	}
}

// The demo shows a DB cluster and a DB instance whose encryption credentials
// AWS reports as recoverably inaccessible, counted broken; a stopped cluster
// reading "stopped (storage still billed)"; and no Neptune row in DB Clusters.
func TestT564Demo_RDSLifecycleWitnesses(t *testing.T) {
	td, rows := t563Merged(t, "dbc")
	var encCluster, stoppedCluster bool
	for _, r := range rows {
		if r.Fields["engine"] == "neptune" {
			t.Errorf("DB Clusters lists Neptune cluster %s", r.ID)
		}
		switch r.Fields["status_raw"] {
		case t564EncRecoverable:
			encCluster = true
			if top := t563AssertWitnessRendered(t, td, rows, r); top.Severity != domain.SevBroken {
				t.Errorf("dbc/%s top finding %s at %v, want Broken", r.ID, top.Code, top.Severity)
			}
		case "stopped":
			stoppedCluster = true
			if top := t563AssertWitnessRendered(t, td, rows, r); top.Phrase != "stopped (storage still billed)" || top.Severity != domain.SevBroken {
				t.Errorf("dbc/%s top finding %q at %v, want \"stopped (storage still billed)\" at Broken", r.ID, top.Phrase, top.Severity)
			}
		}
	}

	dbiTD, dbiRows := t563Merged(t, "dbi")
	var encInstance bool
	for _, r := range dbiRows {
		if r.Fields["status_raw"] != t564EncRecoverable {
			continue
		}
		encInstance = true
		if top := t563AssertWitnessRendered(t, dbiTD, dbiRows, r); top.Severity != domain.SevBroken {
			t.Errorf("dbi/%s top finding %s at %v, want Broken", r.ID, top.Code, top.Severity)
		}
	}

	if !encCluster || !stoppedCluster || !encInstance {
		t.Errorf("demo witnesses: %s cluster %v, stopped cluster %v, %s instance %v; want all three", t564EncRecoverable, encCluster, stoppedCluster, t564EncRecoverable, encInstance)
	}
}
