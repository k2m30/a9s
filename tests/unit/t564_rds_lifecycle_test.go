package unit

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const t564InventedStatus = "acme-flux-capacitating"

var t564NotRecognised = regexp.MustCompile(`(?i)not recogni[sz]e`)

// t564Endpoint answers DescribeDBClusters, DescribeDBClusterSnapshots and
// DescribeDBSubnetGroups the way the regional endpoint DocumentDB, Neptune
// and RDS share does: every engine's resources unless an `engine` filter
// names the engines wanted. The DocumentDB and RDS clients call the same
// endpoint, so both fakes read the one inventory.
type t564Endpoint struct {
	clusters  []t564Cluster
	snapshots []t564Snapshot

	docdbClusterInputs  []*docdb.DescribeDBClustersInput
	docdbSubnetGroupHit int
	rdsSubnetGroupHit   int
}

type t564Cluster struct {
	id, engine, status, subnetGroup string
	autoMinor, iamAuth              bool
	noWriter                        bool
	global, replicaOf               string
}

type t564Snapshot struct{ id, clusterID, engine string }

func t564EngineWanted(filters [][2][]string, engine string) bool {
	for _, f := range filters {
		if f[0][0] == "engine" {
			return slices.Contains(f[1], engine)
		}
	}
	return true
}

func t564DocDBFilters(fs []docdbtypes.Filter) [][2][]string {
	var out [][2][]string
	for _, f := range fs {
		out = append(out, [2][]string{{aws.ToString(f.Name)}, f.Values})
	}
	return out
}

func t564RDSFilters(fs []rdstypes.Filter) [][2][]string {
	var out [][2][]string
	for _, f := range fs {
		out = append(out, [2][]string{{aws.ToString(f.Name)}, f.Values})
	}
	return out
}

func t564ClusterARN(id string) *string {
	return aws.String("arn:aws:rds:us-east-1:123456789012:cluster:" + id)
}

func (c t564Cluster) docdb() docdbtypes.DBCluster {
	return docdbtypes.DBCluster{
		DBClusterIdentifier:         aws.String(c.id),
		DBClusterArn:                t564ClusterARN(c.id),
		Engine:                      aws.String(c.engine),
		EngineVersion:               aws.String("5.0.0"),
		Status:                      aws.String(c.status),
		Endpoint:                    aws.String(c.id + ".cluster-c1a2b3c4d5e6.us-east-1.docdb.amazonaws.com"),
		DeletionProtection:          aws.Bool(true),
		StorageEncrypted:            aws.Bool(true),
		BackupRetentionPeriod:       aws.Int32(7),
		MultiAZ:                     aws.Bool(true),
		MasterUsername:              aws.String("acme_admin"),
		DBSubnetGroup:               aws.String(c.subnetGroup),
		ReplicationSourceIdentifier: t564Opt(c.replicaOf),
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String(c.id + "-1"), IsClusterWriter: aws.Bool(!c.noWriter)},
			{DBInstanceIdentifier: aws.String(c.id + "-2"), IsClusterWriter: aws.Bool(false)},
		},
	}
}

func (c t564Cluster) rds() rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier:              aws.String(c.id),
		DBClusterArn:                     t564ClusterARN(c.id),
		Engine:                           aws.String(c.engine),
		EngineVersion:                    aws.String("16.4"),
		Status:                           aws.String(c.status),
		Endpoint:                         aws.String(c.id + ".cluster-c1a2b3c4d5e6.us-east-1.rds.amazonaws.com"),
		DeletionProtection:               aws.Bool(true),
		StorageEncrypted:                 aws.Bool(true),
		BackupRetentionPeriod:            aws.Int32(7),
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(c.autoMinor),
		IAMDatabaseAuthenticationEnabled: aws.Bool(c.iamAuth),
		MasterUsername:                   aws.String("acme_admin"),
		DBSubnetGroup:                    aws.String(c.subnetGroup),
		ReplicationSourceIdentifier:      t564Opt(c.replicaOf),
		GlobalClusterIdentifier:          t564Opt(c.global),
		DBClusterMembers: []rdstypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String(c.id + "-1"), IsClusterWriter: aws.Bool(!c.noWriter)},
			{DBInstanceIdentifier: aws.String(c.id + "-2"), IsClusterWriter: aws.Bool(false)},
		},
	}
}

