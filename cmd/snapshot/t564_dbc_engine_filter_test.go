package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// t564SharedEndpoint answers DescribeDBClusters the way the regional endpoint
// DocumentDB, Neptune and RDS share does — every engine's clusters unless an
// `engine` filter names the engines wanted — for the DocumentDB and the RDS
// client alike, and records the engine filter each DocumentDB request sent.
type t564SharedEndpoint struct {
	docdbEngineFilters [][]string
}

var t564Clusters = [][2]string{
	{"acme-catalog-docdb", "docdb"},
	{"acme-orders-aurora", "aurora-postgresql"},
	{"acme-ledger-maz", "mysql"},
	{"acme-graph-neptune", "neptune"},
}

func (e *t564SharedEndpoint) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)         //nolint:errcheck // an unreadable body decodes as an empty request
	form, _ := url.ParseQuery(string(body)) //nolint:errcheck // a malformed body carries no filters
	var engines []string
	filtered := false
	for i := 1; form.Get(fmt.Sprintf("Filters.Filter.%d.Name", i)) != ""; i++ {
		if form.Get(fmt.Sprintf("Filters.Filter.%d.Name", i)) != "engine" {
			continue
		}
		filtered = true
		for j := 1; form.Get(fmt.Sprintf("Filters.Filter.%d.Values.Value.%d", i, j)) != ""; j++ {
			engines = append(engines, form.Get(fmt.Sprintf("Filters.Filter.%d.Values.Value.%d", i, j)))
		}
	}
	if strings.Contains(req.Header.Get("User-Agent"), "api/docdb") {
		e.docdbEngineFilters = append(e.docdbEngineFilters, engines)
	}

	var members strings.Builder
	for _, c := range t564Clusters {
		if filtered && !slices.Contains(engines, c[1]) {
			continue
		}
		fmt.Fprintf(&members, "<DBCluster><DBClusterIdentifier>%s</DBClusterIdentifier><DBClusterArn>arn:aws:rds:eu-west-1:123456789012:cluster:%s</DBClusterArn><Engine>%s</Engine><Status>available</Status></DBCluster>", c[0], c[0], c[1])
	}
	xml := "<DescribeDBClustersResponse><DescribeDBClustersResult><DBClusters>" + members.String() + "</DBClusters></DescribeDBClustersResult></DescribeDBClustersResponse>"
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(xml)),
	}, nil
}

// The oracle is only a check on the app if it lists the clusters DB Clusters
// lists: the DocumentDB call scoped with engine=docdb, the Aurora and
// Multi-AZ clusters taken from the RDS call, and no Neptune cluster.
func TestCaptureDBC_ScopesTheDocDBCallAndDropsNeptune(t *testing.T) {
	ep := &t564SharedEndpoint{}
	cfg := aws.Config{
		Region:           "eu-west-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: ep},
		RetryMaxAttempts: 1,
	}
	got, err := captureDBC(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureDBC: %v", err)
	}
	data, ok := got.(dbcData)
	if !ok {
		t.Fatalf("captureDBC returned %T, want dbcData", got)
	}

	if len(ep.docdbEngineFilters) == 0 {
		t.Fatal("captureDBC sent no DocumentDB DescribeDBClusters request")
	}
	for i, engines := range ep.docdbEngineFilters {
		if !slices.Equal(engines, []string{"docdb"}) {
			t.Errorf("DocumentDB request %d engine filter %v, want [docdb]", i, engines)
		}
	}

	source := map[string]string{}
	for _, c := range data.Clusters {
		source[c.DBClusterIdentifier] = c.Source
	}
	want := map[string]string{"acme-catalog-docdb": "docdb", "acme-orders-aurora": "rds", "acme-ledger-maz": "rds"}
	if len(source) != len(want) {
		t.Errorf("captured clusters %v, want %v", source, want)
	}
	for id, src := range want {
		if source[id] != src {
			t.Errorf("%s captured from %q, want %q (all: %v)", id, source[id], src, source)
		}
	}
}
