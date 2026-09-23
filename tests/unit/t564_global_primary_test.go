package unit

import (
	"context"
	"errors"
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

// t564Global is a global database: its members' cluster ARNs, each with
// IsWriter as DescribeGlobalClusters reports it.
type t564Global struct {
	id, engine string
	writer     map[string]bool
}

// t564GlobalEndpoint answers DescribeGlobalClusters from the shared regional
// endpoint one global database per page, so a reader that stops after the
// first page misses the rest; globalErr fails every call.
type t564GlobalEndpoint struct {
	globals              []t564Global
	globalErr            error
	rdsCalls, docdbCalls int
}

func (g *t564GlobalEndpoint) page(marker *string) (t564Global, *string, bool) {
	i := 0
	if marker != nil {
		for i < len(g.globals) && g.globals[i].id != *marker {
			i++
		}
	}
	if i >= len(g.globals) {
		return t564Global{}, nil, false
	}
	var next *string
	if i+1 < len(g.globals) {
		next = aws.String(g.globals[i+1].id)
	}
	return g.globals[i], next, true
}

type t564GlobalDocDB struct {
	t564DocDB
	g *t564GlobalEndpoint
}

func (f t564GlobalDocDB) DescribeGlobalClusters(_ context.Context, in *docdb.DescribeGlobalClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeGlobalClustersOutput, error) {
	f.g.docdbCalls++
	if f.g.globalErr != nil {
		return nil, f.g.globalErr
	}
	out := &docdb.DescribeGlobalClustersOutput{}
	gc, next, ok := f.g.page(in.Marker)
	if ok {
		var members []docdbtypes.GlobalClusterMember
		for arn, w := range gc.writer {
			members = append(members, docdbtypes.GlobalClusterMember{DBClusterArn: aws.String(arn), IsWriter: aws.Bool(w)})
		}
		out.GlobalClusters = []docdbtypes.GlobalCluster{{GlobalClusterIdentifier: aws.String(gc.id), Engine: aws.String(gc.engine), Status: aws.String("available"), GlobalClusterMembers: members}}
	}
	out.Marker = next
	return out, nil
}

type t564GlobalRDS struct {
	t564RDS
	g *t564GlobalEndpoint
}

func (f t564GlobalRDS) DescribeGlobalClusters(_ context.Context, in *rds.DescribeGlobalClustersInput, _ ...func(*rds.Options)) (*rds.DescribeGlobalClustersOutput, error) {
	f.g.rdsCalls++
	if f.g.globalErr != nil {
		return nil, f.g.globalErr
	}
	out := &rds.DescribeGlobalClustersOutput{}
	gc, next, ok := f.g.page(in.Marker)
	if ok {
		var members []rdstypes.GlobalClusterMember
		for arn, w := range gc.writer {
			members = append(members, rdstypes.GlobalClusterMember{DBClusterArn: aws.String(arn), IsWriter: aws.Bool(w)})
		}
		out.GlobalClusters = []rdstypes.GlobalCluster{{GlobalClusterIdentifier: aws.String(gc.id), Engine: aws.String(gc.engine), Status: aws.String("available"), GlobalClusterMembers: members}}
	}
	out.Marker = next
	return out, nil
}

func t564ListDBC(t *testing.T, clusters []t564Cluster, g *t564GlobalEndpoint) (map[string]resource.Resource, error) {
	t.Helper()
	for i := range clusters {
		clusters[i].status, clusters[i].noWriter, clusters[i].autoMinor, clusters[i].iamAuth = "available", true, true, true
		clusters[i].subnetGroup = "acme-db-subnets"
	}
	ep := &t564Endpoint{clusters: clusters}
	clients := &awsclient.ServiceClients{
		DocDB: t564GlobalDocDB{t564DocDB{ep: ep}, g},
		RDS:   t564GlobalRDS{t564RDS{ep: ep}, g},
	}
	res, err := resource.GetPaginatedFetcher("dbc")(context.Background(), clients, "")
	rows := map[string]resource.Resource{}
	for _, r := range res.Resources {
		rows[r.ID] = r
	}
	return rows, err
}

func t564OtherRegionARN(id string) string {
	return "arn:aws:rds:eu-west-1:123456789012:cluster:" + id
}

// A global database's primary carries GlobalClusterIdentifier exactly as its
// secondaries do; only DescribeGlobalClusters says which member is the writer
// (IsWriter). A primary that has lost its writer is broken, a secondary is
// read-only by design — in the RDS lane and the DocumentDB lane alike. The
// member list is walked to its last page, once per list fetch.
func TestT564_GlobalPrimaryWithoutWriterIsBroken(t *testing.T) {
	here := func(id string) string { return aws.ToString(t564ClusterARN(id)) }
	g := &t564GlobalEndpoint{globals: []t564Global{
		{id: "acme-orders-global", engine: "aurora-postgresql", writer: map[string]bool{here("acme-orders-primary"): true, t564OtherRegionARN("acme-orders-dr"): false}},
		{id: "acme-catalog-global", engine: "docdb", writer: map[string]bool{here("acme-catalog-primary"): true, t564OtherRegionARN("acme-catalog-dr"): false}},
		{id: "acme-ledger-global", engine: "aurora-postgresql", writer: map[string]bool{t564OtherRegionARN("acme-ledger-primary"): true, here("acme-ledger-secondary"): false}},
		{id: "acme-billing-global", engine: "aurora-mysql", writer: map[string]bool{t564OtherRegionARN("acme-billing-primary"): true, here("acme-billing-secondary"): false}},
		{id: "acme-reviews-global", engine: "docdb", writer: map[string]bool{t564OtherRegionARN("acme-reviews-primary"): true, here("acme-reviews-secondary"): false}},
	}}
	rows, err := t564ListDBC(t, []t564Cluster{
		{id: "acme-orders-primary", engine: "aurora-postgresql", global: "acme-orders-global"},
		{id: "acme-ledger-secondary", engine: "aurora-postgresql", global: "acme-ledger-global"},
		{id: "acme-billing-secondary", engine: "aurora-mysql", global: "acme-billing-global"},
		{id: "acme-catalog-primary", engine: "docdb"},
		{id: "acme-reviews-secondary", engine: "docdb"},
	}, g)
	if err != nil {
		t.Fatalf("dbc fetch: %v", err)
	}

	for _, id := range []string{"acme-orders-primary", "acme-catalog-primary"} {
		r, ok := rows[id]
		top, _ := domain.TopFinding(r.Findings)
		if !ok || !t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) || top.Severity != domain.SevBroken {
			t.Errorf("global primary %s without a writer: listed %v, findings %v; want %s at Broken", id, ok, r.Findings, awsclient.CodeDBCNoWriter)
		}
	}
	for _, id := range []string{"acme-ledger-secondary", "acme-billing-secondary", "acme-reviews-secondary"} {
		r, ok := rows[id]
		top, _ := domain.TopFinding(r.Findings)
		if !ok || t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) || top.Severity == domain.SevBroken {
			t.Errorf("global secondary %s: listed %v, findings %v; want no writer verdict and nothing Broken", id, ok, r.Findings)
		}
	}

	pages := len(g.globals)
	if g.rdsCalls > pages || g.docdbCalls > pages {
		t.Errorf("DescribeGlobalClusters calls: RDS %d, DocumentDB %d; want at most one walk of %d pages per lane", g.rdsCalls, g.docdbCalls, pages)
	}
}