type t564DocDB struct {
	awsclient.DocDBAPI
	ep *t564Endpoint
}

func (f t564DocDB) DescribeDBClusters(_ context.Context, in *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	f.ep.docdbClusterInputs = append(f.ep.docdbClusterInputs, in)
	out := &docdb.DescribeDBClustersOutput{}
	for _, c := range f.ep.clusters {
		if t564EngineWanted(t564DocDBFilters(in.Filters), c.engine) {
			out.DBClusters = append(out.DBClusters, c.docdb())
		}
	}
	return out, nil
}

func (f t564DocDB) DescribeDBClusterSnapshots(_ context.Context, in *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	out := &docdb.DescribeDBClusterSnapshotsOutput{}
	for _, s := range f.ep.snapshots {
		if t564EngineWanted(t564DocDBFilters(in.Filters), s.engine) {
			out.DBClusterSnapshots = append(out.DBClusterSnapshots, docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String(s.id),
				DBClusterIdentifier:         aws.String(s.clusterID),
				Engine:                      aws.String(s.engine),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				StorageEncrypted:            aws.Bool(true),
			})
		}
	}
	return out, nil
}

func (f t564DocDB) DescribeDBSubnetGroups(_ context.Context, in *docdb.DescribeDBSubnetGroupsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	f.ep.docdbSubnetGroupHit++
	return &docdb.DescribeDBSubnetGroupsOutput{DBSubnetGroups: []docdbtypes.DBSubnetGroup{{
		DBSubnetGroupName: in.DBSubnetGroupName,
		VpcId:             aws.String("vpc-0a1b2c3d4e5f60718"),
		Subnets:           []docdbtypes.Subnet{{SubnetIdentifier: aws.String("subnet-0docdbside000001")}},
	}}}, nil
}

type t564RDS struct {
	awsclient.RDSAPI
	ep        *t564Endpoint
	instances []rdstypes.DBInstance
}

func (f t564RDS) DescribeDBClusters(_ context.Context, in *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	out := &rds.DescribeDBClustersOutput{}
	for _, c := range f.ep.clusters {
		if t564EngineWanted(t564RDSFilters(in.Filters), c.engine) {
			out.DBClusters = append(out.DBClusters, c.rds())
		}
	}
	return out, nil
}

func (f t564RDS) DescribeDBClusterSnapshots(_ context.Context, in *rds.DescribeDBClusterSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	out := &rds.DescribeDBClusterSnapshotsOutput{}
	for _, s := range f.ep.snapshots {
		if t564EngineWanted(t564RDSFilters(in.Filters), s.engine) {
			out.DBClusterSnapshots = append(out.DBClusterSnapshots, rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String(s.id),
				DBClusterIdentifier:         aws.String(s.clusterID),
				Engine:                      aws.String(s.engine),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				StorageEncrypted:            aws.Bool(true),
			})
		}
	}
	return out, nil
}

func (f t564RDS) DescribeDBSubnetGroups(_ context.Context, in *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error) {
	f.ep.rdsSubnetGroupHit++
	return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: []rdstypes.DBSubnetGroup{{
		DBSubnetGroupName: in.DBSubnetGroupName,
		VpcId:             aws.String("vpc-0a1b2c3d4e5f60718"),
		Subnets:           []rdstypes.Subnet{{SubnetIdentifier: aws.String("subnet-0rdsside00000001")}},
	}}}, nil
}

func (f t564RDS) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.instances}, nil
}

// t564ClusterRow lists one otherwise-healthy cluster in the given status
// through the DocumentDB-shaped or the RDS-shaped lane.
func t564ClusterRow(t *testing.T, lane, status string) resource.Resource {
	t.Helper()
	return t564ListCluster(t, lane, t564Cluster{id: "acme-orders-" + lane, status: status})
}

