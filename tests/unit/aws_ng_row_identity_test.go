// EKS scopes a node-group name to its cluster, so two clusters may each own a
// node group called "workers". The ng list is one flat list keyed by
// Resource.ID, so a bare name makes the second one vanish (or resolve to the
// first cluster's row). The bare name stays available as a field, which is
// what the column, the console link and the CloudTrail pivot read.
package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

func ngRowByID(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	t.Fatalf("no ng row with ID %q; got %v", id, ids)
	return resource.Resource{}
}

func activeNodegroup(cluster, name string) *ekstypes.Nodegroup {
	return &ekstypes.Nodegroup{
		NodegroupName: aws.String(name),
		ClusterName:   aws.String(cluster),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"m5.large"},
	}
}

func TestNGRows_SameNameInTwoClustersBothRender(t *testing.T) {
	pf := resource.GetPaginatedFetcher("ng")
	if pf == nil {
		t.Fatal("no registered paginated fetcher for \"ng\"")
	}

	eksFake := &ngTestEKSFake{
		clusters:   []string{"blue", "green"},
		nodegroups: map[string][]string{"blue": {"workers"}, "green": {"workers"}},
		nodegroupDetails: map[string]*ekstypes.Nodegroup{
			"blue/workers":  activeNodegroup("blue", "workers"),
			"green/workers": activeNodegroup("green", "workers"),
		},
	}
	clients := &awsclient.ServiceClients{EKS: eksFake, EC2: fakes.NewEC2()}

	out, err := pf(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("ng fetcher: %v", err)
	}
	if len(out.Resources) != 2 {
		t.Fatalf("fetcher returned %d rows, want 2", len(out.Resources))
	}

	deduped, _ := resource.DedupByID(out.Resources)
	if len(deduped) != 2 {
		ids := make([]string, 0, len(out.Resources))
		for _, r := range out.Resources {
			ids = append(ids, r.ID)
		}
		t.Fatalf("after ID-keyed dedup %d of 2 node groups remain (IDs %v) — one cluster's row was swallowed",
			len(deduped), ids)
	}

	for _, want := range []string{"blue/workers", "green/workers"} {
		row := ngRowByID(t, deduped, want)
		if got := row.Fields["nodegroup_name"]; got != "workers" {
			t.Errorf("row %q: Fields[\"nodegroup_name\"] = %q, want %q", want, got, "workers")
		}
		cluster := strings.SplitN(want, "/", 2)[0]
		if got := row.Fields["cluster_name"]; got != cluster {
			t.Errorf("row %q: Fields[\"cluster_name\"] = %q, want %q", want, got, cluster)
		}
	}
}

// The EKS console addresses a node group by its bare name under its
// cluster; a composite id in the path is a 404.
func TestNGRow_ConsoleURLUsesTheBareName(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil || td.ConsoleURL == nil {
		t.Fatal("the ng type has no ConsoleURL")
	}

	row := resource.Resource{
		ID:   "green/workers",
		Name: "workers",
		Fields: map[string]string{
			"nodegroup_name": "workers",
			"cluster_name":   "green",
		},
	}

	got := td.ConsoleURL(row, "eu-west-2", "123456789012")
	const want = "https://eu-west-2.console.aws.amazon.com/eks/home?region=eu-west-2#/clusters/green/nodegroups/workers"
	if got != want {
		t.Errorf("ConsoleURL =\n  %q\nwant\n  %q", got, want)
	}
}

// CloudTrail never writes "<cluster>/<nodegroup>" as a ResourceName, so the
// pivot must scope by the bare name.
func TestNGRow_CloudTrailFilterUsesTheBareName(t *testing.T) {
	row := resource.Resource{
		ID:   "green/workers",
		Name: "workers",
		Fields: map[string]string{
			"nodegroup_name": "workers",
			"cluster_name":   "green",
		},
	}

	got := resource.BuildCloudTrailFilter(row, "ng")
	// A key starting with "_" is a9s's own and is stripped before the call;
	// what reaches LookupEvents is the lookup attributes, and it takes one.
	lookup := map[string]string{}
	for k, v := range got {
		if !strings.HasPrefix(k, "_") {
			lookup[k] = v
		}
	}
	if len(lookup) != 1 || lookup["ResourceName"] != "workers" {
		t.Errorf("BuildCloudTrailFilter lookup attributes = %v, want map[ResourceName:workers]", lookup)
	}
}

// A degraded row keeps the cluster-scoped ID: a bare-name row would collide
// with a healthy row of that name in another cluster.
func TestNGRow_DegradedRowKeepsTheClusterScopedID(t *testing.T) {
	pf := resource.GetPaginatedFetcher("ng")
	if pf == nil {
		t.Fatal("no registered paginated fetcher for \"ng\"")
	}

	eksFake := &ngTestEKSFake{
		clusters:   []string{"blue", "green"},
		nodegroups: map[string][]string{"blue": {"workers"}, "green": {"workers"}},
		nodegroupDetails: map[string]*ekstypes.Nodegroup{
			"blue/workers": activeNodegroup("blue", "workers"),
		},
	}
	clients := &awsclient.ServiceClients{EKS: eksFake, EC2: fakes.NewEC2()}

	out, err := pf(context.Background(), clients, "")
	if err == nil {
		t.Error("the unanswered DescribeNodegroup is not carried out of the fetcher")
	}
	if len(out.Resources) != 2 {
		t.Fatalf("fetcher returned %d rows, want 2 (one full, one degraded)", len(out.Resources))
	}

	degraded := ngRowByID(t, out.Resources, "green/workers")
	if got := degraded.Fields["nodegroup_name"]; got != "workers" {
		t.Errorf("degraded row Fields[\"nodegroup_name\"] = %q, want %q", got, "workers")
	}
	if got := degraded.Fields["cluster_name"]; got != "green" {
		t.Errorf("degraded row Fields[\"cluster_name\"] = %q, want %q", got, "green")
	}
	if deduped, _ := resource.DedupByID(out.Resources); len(deduped) != 2 {
		t.Error("the degraded row and the healthy row of the same name collapsed into one")
	}
}