// When the global member list cannot be read, a cluster that belongs to a
// global database has no known role: it gets no no-writer verdict either way
// and the list comes back partial. A stand-alone cluster without a writer and
// a replica are judged from their own record and keep their verdicts.
func TestT564_GlobalMemberListFailureLeavesNoVerdict(t *testing.T) {
	g := &t564GlobalEndpoint{globalErr: errors.New("AccessDenied: User is not authorized to perform rds:DescribeGlobalClusters")}
	rows, err := t564ListDBC(t, []t564Cluster{
		{id: "acme-orders-primary", engine: "aurora-postgresql", global: "acme-orders-global"},
		{id: "acme-standalone", engine: "aurora-postgresql"},
		{id: "acme-orders-replica", engine: "aurora-postgresql", replicaOf: t564OtherRegionARN("acme-orders-source")},
	}, g)

	if err == nil {
		t.Error("dbc fetch returned no error; want the list marked partial when DescribeGlobalClusters fails")
	}
	if len(rows) != 3 {
		t.Errorf("dbc fetch listed %d rows, want all 3 kept", len(rows))
	}
	if r := rows["acme-orders-primary"]; t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) {
		t.Errorf("global member with unread role: findings %v, want no writer verdict", r.Findings)
	}
	if r := rows["acme-standalone"]; !t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) {
		t.Errorf("stand-alone cluster without a writer: findings %v, want %s", r.Findings, awsclient.CodeDBCNoWriter)
	}
	if r := rows["acme-orders-replica"]; t564HasCode(r.Findings, awsclient.CodeDBCNoWriter) {
		t.Errorf("replica: findings %v, want no writer verdict", r.Findings)
	}
}