func t564ListCluster(t *testing.T, lane string, c t564Cluster) resource.Resource {
	t.Helper()
	status := c.status
	c.subnetGroup, c.autoMinor, c.iamAuth = "acme-db-subnets", true, true
	var res resource.FetchResult
	var err error
	if lane == "docdb" {
		c.engine = "docdb"
		res, err = awsclient.FetchDocDBClustersPage(context.Background(), t564DocDB{ep: &t564Endpoint{clusters: []t564Cluster{c}}}, "")
	} else {
		c.engine = "aurora-postgresql"
		res, err = awsclient.FetchRDSDBClustersPage(context.Background(), t564RDS{ep: &t564Endpoint{clusters: []t564Cluster{c}}}, "")
	}
	if err != nil || len(res.Resources) != 1 {
		t.Fatalf("%s lane, status %q: %d rows, err %v; want 1 row", lane, status, len(res.Resources), err)
	}
	return res.Resources[0]
}

// t564InstanceRow lists one otherwise-healthy DB instance in the given status.
func t564InstanceRow(t *testing.T, status string) resource.Resource {
	t.Helper()
	db := rdstypes.DBInstance{
		DBInstanceIdentifier:             aws.String("acme-billing-db"),
		DBInstanceArn:                    aws.String("arn:aws:rds:us-east-1:123456789012:db:acme-billing-db"),
		DBInstanceClass:                  aws.String("db.m6g.large"),
		Engine:                           aws.String("postgres"),
		EngineVersion:                    aws.String("16.4"),
		DBInstanceStatus:                 aws.String(status),
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		MasterUsername:                   aws.String("acme_admin"),
		BackupRetentionPeriod:            aws.Int32(7),
		PubliclyAccessible:               aws.Bool(false),
		StorageEncrypted:                 aws.Bool(true),
		DeletionProtection:               aws.Bool(true),
		CACertificateIdentifier:          aws.String("rds-ca-rsa2048-g1"),
		CertificateDetails: &rdstypes.CertificateDetails{
			CAIdentifier: aws.String("rds-ca-rsa2048-g1"),
			ValidTill:    aws.Time(time.Now().AddDate(2, 0, 0)),
		},
		Endpoint: &rdstypes.Endpoint{Address: aws.String("acme-billing-db.c1a2b3c4d5e6.us-east-1.rds.amazonaws.com"), Port: aws.Int32(5432)},
	}
	res, err := awsclient.FetchRDSInstancesPage(context.Background(), t564RDS{instances: []rdstypes.DBInstance{db}}, "")
	if err != nil || len(res.Resources) != 1 {
		t.Fatalf("status %q: %d rows, err %v; want 1 row", status, len(res.Resources), err)
	}
	return res.Resources[0]
}

func t564Top(t *testing.T, what string, r resource.Resource) domain.Finding {
	t.Helper()
	top, ok := domain.TopFinding(r.Findings)
	if !ok {
		t.Errorf("%s: row carries no finding; Status cell %q", what, r.Fields["status"])
		return top
	}
	if r.Fields["status"] != top.Phrase {
		t.Errorf("%s: Status cell %q, want the top finding's phrase %q", what, r.Fields["status"], top.Phrase)
	}
	return top
}

// t564AssertFailedState pins what a failed or terminal lifecycle state
// renders: a phrase of its own, at Broken, coloured and counted as broken,
// and never the wording of a state that resolves by waiting.
func t564AssertFailedState(t *testing.T, shortName, what string, r resource.Resource) domain.Finding {
	t.Helper()
	top := t564Top(t, what, r)
	if top.Severity != domain.SevBroken {
		t.Errorf("%s: severity %v (%s), want Broken", what, top.Severity, top.Code)
	}
	if strings.Contains(top.Phrase, "in progress") {
		t.Errorf("%s: phrase %q claims the state is in progress", what, top.Phrase)
	}
	if strings.Contains(strings.ToLower(top.Detail), "wait for it to return to available") {
		t.Errorf("%s: detail %q tells the operator to wait", what, top.Detail)
	}
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	if got, want := td.ResolveColor(r), resource.ColorFromSeverity(domain.SevBroken); got != want {
		t.Errorf("%s: row colour %v, want the broken colour %v", what, got, want)
	}
	return top
}

