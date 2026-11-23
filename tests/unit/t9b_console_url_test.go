package unit_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

const t9bAccount = "123456789012"

// t9bRDSAccount answers the RDS reads of an account that runs DB engines of
// every family the RDS API serves; every other call goes to the demo fake.
type t9bRDSAccount struct {
	awsclient.RDSAPI
	instances    []rdstypes.DBInstance
	clusters     []rdstypes.DBCluster
	snapshots    []rdstypes.DBSnapshot
	clusterSnaps []rdstypes.DBClusterSnapshot
	events       map[string][]rdstypes.Event
}

func (f *t9bRDSAccount) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.instances}, nil
}

func (f *t9bRDSAccount) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

func (f *t9bRDSAccount) DescribeDBSnapshots(context.Context, *rds.DescribeDBSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	return &rds.DescribeDBSnapshotsOutput{DBSnapshots: f.snapshots}, nil
}

func (f *t9bRDSAccount) DescribeDBClusterSnapshots(context.Context, *rds.DescribeDBClusterSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: f.clusterSnaps}, nil
}

func (f *t9bRDSAccount) DescribeEvents(_ context.Context, in *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	return &rds.DescribeEventsOutput{Events: f.events[aws.ToString(in.SourceIdentifier)]}, nil
}

type t9bDocDBAccount struct {
	awsclient.DocDBAPI
	clusters []docdbtypes.DBCluster
	snaps    []docdbtypes.DBClusterSnapshot
}

func (f *t9bDocDBAccount) DescribeDBClusters(context.Context, *docdb.DescribeDBClustersInput, ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

func (f *t9bDocDBAccount) DescribeDBClusterSnapshots(context.Context, *docdb.DescribeDBClusterSnapshotsInput, ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: f.snaps}, nil
}

func t9bInstance(id, engine string) rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String(id),
		DBInstanceArn:        aws.String("arn:aws:rds:eu-west-1:" + t9bAccount + ":db:" + id),
		Engine:               aws.String(engine),
		DBInstanceStatus:     aws.String("available"),
		DBInstanceClass:      aws.String("db.r6g.large"),
		Endpoint: &rdstypes.Endpoint{
			Address:      aws.String(id + ".c1a2b3c4d5e6.eu-west-1.rds.amazonaws.com"),
			Port:         aws.Int32(5432),
			HostedZoneId: aws.String("Z29XKXDKYMONMX"),
		},
		MultiAZ:               aws.Bool(false),
		StorageEncrypted:      aws.Bool(true),
		DeletionProtection:    aws.Bool(true),
		BackupRetentionPeriod: aws.Int32(7),
	}
}

func t9bRDSCluster(id, engine string) rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier:   aws.String(id),
		DBClusterArn:          aws.String("arn:aws:rds:eu-west-1:" + t9bAccount + ":cluster:" + id),
		Engine:                aws.String(engine),
		Status:                aws.String("available"),
		Endpoint:              aws.String(id + ".cluster-c1a2b3c4d5e6.eu-west-1.rds.amazonaws.com"),
		StorageEncrypted:      aws.Bool(true),
		DeletionProtection:    aws.Bool(true),
		BackupRetentionPeriod: aws.Int32(7),
		DBClusterMembers:      []rdstypes.DBClusterMember{{DBInstanceIdentifier: aws.String(id + "-1"), IsClusterWriter: aws.Bool(true)}},
	}
}

func t9bRDSFamilyClients() *awsclient.ServiceClients {
	clients := demo.NewServiceClients()
	created := aws.Time(time.Now().UTC().AddDate(0, 0, -2))
	clients.RDS = &t9bRDSAccount{
		RDSAPI: clients.RDS,
		instances: []rdstypes.DBInstance{
			t9bInstance("acme-docdb-1", "docdb"),
			t9bInstance("acme-neptune-1", "neptune"),
			t9bInstance("acme-orders", "postgres"),
			t9bInstance("acme-ledger", "mysql"),
			t9bInstance("acme-aurora-pg-1", "aurora-postgresql"),
		},
		clusters: []rdstypes.DBCluster{
			t9bRDSCluster("acme-aurora-mysql", "aurora-mysql"),
			t9bRDSCluster("acme-aurora-pg", "aurora-postgresql"),
			t9bRDSCluster("acme-maz-pg", "postgres"),
			t9bRDSCluster("acme-maz-mysql", "mysql"),
		},
		snapshots: []rdstypes.DBSnapshot{
			{DBSnapshotIdentifier: aws.String("acme-orders-snap"), DBInstanceIdentifier: aws.String("acme-orders"), Engine: aws.String("postgres"), Status: aws.String("available"), SnapshotType: aws.String("manual"), Encrypted: aws.Bool(true), SnapshotCreateTime: created},
			{DBSnapshotIdentifier: aws.String("acme-ledger-snap"), DBInstanceIdentifier: aws.String("acme-ledger"), Engine: aws.String("mysql"), Status: aws.String("available"), SnapshotType: aws.String("manual"), Encrypted: aws.Bool(true), SnapshotCreateTime: created},
		},
		clusterSnaps: []rdstypes.DBClusterSnapshot{
			{DBClusterSnapshotIdentifier: aws.String("acme-aurora-pg-snap"), DBClusterIdentifier: aws.String("acme-aurora-pg"), Engine: aws.String("aurora-postgresql"), Status: aws.String("available"), SnapshotType: aws.String("manual"), StorageEncrypted: aws.Bool(true), SnapshotCreateTime: created},
		},
		events: map[string][]rdstypes.Event{
			"acme-docdb-1": {{SourceIdentifier: aws.String("acme-docdb-1"), SourceType: rdstypes.SourceTypeDbInstance, SourceArn: aws.String("arn:aws:rds:eu-west-1:" + t9bAccount + ":db:acme-docdb-1"), Message: aws.String("DB instance restarted"), EventCategories: []string{"availability"}, Date: created}},
			"acme-orders":  {{SourceIdentifier: aws.String("acme-orders"), SourceType: rdstypes.SourceTypeDbInstance, SourceArn: aws.String("arn:aws:rds:eu-west-1:" + t9bAccount + ":db:acme-orders"), Message: aws.String("DB instance restarted"), EventCategories: []string{"availability"}, Date: created}},
		},
	}
	clients.DocDB = &t9bDocDBAccount{
		DocDBAPI: clients.DocDB,
		clusters: []docdbtypes.DBCluster{{
			DBClusterIdentifier:   aws.String("acme-docdb"),
			DBClusterArn:          aws.String("arn:aws:rds:eu-west-1:" + t9bAccount + ":cluster:acme-docdb"),
			Engine:                aws.String("docdb"),
			Status:                aws.String("available"),
			StorageEncrypted:      aws.Bool(true),
			DeletionProtection:    aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
			DBClusterMembers:      []docdbtypes.DBClusterMember{{DBInstanceIdentifier: aws.String("acme-docdb-1"), IsClusterWriter: aws.Bool(true)}},
		}},
		snaps: []docdbtypes.DBClusterSnapshot{{
			DBClusterSnapshotIdentifier: aws.String("acme-docdb-snap"),
			DBClusterIdentifier:         aws.String("acme-docdb"),
			Engine:                      aws.String("docdb"),
			Status:                      aws.String("available"),
			SnapshotType:                aws.String("manual"),
			StorageEncrypted:            aws.Bool(true),
			SnapshotCreateTime:          created,
		}},
	}
	return clients
}