// The Aurora/RDS DB cluster status table documents these as states a cluster
// does not leave by waiting: encryption credentials lost, a migration or a
// clone that failed, and a stopped cluster. Both SDK shapes of a DB cluster
// read them through the one lifecycle rule.
func TestT564_DBClusterFailedStatesAreBroken(t *testing.T) {
	statuses := []string{"inaccessible-encryption-credentials-recoverable", "migration-failed", "cloning-failed", "stopped"}
	for _, lane := range []string{"docdb", "rds"} {
		phraseOf := map[string]string{}
		for _, status := range statuses {
			top := t564AssertFailedState(t, "dbc", lane+"/"+status, t564ClusterRow(t, lane, status))
			phraseOf[status] = top.Phrase
		}
		for i, a := range statuses {
			for _, b := range statuses[i+1:] {
				if phraseOf[a] == phraseOf[b] {
					t.Errorf("%s: %s and %s both read %q; each state has its own phrase", lane, a, b, phraseOf[a])
				}
			}
		}
	}
}

// The RDS DB instance status table documents these as failed states, and
// delete-precheck, storage-config-upgrade and storage-initialization as
// states an instance passes through on its own.
func TestT564_DBInstanceFailedAndTransitionalStates(t *testing.T) {
	failed := []string{"inaccessible-encryption-credentials-recoverable", "insufficient-capacity", "incompatible-create"}
	phraseOf := map[string]string{}
	for _, status := range failed {
		phraseOf[status] = t564AssertFailedState(t, "dbi", status, t564InstanceRow(t, status)).Phrase
	}
	for i, a := range failed {
		for _, b := range failed[i+1:] {
			if phraseOf[a] == phraseOf[b] {
				t.Errorf("%s and %s both read %q; each state has its own phrase", a, b, phraseOf[a])
			}
		}
	}

	for _, status := range []string{"delete-precheck", "storage-config-upgrade", "storage-initialization"} {
		top := t564Top(t, status, t564InstanceRow(t, status))
		if top.Code != awsclient.CodeDBITransitional || top.Severity != domain.SevWarn || top.Phrase != status {
			t.Errorf("%s: top finding %s %v %q, want %s at Warn reading %q", status, top.Code, top.Severity, top.Phrase, awsclient.CodeDBITransitional, status)
		}
	}
}

// A status in neither AWS table is shown as itself, at Warn, with a detail
// saying a9s does not recognise it — the same on a cluster of either shape
// and on an instance. A recognised healthy status still reads clean.
func TestT564_UnrecognisedStatusRendersTheSameInBothLanes(t *testing.T) {
	rows := map[string]resource.Resource{
		"dbc docdb": t564ClusterRow(t, "docdb", t564InventedStatus),
		"dbc rds":   t564ClusterRow(t, "rds", t564InventedStatus),
		"dbi":       t564InstanceRow(t, t564InventedStatus),
	}
	var details []string
	for what, r := range rows {
		top := t564Top(t, what, r)
		if top.Phrase != t564InventedStatus || top.Severity != domain.SevWarn {
			t.Errorf("%s: top finding %q at %v, want %q at Warn", what, top.Phrase, top.Severity, t564InventedStatus)
		}
		if !t564NotRecognised.MatchString(top.Detail) {
			t.Errorf("%s: detail %q does not say a9s does not recognise the status", what, top.Detail)
		}
		if strings.Contains(strings.ToLower(top.Detail), "wait for it to return to available") || strings.Contains(top.Phrase, "in progress") {
			t.Errorf("%s: %q / %q claims a transition in progress", what, top.Phrase, top.Detail)
		}
		details = append(details, top.Detail)
	}
	if len(slices.Compact(slices.Sorted(slices.Values(details)))) != 1 {
		t.Errorf("the lanes give different details for one unrecognised status: %q", details)
	}

	for what, r := range map[string]resource.Resource{
		"dbc docdb": t564ClusterRow(t, "docdb", "available"),
		"dbc rds":   t564ClusterRow(t, "rds", "available"),
		"dbi":       t564InstanceRow(t, "available"),
	} {
		if len(r.Findings) != 0 {
			t.Errorf("%s healthy available row carries %v, want none", what, r.Findings)
		}
	}
}

// AWS restarts a stopped Aurora cluster after seven days, as it does a
// stopped instance, and bills storage while it is stopped.
func TestT564_StoppedClusterReadsLikeAStoppedInstance(t *testing.T) {
	const want = "stopped (storage still billed)"
	for what, r := range map[string]resource.Resource{
		"dbc docdb": t564ClusterRow(t, "docdb", "stopped"),
		"dbc rds":   t564ClusterRow(t, "rds", "stopped"),
		"dbi":       t564InstanceRow(t, "stopped"),
	} {
		top := t564Top(t, what, r)
		if top.Phrase != want || top.Severity != domain.SevBroken {
			t.Errorf("%s: top finding %q at %v, want %q at Broken", what, top.Phrase, top.Severity, want)
		}
	}
}

func t564RegionEndpoint() *t564Endpoint {
	return &t564Endpoint{
		clusters: []t564Cluster{
			{id: "acme-catalog-docdb", engine: "docdb", status: "available", subnetGroup: "acme-docdb-subnets"},
			{id: "acme-orders-aurora", engine: "aurora-postgresql", status: "available", subnetGroup: "acme-aurora-subnets"},
			{id: "acme-ledger-maz", engine: "mysql", status: "available", subnetGroup: "acme-maz-subnets"},
			{id: "acme-graph-neptune", engine: "neptune", status: "available", subnetGroup: "acme-neptune-subnets", autoMinor: true, iamAuth: true},
		},
		snapshots: []t564Snapshot{
			{id: "rds:acme-catalog-docdb-2026-09-20-03-00", clusterID: "acme-catalog-docdb", engine: "docdb"},
			{id: "rds:acme-orders-aurora-2026-09-20-03-00", clusterID: "acme-orders-aurora", engine: "aurora-postgresql"},
		},
	}
}