// Each engine family has its own console: the RDS, DocumentDB and Neptune user
// guides send the reader to console.aws.amazon.com/rds/, /docdb and
// /neptune/home respectively. DescribeDBInstances also returns Neptune and
// DocumentDB instances (its API reference: "This operation can also return
// information for Amazon Neptune DB instances and Amazon DocumentDB
// instances"), so the RDS API lists rows of all three families.
func t9bConsoleService(engine string) string {
	switch {
	case strings.HasPrefix(engine, "docdb"):
		return "docdb"
	case strings.HasPrefix(engine, "neptune"):
		return "neptune"
	default:
		return "rds"
	}
}

// An RDS-family row opens the console of the service that runs its engine,
// on the row's region: a DocumentDB or Neptune row (instance, cluster,
// snapshot, or an event of such an instance) never opens the RDS console.
func TestT9b_Row7_RDSFamilyConsoleFollowsEngine(t *testing.T) {
	const region = "eu-west-1"
	clients := t9bRDSFamilyClients()
	engineOf := map[string]string{}

	check := func(typ string, r resource.Resource, engine string) {
		t.Helper()
		td := catalog.FindAny(typ)
		u, ok := consolelink.Resolve(*td, r, region, t9bAccount)
		if !ok {
			t.Errorf("%s %s (engine %s): no console link", typ, r.ID, engine)
			return
		}
		parsed, err := url.Parse(u)
		if err != nil {
			t.Errorf("%s %s: console link %q does not parse: %v", typ, r.ID, u, err)
			return
		}
		want := t9bConsoleService(engine)
		if parsed.Host != region+".console.aws.amazon.com" || parsed.Path != "/"+want+"/home" || parsed.Query().Get("region") != region {
			t.Errorf("%s %s (engine %s): console link %q, want the %s console home on %s", typ, r.ID, engine, u, want, region)
		}
	}

	seen := map[string]int{}
	for _, typ := range []string{"dbi", "dbc", "dbi-snap", "dbc-snap"} {
		td := catalog.FindAny(typ)
		rows, _ := unit.DrainFixtures(t, *td, clients)
		for _, r := range rows {
			engine := r.Fields["engine"]
			if typ == "dbi" {
				engineOf[r.ID] = engine
			}
			seen[typ+"/"+t9bConsoleService(engine)]++
			check(typ, r, engine)
		}
	}

	dbi := catalog.FindAny("dbi")
	events := catalog.FindAny("dbi_events")
	dbiRows, _ := unit.DrainFixtures(t, *dbi, clients)
	for i := range dbiRows {
		parent := dbiRows[i]
		if parent.ID != "acme-docdb-1" && parent.ID != "acme-orders" {
			continue
		}
		pctx := resource.ResolveChildContext(dbi.Children[0], &parent, nil)
		got, err := events.ChildFetcher(context.Background(), clients, pctx, "")
		if err != nil {
			t.Fatalf("dbi_events of %s: %v", parent.ID, err)
		}
		for _, ev := range got.Resources {
			seen["dbi_events/"+t9bConsoleService(engineOf[parent.ID])]++
			check("dbi_events", ev, engineOf[parent.ID])
		}
	}

	for _, want := range []string{
		"dbi/docdb", "dbi/neptune", "dbi/rds",
		"dbc/docdb", "dbc/rds",
		"dbi-snap/rds",
		"dbc-snap/docdb", "dbc-snap/rds",
		"dbi_events/docdb", "dbi_events/rds",
	} {
		if seen[want] == 0 {
			t.Errorf("no %s row was listed; the account lists one", want)
		}
	}
}