func t564HasCode(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

// DB Clusters lists the DocumentDB cluster from the DocumentDB call scoped
// with engine=docdb, as the DocumentDB SDK's DescribeDBClusters doc comment
// directs, and the Aurora and Multi-AZ clusters from the RDS call in their
// RDS shape — so the RDS posture checks run on them and their subnet group
// is read from RDS. Neptune is not a DB Clusters row.
func TestT564_DBClusterListIsScopedByEngine(t *testing.T) {
	ep := t564RegionEndpoint()
	clients := &awsclient.ServiceClients{DocDB: t564DocDB{ep: ep}, RDS: t564RDS{ep: ep}}
	fetcher := resource.GetPaginatedFetcher("dbc")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for dbc")
	}
	res, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("dbc fetch: %v", err)
	}

	byID := map[string]resource.Resource{}
	var ids []string
	for _, r := range res.Resources {
		byID[r.ID] = r
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	if want := []string{"acme-catalog-docdb", "acme-ledger-maz", "acme-orders-aurora"}; !slices.Equal(ids, want) {
		t.Errorf("DB Clusters rows %v, want %v", ids, want)
	}

	if len(ep.docdbClusterInputs) == 0 {
		t.Fatal("no DocumentDB DescribeDBClusters call")
	}
	for i, in := range ep.docdbClusterInputs {
		if got := t564DocDBFilters(in.Filters); !slices.EqualFunc(got, [][2][]string{{{"engine"}, {"docdb"}}}, func(a, b [2][]string) bool {
			return slices.Equal(a[0], b[0]) && slices.Equal(a[1], b[1])
		}) {
			t.Errorf("DocumentDB DescribeDBClusters call %d filters %v, want engine=[docdb]", i, got)
		}
	}

	if _, ok := byID["acme-catalog-docdb"].RawStruct.(docdbtypes.DBCluster); !ok {
		t.Errorf("acme-catalog-docdb RawStruct %T, want the DocumentDB shape", byID["acme-catalog-docdb"].RawStruct)
	}
	subnetDef := func() resource.RelatedDef {
		for _, d := range resource.GetRelated("dbc") {
			if d.TargetType == "subnet" {
				return d
			}
		}
		t.Fatal("dbc has no subnet pivot")
		return resource.RelatedDef{}
	}()
	for _, id := range []string{"acme-orders-aurora", "acme-ledger-maz"} {
		r, ok := byID[id]
		if !ok {
			continue
		}
		if _, ok := r.RawStruct.(rdstypes.DBCluster); !ok {
			t.Errorf("%s RawStruct %T, want the RDS shape", id, r.RawStruct)
		}
		for _, code := range []domain.FindingCode{awsclient.CodeDBCMinorUpgradeOff, awsclient.CodeDBCIAMAuthOff} {
			if !t564HasCode(r.Findings, code) {
				t.Errorf("%s findings %v, want %s", id, r.Findings, code)
			}
		}
		docdbBefore, rdsBefore := ep.docdbSubnetGroupHit, ep.rdsSubnetGroupHit
		got := subnetDef.Checker(context.Background(), clients, r, resource.ResourceCache{}).ResourceIDs()
		if !slices.Equal(got, []string{"subnet-0rdsside00000001"}) || ep.docdbSubnetGroupHit != docdbBefore || ep.rdsSubnetGroupHit == rdsBefore {
			t.Errorf("%s subnet pivot %v (DocumentDB calls +%d, RDS calls +%d), want the RDS subnet group's subnet", id, got, ep.docdbSubnetGroupHit-docdbBefore, ep.rdsSubnetGroupHit-rdsBefore)
		}
	}
}

// DB Cluster Snapshots reads DocumentDB snapshots from the DocumentDB call
// and Aurora snapshots from the RDS call, in the RDS shape.
func TestT564_DBClusterSnapshotListIsScopedByEngine(t *testing.T) {
	ep := t564RegionEndpoint()
	fetcher := resource.GetPaginatedFetcher("dbc-snap")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for dbc-snap")
	}
	res, err := fetcher(context.Background(), &awsclient.ServiceClients{DocDB: t564DocDB{ep: ep}, RDS: t564RDS{ep: ep}}, "")
	if err != nil {
		t.Fatalf("dbc-snap fetch: %v", err)
	}
	shapes := map[string]string{}
	for _, r := range res.Resources {
		switch r.RawStruct.(type) {
		case docdbtypes.DBClusterSnapshot:
			shapes[r.ID] = "docdb"
		case rdstypes.DBClusterSnapshot:
			shapes[r.ID] = "rds"
		default:
			shapes[r.ID] = "?"
		}
	}
	want := map[string]string{
		"rds:acme-catalog-docdb-2026-09-20-03-00": "docdb",
		"rds:acme-orders-aurora-2026-09-20-03-00": "rds",
	}
	for id, shape := range want {
		if shapes[id] != shape {
			t.Errorf("%s listed in shape %q, want %q (all: %v)", id, shapes[id], shape, shapes)
		}
	}
}

func t564Opt(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

// A replica cluster — an Aurora cross-Region read replica, a DocumentDB
// replica — names its source in ReplicationSourceIdentifier and has no writer
// because it is read-only by design (Aurora User Guide: "The secondary
// cluster is read-only"), so it is not "no writer" and is not broken. A
// stand-alone available cluster with no writer still is.
func TestT564_SecondaryClusterWithoutWriterIsNotBroken(t *testing.T) {
	const primaryARN = "arn:aws:rds:eu-west-1:123456789012:cluster:acme-orders-primary"
	secondaries := map[string]resource.Resource{
		"rds cross-Region replica": t564ListCluster(t, "rds", t564Cluster{id: "acme-orders-replica", status: "available", noWriter: true, replicaOf: primaryARN}),
		"docdb replica": t564ListCluster(t, "docdb", t564Cluster{id: "acme-catalog-replica", status: "available", noWriter: true,
			replicaOf: "arn:aws:rds:eu-west-1:123456789012:cluster:acme-catalog-primary"}),
	}
	for what, r := range secondaries {
		if len(r.Findings) != 0 {
			t.Errorf("%s: findings %v, Status cell %q; want none — a secondary has no writer by design", what, r.Findings, r.Fields["status"])
		}
	}

	for _, lane := range []string{"docdb", "rds"} {
		r := t564ListCluster(t, lane, t564Cluster{id: "acme-orders-standalone", status: "available", noWriter: true})
		if !t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) {
			t.Errorf("%s stand-alone cluster without a writer: findings %v, want %s", lane, r.Findings, awsclient.CodeDBCNoWriter)
		}
		if top, _ := domain.TopFinding(r.Findings); top.Severity != domain.SevBroken {
			t.Errorf("%s stand-alone cluster without a writer: top %s at %v, want Broken", lane, top.Code, top.Severity)
		}
	}
}

// RDS DescribeDBInstances also returns Neptune instances, whose
// DBClusterIdentifier names a Neptune cluster. DB Clusters does not list
// Neptune, so a Neptune instance's DB Clusters pivot is a proven zero; an
// Aurora instance's still names its cluster.
func TestT564_NeptuneInstanceDBClustersPivotIsProvenZero(t *testing.T) {
	instance := func(id, engine, cluster string) resource.Resource {
		res, err := awsclient.FetchRDSInstancesPage(context.Background(), t564RDS{instances: []rdstypes.DBInstance{{
			DBInstanceIdentifier: aws.String(id),
			DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:" + id),
			DBInstanceClass:      aws.String("db.r6g.large"),
			Engine:               aws.String(engine),
			DBInstanceStatus:     aws.String("available"),
			DBClusterIdentifier:  aws.String(cluster),
			StorageEncrypted:     aws.Bool(true),
		}}}, "")
		if err != nil || len(res.Resources) != 1 {
			t.Fatalf("%s: %d rows, err %v; want 1 row", id, len(res.Resources), err)
		}
		return res.Resources[0]
	}

	ep := t564RegionEndpoint()
	clients := &awsclient.ServiceClients{DocDB: t564DocDB{ep: ep}, RDS: t564RDS{ep: ep}}
	clusters, err := resource.GetPaginatedFetcher("dbc")(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("dbc fetch: %v", err)
	}
	cache := resource.ResourceCache{"dbc": {Resources: clusters.Resources}}

	var def resource.RelatedDef
	for _, d := range resource.GetRelated("dbi") {
		if d.TargetType == "dbc" {
			def = d
		}
	}
	if def.Checker == nil {
		t.Fatal("dbi has no dbc pivot")
	}

	got := def.Checker(context.Background(), clients, instance("acme-graph-neptune-1", "neptune", "acme-graph-neptune"), cache)
	pz := resource.ProvenZero("dbc", "")
	if got.State() != pz.State() || got.Coverage() != pz.Coverage() || got.Count() != 0 || len(got.ResourceIDs()) != 0 || got.Err() != nil || got.Truncated() {
		t.Errorf("Neptune instance DB Clusters pivot: state %v coverage %v count %d ids %v err %v truncated %v; want a proven zero",
			got.State(), got.Coverage(), got.Count(), got.ResourceIDs(), got.Err(), got.Truncated())
	}

	aurora := def.Checker(context.Background(), clients, instance("acme-orders-aurora-1", "aurora-postgresql", "acme-orders-aurora"), cache)
	if ids := aurora.ResourceIDs(); !slices.Equal(ids, []string{"acme-orders-aurora"}) {
		t.Errorf("Aurora instance DB Clusters pivot %v (state %v), want [acme-orders-aurora]", ids, aurora.State())
	}
}
